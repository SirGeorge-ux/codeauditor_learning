// AuditEvent represents a single event streamed during audit execution.
// Zero framework imports — pure TypeScript domain model.
export interface AuditEvent {
  type: 'stdout' | 'stderr' | 'error' | 'complete' | 'llm_token' | 'llm_analysis' | 'llm_error';
  data: string;
  timestamp: string;
}
