// Challenge — core domain entity representing a code-audit challenge (v2 model).
//
// Zero framework imports. Pure TypeScript domain model.
// v1 fields `codeSmell` and `repoUrl` are removed; the v2 model uses
// `expectedFindings` for objective scoring and `sourceRepo`/`origin` for
// provenance. `description` is context-only and MUST NOT name the smell,
// vulnerability, or fix.
export type ChallengeDifficulty = 'junior' | 'mid' | 'senior' | 'architect';
export type ChallengeStatus = 'available' | 'in_progress' | 'completed';
export type ChallengeOrigin = 'curated' | 'generated' | 'imported' | 'gogs' | 'github';

// Progressive hint revealed to the auditor. Exactly 3 per challenge.
// Level 1 is free; levels 2 and 3 cost points (deducted from the score).
export interface Hint {
  level: number; // 1 | 2 | 3
  content: string;
  costPoints: number; // 0 for level 1
}

// Objective finding the auditor is expected to discover. Matched by
// category/severity against the user's audit output for scoring.
export interface ExpectedFinding {
  severity: string;
  category: string;
  message: string;
  evidence: string;
  suggestedFix: string;
}

// Functional test case used by the scoring engine. Each passing test
// contributes `weight × 30` points to the score.
export interface TestCase {
  name: string;
  input: unknown;
  expectedOutput: unknown;
  weight: number;
}

// Linter configuration for objective evaluation. Each clean rule
// contributes 20 points to the lint score.
export interface LinterRule {
  type: string;
  config: Record<string, unknown>;
}

export interface Challenge {
  id: string;
  title: string;
  description: string; // context-only, no spoilers
  difficulty: ChallengeDifficulty;
  category: string;
  language: string;
  code: string;
  status: ChallengeStatus;
  createdAt: Date;
  createdBy: string;

  // v2 objective scoring fields
  learningObjectives: string[];
  hints: Hint[]; // exactly 3, progressive
  commonMistakes: string[];
  estimatedTimeMinutes: number;
  expectedFindings: ExpectedFinding[]; // 1-3 items
  testCases: TestCase[]; // 2-4 items
  linterRules: LinterRule[];
  basePoints: number;
  bonusPoints: number;
  penaltyPerHint: number;
  timeBonus: boolean;
  origin: ChallengeOrigin;

  // Provenance (optional — set when challenge originates from a repo)
  sourceRepo?: string;
  sourcePath?: string;
  generatedBy?: string;

  // Solution (only present when ?reveal=true and permission allows)
  solutionCode?: string;
  solutionExplanation?: string;
}
