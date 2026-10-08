package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// tutorFakeRepo is a deterministic in-memory fake for tutor-lookup tests.
// It is intentionally minimal — it only needs to honor InsertTerm and
// SearchTerms for the cases the tutor exercises.
type tutorFakeRepo struct {
	byUserTerm map[string]*models.Term
	allByUser  map[string][]*models.Term
}

func newTutorFakeRepo() *tutorFakeRepo {
	return &tutorFakeRepo{
		byUserTerm: make(map[string]*models.Term),
		allByUser:  make(map[string][]*models.Term),
	}
}

func (f *tutorFakeRepo) InsertTerm(_ context.Context, userID string, term *models.Term) (*models.Term, error) {
	key := userID + "|" + strings.ToLower(term.Term)
	if _, exists := f.byUserTerm[key]; exists {
		return nil, ports.ErrTermConflict
	}
	clone := *term
	clone.UserID = userID
	clone.Synonyms = cloneSlice(clone.Synonyms)
	clone.Tags = cloneSlice(clone.Tags)
	f.byUserTerm[key] = &clone
	f.allByUser[userID] = append(f.allByUser[userID], &clone)
	return &clone, nil
}

func (f *tutorFakeRepo) GetTerm(_ context.Context, userID, termID string) (*models.Term, error) {
	for _, t := range f.byUserTerm {
		if t.UserID == userID && t.ID == termID {
			clone := *t
			return &clone, nil
		}
	}
	return nil, ports.ErrTermNotFound
}

func (f *tutorFakeRepo) UpdateTerm(_ context.Context, userID string, term *models.Term) error {
	if _, ok := f.byUserTerm[userID+"|"+term.ID]; !ok {
		return ports.ErrTermNotFound
	}
	clone := *term
	clone.UserID = userID
	f.byUserTerm[userID+"|"+term.ID] = &clone
	return nil
}

func (f *tutorFakeRepo) DeleteTerm(_ context.Context, userID, termID string) error {
	if _, ok := f.byUserTerm[userID+"|"+termID]; !ok {
		return ports.ErrTermNotFound
	}
	delete(f.byUserTerm, userID+"|"+termID)
	return nil
}

func (f *tutorFakeRepo) ListTerms(_ context.Context, userID, category, tag string) ([]*models.Term, error) {
	out := []*models.Term{}
	for _, t := range f.allByUser[userID] {
		if category != "" && t.Category != category {
			continue
		}
		if tag != "" && !containsStringHelper(t.Tags, tag) {
			continue
		}
		clone := *t
		out = append(out, &clone)
	}
	return out, nil
}

// SearchTerms honors the production contract used elsewhere in this
// package (word-prefix AND on term/explanation/synonyms/tags).
func (f *tutorFakeRepo) SearchTerms(_ context.Context, userID, query string) ([]*models.Term, error) {
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		return f.ListTerms(context.Background(), userID, "", "")
	}
	out := []*models.Term{}
	for _, t := range f.allByUser[userID] {
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
		out = append(out, &clone)
	}
	return out, nil
}

// --- Tests for TutorLookupService ---

func TestTutorLookup_SearchAndUseFoundTerm(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)
	ctx := context.Background()

	// Seed: "PR" -> Pull Request explanation.
	if _, err := dict.AddTerm(ctx, mustAddInput("user-1", "PR", "Pull Request on a repository, code review request.", "Concepto")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := tutor.TutorLookup(ctx, TutorLookupInput{
		UserID: "user-1", Query: "PR", Level: 0,
	})
	if err != nil {
		t.Fatalf("TutorLookup: %v", err)
	}
	if !got.Found {
		t.Errorf("Found = false, want true (term 'PR' exists)")
	}
	if got.Term == nil || got.Term.Term != "pr" {
		t.Errorf("Term = %+v, want 'pr'", got.Term)
	}
	if got.Source != "dictionary" {
		t.Errorf("Source = %q, want 'dictionary'", got.Source)
	}
	if got.SocraticLevel != 0 {
		t.Errorf("SocraticLevel = %d, want 0", got.SocraticLevel)
	}
	if !strings.Contains(got.Framing, "Pull Request") {
		t.Errorf("Framing should reference the dictionary explanation, got %q", got.Framing)
	}
}

func TestTutorLookup_SearchWithoutHit_FallsBackWithoutLeakingDictionaryAbsence(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)
	ctx := context.Background()

	got, err := tutor.TutorLookup(ctx, TutorLookupInput{
		UserID: "user-1", Query: "Qu\u00e9 es monad?", Level: 0,
	})
	if err != nil {
		t.Fatalf("TutorLookup missing: %v", err)
	}
	if got.Found {
		t.Errorf("Found = true, want false (no term exists)")
	}
	if got.Term != nil {
		t.Errorf("Term should be nil on miss, got %+v", got.Term)
	}
	if got.Source != "general" {
		t.Errorf("Source = %q, want 'general'", got.Source)
	}
	// Framing MUST NOT mention 'dictionary' to the user — spec scenario
	// "Tutor finds no matching term" requires silent fallback.
	if strings.Contains(strings.ToLower(got.Framing), "dictionary") {
		t.Errorf("fallback framing leaked 'dictionary' detail: %q", got.Framing)
	}
	// It SHOULD hint at the user's subject "monad" so the LLM keeps it.
	if !strings.Contains(strings.ToLower(got.Framing), "monad") {
		t.Errorf("fallback framing lost the user's subject: %q", got.Framing)
	}
}

func TestTutorLookup_RespectsSocraticLevel(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)
	ctx := context.Background()

	if _, err := dict.AddTerm(ctx, mustAddInput("user-1", "PR", "Pull Request on a repository, code review request.", "Concepto")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cases := []struct {
		level    int
		mustHave []string
		mustNot  []string
	}{
		// Level 0 (direct) — full explanation surfaced.
		{0, []string{"Pull Request"}, nil},
		// Level 1 (guided) — explanation surfaced, then a follow-up question.
		{1, []string{"Pull Request", "deeper"}, nil},
		// Level 2 (socratic) — only the term name is hinted; the explanation
		// is NOT surfaced verbatim. The tutor is expected to elicit it.
		{2, []string{"PR"}, []string{"code review request"}},
		// Level 3 (discovery) — pure provocation, no explanation at all.
		{3, []string{"PR"}, []string{"code review request", "Pull Request"}},
	}

	for _, c := range cases {
		t.Run("level_"+itoa(c.level), func(t *testing.T) {
			got, err := tutor.TutorLookup(ctx, TutorLookupInput{
				UserID: "user-1", Query: "PR", Level: c.level,
			})
			if err != nil {
				t.Fatalf("TutorLookup level=%d: %v", c.level, err)
			}
			if !got.Found {
				t.Fatalf("Found = false, want true (term exists)")
			}
			if got.SocraticLevel != c.level {
				t.Errorf("SocraticLevel echo = %d, want %d", got.SocraticLevel, c.level)
			}
			gotLower := strings.ToLower(got.Framing)
			for _, want := range c.mustHave {
				if !strings.Contains(gotLower, strings.ToLower(want)) {
					t.Errorf("level %d framing missing %q: got %q", c.level, want, got.Framing)
				}
			}
			for _, deny := range c.mustNot {
				if strings.Contains(gotLower, strings.ToLower(deny)) {
					t.Errorf("level %d framing leaked forbidden %q: got %q", c.level, deny, got.Framing)
				}
			}
		})
	}
}

func TestTutorLookup_ClampsOutOfRangeLevel(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)
	ctx := context.Background()

	cases := []struct{ in, want int }{
		{-2, 0}, {-1, 0}, {0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 3}, {99, 3},
	}
	for _, c := range cases {
		got, err := tutor.TutorLookup(ctx, TutorLookupInput{
			UserID: "user-1", Query: "no-such-term-evar", Level: c.in,
		})
		if err != nil {
			t.Fatalf("clamp test in=%d: %v", c.in, err)
		}
		if got.SocraticLevel != c.want {
			t.Errorf("clamp in=%d -> level=%d, want %d", c.in, got.SocraticLevel, c.want)
		}
	}
}

func TestTutorLookup_RejectsEmptyUserID(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)

	_, err := tutor.TutorLookup(context.Background(), TutorLookupInput{
		UserID: "", Query: "x", Level: 1,
	})
	if !errors.Is(err, ErrDictionaryValidation) {
		t.Errorf("err = %v, want ErrDictionaryValidation", err)
	}
}

func TestTutorLookup_EmptyQuery_ReturnsEmptyResult(t *testing.T) {
	repo := newTutorFakeRepo()
	dict := NewDictionaryService(repo)
	tutor := NewTutorLookupService(dict)

	got, err := tutor.TutorLookup(context.Background(), TutorLookupInput{
		UserID: "user-1", Query: "  ", Level: 0,
	})
	if err != nil {
		t.Fatalf("empty query: %v", err)
	}
	if got.Found || got.Term != nil {
		t.Errorf("empty query should be a clean miss, got %+v", got)
	}
	if got.Framing == "" {
		t.Errorf("empty-query framing should still be a non-empty general opener")
	}
}

func TestTutorLookup_NilService_ReturnsUnavailable(t *testing.T) {
	var svc *TutorLookupService
	_, err := svc.TutorLookup(context.Background(), TutorLookupInput{
		UserID: "user-1", Query: "x", Level: 0,
	})
	if !errors.Is(err, ErrTutorLookupUnavailable) {
		t.Errorf("err = %v, want ErrTutorLookupUnavailable", err)
	}
}

func TestNewTutorLookupService_NilDict_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("expected panic when calling NewTutorLookupService(nil)")
		}
	}()
	_ = NewTutorLookupService(nil)
}

// itoa is a tiny decimal int -> string conversion used to name subtests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := "0123456789"
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	if negative {
		out = "-" + out
	}
	return out
}
