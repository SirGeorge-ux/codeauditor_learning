// Finding — represents a security or code quality issue found during audit.
//
// Severity levels follow CVSS-style categories.
export type FindingSeverity = 'critical' | 'high' | 'medium' | 'low' | 'info';

export interface Finding {
  id: string;
  sessionId: string;
  ruleId: string;
  severity: FindingSeverity;
  file: string;
  line: number;
  message: string;
  cweId?: string;
  sastId?: string;
  llmExplanation?: string;
  detectedAt: Date;
}
