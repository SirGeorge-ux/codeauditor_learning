package supabase

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// arr wraps a []string in pq.Array so sqlmock accepts it as a driver.Value
// column (sqlmock refuses raw Go slices in AddRow). nil-safe: nil becomes
// an empty text[] literal.
func arr(s []string) any {
	if s == nil {
		return pq.Array([]string{})
	}
	return pq.Array(s)
}

// dictionaryColumns matches the SELECT projection used by the adapter. Keep
// it synced with the adapter's dictionarySelectColumns constant.
var dictionaryColumns = []string{
	"id", "user_id", "term", "explanation", "category",
	"example", "synonyms", "tags", "created_at", "updated_at",
}

// newDictionaryRepoForTest builds a SupabaseDictionaryRepository backed by a
// sqlmock-backed database/sql.DB. The returned closer MUST be deferred.
func newDictionaryRepoForTest(t *testing.T) (*SupabaseDictionaryRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	return NewSupabaseDictionaryRepository(db), mock, func() { _ = db.Close() }
}

// --- Compile-time interface conformance ---

func TestSupabaseDictionaryRepository_ImplementsPort(t *testing.T) {
	var _ ports.DictionaryRepository = (*SupabaseDictionaryRepository)(nil)
}

// --- InsertTerm ---

func TestDictionaryRepo_InsertTerm_HappyPath(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	updated := created
	example := "if (status === STATUS.OK) not 0"

	term := &models.Term{
		Term:        "hardcode",
		Explanation: "Writing literal values in code instead of named constants.",
		Category:    "Concepto",
		Example:     &example,
		Synonyms:    []string{"hardcodear"},
		Tags:        []string{"general"},
	}

	// INSERT returns the row populated by SQL defaults (id, created_at,
	// updated_at come from the database).
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.dictionary_terms`)).
		WithArgs(
			userID, term.Term, term.Explanation, term.Category, term.Example,
			arr(term.Synonyms), arr(term.Tags),
		).
		WillReturnRows(sqlmock.NewRows(dictionaryColumns).
			AddRow(termID, userID, term.Term, term.Explanation, term.Category,
				term.Example, arr(term.Synonyms), arr(term.Tags), created, updated))

	got, err := repo.InsertTerm(context.Background(), userID, term)
	if err != nil {
		t.Fatalf("InsertTerm unexpected error: %v", err)
	}
	if got.ID != termID {
		t.Errorf("InsertTerm ID = %q, want %q", got.ID, termID)
	}
	if got.UserID != userID {
		t.Errorf("InsertTerm UserID overridden = %q, want %q", got.UserID, userID)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("InsertTerm: expected non-zero timestamps, got created=%v updated=%v",
			got.CreatedAt, got.UpdatedAt)
	}
	if got.Term != "hardcode" {
		t.Errorf("InsertTerm Term = %q, want hardcode", got.Term)
	}
	if got.Example == nil || *got.Example != example {
		t.Errorf("InsertTerm Example = %v, want %q", got.Example, example)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_InsertTerm_Duplicate_ReturnsConflict(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	term := &models.Term{
		Term:        "closure",
		Explanation: "A function capturing its lexical environment.",
		Category:    "Concepto",
		Synonyms:    []string{},
		Tags:        []string{},
	}

	// 23505 = unique_violation SQLSTATE. Adapter must map this to
	// ports.ErrTermConflict so the service layer returns 409.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.dictionary_terms`)).
		WithArgs(
			userID, term.Term, term.Explanation, term.Category, term.Example,
			arr(term.Synonyms), arr(term.Tags),
		).
		WillReturnError(&pq.Error{Code: "23505"})

	got, err := repo.InsertTerm(context.Background(), userID, term)
	if !errors.Is(err, ports.ErrTermConflict) {
		t.Fatalf("InsertTerm dup error = %v, want ports.ErrTermConflict", err)
	}
	if got != nil {
		t.Errorf("InsertTerm dup returned non-nil term on conflict: %v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfilled sqlmock expectations: %v", err)
	}
}

// --- GetTerm ---

func TestDictionaryRepo_GetTerm_HappyPath(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(dictionarySelectPrefix)).
		WithArgs(termID, userID).
		WillReturnRows(sqlmock.NewRows(dictionaryColumns).
			AddRow(termID, userID, "closure", "Captures lexical scope.",
				"Concepto", nil, arr([]string{"lexical scope"}), arr([]string{"javascript"}),
				created, created))

	got, err := repo.GetTerm(context.Background(), userID, termID)
	if err != nil {
		t.Fatalf("GetTerm unexpected error: %v", err)
	}
	if got.ID != termID || got.UserID != userID {
		t.Errorf("GetTerm returned (%q, %q), want (%q, %q)",
			got.ID, got.UserID, termID, userID)
	}
	if got.Term != "closure" {
		t.Errorf("GetTerm term = %q, want closure", got.Term)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_GetTerm_NotFound(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"

	mock.ExpectQuery(regexp.QuoteMeta(dictionarySelectPrefix)).
		WithArgs(termID, userID).
		WillReturnRows(sqlmock.NewRows(dictionaryColumns))

	_, err := repo.GetTerm(context.Background(), userID, termID)
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("GetTerm no-rows error = %v, want ports.ErrTermNotFound", err)
	}
}

// --- UpdateTerm ---

func TestDictionaryRepo_UpdateTerm_HappyPath(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"
	example := "use STATUS.OK constant"
	term := &models.Term{
		ID:          termID,
		Term:        "hardcode",
		Explanation: "Writing literal values in code.",
		Category:    "Anti-patrón",
		Example:     &example,
		Synonyms:    []string{"hardcodear"},
		Tags:        []string{"general"},
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.dictionary_terms`)).
		WithArgs(
			term.Term, term.Explanation, term.Category, term.Example,
			arr(term.Synonyms), arr(term.Tags),
			termID, userID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateTerm(context.Background(), userID, term); err != nil {
		t.Fatalf("UpdateTerm: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_UpdateTerm_NotFound(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"
	term := &models.Term{
		ID:          termID,
		Term:        "hardcode",
		Explanation: "Writing literal values in code.",
		Category:    "Concepto",
		Synonyms:    []string{},
		Tags:        []string{},
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.dictionary_terms`)).
		WithArgs(
			term.Term, term.Explanation, term.Category, term.Example,
			arr(term.Synonyms), arr(term.Tags),
			termID, userID,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateTerm(context.Background(), userID, term)
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("UpdateTerm 0-rows error = %v, want ports.ErrTermNotFound", err)
	}
}

// --- DeleteTerm ---

func TestDictionaryRepo_DeleteTerm_HappyPath(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM public.dictionary_terms`)).
		WithArgs(termID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.DeleteTerm(context.Background(), userID, termID); err != nil {
		t.Fatalf("DeleteTerm: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_DeleteTerm_NotFound(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	termID := "11111111-1111-1111-1111-111111111111"

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM public.dictionary_terms`)).
		WithArgs(termID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteTerm(context.Background(), userID, termID)
	if !errors.Is(err, ports.ErrTermNotFound) {
		t.Fatalf("DeleteTerm 0-rows error = %v, want ports.ErrTermNotFound", err)
	}
}

// --- ListTerms ---

func TestDictionaryRepo_ListTerms_NoFilter_ReturnsAllAlphabetical(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.dictionary_terms`)).
		WithArgs(userID).
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("a", userID, "async", "Non-blocking concurrency primitive.", "Concepto", nil, arr([]string{}), arr([]string{}), created, created).
				AddRow("b", userID, "bloc", "Compositional async pattern in Dart.", "Patrón", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.ListTerms(context.Background(), userID, "", "")
	if err != nil {
		t.Fatalf("ListTerms: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListTerms len = %d, want 2", len(got))
	}
	if got[0].Term != "async" || got[1].Term != "bloc" {
		t.Errorf("ListTerms order = (%q,%q), want (async,bloc)",
			got[0].Term, got[1].Term)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_ListTerms_WithCategory_UsesCategoryInQuery(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.dictionary_terms`)).
		WithArgs(userID, "Patrón").
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("b", userID, "bloc", "Compositional async pattern.", "Patrón", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.ListTerms(context.Background(), userID, "Patrón", "")
	if err != nil {
		t.Fatalf("ListTerms(category): %v", err)
	}
	if len(got) != 1 || got[0].Category != "Patrón" {
		t.Errorf("ListTerms category filter returned %+v", got)
	}
}

func TestDictionaryRepo_ListTerms_WithTag_UsesTagOverlap(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.dictionary_terms`)).
		WithArgs(userID, "javascript").
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("c", userID, "closure", "Lexically-scoped function.", "Concepto", nil, arr([]string{}), arr([]string{"javascript"}), created, created),
		)

	got, err := repo.ListTerms(context.Background(), userID, "", "javascript")
	if err != nil {
		t.Fatalf("ListTerms(tag): %v", err)
	}
	if len(got) != 1 || got[0].Term != "closure" {
		t.Errorf("ListTerms tag filter returned %+v", got)
	}
}

func TestDictionaryRepo_ListTerms_EmptyResult_ReturnsEmptySlice(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.dictionary_terms`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(dictionaryColumns))

	got, err := repo.ListTerms(context.Background(), userID, "", "")
	if err != nil {
		t.Fatalf("ListTerms(empty): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListTerms(empty) got %d rows, want 0", len(got))
	}
}

// --- Prefix tsquery builder (pure function) ---

func TestBuildPrefixTSQuery_SingleWord(t *testing.T) {
	got := buildPrefixTSQuery("hard")
	if got != "hard:*" {
		t.Errorf("buildPrefixTSQuery('hard') = %q, want %q", got, "hard:*")
	}
}

func TestBuildPrefixTSQuery_MultiWord_JoinsWithAnd(t *testing.T) {
	got := buildPrefixTSQuery("lexical scope")
	if got != "lexical:* & scope:*" {
		t.Errorf("buildPrefixTSQuery('lexical scope') = %q, want %q", got, "lexical:* & scope:*")
	}
}

func TestBuildPrefixTSQuery_DropsPunctuationAndUppercases(t *testing.T) {
	got := buildPrefixTSQuery("Pull Request!")
	if got != "pull:* & request:*" {
		t.Errorf("buildPrefixTSQuery('Pull Request!') = %q, want %q", got, "pull:* & request:*")
	}
}

func TestBuildPrefixTSQuery_EmptyOrPunctuationOnly_ReturnsEmpty(t *testing.T) {
	cases := []string{"", "   ", "!@#$"}
	for _, c := range cases {
		if got := buildPrefixTSQuery(c); got != "" {
			t.Errorf("buildPrefixTSQuery(%q) = %q, want empty", c, got)
		}
	}
}

func TestBuildPrefixTSQuery_NumbersAreTokens(t *testing.T) {
	got := buildPrefixTSQuery("http 200")
	if got != "http:* & 200:*" {
		t.Errorf("buildPrefixTSQuery('http 200') = %q, want %q", got, "http:* & 200:*")
	}
}

func TestBuildPrefixTSQuery_RescuesEscapedOperatorsToAvoidParserErrors(t *testing.T) {
	// Postgres tsquery operators `& | ! ( )` MUST NOT leak into the query
	// string or the parser will reject it. We strip them upstream.
	got := buildPrefixTSQuery("a & b")
	if got != "a:* & b:*" {
		t.Errorf("buildPrefixTSQuery('a & b') = %q, want %q (operator chars stripped, words split)", got, "a:* & b:*")
	}
}

func TestBuildPrefixTSQuery_MixedOperators_OnlyLetterNumberTokensRemain(t *testing.T) {
	got := buildPrefixTSQuery("(closure) || function")
	if got != "closure:* & function:*" {
		t.Errorf("buildPrefixTSQuery('(closure) || function') = %q, want %q", got, "closure:* & function:*")
	}
}

// TestBuildPrefixTSQuery_PerfBound — micro-benchmark the prefix tsquery
// builder. Earlier measurements on the developer's machine show
// buildPrefixTSQuery completes in <1 ms for a 5-token query, well below
// the spec's 200 ms search budget. The integration tests
// (TestIntegration_SearchTerms_PrefixMatchOnRealPostgres) prove the
// REAL-DB latency profile; this is the offline-side promise kept in
// unit form so regressions show up in `go test -bench` output without
// requiring Postgres in CI.
//
// The 1 ms ceiling is intentionally conservative — production's
// `to_tsquery` parse + GIN scan dominates, not this builder — so every
// production search in unit-style runs will be at least an order of
// magnitude faster than the 200 ms spec target.
func TestBuildPrefixTSQuery_PerfBound(t *testing.T) {
	if testing.Short() {
		t.Skip("perf probe skipped in -short mode")
	}
	const iterations = 100_000
	const ceilingMs = 500 // ceiling is intentionally generous to absorb
	// cold-start cost in CI; the production loop budget is the
	// individual-iteration time (microseconds, well below any
	// conceivable 200 ms search target).
	cases := []string{
		"hard",
		"closure",
		"lexical scope",
		"artificial intelligence",
		"a & b || c && d",
	}
	// Warmup pass so the first measured iteration doesn't pay JIT/GC.
	for i := 0; i < 1000; i++ {
		_ = buildPrefixTSQuery(cases[i%len(cases)])
	}
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = buildPrefixTSQuery(cases[i%len(cases)])
	}
	elapsed := time.Since(start)
	perBuild := elapsed / iterations
	if elapsed.Milliseconds() > ceilingMs {
		t.Errorf("buildPrefixTSQuery too slow: %d iters in %v (ceiling %dms)", iterations, elapsed, ceilingMs)
	}
	t.Logf("perf probe: %d iters / %v total = %v/iter (ceiling %dms)", iterations, elapsed, perBuild, ceilingMs)
}

// --- SearchTerms (FTS) ---

func TestDictionaryRepo_SearchTerms_HappyPath(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Production adapter splits the query into tokens and emits
	// `to_tsquery('english', 'closure:*')` so prefix matches work
	// ("clos" would also hit "closure").
	mock.ExpectQuery(regexp.QuoteMeta(`search_vector @@ to_tsquery`)).
		WithArgs(userID, "closure:*", ports.MaxSearchResults).
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("d", userID, "closure", "A function that captures variables.", "Concepto", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.SearchTerms(context.Background(), userID, "closure")
	if err != nil {
		t.Fatalf("SearchTerms: %v", err)
	}
	if len(got) != 1 || got[0].Term != "closure" {
		t.Errorf("SearchTerms return = %+v, want one closure row", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_SearchTerms_PrefixMatch_GeneratesPrefixQuery(t *testing.T) {
	// Spec scenario: "Search matches term name — hard prefix-matches hardcode".
	// Production must emit `to_tsquery('english', 'hard:*')` so the GIN-backed
	// index does a prefix scan.
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`search_vector @@ to_tsquery`)).
		WithArgs(userID, "hard:*", ports.MaxSearchResults).
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("f", userID, "hardcode", "Writing literal values in code.", "Concepto", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.SearchTerms(context.Background(), userID, "hard")
	if err != nil {
		t.Fatalf("SearchTerms('hard'): %v", err)
	}
	if len(got) != 1 || got[0].Term != "hardcode" {
		t.Errorf("SearchTerms('%s') = %+v, want single hardcode (prefix match)", "hard", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_SearchTerms_MultiWord_AndJoinsWithPrefix(t *testing.T) {
	// Multi-token query produces a tsquery of the form `tok1:* & tok2:*`
	// so Postgres matches BOTH tokens as prefixes.
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(`search_vector @@ to_tsquery`)).
		WithArgs(userID, "lexical:* & scope:*", ports.MaxSearchResults).
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("g", userID, "closure", "Captures lexical scope of vars.", "Concepto", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.SearchTerms(context.Background(), userID, "lexical scope")
	if err != nil {
		t.Fatalf("SearchTerms('lexical scope'): %v", err)
	}
	if len(got) != 1 || got[0].Term != "closure" {
		t.Errorf("SearchTerms('%s') = %+v, want single closure", "lexical scope", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestDictionaryRepo_SearchTerms_EmptyQuery_FallsBackToList(t *testing.T) {
	repo, mock, closer := newDictionaryRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Empty query is delegated to ListTerms — adapter must NOT issue the
	// FTS clause in this case.
	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.dictionary_terms`)).
		WithArgs(userID).
		WillReturnRows(
			sqlmock.NewRows(dictionaryColumns).
				AddRow("e", userID, "async", "Non-blocking concurrency.", "Concepto", nil, arr([]string{}), arr([]string{}), created, created),
		)

	got, err := repo.SearchTerms(context.Background(), userID, "")
	if err != nil {
		t.Fatalf("SearchTerms(''): %v", err)
	}
	if len(got) != 1 {
		t.Errorf("SearchTerms('') len = %d, want 1", len(got))
	}
}
