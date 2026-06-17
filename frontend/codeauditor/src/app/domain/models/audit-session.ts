// AuditSession — represents a single code audit session.
//
// This is a domain entity stub. The full implementation will include
// stages: pending → scanning → analyzing → complete | failed.
export interface AuditSession {
  id: string;
  repositoryUrl: string;
  branch: string;
  status: 'pending' | 'scanning' | 'analyzing' | 'complete' | 'failed';
  createdAt: Date;
  updatedAt: Date;
  findingsCount: number;
  llmModel?: string;
}
