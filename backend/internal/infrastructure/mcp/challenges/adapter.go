package challenges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// NewServiceAdapter returns the production wiring for the mcp-challenges
// tool group. The adapter bridges *services.ChallengeService to the local
// Service interface, owning the JSON wire shape so the tool side stays
// consistent. The adapter implements GenerateChallenge as a stub returning
// ErrNotImplemented until the S5 LLM cascade lands.
func NewServiceAdapter(svc *services.ChallengeService) Service {
	return &serviceAdapter{svc: svc}
}

// serviceAdapter bridges *services.ChallengeService to the local Service
// interface. Each method projects the service's typed return into the
// JSON envelope the LLM consumes.
type serviceAdapter struct {
	svc *services.ChallengeService
}

// ListChallenges lists every challenge visible to the user. The service
// returns models.Challenge without solution fields — we marshall each to
// the same wire shape so the tool receives a uniform []json.RawMessage.
func (a *serviceAdapter) ListChallenges(ctx context.Context, userID string) ([]json.RawMessage, error) {
	challenges, err := a.svc.GetAll(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(challenges))
	for _, c := range challenges {
		raw, err := json.Marshal(encodeChallenge(&c, false))
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// GetChallenge fetches a single challenge. reveal=true uses the
// GetChallengeWithDetails entry point so the solution is included;
// otherwise the public shape is returned.
func (a *serviceAdapter) GetChallenge(ctx context.Context, userID, id string, reveal bool) (json.RawMessage, error) {
	var (
		c   models.Challenge
		err error
	)
	if reveal {
		c, err = a.svc.GetChallengeWithDetails(ctx, id, userID)
	} else {
		c, err = a.svc.GetByID(ctx, id, userID)
	}
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, err
	}
	return json.Marshal(encodeChallenge(&c, reveal))
}

// CreateTempChallenge inserts (or returns the dedup hit of) a draft
// challenge. The service's ValidateInput path runs first via ChallengeService
// returning a sentinel error; we surface those as ErrInvalidParams.
func (a *serviceAdapter) CreateTempChallenge(ctx context.Context, userID string, in challengeInputEnvelope) (json.RawMessage, error) {
	c, created, err := a.svc.Create(ctx, in, userID)
	if err != nil {
		if services.IsValidationError(err) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	payload := encodeChallenge(c, false)
	payload["deduped"] = !created
	return json.Marshal(payload)
}

// GenerateChallenge delegates to ChallengeService.GenerateChallenge. The
// "not implemented" stub was un-stubbed in PR 2; the canonical business
// decision now lives in the service (ErrChallengeGenerationNotWired +
// ErrChallengeGenerationInvalidJSON/Invalid/FailedSandbox) so callers
// across transports branch the same way. We translate the MCP wire
// shape (GenerateHint, including the new topicId + level fields) into
// the domain shape (models.GenerateChallengeHint) and propagate any
// service error verbatim to the tool layer, where the dispatcher maps
// internal errors to JSON-RPC -32603.
//
// The userID is forwarded to the service so the persisted row carries
// CreatedBy=userID per the spec's generated-challenge persistence
// contract. When the service is unwired, the userID is not consumed
// (the sentinel short-circuits before row creation).
func (a *serviceAdapter) GenerateChallenge(ctx context.Context, userID string, hint GenerateHint) (json.RawMessage, error) {
	c, err := a.svc.GenerateChallenge(ctx, userID, models.GenerateChallengeHint{
		Language:      hint.Language,
		Difficulty:    hint.Difficulty,
		Focus:         hint.Focus,
		TopicID:       hint.TopicID,
		Level:         hint.Level,
		SourceIssueID: hint.SourceIssueID,
	})
	if err != nil {
		// Every sentinel (ErrChallengeGenerationNotWired /
		// ErrChallengeGenerationInvalidJSON / ErrChallengeGenerationInvalid
		// / ErrChallengeGenerationFailedSandbox) travels verbatim so
		// the dispatcher can map it to JSON-RPC -32603 without the
		// adapter re-wrapping. The canonical business sentinels live
		// in services so HTTP + MCP + future transports branch the
		// same way.
		return nil, err
	}
	// Production adapters can construct the wire from the persisted row;
	// solution_code is included so the LLM-side tool chain (and the
	// dojo's "reveal" flow) sees it on a successful generation.
	return json.Marshal(encodeChallenge(&c, true))
}

// ValidateChallenge runs the v2 validator and shapes the result. The
// service returns the same sentinels the HTTP handler uses so the LLM gets
// the same actionable feedback.
func (a *serviceAdapter) ValidateChallenge(_ context.Context, _ string, in validateChallengeInput) (ValidateResult, error) {
	err := a.svc.ValidateChallenge(in)
	if err == nil {
		return ValidateResult{Valid: true, Errors: []string{}}, nil
	}
	if services.IsValidationError(err) {
		return ValidateResult{Valid: false, Errors: []string{err.Error()}}, nil
	}
	return ValidateResult{}, err
}

// encodeChallenge projects a models.Challenge into the wire shape the LLM
// consumes. Keep SolutionCode/SolutionExplanation in the projection only
// when includeSolution is true — the field set without the solution is the
// canonical "public" shape used by both list_challenges and the HTTP GET
// endpoint.
func encodeChallenge(c *models.Challenge, includeSolution bool) map[string]any {
	out := map[string]any{
		"id":                   c.ID,
		"title":                c.Title,
		"description":          c.Description,
		"difficulty":           c.Difficulty,
		"category":             c.Category,
		"language":             c.Language,
		"code":                 c.Code,
		"status":               c.Status,
		"sourceRepo":           c.SourceRepo,
		"createdAt":            c.CreatedAt,
		"learningObjectives":   c.LearningObjectives,
		"hints":                c.Hints,
		"commonMistakes":       c.CommonMistakes,
		"estimatedTimeMinutes": c.EstimatedTimeMinutes,
		"expectedFindings":     c.ExpectedFindings,
		"testCases":            c.TestCases,
		"linterRules":          c.LinterRules,
		"basePoints":           c.BasePoints,
		"bonusPoints":          c.BonusPoints,
		"penaltyPerHint":       c.PenaltyPerHint,
		"timeBonus":            c.TimeBonus,
		"origin":               c.Origin,
		"sourcePath":           c.SourcePath,
		"generatedBy":          c.GeneratedBy,
		"createdBy":            c.CreatedBy,
		"sourceIssueId":        c.SourceIssueID,
	}
	if c.RepoURL != "" {
		out["repoUrl"] = c.RepoURL
	}
	if c.CodeSmell != "" {
		out["codeSmell"] = c.CodeSmell
	}
	if c.UserID != nil && *c.UserID != "" {
		out["userId"] = *c.UserID
	}
	if includeSolution {
		out["solutionCode"] = c.SolutionCode
		out["solutionExplanation"] = c.SolutionExplanation
	}
	return out
}
