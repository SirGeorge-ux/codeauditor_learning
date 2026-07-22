package services

import (
	"context"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Fake repository for service-level tests ---

// fakeProgressRepo is an in-memory UserProgressRepository that replicates the
// spec's upsert-default-on-first-read behavior. It is NOT safe for concurrent
// use; tests are sequential.
type fakeProgressRepo struct {
	languages map[string]*models.LanguageProgress // key: userID|language
	profiles  map[string]*models.LearningProfile  // key: userID
}

func newFakeProgressRepo() *fakeProgressRepo {
	return &fakeProgressRepo{
		languages: make(map[string]*models.LanguageProgress),
		profiles:  make(map[string]*models.LearningProfile),
	}
}

func key(userID, language string) string { return userID + "|" + language }

// Compile-time check.
var _ ports.UserProgressRepository = (*fakeProgressRepo)(nil)

func (f *fakeProgressRepo) GetLanguageProgress(_ context.Context, userID, language string) (*models.LanguageProgress, error) {
	if p, ok := f.languages[key(userID, language)]; ok {
		return cloneProgress(p), nil
	}
	// Upsert default in-memory.
	def := defaultLanguageProgressForTest(userID, language)
	f.languages[key(userID, language)] = def
	return cloneProgress(def), nil
}

func (f *fakeProgressRepo) UpdateLanguageProgress(_ context.Context, p *models.LanguageProgress) error {
	f.languages[key(p.UserID, p.Language)] = cloneProgress(p)
	return nil
}

func (f *fakeProgressRepo) GetAllLanguageProgress(_ context.Context, userID string) ([]*models.LanguageProgress, error) {
	var out []*models.LanguageProgress
	for k, p := range f.languages {
		if len(k) >= len(userID)+1 && k[:len(userID)] == userID && k[len(userID)] == '|' {
			out = append(out, cloneProgress(p))
		}
	}
	return out, nil
}

func (f *fakeProgressRepo) GetLearningProfile(_ context.Context, userID string) (*models.LearningProfile, error) {
	if p, ok := f.profiles[userID]; ok {
		return p, nil
	}
	def := models.DefaultLearningProfile(userID)
	f.profiles[userID] = def
	return def, nil
}

func (f *fakeProgressRepo) UpdateLearningProfile(_ context.Context, p *models.LearningProfile) error {
	f.profiles[p.UserID] = p
	return nil
}

// cloneProgress shallow-copies a LanguageProgress, cloning the two slices.
func cloneProgress(p *models.LanguageProgress) *models.LanguageProgress {
	if p == nil {
		return nil
	}
	cp := *p
	if p.TopicsDominados != nil {
		cp.TopicsDominados = append([]string(nil), p.TopicsDominados...)
	}
	if p.CompletedChallengeIDs != nil {
		cp.CompletedChallengeIDs = append([]string(nil), p.CompletedChallengeIDs...)
	}
	if p.UltimoCompletado != nil {
		t := *p.UltimoCompletado
		cp.UltimoCompletado = &t
	}
	return &cp
}

// defaultLanguageProgressForTest mirrors the adapter's default row.
func defaultLanguageProgressForTest(userID, language string) *models.LanguageProgress {
	return &models.LanguageProgress{
		UserID:                userID,
		Language:              language,
		Rango:                 "F",
		Puntos:                0,
		ChallengesCompletados: 0,
		ChallengesIntentados:  0,
		TasaExito:             0,
		TopicsDominados:       []string{},
		CompletedChallengeIDs: []string{},
		RangoCodeHealth:       "Junior",
		PuntosCodeHealth:      0,
		ReposAnalizados:       0,
		IssuesEncontrados:     0,
	}
}

// seedProgress seeds the fake repo with an explicit row (used to test "existing
// row returned unchanged" + range accumulation scenarios).
func (f *fakeProgressRepo) seedProgress(p *models.LanguageProgress) {
	f.languages[key(p.UserID, p.Language)] = p
}

// --- Range boundary tests (pure functions) ---

func TestRangoFromPuntosFree_Boundaries(t *testing.T) {
	cases := []struct {
		puntos int
		want   string
	}{
		{0, "F"},
		{49, "F"},
		{50, "E"},
		{149, "E"},
		{150, "D"},
		{399, "D"},
		{400, "C"},
		{899, "C"},
		{900, "B"},
		{1999, "B"},
		{2000, "A"},
		{3999, "A"},
		{4000, "S"},
		{10000, "S"},
	}
	for _, c := range cases {
		if got := models.RangoFromPuntosFree(c.puntos); got != c.want {
			t.Errorf("RangoFromPuntosFree(%d) = %q, want %q", c.puntos, got, c.want)
		}
	}
}

func TestRangoFromPuntosCodeHealth_Boundaries(t *testing.T) {
	cases := []struct {
		puntos int
		want   string
	}{
		{0, "Junior"},
		{99, "Junior"},
		{100, "Mid"},
		{499, "Mid"},
		{500, "Senior"},
		{1999, "Senior"},
		{2000, "Architect"},
		{99999, "Architect"},
	}
	for _, c := range cases {
		if got := models.RangoFromPuntosCodeHealth(c.puntos); got != c.want {
			t.Errorf("RangoFromPuntosCodeHealth(%d) = %q, want %q", c.puntos, got, c.want)
		}
	}
}

// --- tasa_exito tests ---

func TestTasaExito_DivisionByZeroGuard(t *testing.T) {
	if got := tasaExito(0, 0); got != 0 {
		t.Errorf("tasaExito(0,0) = %v, want 0", got)
	}
	if got := tasaExito(5, 0); got != 0 {
		t.Errorf("tasaExito(5,0) = %v, want 0", got)
	}
}

func TestTasaExito_NormalRatios(t *testing.T) {
	cases := []struct {
		completados, intentados int
		want                    float64
	}{
		{0, 10, 0.0},
		{5, 10, 0.5},
		{10, 10, 1.0},
		{1, 3, 1.0 / 3.0},
	}
	for _, c := range cases {
		if got := tasaExito(c.completados, c.intentados); got != c.want {
			t.Errorf("tasaExito(%d,%d) = %v, want %v", c.completados, c.intentados, got, c.want)
		}
	}
}

// --- Median / global rank tests ---

func TestMedianOrdinal_OddEvenEmpty(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want int
	}{
		{"empty", []int{}, 0},
		{"single", []int{3}, 3},
		{"odd 3 (D,B,F)", []int{2, 4, 0}, 2},
		{"even 2 (C,B)", []int{3, 4}, 3},           // (3+4)/2 = 3 (floor)
		{"even 4 (F,F,S,S)", []int{0, 0, 6, 6}, 3}, // (0+6)/2 = 3
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := medianOrdinal(c.in); got != c.want {
				t.Errorf("medianOrdinal(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestGlobalRank_ViaService(t *testing.T) {
	ctx := context.Background()
	t.Run("no progress defaults to Junior", func(t *testing.T) {
		repo := newFakeProgressRepo()
		svc := NewUserProgressService(repo)
		got, err := svc.GetGlobalRank(ctx, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "Junior" {
			t.Errorf("GetGlobalRank(empty) = %q, want Junior", got)
		}
	})
	t.Run("median of 3 languages D,B,F -> D", func(t *testing.T) {
		repo := newFakeProgressRepo()
		repo.seedProgress(&models.LanguageProgress{UserID: "u1", Language: "typescript", Rango: "D"})
		repo.seedProgress(&models.LanguageProgress{UserID: "u1", Language: "python", Rango: "B"})
		repo.seedProgress(&models.LanguageProgress{UserID: "u1", Language: "rust", Rango: "F"})
		svc := NewUserProgressService(repo)
		got, err := svc.GetGlobalRank(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "D" {
			t.Errorf("GetGlobalRank(3 langs D,B,F) = %q, want D", got)
		}
	})
	t.Run("even 2 languages C,B -> C (floor)", func(t *testing.T) {
		repo := newFakeProgressRepo()
		repo.seedProgress(&models.LanguageProgress{UserID: "u1", Language: "go", Rango: "C"})
		repo.seedProgress(&models.LanguageProgress{UserID: "u1", Language: "typescript", Rango: "B"})
		svc := NewUserProgressService(repo)
		got, err := svc.GetGlobalRank(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "C" {
			t.Errorf("GetGlobalRank(2 langs C,B) = %q, want C", got)
		}
	})
}

// --- RecordAuditCompletion tests (service integration with fake repo) ---

func TestRecordAuditCompletion_DefaultRowCreatedOnFirstRead(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	// First read for a user/language that doesn't exist yet must upsert a
	// default row with rango=F, rangoCodeHealth=Junior, puntos=0.
	p, err := repo.GetLanguageProgress(ctx, "alice", "typescript")
	if err != nil {
		t.Fatalf("GetLanguageProgress: %v", err)
	}
	if p.Rango != "F" {
		t.Errorf("default rango = %q, want F", p.Rango)
	}
	if p.RangoCodeHealth != "Junior" {
		t.Errorf("default rangoCodeHealth = %q, want Junior", p.RangoCodeHealth)
	}
	if p.Puntos != 0 {
		t.Errorf("default puntos = %d, want 0", p.Puntos)
	}
	if p.ChallengesCompletados != 0 || p.ChallengesIntentados != 0 {
		t.Errorf("default counters = (%d/%d), want 0/0", p.ChallengesCompletados, p.ChallengesIntentados)
	}
	// Sanity: service was constructed and can record.
	if err := svc.RecordAuditCompletion(ctx, "alice", "typescript", "ch-1", 30, []string{"generics"}, true); err != nil {
		t.Fatalf("RecordAuditCompletion: %v", err)
	}
}

func TestRecordAuditCompletion_PointsAccumulateAcrossChallenges(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	if err := svc.RecordAuditCompletion(ctx, "bob", "rust", "ch-a", 30, nil, true); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := svc.RecordAuditCompletion(ctx, "bob", "rust", "ch-b", 20, nil, true); err != nil {
		t.Fatalf("second call: %v", err)
	}

	p, err := repo.GetLanguageProgress(ctx, "bob", "rust")
	if err != nil {
		t.Fatalf("GetLanguageProgress: %v", err)
	}
	if p.Puntos != 50 {
		t.Errorf("puntos = %d, want 50 (30+20)", p.Puntos)
	}
	if p.ChallengesCompletados != 2 {
		t.Errorf("completados = %d, want 2", p.ChallengesCompletados)
	}
	if p.ChallengesIntentados != 2 {
		t.Errorf("intentados = %d, want 2", p.ChallengesIntentados)
	}
	wantExito := 1.0 // 2/2
	if p.TasaExito != wantExito {
		t.Errorf("tasa_exito = %v, want %v", p.TasaExito, wantExito)
	}
	// 50 puntos crosses into E range.
	if p.Rango != "E" {
		t.Errorf("rango = %q, want E (50 puntos)", p.Rango)
	}
	if !containsString(p.CompletedChallengeIDs, "ch-a") || !containsString(p.CompletedChallengeIDs, "ch-b") {
		t.Errorf("completed_challenge_ids = %v, want both ch-a and ch-b", p.CompletedChallengeIDs)
	}
}

func TestRecordAuditCompletion_SameChallengeIDTwice_OnlyFirstCounts(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	// First completion: score=50.
	if err := svc.RecordAuditCompletion(ctx, "carol", "go", "ch-same", 50, []string{"goroutines"}, true); err != nil {
		t.Fatalf("first call: %v", err)
	}
	after1, err := repo.GetLanguageProgress(ctx, "carol", "go")
	if err != nil {
		t.Fatalf("get after 1: %v", err)
	}
	if after1.Puntos != 50 {
		t.Fatalf("puntos after 1st = %d, want 50", after1.Puntos)
	}

	// Second completion of the SAME challengeID with score=100.
	if err := svc.RecordAuditCompletion(ctx, "carol", "go", "ch-same", 100, []string{"goroutines", "channels"}, true); err != nil {
		t.Fatalf("second call: %v", err)
	}
	after2, err := repo.GetLanguageProgress(ctx, "carol", "go")
	if err != nil {
		t.Fatalf("get after 2: %v", err)
	}
	// puntos should NOT have increased.
	if after2.Puntos != 50 {
		t.Errorf("puntos after 2nd = %d, want 50 (no double-counting)", after2.Puntos)
	}
	// completados should NOT have increased.
	if after2.ChallengesCompletados != 1 {
		t.Errorf("completados after 2nd = %d, want 1", after2.ChallengesCompletados)
	}
	// intentados SHOULD have increased.
	if after2.ChallengesIntentados != 2 {
		t.Errorf("intentados after 2nd = %d, want 2", after2.ChallengesIntentados)
	}
	// tasa_exito should now be 1/2 = 0.5.
	if after2.TasaExito != 0.5 {
		t.Errorf("tasa_exito after 2nd = %v, want 0.5", after2.TasaExito)
	}
	// The second call's `learningObjectives` (channels) should NOT have been added
	// because the challenge was already completed.
	if containsString(after2.TopicsDominados, "channels") {
		t.Errorf("topics_dominados = %v, expected channels NOT to be added on duplicate", after2.TopicsDominados)
	}
	if !containsString(after2.TopicsDominados, "goroutines") {
		t.Errorf("topics_dominados = %v, expected goroutines to be present", after2.TopicsDominados)
	}
}

// TestRecordAuditCompletion_FailedAudit_OnlyIntentados verifies the spec's
// "Failed audit only increments intentados" scenario (Req 4). A failed audit
// (completed == false) MUST bump challenges_intentados and stamp
// ultimo_completado, but MUST NOT award puntos, bump challenges_completados,
// add topics, or touch the dedup array. A subsequent success on the same
// challengeID still earns full credit — the failed attempt must not poison dedup.
func TestRecordAuditCompletion_FailedAudit_OnlyIntentados(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	// A failed audit awards nothing but counts as an attempt.
	if err := svc.RecordAuditCompletion(ctx, "frank", "typescript", "ch-fail", 50, []string{"generics"}, false); err != nil {
		t.Fatalf("failed call: %v", err)
	}
	p, err := repo.GetLanguageProgress(ctx, "frank", "typescript")
	if err != nil {
		t.Fatalf("get after failure: %v", err)
	}
	if p.ChallengesIntentados != 1 {
		t.Errorf("intentados = %d, want 1 (a failed audit counts as an attempt)", p.ChallengesIntentados)
	}
	if p.Puntos != 0 {
		t.Errorf("puntos = %d, want 0 (failed audit awards no puntos)", p.Puntos)
	}
	if p.ChallengesCompletados != 0 {
		t.Errorf("completados = %d, want 0 (failed audit does not complete a challenge)", p.ChallengesCompletados)
	}
	if len(p.TopicsDominados) != 0 {
		t.Errorf("topics_dominados = %v, want empty (failed audit adds no topics)", p.TopicsDominados)
	}
	if containsString(p.CompletedChallengeIDs, "ch-fail") {
		t.Errorf("completed_challenge_ids = %v, expected ch-fail NOT to be added on failure", p.CompletedChallengeIDs)
	}
	if p.TasaExito != 0 {
		t.Errorf("tasa_exito = %v, want 0 (0 completados / 1 intentado)", p.TasaExito)
	}
	// ultimo_completado SHOULD be stamped on a failed attempt.
	if p.UltimoCompletado == nil {
		t.Errorf("ultimo_completado = nil, want a timestamp (failed attempts still stamp the attempt time)")
	}

	// A subsequent successful audit on the SAME challengeID must still earn
	// full credit — the failed attempt must not have poisoned the dedup array.
	if err := svc.RecordAuditCompletion(ctx, "frank", "typescript", "ch-fail", 50, []string{"generics"}, true); err != nil {
		t.Fatalf("success call: %v", err)
	}
	p2, err := repo.GetLanguageProgress(ctx, "frank", "typescript")
	if err != nil {
		t.Fatalf("get after success: %v", err)
	}
	if p2.Puntos != 50 {
		t.Errorf("puntos after success = %d, want 50 (failed attempt must not block credit)", p2.Puntos)
	}
	if p2.ChallengesCompletados != 1 {
		t.Errorf("completados after success = %d, want 1", p2.ChallengesCompletados)
	}
	if p2.ChallengesIntentados != 2 {
		t.Errorf("intentados after success = %d, want 2", p2.ChallengesIntentados)
	}
	if !containsString(p2.TopicsDominados, "generics") {
		t.Errorf("topics_dominados = %v, expected generics present after success", p2.TopicsDominados)
	}
	if !containsString(p2.CompletedChallengeIDs, "ch-fail") {
		t.Errorf("completed_challenge_ids = %v, expected ch-fail recorded after success", p2.CompletedChallengeIDs)
	}
	if p2.TasaExito != 0.5 {
		t.Errorf("tasa_exito = %v, want 0.5 (1 completado / 2 intentados)", p2.TasaExito)
	}
}

func TestRecordAuditCompletion_TopicsDedupWithinFirstCompletion(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	objectives := []string{"traits", "generics", "traits", "lifetimes"}
	if err := svc.RecordAuditCompletion(ctx, "dave", "rust", "ch-dup-topics", 40, objectives, true); err != nil {
		t.Fatalf("call: %v", err)
	}
	p, err := repo.GetLanguageProgress(ctx, "dave", "rust")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if count := countTopics(p.TopicsDominados, "traits"); count != 1 {
		t.Errorf("traits appears %d times in topics_dominados %v, want 1 (dedup)", count, p.TopicsDominados)
	}
	// Should have 3 unique topics: traits, generics, lifetimes.
	if len(p.TopicsDominados) != 3 {
		t.Errorf("topics_dominados length = %d, want 3 (got %v)", len(p.TopicsDominados), p.TopicsDominados)
	}
}

func TestAppendCappedFIFO_FIFOCapAt500(t *testing.T) {
	// Under cap: just appends.
	s := []string{"a", "b"}
	out := appendCappedFIFO(s, "c", 500)
	if len(out) != 3 || out[2] != "c" {
		t.Fatalf("under-cap append = %v, want [a b c]", out)
	}

	// At cap: evicts oldest (FIFO) before appending the new entry.
	cap := 3
	full := []string{"x", "y", "z"} // already at cap
	out = appendCappedFIFO(full, "w", cap)
	if len(out) != cap {
		t.Fatalf("length after capped append = %d, want %d", len(out), cap)
	}
	if out[0] != "y" {
		t.Errorf("first element after FIFO eviction = %q, want y (oldest x evicted)", out[0])
	}
	if out[cap-1] != "w" {
		t.Errorf("last element = %q, want w (newly appended)", out[cap-1])
	}
}

func TestRecordAuditCompletion_CompletedChallengeIDsCapAt500(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProgressRepo()
	svc := NewUserProgressService(repo)

	// Seed a row that already has 500 completed challenge IDs.
	existing := defaultLanguageProgressForTest("eve", "python")
	existing.CompletedChallengeIDs = make([]string, 0, MaxCompletedChallengeIDs)
	for i := 0; i < MaxCompletedChallengeIDs; i++ {
		existing.CompletedChallengeIDs = append(existing.CompletedChallengeIDs, idForIndex(i))
	}
	repo.seedProgress(existing)

	// Complete one brand-new challenge.
	if err := svc.RecordAuditCompletion(ctx, "eve", "python", "ch-new", 10, nil, true); err != nil {
		t.Fatalf("call: %v", err)
	}
	p, err := repo.GetLanguageProgress(ctx, "eve", "python")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(p.CompletedChallengeIDs) != MaxCompletedChallengeIDs {
		t.Fatalf("completed_challenge_ids length = %d, want %d (FIFO cap)",
			len(p.CompletedChallengeIDs), MaxCompletedChallengeIDs)
	}
	// The oldest entry (idForIndex(0)) should have been evicted.
	if containsString(p.CompletedChallengeIDs, idForIndex(0)) {
		t.Errorf("expected oldest entry %q to be evicted; got %v", idForIndex(0), p.CompletedChallengeIDs)
	}
	// The new entry should be at the tail.
	if p.CompletedChallengeIDs[len(p.CompletedChallengeIDs)-1] != "ch-new" {
		t.Errorf("expected ch-new at the tail; got %v", p.CompletedChallengeIDs)
	}
}

// --- helpers ---

func countTopics(s []string, v string) int {
	c := 0
	for _, item := range s {
		if item == v {
			c++
		}
	}
	return c
}

func idForIndex(i int) string {
	// Build a stable string ID per index (e.g. "id-0000").
	const digits = "0123456789"
	return "id-" + string(digits[(i/1000)%10]) + string(digits[(i/100)%10]) + string(digits[(i/10)%10]) + string(digits[i%10])
}
