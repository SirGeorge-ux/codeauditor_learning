package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Fake repository for DictionaryService tests ---

// fakeDictionaryRepo is an in-memory implementation of
// ports.DictionaryRepository. It mirrors the Supabase adapter's behavior of
// generating a fresh id when the caller passes one empty (PostgreSQL's
// `gen_random_uuid()` default + RETURNING). Not safe for concurrent use;
// tests are sequential.
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
	// Deterministic, increment-only ids — readable in failure messages.
	term.ID = "fake-id-" + intToStr(f.next)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	return out
}

func dictKey(userID, termID string) string { return userID + "|" + termID }

// Compile-time check.
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
	clone.Synonyms = cloneSlice(clone.Synonyms)
	clone.Tags = cloneSlice(clone.Tags)
	if clone.Example != nil {
		v := *clone.Example
		clone.Example = &v
	}
	f.terms[dictKey(userID, term.ID)] = &clone
	out := clone
	return &out, nil
}

func (f *fakeDictionaryRepo) GetTerm(_ context.Context, userID, termID string) (*models.Term, error) {
	t, ok := f.terms[dictKey(userID, termID)]
	if !ok {
		return nil, ports.ErrTermNotFound
	}
	clone := *t
	clone.Synonyms = cloneSlice(clone.Synonyms)
	clone.Tags = cloneSlice(clone.Tags)
	return &clone, nil
}

func (f *fakeDictionaryRepo) UpdateTerm(_ context.Context, userID string, term *models.Term) error {
	existing, ok := f.terms[dictKey(userID, term.ID)]
	if !ok {
		return ports.ErrTermNotFound
	}
	clone := *term
	clone.UserID = userID
	clone.Synonyms = cloneSlice(clone.Synonyms)
	clone.Tags = cloneSlice(clone.Tags)
	if clone.Example == nil && existing.Example != nil {
		clone.Example = existing.Example
	}
	*existing = clone
	return nil
}

func (f *fakeDictionaryRepo) DeleteTerm(_ context.Context, userID, termID string) error {
	if _, ok := f.terms[dictKey(userID, termID)]; !ok {
		return ports.ErrTermNotFound
	}
	delete(f.terms, dictKey(userID, termID))
	return nil
}

func (f *fakeDictionaryRepo) ListTerms(_ context.Context, userID, category, tag string) ([]*models.Term, error) {
	out := make([]*models.Term, 0, len(f.terms))
	for _, t := range f.terms {
		if t.UserID != userID {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		if tag != "" && !containsStringHelper(t.Tags, tag) {
			continue
		}
		clone := *t
		clone.Synonyms = cloneSlice(clone.Synonyms)
		clone.Tags = cloneSlice(clone.Tags)
		out = append(out, &clone)
	}
	// Production SQL orders by `lower(term) ASC`; the fake mirrors that
	// contract so tests asserting alphabetical order don't depend on map
	// iteration order.
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Term) < strings.ToLower(out[j].Term)
	})
	return out, nil
}

// searchTokens splits a free-text query into trimmed lowercase tokens
// (whitespace separated). Mirrors the production adapter's `to_tsquery`
// token splitting so the fake behaves like the real GIN-backed FTS for
// both infix ("literal") and prefix ("hard" matching "hardcode") cases.
func searchTokens(query string) []string {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return nil
	}
	return strings.Fields(q)
}

// tokenMatchesHaystack reports whether ANY token matches ANY word in the
// haystack as a prefix (case-insensitive). This mirrors Postgres'
// `to_tsquery('english', token || ':*')` semantics used in production:
// word-prefix matching so "hard" hits "hardcode", "hardcoded", etc.
func tokenMatchesHaystack(tokens []string, haystack string) bool {
	if len(tokens) == 0 {
		return true
	}
	hWords := strings.Fields(strings.ToLower(haystack))
	for _, tok := range tokens {
		for _, w := range hWords {
			if strings.HasPrefix(w, tok) {
				return true
			}
		}
	}
	return false
}

func (f *fakeDictionaryRepo) SearchTerms(_ context.Context, userID, query string) ([]*models.Term, error) {
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		return f.ListTerms(context.Background(), userID, "", "")
	}
	out := make([]*models.Term, 0, len(f.terms))
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
		if !tokenMatchesHaystack(tokens, haystack) {
			continue
		}
		clone := *t
		clone.Synonyms = cloneSlice(clone.Synonyms)
		clone.Tags = cloneSlice(clone.Tags)
		out = append(out, &clone)
	}
	// Production returns ranked by ts_rank DESC; the fake keeps insertion
	// order (already matches ranking for tests) but caps at MaxSearchResults.
	if len(out) > ports.MaxSearchResults {
		out = out[:ports.MaxSearchResults]
	}
	return out, nil
}

// containsStringHelper reports whether needle appears in haystack.
//
// Renamed (was "containsString") to avoid a same-package collision with the
// helper of the same name in user_progress_service_test.go's fake repo.
func containsStringHelper(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func cloneSlice(src []string) []string {
	if src == nil {
		return []string{}
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// mustAddInput is a test convenience: build a fully-valid AddTermInput with
// the given core fields. The explanation length is checked by the service,
// so callers must pass >= 10 chars.
func mustAddInput(userID, term, explanation, category string) AddTermInput {
	return AddTermInput{
		UserID:      userID,
		Term:        term,
		Explanation: explanation,
		Category:    category,
	}
}

// --- Validation unit tests (pure functions on AddTermInput) ---

func TestDictionaryService_AddTerm_RejectsEmptyUserID(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	_, err := svc.AddTerm(context.Background(), mustAddInput("", "closure", "A function capturing scope.", "Concepto"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm empty userID error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_AddTerm_RejectsShortExplanation(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	_, err := svc.AddTerm(context.Background(), mustAddInput("user-1", "closure", "too short", "Concepto"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm short explanation error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_AddTerm_RejectsTooLongTerm(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	longTerm := strings.Repeat("a", 51)
	_, err := svc.AddTerm(context.Background(), mustAddInput("user-1", longTerm, "Valid explanation length here.", "Concepto"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm >50 term error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_AddTerm_RejectsTooLongExplanation(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	long := strings.Repeat("x", 501)
	_, err := svc.AddTerm(context.Background(), mustAddInput("user-1", "closure", long, "Concepto"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm >500 explanation error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_AddTerm_RejectsInvalidCategory(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	_, err := svc.AddTerm(context.Background(), mustAddInput("user-1", "singleton",
		"A class with only one instance.", "Design Pattern"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm invalid category error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_AddTerm_RejectsEmptyTerm(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	_, err := svc.AddTerm(context.Background(), mustAddInput("user-1", "", "Has minimum length requirement.", "Concepto"))
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("AddTerm empty term error = %v, want ErrDictionaryValidation", err)
	}
}

// --- Duplicate detection ---

func TestDictionaryService_AddTerm_Duplicate_ReturnsConflict(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	_, err := svc.AddTerm(ctx, mustAddInput("user-1", "closure",
		"A function capturing scope.", "Concepto"))
	if err != nil {
		t.Fatalf("first AddTerm: %v", err)
	}
	_, err = svc.AddTerm(ctx, mustAddInput("user-1", "Closure",
		"Different explanation here.", "Concepto"))
	if !errors.Is(err, ports.ErrTermConflict) {
		t.Fatalf("duplicate AddTerm (different case) error = %v, want ports.ErrTermConflict", err)
	}
}

// --- Happy path: lowercase + persisted as-is ---

func TestDictionaryService_AddTerm_HappyPath_LowercasesAndPersists(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	in := mustAddInput("user-1", "Hardcode", "Writing literal values in code.", "Concepto")
	in.Synonyms = []string{"hardcodear"}
	in.Tags = []string{"general"}

	got, err := svc.AddTerm(ctx, in)
	if err != nil {
		t.Fatalf("AddTerm happy: %v", err)
	}
	if got.Term != "hardcode" {
		t.Errorf("AddTerm Term = %q, want hardcode (lowercased)", got.Term)
	}
	if got.UserID != "user-1" {
		t.Errorf("AddTerm UserID = %q, want user-1 (from JWT)", got.UserID)
	}
	if got.ID == "" {
		t.Errorf("AddTerm ID must be assigned by repo, got empty")
	}
	if len(got.Synonyms) != 1 || got.Synonyms[0] != "hardcodear" {
		t.Errorf("AddTerm Synonyms = %v, want [hardcodear]", got.Synonyms)
	}
}

// --- Get / Update / Delete ---

func TestDictionaryService_GetTerm_HappyPath(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	created, err := svc.AddTerm(ctx, mustAddInput("user-1", "hardcode",
		"Writing literal values in code.", "Concepto"))
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	got, err := svc.GetTerm(ctx, "user-1", created.ID)
	if err != nil {
		t.Fatalf("GetTerm: %v", err)
	}
	if got.Term != "hardcode" {
		t.Errorf("GetTerm Term = %q, want hardcode", got.Term)
	}
}

func TestDictionaryService_GetTerm_NotFound(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	_, err := svc.GetTerm(context.Background(), "user-1", "missing-id")
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("GetTerm missing error = %v, want ports.ErrTermNotFound", err)
	}
}

func TestDictionaryService_UpdateTerm_HappyPath(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	created, err := svc.AddTerm(ctx, mustAddInput("user-1", "hardcode",
		"Original explanation length.", "Concepto"))
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	if _, err := svc.UpdateTerm(ctx, "user-1", created.ID, UpdateTermInput{
		Term:        "hardcode",
		Explanation: "Refined explanation in cat category here.",
		Category:    "Anti-patrón",
	}); err != nil {
		t.Fatalf("UpdateTerm: %v", err)
	}
	got, err := svc.GetTerm(ctx, "user-1", created.ID)
	if err != nil {
		t.Fatalf("GetTerm after update: %v", err)
	}
	if got.Category != "Anti-patrón" {
		t.Errorf("updated Category = %q, want Anti-patrón", got.Category)
	}
	if got.Explanation != "Refined explanation in cat category here." {
		t.Errorf("updated Explanation = %q, want refined", got.Explanation)
	}
}

func TestDictionaryService_UpdateTerm_RejectsInvalidCategory(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	created, err := svc.AddTerm(ctx, mustAddInput("user-1", "hardcode",
		"Original explanation length.", "Concepto"))
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	_, err = svc.UpdateTerm(ctx, "user-1", created.ID, UpdateTermInput{
		Term:        "hardcode",
		Explanation: "Trying invalid category on update.",
		Category:    "Not A Category",
	})
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Fatalf("UpdateTerm invalid category error = %v, want ErrDictionaryValidation", err)
	}
}

func TestDictionaryService_DeleteTerm_HappyPath(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()

	created, err := svc.AddTerm(ctx, mustAddInput("user-1", "hardcode",
		"Writing literal values in code.", "Concepto"))
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	if err := svc.DeleteTerm(ctx, "user-1", created.ID); err != nil {
		t.Fatalf("DeleteTerm: %v", err)
	}
	_, err = svc.GetTerm(ctx, "user-1", created.ID)
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("GetTerm after delete error = %v, want ports.ErrTermNotFound", err)
	}
}

func TestDictionaryService_DeleteTerm_NotFound(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	err := svc.DeleteTerm(context.Background(), "user-1", "missing-id")
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("DeleteTerm missing error = %v, want ports.ErrTermNotFound", err)
	}
}

// --- List + Search ---

func TestDictionaryService_ListTerms_NoFilter_ReturnsAll(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()
	seed := func(term, cat string) {
		_, err := svc.AddTerm(ctx, mustAddInput("user-1", term, "Seeded term with enough text.", cat))
		if err != nil {
			t.Fatalf("seed %q: %v", term, err)
		}
	}
	seed("async", "Concepto")
	seed("bloc", "Patrón")

	got, err := svc.ListTerms(ctx, "user-1", "", "")
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListTerms len = %d, want 2", len(got))
	}
	if got[0].Term != "async" || got[1].Term != "bloc" {
		t.Errorf("ListTerms order = (%q,%q), want (async,bloc)", got[0].Term, got[1].Term)
	}
}

func TestDictionaryService_ListTerms_FilterByCategory(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()
	mustAdd := func(term, cat string) {
		if _, err := svc.AddTerm(ctx, mustAddInput("user-1", term, "Seeded term with enough text.", cat)); err != nil {
			t.Fatalf("seed %q: %v", term, err)
		}
	}
	mustAdd("async", "Concepto")
	mustAdd("bloc", "Patrón")
	mustAdd("factory", "Patrón")

	got, err := svc.ListTerms(ctx, "user-1", "Patrón", "")
	if err != nil {
		t.Fatalf("ListTerms(category): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListTerms(category) len = %d, want 2", len(got))
	}
	for _, term := range got {
		if term.Category != "Patrón" {
			t.Errorf("ListTerms(category) returned %q (cat %q), want only Patrón",
				term.Term, term.Category)
		}
	}
}

func TestDictionaryService_SearchTerms_FindsByTermAndExplanationAndSynonymAndTag(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()
	in := mustAddInput("user-1", "hardcode", "Writing literal values in code.", "Concepto")
	in.Synonyms = []string{"hardcodear"}
	in.Tags = []string{"general"}
	if _, err := svc.AddTerm(ctx, in); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Spec: search MUST match term, explanation, synonyms, and tags.
	cases := []struct{ name, query, wantTerm string }{
		{"term name", "hard", "hardcode"},
		{"explanation text", "literal", "hardcode"},
		{"synonym", "hardcodear", "hardcode"},
		{"tag", "general", "hardcode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := svc.SearchTerms(ctx, "user-1", c.query)
			if err != nil {
				t.Fatalf("SearchTerms: %v", err)
			}
			if len(got) != 1 || got[0].Term != c.wantTerm {
				t.Errorf("SearchTerms(%q) = %+v, want single %q", c.query, got, c.wantTerm)
			}
		})
	}
}

func TestDictionaryService_SearchTerms_FindsMatchInExplanation(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()
	if _, err := svc.AddTerm(ctx, mustAddInput("user-1", "closure",
		"Captures lexical scope of the variables.", "Concepto")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := svc.SearchTerms(ctx, "user-1", "lexical")
	if err != nil {
		t.Fatalf("SearchTerms: %v", err)
	}
	if len(got) != 1 || got[0].Term != "closure" {
		t.Errorf("SearchTerms(lexical) = %+v, want single closure", got)
	}
}

func TestDictionaryService_SearchTerms_CapsAtMaxResults(t *testing.T) {
	repo := newFakeDictionaryRepo()
	svc := NewDictionaryService(repo)
	ctx := context.Background()
	// Seed 25 matching terms; service contract says cap at MaxSearchResults.
	for i := 0; i < 25; i++ {
		letter := string(rune('a' + i%26))
		if _, err := svc.AddTerm(ctx, mustAddInput("user-1", "match"+letter,
			"contains the word match somewhere here.", "Concepto")); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	got, err := svc.SearchTerms(ctx, "user-1", "match")
	if err != nil {
		t.Fatalf("SearchTerms: %v", err)
	}
	if len(got) != 20 {
		t.Errorf("SearchTerms cap = %d, want %d (caps at MaxSearchResults=20)",
			len(got), ports.MaxSearchResults)
	}
}
