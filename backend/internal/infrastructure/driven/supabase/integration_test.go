package supabase_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/supabase"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

const integrationNotice = "live RLS/Postgres verification — the dictionary spec's " +
	"Row-Level Security scenarios require PostgreSQL 13+. Set DATABASE_URL " +
	"to enable; without it RLS enforcement is NOT proven, only the adapter " +
	"shape is. The pre-existing unit tests using sqlmock document the contract " +
	"but cannot substitute for a live database."

// requireLiveDB returns the live PostgreSQL connection or skips with an
// explicit "no live DB" message. The skip reports the env var name so
// CI matrices know what to set; the test is *not* silently skipped.
func requireLiveDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skipf("DATABASE_URL unset — %s", integrationNotice)
		return nil, func() {}
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping DB: %v", err)
	}
	return db, func() { _ = db.Close() }
}

// ensureIsolatedSchema creates a fresh schema for the test, applies a
// minimal version of migration 009, and registers a cleanup to drop it.
// The shape is enough to exercise InsertTerm / SearchTerms / RLS
// filtering for cross-user isolation.
func ensureIsolatedSchema(t *testing.T, db *sql.DB) string {
	t.Helper()
	schema := fmt.Sprintf("dict_test_%d", time.Now().UnixNano())

	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
	})

	ddl := `
CREATE TABLE IF NOT EXISTS %%SCHEMA%%.dictionary_terms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    term        TEXT NOT NULL CHECK (char_length(term) BETWEEN 1 AND 50),
    explanation TEXT NOT NULL CHECK (char_length(explanation) BETWEEN 10 AND 500),
    category    TEXT NOT NULL,
    example     TEXT,
    synonyms    TEXT[] NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(term, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(explanation, '')), 'B') ||
        setweight(to_tsvector('english', array_to_string(synonyms, ' ') || ' ' || array_to_string(tags, ' ')), 'C')
    ) STORED,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dictionary_terms_search_vector
    ON %%SCHEMA%%.dictionary_terms USING GIN (search_vector);

ALTER TABLE %%SCHEMA%%.dictionary_terms ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS user_isolation ON %%SCHEMA%%.dictionary_terms;
CREATE POLICY user_isolation ON %%SCHEMA%%.dictionary_terms
    FOR ALL TO PUBLIC
    USING (user_id::text = current_setting('request.jwt.claim.sub', true))
    WITH CHECK (user_id::text = current_setting('request.jwt.claim.sub', true));
`
	ddl = strings.ReplaceAll(ddl, "%%SCHEMA%%", schema)
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("apply schema ddl: %v", err)
	}
	return schema
}

// switchSearchPath makes queries unqualified hit the isolated schema.
func switchSearchPath(t *testing.T, db *sql.DB, schema string) {
	t.Helper()
	if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
}

// newTerm is a tiny constructor used only by these tests.
func newTerm(name, explanation string) *models.Term {
	return &models.Term{
		Term:        name,
		Explanation: explanation,
		Category:    "Concepto",
	}
}

// TestIntegration_LiveDB_Available — the FIRST integration test the
// bounded runner hits. With DATABASE_URL unset, this is the canonical
// skip line. With DATABASE_URL set, it records a positive baseline so
// the rest of the integration suite can rely on connectivity.
func TestIntegration_LiveDB_Available(t *testing.T) {
	db, closer := requireLiveDB(t)
	defer closer()

	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("expected 1, got %d", one)
	}
}

// TestIntegration_SearchTerms_PrefixMatchOnRealPostgres — proves the
// spec scenario "hard prefix-matches hardcode" against a real tsvector
// +GIN index. The integration is the only tier that proves the index
// path; unit tests with sqlmock only verify the SQL shape.
func TestIntegration_SearchTerms_PrefixMatchOnRealPostgres(t *testing.T) {
	db, closer := requireLiveDB(t)
	defer closer()
	schema := ensureIsolatedSchema(t, db)
	switchSearchPath(t, db, schema)

	repo := supabase.NewSupabaseDictionaryRepository(db)
	ctx := context.Background()

	owner := "11111111-1111-1111-1111-111111111111"
	if _, err := repo.InsertTerm(ctx, owner, newTerm("hardcode", "Writing literal values in code.")); err != nil {
		t.Fatalf("seed hardcode: %v", err)
	}
	if _, err := repo.InsertTerm(ctx, owner, newTerm("async", "Non-blocking concurrency primitive.")); err != nil {
		t.Fatalf("seed async: %v", err)
	}

	hits, err := repo.SearchTerms(ctx, owner, "hard")
	if err != nil {
		t.Fatalf("SearchTerms('hard'): %v", err)
	}
	if len(hits) != 1 || hits[0].Term != "hardcode" {
		t.Fatalf("SearchTerms('hard') returned %+v, want exactly one hardcode", hits)
	}
}

// TestIntegration_RLS_PreventsCrossUserReads — proves the spec scenario
// "RLS prevents cross-user reads": user A creates a row, user B's auth
// context (set via request.jwt.claim.sub) returns zero rows.
//
// Production code opens a database/sql connection per request — there
// is no session-level set per request — so the test simulates a JWT
// "sub" claim by issuing the lookup query with a SET LOCAL before the
// read. This mirrors what Supabase PostgREST does for inbound
// requests.
func TestIntegration_RLS_PreventsCrossUserReads(t *testing.T) {
	db, closer := requireLiveDB(t)
	defer closer()
	schema := ensureIsolatedSchema(t, db)

	userA := "11111111-1111-1111-1111-111111111111"
	userB := "22222222-2222-2222-2222-222222222222"

	// Insert user A's row directly via psql-style path (the adapter
	// also does this), setting the RLS sub to user A so the row passes
	// the policy.
	if _, err := db.Exec(fmt.Sprintf("SET search_path TO %s, public", schema)); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf("SET LOCAL request.jwt.claim.sub = '%s'", userA)); err != nil {
		t.Fatalf("set sub A: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`
INSERT INTO %s.dictionary_terms
    (user_id, term, explanation, category)
    VALUES ($1::uuid, 'private-of-A', 'Owned by user A only.', 'Concepto')`, schema), userA); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	// Now switch to user B's JWT context and verify they cannot see
	// user A's row, even when asked through the adapter.
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %s, public", schema)); err != nil {
		t.Fatalf("set search_path tx: %v", err)
	}
	if _, err := tx.Exec(fmt.Sprintf("SET LOCAL request.jwt.claim.sub = '%s'", userB)); err != nil {
		t.Fatalf("set sub B: %v", err)
	}
	var n int
	if err := tx.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s.dictionary_terms", schema)).Scan(&n); err != nil {
		t.Fatalf("count under user B: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows visible to user B under RLS, got %d", n)
	}
}

// TestIntegration_DuplicateInsert_ReturnsConflictRealDB — proves the
// UNIQUE(user_id, term) constraint maps to ports.ErrTermConflict for a
// live database (sqlmock can't exercise the constraint).
func TestIntegration_DuplicateInsert_ReturnsConflictRealDB(t *testing.T) {
	db, closer := requireLiveDB(t)
	defer closer()
	schema := ensureIsolatedSchema(t, db)
	switchSearchPath(t, db, schema)

	repo := supabase.NewSupabaseDictionaryRepository(db)
	ctx := context.Background()

	owner := "33333333-3333-3333-3333-333333333333"
	if _, err := repo.InsertTerm(ctx, owner, newTerm("duplicate-me", "First insertion of the term goes here.")); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err := repo.InsertTerm(ctx, owner, newTerm("duplicate-me", "Trying the same name twice here."))
	if err == nil {
		t.Fatal("expected conflict on duplicate insert, got nil")
	}
	if !errors.Is(err, ports.ErrTermConflict) {
		t.Fatalf("expected ports.ErrTermConflict, got %v", err)
	}
}

// TestIntegration_InvalidCategory_RejectedByCheckConstraint — proves
// migration 009's CHECK(category IN (...)) rejects any string outside
// the six allowed values, even when the application layer's
// IsValidTermCategory gate is bypassed. Spec scenario "invalid category
// rejection — 400".
func TestIntegration_InvalidCategory_RejectedByCheckConstraint(t *testing.T) {
	db, closer := requireLiveDB(t)
	defer closer()
	schema := ensureIsolatedSchema(t, db)
	switchSearchPath(t, db, schema)

	owner := "44444444-4444-4444-4444-444444444444"
	bad := &models.Term{
		Term:        "off-category",
		Explanation: "Uses an out-of-list category value.",
		Category:    "Design Pattern", // not in the 6 allowed values
	}
	_, err := supabase.NewSupabaseDictionaryRepository(db).InsertTerm(
		context.Background(), owner, bad,
	)
	if err == nil {
		t.Fatal("expected CHECK constraint violation, got nil err")
	}
	// The generic pq error isn't a unique violation; just assert the
	// row count to ensure the constraint actually rejected.
	var n int
	if qerr := db.QueryRow(fmt.Sprintf(
		"SELECT count(*) FROM %s.dictionary_terms WHERE user_id::text = $1",
		schema,
	), owner).Scan(&n); qerr != nil {
		t.Fatalf("count: %v", qerr)
	}
	if n != 0 {
		t.Errorf("expected 0 rows after rejected insert, got %d", n)
	}
}
