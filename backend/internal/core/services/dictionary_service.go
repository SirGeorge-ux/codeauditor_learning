package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrDictionaryValidation is returned by DictionaryService when an input
// fails the per-field rules defined by the dictionary S2 spec:
//
//	term:        required, max 50 chars, lowercase
//	explanation: required, 10..500 chars
//	category:    required, MUST be in models.AllowedTermCategories
//
// The driving layer (HTTP handler, MCP tool wrapper) translates this into
// a 400 Bad Request with field-level error detail.
var ErrDictionaryValidation = errors.New("dictionary input validation failed")

// DictionaryService owns CRUD, validation, and lowercasing for the user's
// personal tech glossary. It depends on ports.DictionaryRepository for
// persistence — there is no SQL in this type.
//
// Hexagonal contract: this type imports ONLY core/domain/models, ports, and
// the standard library. It does NOT touch database/sql, lib/pq, or any
// HTTP / MCP SDK.
//
// ID generation lives in the adapter (PostgreSQL `gen_random_uuid()`
// default + RETURNING). The service does not pre-assign IDs so:
//   - the canonical source of truth for the ID is the DB column,
//   - tests do not need a UUID dependency in stdlib.
type DictionaryService struct {
	repo ports.DictionaryRepository
}

// NewDictionaryService constructs a DictionaryService backed by the given
// repository port.
func NewDictionaryService(repo ports.DictionaryRepository) *DictionaryService {
	return &DictionaryService{repo: repo}
}

// validateTerm enforces the per-field rules from the S2 spec.
//
// Returns ErrDictionaryValidation with a wrapped detail message; the caller
// must surface the detail to the user (handler converts to 400 + field error).
func validateTerm(term *models.Term) error {
	if term == nil {
		return fmt.Errorf("%w: nil term", ErrDictionaryValidation)
	}
	if strings.TrimSpace(term.Term) == "" {
		return fmt.Errorf("%w: term is required", ErrDictionaryValidation)
	}
	if len(term.Term) > 50 {
		return fmt.Errorf("%w: term must be <= 50 chars", ErrDictionaryValidation)
	}
	if len(term.Explanation) < 10 {
		return fmt.Errorf("%w: explanation must be >= 10 chars", ErrDictionaryValidation)
	}
	if len(term.Explanation) > 500 {
		return fmt.Errorf("%w: explanation must be <= 500 chars", ErrDictionaryValidation)
	}
	if !models.IsValidTermCategory(term.Category) {
		return fmt.Errorf("%w: category %q not in allowed list", ErrDictionaryValidation, term.Category)
	}
	for _, s := range term.Synonyms {
		if len(s) > 50 {
			return fmt.Errorf("%w: synonym %q must be <= 50 chars", ErrDictionaryValidation, s)
		}
	}
	for _, tag := range term.Tags {
		if len(tag) > 30 {
			return fmt.Errorf("%w: tag %q must be <= 30 chars", ErrDictionaryValidation, tag)
		}
	}
	return nil
}

// AddTermInput is the validated input shape consumed by AddTerm. The
// service layer lowercases the Term before persisting so the spec's unique
// constraint catches case-only duplicates ("PR" == "pr").
type AddTermInput struct {
	UserID      string
	Term        string
	Explanation string
	Category    string
	Example     *string
	Synonyms    []string
	Tags        []string
}

// toModel projects the validated input into a freshly-allocated models.Term
// with a lowercased Term. The ID is intentionally empty — the adapter fills
// it via the SQL `gen_random_uuid()` default + RETURNING clause.
func (in AddTermInput) toModel() *models.Term {
	out := &models.Term{
		Term:        strings.ToLower(strings.TrimSpace(in.Term)),
		Explanation: in.Explanation,
		Category:    in.Category,
		Example:     in.Example,
		Synonyms:    in.Synonyms,
		Tags:        in.Tags,
	}
	if out.Synonyms == nil {
		out.Synonyms = []string{}
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	return out
}

// AddTerm validates, normalizes, and persists a new term. The userID is
// always taken from the authenticated context (the input field mirrors it
// for clarity); the adapter overrides the term's UserID with the same
// value. Returns ErrDictionaryValidation or ports.ErrTermConflict for the
// spec's documented error paths.
func (s *DictionaryService) AddTerm(ctx context.Context, in AddTermInput) (*models.Term, error) {
	if strings.TrimSpace(in.UserID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrDictionaryValidation)
	}
	term := in.toModel()
	if err := validateTerm(term); err != nil {
		return nil, err
	}
	return s.repo.InsertTerm(ctx, in.UserID, term)
}

// GetTerm returns a single term by id, scoped to userID. Returns
// ports.ErrTermNotFound when the row does not exist (or belongs to a
// different user, indistinguishable from "missing" so we never leak
// information across tenant boundaries).
func (s *DictionaryService) GetTerm(ctx context.Context, userID, termID string) (*models.Term, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(termID) == "" {
		return nil, fmt.Errorf("%w: user_id and term_id required", ErrDictionaryValidation)
	}
	return s.repo.GetTerm(ctx, userID, termID)
}

// UpdateTermInput is the partial-update shape. Term and category are
// re-validated by the service so the DB cannot end up holding rows that
// violate the spec's CHECK constraints even if the schema is migrated.
type UpdateTermInput struct {
	Term        string
	Explanation string
	Category    string
	Example     *string
	Synonyms    []string
	Tags        []string
}

// UpdateTerm applies the partial replacement to an existing term. Returns
// ports.ErrTermNotFound if the term does not belong to userID.
func (s *DictionaryService) UpdateTerm(ctx context.Context, userID, termID string, in UpdateTermInput) (*models.Term, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(termID) == "" {
		return nil, fmt.Errorf("%w: user_id and term_id required", ErrDictionaryValidation)
	}
	candidate := &models.Term{
		ID:          termID,
		Term:        strings.ToLower(strings.TrimSpace(in.Term)),
		Explanation: in.Explanation,
		Category:    in.Category,
		Example:     in.Example,
		Synonyms:    in.Synonyms,
		Tags:        in.Tags,
	}
	if candidate.Synonyms == nil {
		candidate.Synonyms = []string{}
	}
	if candidate.Tags == nil {
		candidate.Tags = []string{}
	}
	if err := validateTerm(candidate); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTerm(ctx, userID, candidate); err != nil {
		return nil, err
	}
	return s.repo.GetTerm(ctx, userID, termID)
}

// DeleteTerm removes a term by id, scoped to userID. Returns
// ports.ErrTermNotFound if the row does not exist.
func (s *DictionaryService) DeleteTerm(ctx context.Context, userID, termID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(termID) == "" {
		return fmt.Errorf("%w: user_id and term_id required", ErrDictionaryValidation)
	}
	return s.repo.DeleteTerm(ctx, userID, termID)
}

// ListTerms returns every term for userID, optionally filtered by category
// and / or single tag.
func (s *DictionaryService) ListTerms(ctx context.Context, userID, category, tag string) ([]*models.Term, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrDictionaryValidation)
	}
	return s.repo.ListTerms(ctx, userID, category, tag)
}

// SearchTerms returns up to ports.MaxSearchResults ranked by ts_rank on
// the tables' generated search_vector. An empty query falls through to
// ListTerms. The service-layer cap is a safety belt on top of the repo
// LIMIT — the adapter may over-return if the user has millions of exact
// matches (unlikely at 1K terms but the spec mandates bounded responses).
func (s *DictionaryService) SearchTerms(ctx context.Context, userID, query string) ([]*models.Term, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrDictionaryValidation)
	}
	out, err := s.repo.SearchTerms(ctx, userID, query)
	if err != nil {
		return nil, err
	}
	if len(out) > ports.MaxSearchResults {
		out = out[:ports.MaxSearchResults]
	}
	return out, nil
}
