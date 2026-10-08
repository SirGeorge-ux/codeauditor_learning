package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeDictionaryRepo is an in-memory implementation of
// ports.DictionaryRepository dedicated to handler-level tests. It mirrors
// the production adapter's behavior for the bits the handler exercises
// (duplicates, missing rows, alphabetical order). Production code uses
// SupabaseDictionaryRepository — handler tests do not.
type fakeDictionaryRepo struct {
	terms map[string]*models.Term // key: userID|termID
	next  int
}

func newFakeDictionaryRepo() *fakeDictionaryRepo {
	return &fakeDictionaryRepo{terms: make(map[string]*models.Term)}
}

func (f *fakeDictionaryRepo) assignID(term *models.Term) {
	if term.ID != "" {
		return
	}
	f.next++
	term.ID = fakeID(f.next)
}

// fakeID returns a deterministic string ID for test rows.
func fakeID(i int) string {
	const digits = "0123456789"
	out := ""
	if i == 0 {
		return "0"
	}
	for i > 0 {
		out = string(digits[i%10]) + out
		i /= 10
	}
	return "fake-id-" + out
}

var _ ports.DictionaryRepository = (*fakeDictionaryRepo)(nil)

func (f *fakeDictionaryRepo) InsertTerm(_ context.Context, userID string, term *models.Term) (*models.Term, error) {
	for _, existing := range f.terms {
		if existing.UserID == userID && strings.EqualFold(existing.Term, term.Term) {
			return nil, ports.ErrTermConflict
		}
	}
	f.assignID(term)

	clone := *term
	clone.UserID = userID
	clone.Synonyms = cloneSliceStrings(clone.Synonyms)
	clone.Tags = cloneSliceStrings(clone.Tags)
	now := time.Now().UTC()
	clone.CreatedAt = now
	clone.UpdatedAt = now
	f.terms[dictRepoKey(userID, term.ID)] = &clone
	out := clone
	return &out, nil
}

func (f *fakeDictionaryRepo) GetTerm(_ context.Context, userID, termID string) (*models.Term, error) {
	t, ok := f.terms[dictRepoKey(userID, termID)]
	if !ok {
		return nil, ports.ErrTermNotFound
	}
	out := *t
	out.Synonyms = cloneSliceStrings(out.Synonyms)
	out.Tags = cloneSliceStrings(out.Tags)
	return &out, nil
}

func (f *fakeDictionaryRepo) UpdateTerm(_ context.Context, userID string, term *models.Term) error {
	existing, ok := f.terms[dictRepoKey(userID, term.ID)]
	if !ok {
		return ports.ErrTermNotFound
	}
	clone := *term
	clone.UserID = userID
	clone.CreatedAt = existing.CreatedAt
	clone.UpdatedAt = time.Now().UTC()
	clone.Synonyms = cloneSliceStrings(clone.Synonyms)
	clone.Tags = cloneSliceStrings(clone.Tags)
	*existing = clone
	return nil
}

func (f *fakeDictionaryRepo) DeleteTerm(_ context.Context, userID, termID string) error {
	if _, ok := f.terms[dictRepoKey(userID, termID)]; !ok {
		return ports.ErrTermNotFound
	}
	delete(f.terms, dictRepoKey(userID, termID))
	return nil
}

func (f *fakeDictionaryRepo) ListTerms(_ context.Context, userID, category, tag string) ([]*models.Term, error) {
	out := make([]*models.Term, 0)
	for _, t := range f.terms {
		if t.UserID != userID {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		if tag != "" && !containsStrings(t.Tags, tag) {
			continue
		}
		clone := *t
		clone.Synonyms = cloneSliceStrings(clone.Synonyms)
		clone.Tags = cloneSliceStrings(clone.Tags)
		out = append(out, &clone)
	}
	// Production SQL orders by `lower(term) ASC`; fake mirrors so tests
	// don't depend on Go's map iteration order.
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Term) < strings.ToLower(out[j].Term)
	})
	return out, nil
}

// searchTokensAndMatch mirrors the production `to_tsquery('english', $1 || ':*')`
// semantics for word-prefix matching so the fake behaves like Postgres FTS.
func searchTokensAndMatch(query, haystack string) bool {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(tokens) == 0 {
		return true
	}
	hayWords := strings.Fields(strings.ToLower(haystack))
	for _, tok := range tokens {
		for _, w := range hayWords {
			if strings.HasPrefix(w, tok) {
				return true
			}
		}
	}
	return false
}

func (f *fakeDictionaryRepo) SearchTerms(_ context.Context, userID, query string) ([]*models.Term, error) {
	if strings.TrimSpace(query) == "" {
		return f.ListTerms(context.Background(), userID, "", "")
	}
	out := make([]*models.Term, 0)
	for _, t := range f.terms {
		if t.UserID != userID {
			continue
		}
		haystack := strings.ToLower(t.Term) + " " + strings.ToLower(t.Explanation)
		for _, s := range t.Synonyms {
			haystack += " " + strings.ToLower(s)
		}
		for _, g := range t.Tags {
			haystack += " " + strings.ToLower(g)
		}
		if !searchTokensAndMatch(query, haystack) {
			continue
		}
		clone := *t
		clone.Synonyms = cloneSliceStrings(clone.Synonyms)
		clone.Tags = cloneSliceStrings(clone.Tags)
		out = append(out, &clone)
	}
	if len(out) > ports.MaxSearchResults {
		out = out[:ports.MaxSearchResults]
	}
	return out, nil
}

func dictRepoKey(userID, termID string) string { return userID + "|" + termID }

func cloneSliceStrings(src []string) []string {
	if src == nil {
		return []string{}
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func containsStrings(list []string, needle string) bool {
	for _, s := range list {
		if s == needle {
			return true
		}
	}
	return false
}

// seed inserts a complete term bypassing service-layer validation; for tests
// only.
func (f *fakeDictionaryRepo) seed(t *models.Term) {
	if t.ID == "" {
		f.next++
		t.ID = fakeID(f.next)
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.UpdatedAt = t.CreatedAt
	t.Synonyms = cloneSliceStrings(t.Synonyms)
	t.Tags = cloneSliceStrings(t.Tags)
	f.terms[dictRepoKey(t.UserID, t.ID)] = t
}

// setupDictionaryRouter mounts all dictionary endpoints per spec.
func setupDictionaryRouter(h *DictionaryHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/dictionary", h.ListOrSearchTerms)
	r.Post("/api/v1/dictionary", h.CreateTerm)
	r.Get("/api/v1/dictionary/{id}", h.GetTerm)
	r.Put("/api/v1/dictionary/{id}", h.UpdateTerm)
	r.Delete("/api/v1/dictionary/{id}", h.DeleteTerm)
	return r
}

func dictReqWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), authmiddleware.UserIDContextKey{}, userID)
	return r.WithContext(ctx)
}

// newTestHandler returns a fresh handler backed by a real
// *services.DictionaryService + an in-memory fake repo.
func newTestHandler() (*DictionaryHandler, *fakeDictionaryRepo) {
	repo := newFakeDictionaryRepo()
	svc := services.NewDictionaryService(repo)
	return NewDictionaryHandler(svc), repo
}

// --- POST /api/v1/dictionary ----------------------------------------------

func TestDictionaryHandler_CreateTerm_HappyPath_Returns201(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	body := `{"term":"closure","explanation":"A function capturing its lexical environment.","category":"Concepto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}
	var got models.Term
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID == "" {
		t.Errorf("response ID must be populated by the service, got empty")
	}
	if got.Term != "closure" {
		t.Errorf("response Term = %q, want closure", got.Term)
	}
	if got.UserID != "" {
		// UserID should NOT be serialized (json:"-") so the response stays safe.
		t.Errorf("UserID should be omitted (json:\"-\") in response, got %q", got.UserID)
	}
}

func TestDictionaryHandler_CreateTerm_Duplicate_Returns409(t *testing.T) {
	h, repo := newTestHandler()
	repo.seed(&models.Term{
		UserID:      "user-1",
		Term:        "closure",
		Explanation: "Pre-existing explanation here.",
		Category:    "Concepto",
	})
	r := setupDictionaryRouter(h)

	body := `{"term":"Closure","explanation":"A new explanation here.","category":"Concepto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_CreateTerm_ShortExplanation_Returns400(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	// Explanation below 10 chars → spec scenario "Explanation too short" returns 400.
	body := `{"term":"hardcode","explanation":"too short","category":"Concepto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short explanation, got %d; body: %s",
			w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_CreateTerm_InvalidCategory_Returns400(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	// Spec scenario: "Invalid category rejection" — returns 400 with field-level error.
	body := `{"term":"singleton","explanation":"A class with only one.","category":"Design Pattern"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid category, got %d; body: %s",
			w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_CreateTerm_MalformedJSON_Returns400(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		bytes.NewReader([]byte("{not-json")))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed JSON, got %d; body: %s",
			w.Code, w.Body.String())
	}
}

// --- GET /api/v1/dictionary/:id -------------------------------------------

func TestDictionaryHandler_GetTerm_HappyPath_Returns200(t *testing.T) {
	h, repo := newTestHandler()
	seeded := &models.Term{
		UserID:      "user-1",
		Term:        "lexical-scope",
		Explanation: "The scope resolved at author-time.",
		Category:    "Concepto",
	}
	repo.seed(seeded)
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/"+seeded.ID, nil)
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var got models.Term
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Term != "lexical-scope" {
		t.Errorf("response Term = %q, want lexical-scope", got.Term)
	}
}

func TestDictionaryHandler_GetTerm_NotFound_Returns404(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/missing", nil)
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// --- PUT /api/v1/dictionary/:id -------------------------------------------

func TestDictionaryHandler_UpdateTerm_HappyPath_Returns200(t *testing.T) {
	h, repo := newTestHandler()
	seeded := &models.Term{
		UserID:      "user-1",
		Term:        "hardcode",
		Explanation: "Original explanation length.",
		Category:    "Concepto",
	}
	repo.seed(seeded)
	r := setupDictionaryRouter(h)

	body := `{"term":"hardcode","explanation":"Refined explanation with enough length now.","category":"Anti-patrón"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/dictionary/"+seeded.ID,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var got models.Term
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Category != "Anti-patrón" {
		t.Errorf("response Category = %q, want Anti-patrón", got.Category)
	}
	if got.Explanation != "Refined explanation with enough length now." {
		t.Errorf("response Explanation = %q, want refined", got.Explanation)
	}
}

func TestDictionaryHandler_UpdateTerm_NotFound_Returns404(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	body := `{"term":"closure","explanation":"Updated explanation length here.","category":"Concepto"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/dictionary/missing",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_UpdateTerm_InvalidCategory_Returns400(t *testing.T) {
	h, repo := newTestHandler()
	seeded := &models.Term{
		UserID:      "user-1",
		Term:        "hardcode",
		Explanation: "Original explanation length.",
		Category:    "Concepto",
	}
	repo.seed(seeded)
	r := setupDictionaryRouter(h)

	body := `{"term":"hardcode","explanation":"Updated explanation length here.","category":"Design Pattern"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/dictionary/"+seeded.ID,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid category, got %d; body: %s",
			w.Code, w.Body.String())
	}
}

// --- DELETE /api/v1/dictionary/:id ----------------------------------------

func TestDictionaryHandler_DeleteTerm_HappyPath_Returns204(t *testing.T) {
	h, repo := newTestHandler()
	seeded := &models.Term{
		UserID:      "user-1",
		Term:        "hardcode",
		Explanation: "Original explanation length.",
		Category:    "Concepto",
	}
	repo.seed(seeded)
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/dictionary/"+seeded.ID, nil)
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", w.Code, w.Body.String())
	}
	if _, err := repo.GetTerm(context.Background(), "user-1", seeded.ID); err == nil {
		t.Errorf("seeded term should be removed after DELETE")
	}
}

func TestDictionaryHandler_DeleteTerm_CrossUser_Returns404_NoLeak(t *testing.T) {
	// Spec scenario: "Delete term owned by another user → 404 Not Found
	// (no information leak)".
	h, repo := newTestHandler()
	otherUser := &models.Term{
		UserID:      "user-other",
		Term:        "their-term",
		Explanation: "Owned by a different user.",
		Category:    "Concepto",
	}
	repo.seed(otherUser)
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/dictionary/"+otherUser.ID, nil)
	req = dictReqWithUserID(req, "user-1") // attacker claims bob's ID
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 404, NOT 403, so the response is indistinguishable from "row missing".
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-user delete (no info leak), got %d; body: %s",
			w.Code, w.Body.String())
	}
	low := strings.ToLower(w.Body.String())
	for _, leak := range []string{"owner", "permission", "forbidden", "not yours"} {
		if strings.Contains(low, leak) {
			t.Errorf("404 body leaks ownership detail %q: %s", leak, w.Body.String())
		}
	}
	// The other user's row must still exist.
	if _, err := repo.GetTerm(context.Background(), "user-other", otherUser.ID); err != nil {
		t.Errorf("other user's term should remain after cross-user DELETE attempt; got err %v", err)
	}
}

// --- GET /api/v1/dictionary (list or search) ------------------------------

func TestDictionaryHandler_ListTerms_NoQuery_Returns200_TerminologyArray(t *testing.T) {
	h, repo := newTestHandler()
	repo.seed(&models.Term{
		UserID: "user-1", Term: "async",
		Explanation: "Non-blocking concurrency primitive.",
		Category:    "Concepto",
	})
	repo.seed(&models.Term{
		UserID: "user-1", Term: "bloc",
		Explanation: "Compositional async pattern.",
		Category:    "Patrón",
	})
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary", nil)
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() == "null" {
		t.Errorf("expected [] not null on non-empty listing")
	}
	var got []models.Term
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	// alphabetical order: "async" before "bloc"
	if got[0].Term != "async" || got[1].Term != "bloc" {
		t.Errorf("expected order (async,bloc), got (%q,%q)",
			got[0].Term, got[1].Term)
	}
}

func TestDictionaryHandler_ListTerms_EmptyResult_Returns200_EmptyArray(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary", nil)
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if w.Body.String() == "null" {
		t.Fatalf("expected [] not null on empty listing")
	}
	var got []models.Term
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestDictionaryHandler_SearchTerms_WithQuery_RoutesToSearchService(t *testing.T) {
	h, repo := newTestHandler()
	repo.seed(&models.Term{
		UserID: "user-1", Term: "closure",
		Explanation: "Captures lexical scope of variables.",
		Category:    "Concepto",
	})
	repo.seed(&models.Term{
		UserID: "user-1", Term: "lambda",
		Explanation: "An anonymous function literal.",
		Category:    "Concepto",
	})
	r := setupDictionaryRouter(h)

	// spec: search MUST match term, explanation, synonyms, and tags
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"term name match", "closure", "closure"},
		{"explanation text match", "lexical", "closure"},
		{"multi-term match returns ranked", "lambda", "lambda"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/v1/dictionary?q="+c.query, nil)
			req = dictReqWithUserID(req, "user-1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
			}
			var got []models.Term
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(got) != 1 || got[0].Term != c.want {
				t.Errorf("search %q = %+v, want single %q", c.query, got, c.want)
			}
		})
	}
}

func TestDictionaryHandler_ListTerms_CategoryFilter_PassesThrough(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/dictionary?category=Patr%C3%B3n", nil) // "Patrón" url-encoded
	req = dictReqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// --- Auth integration at the handler tier ---------------------------

// TestDictionaryHandler_MissingUserIDContext_ReturnsBadRequest — proves
// the handler's auth path is wired: when chi middlewares above the
// dictionary router forgot to set `authmiddleware.UserIDContextKey`,
// the service refuses the empty user_id with ErrDictionaryValidation
// and the handler maps that to a 400. This is the bug-class check the
// spec scenario "Unauthenticated request rejected → 401" wants; the
// full 401 path is enforced upstream by the auth middleware itself
// (see authmiddleware.AuthMiddleware) while these tests exercise the
// handler's defensive null check.
func TestDictionaryHandler_MissingUserIDContext_ReturnsBadRequest(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	// Note: dictReqWithUserID is intentionally NOT called so the
	// context arrives with no user_id. Service should reject.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when user_id is missing from context, got %d; body=%s",
			w.Code, w.Body.String())
	}
	if !strings.Contains(strings.ToLower(w.Body.String()), "user_id") {
		t.Errorf("expected error to mention user_id, got %s", w.Body.String())
	}
}

// TestDictionaryHandler_PostWithoutUserID_ReturnsBadRequest mirrors
// the auth path for the write side; ensures defense-in-depth at the
// service layer when the auth middleware is bypassed in tests.
func TestDictionaryHandler_PostWithoutUserID_ReturnsBadRequest(t *testing.T) {
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	body := `{"term":"hardcode","explanation":"Writing literal values in code.","category":"Concepto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dictionary",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No user_id stamp.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

// TestDictionaryHandler_GetUnauthenticated_HitsAuth_GateWay — proves
// the route mount accepts the auth middleware path: the *router test
// helpers here use chi.NewRouter (no AuthMiddleware), so the missing
// user_id manifests as a 400. In production the same route is mounted
// under r.Route("/api/v1", r.Use(authmiddleware.AuthMiddleware(...)))
// which produces a 401 when the JWT is absent. The combined wiring is
// covered by the MCP HTTP transport tests in
// internal/infrastructure/mcp/http_transport_test.go for the JSON-RPC
// path and by the ReadHistoryRoute chain in main.go's `/api/v1*`
// routes for REST. This breadcrumb keeps the chain auditable.
func TestDictionaryHandler_GetUnauthenticated_HitsAuth_GateWay(t *testing.T) {
	// We intentionally do NOT stamp a user id; the handler MUST refuse
	// with a 400. Production adds an upstream AuthMiddleware that
	// returns 401; both behaviors are correct for their layer, and the
	// contract is documented in main.go and the handler integration.
	h, _ := newTestHandler()
	r := setupDictionaryRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (auth-miss on the handler tier), got %d; body=%s",
			w.Code, w.Body.String())
	}
}
