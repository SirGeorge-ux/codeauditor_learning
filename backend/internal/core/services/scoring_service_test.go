package services

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

func TestScoringService_Calculate(t *testing.T) {
	// Helper to build a standard challenge for tests
	baseChallenge := func() models.Challenge {
		return models.Challenge{
			BasePoints:           100,
			BonusPoints:          50,
			PenaltyPerHint:       0,
			TimeBonus:            true,
			EstimatedTimeMinutes: 15,
			TestCases: []models.TestCase{
				{Name: "test1", Weight: 5},
				{Name: "test2", Weight: 5},
			},
			LinterRules: []models.LinterRule{
				{Type: "eslint", Config: nil},
			},
			ExpectedFindings: []models.ExpectedFinding{
				{Category: "security", Severity: "high"},
				{Category: "style", Severity: "medium"},
				{Category: "performance", Severity: "low"},
			},
			Hints: []models.Hint{
				{Level: 1, Content: "hint1", CostPoints: 0},
				{Level: 2, Content: "hint2", CostPoints: 10},
				{Level: 3, Content: "hint3", CostPoints: 25},
			},
		}
	}

	tests := []struct {
		name      string
		challenge  models.Challenge
		input      models.AuditInput
		want       models.ScoreBreakdown
	}{
		{
			name:     "perfect solution scores maximum",
			challenge: baseChallenge(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: []bool{true, true, true},
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  10, // under 15 min, time bonus applies
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,  // 2 * 5 * 30
				LintPoints:      20,   // 1 * 20
				FindingsMatched: 30,   // 3 * 10
				HintsPenalty:    0,
				TimeBonus:       10,   // 10% of 100
				Total:           460,  // 100 + 300 + 20 + 30 + 0 + 10
			},
		},
		{
			name:     "all hints used applies full penalty",
			challenge: baseChallenge(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: []bool{true, true, true},
				HintsUsed:       []bool{true, true, true},
				ElapsedMinutes:  10,
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,
				LintPoints:      20,
				FindingsMatched: 30,
				HintsPenalty:    35,   // 0 + 10 + 25
				TimeBonus:       10,
				Total:           425,  // 100 + 300 + 20 + 30 - 35 + 10
			},
		},
		{
			name:     "zero tests passed yields only base and findings",
			challenge: baseChallenge(),
			input: models.AuditInput{
				TestsPassed:     []bool{false, false},
				LintClean:       []bool{false},
				FindingsMatched: []bool{true, true, true},
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  10,
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      0,
				LintPoints:      0,
				FindingsMatched: 30,
				HintsPenalty:    0,
				TimeBonus:       10,
				Total:           140,  // 100 + 0 + 0 + 30 - 0 + 10
			},
		},
		{
			name:     "partial findings matched yields partial points",
			challenge: baseChallenge(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: []bool{true, false, false}, // only 1 of 3
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  10,
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,
				LintPoints:      20,
				FindingsMatched: 10,  // 1 * 10
				HintsPenalty:    0,
				TimeBonus:       10,
				Total:           440,  // 100 + 300 + 20 + 10 - 0 + 10
			},
		},
		{
			name:     "time bonus when finishing within estimated time",
			challenge: baseChallenge(),
			input: models.AuditInput{
				TestsPassed:     []bool{false, false},
				LintClean:       []bool{false},
				FindingsMatched: []bool{false, false, false},
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  5, // under 15 min
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      0,
				LintPoints:      0,
				FindingsMatched: 0,
				HintsPenalty:    0,
				TimeBonus:       10,  // 10% of 100
				Total:           110,
			},
		},
		{
			name: "no time bonus when over estimated time",
			challenge: func() models.Challenge {
				c := baseChallenge()
				return c
			}(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: []bool{true, true, true},
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  20, // over 15 min, no time bonus
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,
				LintPoints:      20,
				FindingsMatched: 30,
				HintsPenalty:    0,
				TimeBonus:       0,
				Total:           450,
			},
		},
		{
			name: "score floors at zero when penalty exceeds positive",
			challenge: func() models.Challenge {
				c := baseChallenge()
				c.BasePoints = 50
				c.Hints[2].CostPoints = 80 // hint 3 costs 80
				return c
			}(),
			input: models.AuditInput{
				TestsPassed:     []bool{false, false},
				LintClean:       []bool{false},
				FindingsMatched: []bool{false, false, false},
				HintsUsed:       []bool{false, false, true}, // use only hint 3 → 80 penalty
				ElapsedMinutes:  10,
			},
			want: models.ScoreBreakdown{
				BasePoints:      50,
				TestPoints:      0,
				LintPoints:      0,
				FindingsMatched: 0,
				HintsPenalty:    80,
				TimeBonus:       5, // 10% of 50
				Total:           0,  // 50 + 0 + 0 + 0 - 80 + 5 = -25 → floor at 0
			},
		},
		{
			name: "no expected findings yields zero findings points",
			challenge: func() models.Challenge {
				c := baseChallenge()
				c.ExpectedFindings = []models.ExpectedFinding{}
				return c
			}(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: nil, // empty expected_findings → no possible matches
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  10,
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,
				LintPoints:      20,
				FindingsMatched: 0,
				HintsPenalty:    0,
				TimeBonus:       10,
				Total:           430,
			},
		},
		{
			name: "time bonus disabled on challenge with TimeBonus=false",
			challenge: func() models.Challenge {
				c := baseChallenge()
				c.TimeBonus = false
				return c
			}(),
			input: models.AuditInput{
				TestsPassed:     []bool{true, true},
				LintClean:       []bool{true},
				FindingsMatched: []bool{true, true, true},
				HintsUsed:       []bool{false, false, false},
				ElapsedMinutes:  5, // well under time but TimeBonus disabled
			},
			want: models.ScoreBreakdown{
				BasePoints:      100,
				TestPoints:      300,
				LintPoints:      20,
				FindingsMatched: 30,
				HintsPenalty:    0,
				TimeBonus:       0,
				Total:           450,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewScoringService()
			got := svc.Calculate(tt.challenge, tt.input)

			if got.BasePoints != tt.want.BasePoints {
				t.Errorf("BasePoints = %d, want %d", got.BasePoints, tt.want.BasePoints)
			}
			if got.TestPoints != tt.want.TestPoints {
				t.Errorf("TestPoints = %d, want %d", got.TestPoints, tt.want.TestPoints)
			}
			if got.LintPoints != tt.want.LintPoints {
				t.Errorf("LintPoints = %d, want %d", got.LintPoints, tt.want.LintPoints)
			}
			if got.FindingsMatched != tt.want.FindingsMatched {
				t.Errorf("FindingsMatched = %d, want %d", got.FindingsMatched, tt.want.FindingsMatched)
			}
			if got.HintsPenalty != tt.want.HintsPenalty {
				t.Errorf("HintsPenalty = %d, want %d", got.HintsPenalty, tt.want.HintsPenalty)
			}
			if got.TimeBonus != tt.want.TimeBonus {
				t.Errorf("TimeBonus = %d, want %d", got.TimeBonus, tt.want.TimeBonus)
			}
			if got.Total != tt.want.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.want.Total)
			}
		})
	}
}