package ports

import (
	"context"
	"errors"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrTermNotFound is returned by DictionaryRepository operations when the
// requested (userID, termID) pair does not exist. Service / driving layers
// translate this into a 404 Not Found without leaking internal SQL details.
var ErrTermNotFound = errors.New("dictionary term not found")

// ErrTermConflict is returned by InsertTerm when the call would violate the
// unique(user_id, lower(term)) constraint. Service / driving layers
// translate this into a 409 Conflict, matching the spec's "Duplicate term
// rejection" scenario.
var ErrTermConflict = errors.New("dictionary term already exists for user")

// DictionaryRepository is the driven (secondary) port that persists the
// user's personal tech dictionary. Implementations live in
// infrastructure/driven (e.g. Supabase).
//
// Per the dictionary S2 spec, all read paths MUST scope results to userID so
// adapter implementations enforce RLS / filter-by-user_id at the SQL layer.
// The service layer is then free to assume "if it came back, it belongs to
// me".
type DictionaryRepository interface {
	// InsertTerm creates a new dictionary entry for userID. Returns
	// ErrTermConflict if a term with the same lowercase name already exists
	// for that user. The Term.ID and timestamps are filled by the adapter
	// using SQL defaults / NOW(); Term.UserID is overridden with userID.
	InsertTerm(ctx context.Context, userID string, term *models.Term) (*models.Term, error)

	// GetTerm loads a single term by id, scoped to userID. Returns
	// ErrTermNotFound if no row matches.
	GetTerm(ctx context.Context, userID, termID string) (*models.Term, error)

	// UpdateTerm applies an existing term's mutable fields (term,
	// explanation, category, example, synonyms, tags, updated_at) by id,
	// scoped to userID. Returns ErrTermNotFound if no row matches.
	UpdateTerm(ctx context.Context, userID string, term *models.Term) error

	// DeleteTerm removes the term by id, scoped to userID. Returns
	// ErrTermNotFound if no row matches.
	DeleteTerm(ctx context.Context, userID, termID string) error

	// ListTerms returns terms belonging to userID, ordered alphabetically
	// by lower(term). Optional category / tag filters narrow the result set.
	// The empty-list case is not an error.
	ListTerms(ctx context.Context, userID, category, tag string) ([]*models.Term, error)

	// SearchTerms runs a ranked full-text search over Term, Explanation,
	// Synonyms and Tags for the given userID. query is matched against the
	// generated tsvector column. Returns at most MaxSearchResults (cap 20)
	// ranked by ts_rank desc. An empty query falls through to ListTerms so
	// the caller gets a complete browse view.
	SearchTerms(ctx context.Context, userID, query string) ([]*models.Term, error)
}

// MaxSearchResults caps the ranked full-text search hit count to keep the
// MCP tutor's response bounded.
const MaxSearchResults = 20
