import { AuditRepository } from "../domain/ports/audit-repository.port";
import { LLMPort } from "../domain/ports/llm.port";

/**
 * AuditUseCase — orchestrates the full audit workflow.
 *
 * Responsibilities:
 * 1. Receive a repository URL and start an audit session
 * 2. Trigger scanning (SAST) via sandbox execution
 * 3. Enrich findings with LLM explanations
 * 4. Persist results via the AuditRepository
 *
 * This is a stub — real implementation will wire the full flow.
 */
export class AuditUseCase {
  constructor(
    private readonly auditRepo: AuditRepository,
    private readonly llm: LLMPort
  ) {}

  async startAudit(repositoryUrl: string, branch: string): Promise<string> {
    // Stub: create a pending session and return its ID
    const session = await this.auditRepo.createSession({
      repositoryUrl,
      branch,
      status: "pending",
      findingsCount: 0,
    });
    return session.id;
  }

  async getSession(sessionId: string) {
    return this.auditRepo.getSession(sessionId);
  }

  async listSessions() {
    return this.auditRepo.listSessions();
  }

  async getFindings(sessionId: string) {
    return this.auditRepo.getFindingsForSession(sessionId);
  }
}