// ScoreBreakdown — transparent score breakdown returned by the scoring engine.
//
// Zero framework imports. Pure TypeScript domain model mirroring the Go
// `models.ScoreBreakdown` struct so the frontend can render each component
// of the audit score line-by-line.
//
// Formula:
//   total = basePoints + testPoints + lintPoints + findingsMatched
//           - hintsPenalty + timeBonus   (floored at 0)
//   testPoints    = Σ(test_passed × weight × 30)
//   lintPoints     = Σ(lint_clean × 20)
//   findingsMatched = Σ(expected_finding_matched × 10)
//   hintsPenalty  = Σ(hint_used × cost_points)
//   timeBonus     = 10% of basePoints when timeBonus=true and
//                   elapsedMinutes < estimatedTimeMinutes
export interface ScoreBreakdown {
  basePoints: number;
  testPoints: number;
  lintPoints: number;
  findingsMatched: number; // count of matched findings × 10
  hintsPenalty: number;
  timeBonus: number;
  total: number;
}
