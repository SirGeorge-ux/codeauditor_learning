package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrChallengeNotFound is re-exported here for backward compatibility
// with callers (HTTP handlers, MCP adapters) that historically
// branch against services.ErrChallengeNotFound. The canonical
// definition lives in ports so the driven adapter can return it
// directly without a translation layer.
var ErrChallengeNotFound = ports.ErrChallengeNotFound

// ErrInvalidDifficulty is returned when a challenge has an invalid difficulty level.
var ErrInvalidDifficulty = errors.New("invalid difficulty: must be one of junior, mid, senior, architect")

// ErrInvalidExpectedFindings is returned when expected_findings count is outside 1-3.
var ErrInvalidExpectedFindings = errors.New("expected_findings must have 1-3 items")

// ErrInvalidTestCases is returned when test_cases count is outside 2-4.
var ErrInvalidTestCases = errors.New("test_cases must have 2-4 items")

// ErrInvalidHintsCount is returned when hints count is not exactly 3.
var ErrInvalidHintsCount = errors.New("exactly 3 hints required")

// ErrSpoilerInDescription is returned when the description contains spoiler keywords.
var ErrSpoilerInDescription = errors.New("description contains spoiler content")

// ErrChallengeGenerateNotImplemented is returned by ChallengeService.GenerateChallenge
// until the S5 LLM cascade lands. The MCP mcp-challenges.generate_challenge
// tool surfaces this sentinel so the LLM caller gets an actionable
// "enable via S5 cascade" hint instead of a stack trace. Keeping the
// sentinel here (not in the MCP adapter) lets future callers — HTTP
// handlers, other transport adapters — branch with errors.Is without
// importing the MCP layer.
var ErrChallengeGenerateNotImplemented = errors.New("challenge generation not implemented (S5 LLM cascade)")

// validDifficulties contains allowed difficulty values.
var validDifficulties = map[string]bool{
	"junior":    true,
	"mid":       true,
	"senior":    true,
	"architect": true,
}

// spoilerKeywords is a deny-list of smell/vulnerability names that MUST NOT
// appear in a challenge description. The check is case-insensitive.
var spoilerKeywords = []string{
	"sql injection", "xss", "cross-site scripting", "god function",
	"callback hell", "prop mutation", "dead code", "silent failure",
	"poor naming", "n+1", "race condition", "memory leak",
}

// validateChallengeV2 checks v2 model constraints on a CreateChallengeInput.
// It MUST be called before persisting a new challenge.
func validateChallengeV2(input models.CreateChallengeInput) error {
	// expected_findings: 1-3 items
	if len(input.ExpectedFindings) < 1 || len(input.ExpectedFindings) > 3 {
		return ErrInvalidExpectedFindings
	}
	// test_cases: 2-4 items
	if len(input.TestCases) < 2 || len(input.TestCases) > 4 {
		return ErrInvalidTestCases
	}
	// hints: exactly 3 items
	if len(input.Hints) != 3 {
		return ErrInvalidHintsCount
	}
	// description: no spoiler keywords (case-insensitive)
	lowerDesc := strings.ToLower(input.Description)
	for _, kw := range spoilerKeywords {
		if strings.Contains(lowerDesc, kw) {
			return ErrSpoilerInDescription
		}
	}
	return nil
}

// IsValidationError returns true if the error is one of the known challenge
// validation errors. Used by the HTTP handler to decide 400 vs 500.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidDifficulty) ||
		errors.Is(err, ErrInvalidExpectedFindings) ||
		errors.Is(err, ErrInvalidTestCases) ||
		errors.Is(err, ErrInvalidHintsCount) ||
		errors.Is(err, ErrSpoilerInDescription)
}

// ChallengeService retrieves and creates challenges through the
// ChallengeRepository port. The service owns business rules
// (validation, normalization, dedup decision, response shaping);
// the port owns the SQL.
type ChallengeService struct {
	repo ports.ChallengeRepository
}

// NewChallengeService creates a new ChallengeService backed by the
// given repository. The repository MUST be non-nil; nil-safe wiring
// is the wiring layer's job (e.g. cmd/api/wiring_test.go passes nil
// to verify registry count without invoking any tool).
func NewChallengeService(repo ports.ChallengeRepository) *ChallengeService {
	return &ChallengeService{repo: repo}
}

// generateUUID generates a version 4 UUID using crypto/rand.
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// Create inserts a new challenge or returns the existing one if
// source_repo+user_id already exists. Returns (challenge, created,
// nil) where created is true for new inserts and false for dedup
// hits.
func (s *ChallengeService) Create(ctx context.Context, input models.CreateChallengeInput, userID string) (*models.Challenge, bool, error) {
	// Validate difficulty
	if !validDifficulties[input.Difficulty] {
		return nil, false, ErrInvalidDifficulty
	}

	// Validate v2 model constraints (expected_findings, test_cases,
	// hints, no spoilers)
	if err := validateChallengeV2(input); err != nil {
		return nil, false, err
	}

	// Normalize source_repo for dedup (lowercase + trim whitespace).
	normalizedRepo := strings.ToLower(strings.TrimSpace(input.SourceRepo))

	// Check for duplicate — the port returns ErrChallengeNotFound on
	// miss (not sql.ErrNoRows), keeping the SQL detail out of core.
	existing, err := s.repo.FindBySourceRepoAndUser(ctx, normalizedRepo, userID)
	if err == nil {
		// Found existing challenge (dedup hit)
		return &existing, false, nil
	}
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		return nil, false, fmt.Errorf("checking for duplicate challenge: %w", err)
	}

	// No duplicate — generate UUID, then insert.
	id, err := generateUUID()
	if err != nil {
		return nil, false, fmt.Errorf("generating UUID: %w", err)
	}

	status := "available"

	// Build the persisted model. SourceRepo is already normalized;
	// the adapter stores exactly what we pass.
	challenge := &models.Challenge{
		ID:                   id,
		Title:                input.Title,
		Description:          input.Description,
		Difficulty:           input.Difficulty,
		Category:             input.Category,
		Language:             input.Language,
		RepoURL:              input.RepoURL,
		Code:                 input.Code,
		CodeSmell:            input.CodeSmell,
		Status:               status,
		UserID:               &userID,
		SourceRepo:           normalizedRepo,
		LearningObjectives:   input.LearningObjectives,
		Hints:                input.Hints,
		CommonMistakes:       input.CommonMistakes,
		EstimatedTimeMinutes: input.EstimatedTimeMinutes,
		ExpectedFindings:     input.ExpectedFindings,
		TestCases:            input.TestCases,
		LinterRules:          input.LinterRules,
		SolutionCode:         input.SolutionCode,
		SolutionExplanation:  input.SolutionExplanation,
		BasePoints:           input.BasePoints,
		BonusPoints:          input.BonusPoints,
		PenaltyPerHint:       input.PenaltyPerHint,
		TimeBonus:            input.TimeBonus,
		Origin:               input.Origin,
		SourcePath:           input.SourcePath,
		GeneratedBy:          input.GeneratedBy,
		CreatedBy:            input.CreatedBy,
	}

	createdAt, err := s.repo.Insert(ctx, challenge)
	if err != nil {
		return nil, false, fmt.Errorf("inserting challenge: %w", err)
	}
	challenge.CreatedAt = createdAt
	return challenge, true, nil
}

// GetAll returns all challenges visible to the given user: seeds
// (NULL user_id) plus user-owned. Solution fields (solution_code,
// solution_explanation) are NOT included in the response.
func (s *ChallengeService) GetAll(ctx context.Context, userID string) ([]models.Challenge, error) {
	return s.repo.ListVisibleToUser(ctx, userID)
}

// GetByID returns a single challenge by ID visible to the given user,
// or ErrChallengeNotFound. Solution fields are NOT included — use
// GetChallengeWithDetails for the full challenge.
func (s *ChallengeService) GetByID(ctx context.Context, id string, userID string) (models.Challenge, error) {
	return s.repo.FindByIDVisibleToUser(ctx, id, userID)
}

// GetChallengeWithDetails returns a challenge including solution_code and
// solution_explanation. Use this only when the caller is authorized to see
// the solution (tutor context or reveal flag).
func (s *ChallengeService) GetChallengeWithDetails(ctx context.Context, id string, userID string) (models.Challenge, error) {
	return s.repo.FindByIDWithSolutionVisibleToUser(ctx, id, userID)
}

// ValidateChallenge validates a CreateChallengeInput without persisting it.
// It is the public surface of validateChallengeV2 so the MCP
// mcp-challenges.validate_challenge tool can give the LLM actionable
// feedback on a draft challenge BEFORE the user commits to it. The
// returned error (when non-nil) is one of the IsValidationError sentinels
// declared above so callers can branch with errors.Is.
func (s *ChallengeService) ValidateChallenge(input models.CreateChallengeInput) error {
	return validateChallengeV2(input)
}

// GenerateChallenge returns a freshly-authored challenge. PR 2 ships a
// stub returning ErrChallengeGenerateNotImplemented; the full LLM-driven
// implementation lands with the S5 LLM cascade (OpenRouter + Ollama).
// The MCP mcp-challenges.generate_challenge tool delegates here so the
// "not implemented" business decision lives in core/services — not in
// the transport adapter. Callers branch with errors.Is to surface a
// stable JSON-RPC -32603 regardless of transport.
//
// The stub takes no fields and does not touch the repository, so a
// nil repo is safe here.
func (s *ChallengeService) GenerateChallenge(_ context.Context, _ models.GenerateChallengeHint) (models.Challenge, error) {
	return models.Challenge{}, ErrChallengeGenerateNotImplemented
}
