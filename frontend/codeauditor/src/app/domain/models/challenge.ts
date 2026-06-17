// Challenge — core domain entity representing a code-audit challenge.
//
// Zero framework imports. Pure TypeScript domain model.
export type ChallengeDifficulty = 'junior' | 'mid' | 'senior' | 'architect';
export type ChallengeStatus = 'available' | 'in_progress' | 'completed';

export interface Challenge {
  id: string;
  title: string;
  description: string;
  difficulty: ChallengeDifficulty;
  category: string;
  language: string;
  repoUrl: string;
  code: string;
  codeSmell: string;
  status: ChallengeStatus;
  createdAt: Date;
}
