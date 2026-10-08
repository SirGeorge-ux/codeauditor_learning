package supabase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time check: SupabaseDictionaryRepository implements
// ports.DictionaryRepository.
var _ ports.DictionaryRepository = (*SupabaseDictionaryRepository)(nil)

// pqUniqueViolation is the SQLSTATE for a unique-constraint violation in
// PostgreSQL (lib/pq exposes it through *pq.Error.Code).
const pqUniqueViolation = "23505"

// SupabaseDictionaryRepository is the Supabase/PostgreSQL adapter for the
// DictionaryRepository port. It owns all SQL and array marshaling concerns.
type SupabaseDictionaryRepository struct {
	db *sql.DB
}

// NewSupabaseDictionaryRepository constructs the adapter backed by the given
// database/sql connection.
func NewSupabaseDictionaryRepository(db *sql.DB) *SupabaseDictionaryRepository {
	return &SupabaseDictionaryRepository{db: db}
}

// dictionarySelectColumns is the projection used by every read path so the
// scan function is the only place column order is decided.
const dictionarySelectColumns = `id, user_id, term, explanation, category,
	example, synonyms, tags, created_at, updated_at`

// dictionarySelectPrefix is the FROM-and-onward fragment shared by every read
// query. Kept in sync with dictionarySelectColumns.
const dictionarySelectPrefix = `SELECT ` + dictionarySelectColumns + `
	FROM public.dictionary_terms`

// scanTerm maps a SQL row into a models.Term, normalizing nil arrays to
// empty (non-nil) slices so the JSON wire shape is always ["..."] not null.
func scanTerm(scanner interface{ Scan(dest ...any) error }) (*models.Term, error) {
	var (
		term      models.Term
		example   sql.NullString
		synonyms  pq.StringArray
		tags      pq.StringArray
		createdAt sql.NullTime
		updatedAt sql.NullTime
	)
	err := scanner.Scan(
		&term.ID, &term.UserID, &term.Term, &term.Explanation, &term.Category,
		&example, &synonyms, &tags, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if example.Valid {
		v := example.String
		term.Example = &v
	}
	if synonyms == nil {
		term.Synonyms = []string{}
	} else {
		term.Synonyms = []string(synonyms)
	}
	if tags == nil {
		term.Tags = []string{}
	} else {
		term.Tags = []string(tags)
	}
	if createdAt.Valid {
		term.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		term.UpdatedAt = updatedAt.Time
	}
	return &term, nil
}

// normalizeTermFill fills optional Term fields to non-nil empties + a
// nil-safe Example pointer so the SQL adapter can pass them cleanly to
// pq.Array. The helper does NOT mutate the caller's struct — it builds the
// three SQL-facing values reused by both the INSERT and UPDATE paths.
func normalizeTermFill(t *models.Term) (example *string, synonyms, tags []string) {
	if t.Example != nil {
		v := *t.Example
		example = &v
	}
	if t.Synonyms == nil {
		synonyms = []string{}
	} else {
		synonyms = t.Synonyms
	}
	if t.Tags == nil {
		tags = []string{}
	} else {
		tags = t.Tags
	}
	return example, synonyms, tags
}

// isUniqueViolation reports whether err is a Postgres unique-violation error.
// The adapter uses this to translate a DB-level conflict into the port's
// ErrTermConflict sentinel so the service / driving layer can map it to 409.
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == pqUniqueViolation
	}
	return false
}

// InsertTerm creates a new dictionary entry for userID. Returns
// ports.ErrTermConflict when the unique(user_id, term) constraint rejects
// the row.
func (r *SupabaseDictionaryRepository) InsertTerm(ctx context.Context, userID string, term *models.Term) (*models.Term, error) {
	example, synonyms, tags := normalizeTermFill(term)

	const query = `INSERT INTO public.dictionary_terms (user_id, term, explanation, category, example, synonyms, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + dictionarySelectColumns

	row := r.db.QueryRowContext(
		ctx, query,
		userID, term.Term, term.Explanation, term.Category,
		example, pq.Array(synonyms), pq.Array(tags),
	)
	out, err := scanTerm(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ports.ErrTermConflict
		}
		return nil, fmt.Errorf("insert dictionary term: %w", err)
	}
	return out, nil
}

// GetTerm loads a single term by id scoped to userID. Returns
// ports.ErrTermNotFound when no row matches.
func (r *SupabaseDictionaryRepository) GetTerm(ctx context.Context, userID, termID string) (*models.Term, error) {
	const query = dictionarySelectPrefix + `
		WHERE id = $1 AND user_id = $2`
	row := r.db.QueryRowContext(ctx, query, termID, userID)
	out, err := scanTerm(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrTermNotFound
		}
		return nil, fmt.Errorf("get dictionary term: %w", err)
	}
	return out, nil
}

// UpdateTerm applies mutable fields of term by id, scoped to userID. Returns
// ports.ErrTermNotFound when no row matches.
func (r *SupabaseDictionaryRepository) UpdateTerm(ctx context.Context, userID string, term *models.Term) error {
	example, synonyms, tags := normalizeTermFill(term)

	const query = `UPDATE public.dictionary_terms
		SET term = $1,
		    explanation = $2,
		    category = $3,
		    example = $4,
		    synonyms = $5,
		    tags = $6,
		    updated_at = NOW()
		WHERE id = $7 AND user_id = $8`

	res, err := r.db.ExecContext(
		ctx, query,
		term.Term, term.Explanation, term.Category,
		example, pq.Array(synonyms), pq.Array(tags),
		term.ID, userID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ports.ErrTermConflict
		}
		return fmt.Errorf("update dictionary term: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update dictionary term rows-affected: %w", err)
	}
	if n == 0 {
		return ports.ErrTermNotFound
	}
	return nil
}

// DeleteTerm removes a term by id scoped to userID. Returns
// ports.ErrTermNotFound when no row matches.
func (r *SupabaseDictionaryRepository) DeleteTerm(ctx context.Context, userID, termID string) error {
	const query = `DELETE FROM public.dictionary_terms
		WHERE id = $1 AND user_id = $2`

	res, err := r.db.ExecContext(ctx, query, termID, userID)
	if err != nil {
		return fmt.Errorf("delete dictionary term: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete dictionary term rows-affected: %w", err)
	}
	if n == 0 {
		return ports.ErrTermNotFound
	}
	return nil
}

// ListTerms returns terms for userID ordered alphabetically. Optional
// category and tag narrow the result set. An empty (no filter) request
// returns all terms; category == "" and tag == "" both bypass the WHERE
// clause additions.
func (r *SupabaseDictionaryRepository) ListTerms(ctx context.Context, userID, category, tag string) ([]*models.Term, error) {
	args := []any{userID}
	whereByUser := "user_id = $1"
	if category != "" {
		whereByUser += " AND category = $2"
		args = append(args, category)
	}
	if tag != "" {
		// "$N = ANY(tags)" matches when the tag array contains the value.
		idx := len(args) + 1
		whereByUser += fmt.Sprintf(" AND $%d = ANY(tags)", idx)
		args = append(args, tag)
	}

	query := dictionarySelectPrefix + "\n\t\tWHERE " + whereByUser + "\n\t\tORDER BY lower(term) ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dictionary terms: %w", err)
	}
	defer rows.Close()

	out := []*models.Term{}
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dictionary term row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dictionary term rows: %w", err)
	}
	return out, nil
}

// buildPrefixTSQuery translates a free-text search string into a Postgres
// `to_tsquery` expression where every word carries a `:*` prefix operator.
// This matches the spec scenario "Search matches term name — `hard` finds
// `hardcode`" while keeping the GIN-backed index in play (lexeme != substring).
//
// Example inputs:
//
//	"hard"        -> "hard:*"
//	"lexical"     -> "lexical:*"
//	"lexical sc"  -> "lexical:* & sc:*"
//	"" | "  ?"    -> ""  (caller treats empty as ListTerms fallback)
//
// Tokens that contain characters Postgres' tsquery parser rejects (& | ! ( ))
// are dropped so a search like "c++" becomes a no-op rather than a syntax
// error; callers see an empty result list, not a 500. Spec target: <200ms
// even on 1K rows because the GIN index still drives the scan.
func buildPrefixTSQuery(query string) string {
	tokens := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return false
		}
		return true
	})
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, t+":*")
	}
	return strings.Join(parts, " & ")
}

// SearchTerms runs a ranked full-text search using Postgres tsvector +
// to_tsquery('english', $2 || ':*') for word-prefix matching. Capped at
// ports.MaxSearchResults (20). An empty query falls through to ListTerms
// for a complete browse view, matching the spec scenario "Empty query
// returns all terms".
func (r *SupabaseDictionaryRepository) SearchTerms(ctx context.Context, userID, query string) ([]*models.Term, error) {
	if query == "" {
		return r.ListTerms(ctx, userID, "", "")
	}

	tsquery := buildPrefixTSQuery(query)
	if tsquery == "" {
		// Empty tsquery (e.g. user typed only punctuation) — no lexemes to
		// rank. Return an empty list rather than execute a query that would
		// either syntax-error or scan every row.
		return []*models.Term{}, nil
	}

	const sql = dictionarySelectPrefix + `
		WHERE user_id = $1
		  AND search_vector @@ to_tsquery('english', $2)
		ORDER BY ts_rank(search_vector, to_tsquery('english', $2)) DESC
		LIMIT $3`

	rows, err := r.db.QueryContext(ctx, sql, userID, tsquery, ports.MaxSearchResults)
	if err != nil {
		return nil, fmt.Errorf("search dictionary terms: %w", err)
	}
	defer rows.Close()

	out := []*models.Term{}
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dictionary term row: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dictionary term rows: %w", err)
	}
	return out, nil
}
