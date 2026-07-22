package services

import (
	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ScoringService calculates an audit score for a challenge using pure Go logic.
// It imports only core/domain/models and stdlib — no infrastructure dependencies.
type ScoringService struct{}

// NewScoringService creates a new ScoringService.
func NewScoringService() *ScoringService {
	return &ScoringService{}
}

// Calculate computes a ScoreBreakdown from a Challenge and the user's AuditInput.
//
// Formula:
//
//	score = base_points
//	      + Σ(test_passed × weight × 30)
//	      + Σ(lint_clean × 20)
//	      + Σ(expected_finding_matched × 10)
//	      - Σ(hint_used × hint_cost)
//	      + time_bonus
//
// The score is floored at 0 (never negative).
// Time bonus: +10% of base_points when challenge.TimeBonus is true and the
// user finishes before EstimatedTimeMinutes.
func (s *ScoringService) Calculate(challenge models.Challenge, input models.AuditInput) models.ScoreBreakdown {
	basePoints := challenge.BasePoints
	if basePoints < 0 {
		basePoints = 0
	}

	testPoints := s.calculateTestPoints(challenge, input)
	lintPoints := s.calculateLintPoints(challenge, input)
	findingsPoints := s.calculateFindingsPoints(challenge, input)
	hintsPenalty := s.calculateHintsPenalty(challenge, input)
	timeBonus := s.calculateTimeBonus(challenge, input)

	total := basePoints + testPoints + lintPoints + findingsPoints - hintsPenalty + timeBonus
	if total < 0 {
		total = 0
	}

	return models.ScoreBreakdown{
		BasePoints:      basePoints,
		TestPoints:      testPoints,
		LintPoints:      lintPoints,
		FindingsMatched: findingsPoints,
		HintsPenalty:    hintsPenalty,
		TimeBonus:       timeBonus,
		Total:           total,
	}
}

// calculateTestPoints sums weight*30 for each passed test case.
func (s *ScoringService) calculateTestPoints(challenge models.Challenge, input models.AuditInput) int {
	points := 0
	for i, tc := range challenge.TestCases {
		if i < len(input.TestsPassed) && input.TestsPassed[i] {
			points += tc.Weight * 30
		}
	}
	return points
}

// calculateLintPoints sums 20 for each clean linter rule.
func (s *ScoringService) calculateLintPoints(challenge models.Challenge, input models.AuditInput) int {
	points := 0
	for i := range challenge.LinterRules {
		if i < len(input.LintClean) && input.LintClean[i] {
			points += 20
		}
	}
	return points
}

// calculateFindingsPoints sums 10 for each matched expected finding.
func (s *ScoringService) calculateFindingsPoints(challenge models.Challenge, input models.AuditInput) int {
	points := 0
	for i := range challenge.ExpectedFindings {
		if i < len(input.FindingsMatched) && input.FindingsMatched[i] {
			points += 10
		}
	}
	return points
}

// calculateHintsPenalty sums the CostPoints of each consumed hint.
func (s *ScoringService) calculateHintsPenalty(challenge models.Challenge, input models.AuditInput) int {
	penalty := 0
	for i, h := range challenge.Hints {
		if i < len(input.HintsUsed) && input.HintsUsed[i] {
			penalty += h.CostPoints
		}
	}
	return penalty
}

// calculateTimeBonus returns 10% of base_points when the user finishes under
// the estimated time and the challenge has TimeBonus enabled.
func (s *ScoringService) calculateTimeBonus(challenge models.Challenge, input models.AuditInput) int {
	if !challenge.TimeBonus {
		return 0
	}
	if challenge.EstimatedTimeMinutes <= 0 {
		return 0
	}
	if input.ElapsedMinutes >= float64(challenge.EstimatedTimeMinutes) {
		return 0
	}
	return challenge.BasePoints / 10
}