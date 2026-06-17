import { AuditSession } from '../models/audit-session';
import { Finding } from '../models/finding';

/**
 * AuditRepository — drives the audit workflow from the application's perspective.
 *
 * The application layer calls these methods. The infrastructure layer
 * provides concrete implementations (e.g., Supabase adapter).
 */
export interface AuditRepository {
  createSession(
    session: Omit<AuditSession, 'id' | 'createdAt' | 'updatedAt'>,
  ): Promise<AuditSession>;
  getSession(id: string): Promise<AuditSession | null>;
  listSessions(): Promise<AuditSession[]>;
  updateSessionStatus(id: string, status: AuditSession['status']): Promise<void>;
  addFinding(finding: Omit<Finding, 'id' | 'detectedAt'>): Promise<Finding>;
  getFindingsForSession(sessionId: string): Promise<Finding[]>;
}
