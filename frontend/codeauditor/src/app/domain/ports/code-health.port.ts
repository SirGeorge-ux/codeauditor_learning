// code-health.port.ts — domain port for the code-health capability
// (S4 — code-health-dashboard change).
//
// The frontend talks to the Go MCP server's `mcp-code-health.*` group
// (see openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 + backend/internal/infrastructure/mcp/code_health/*)
// through this port. The application layer (CodeHealthUseCase) depends
// on this interface; the infrastructure layer provides the adapter
// in `infrastructure/repositories/code-health.repository.ts`.
//
// Hexagonal contract: this file imports ONLY the domain models
// (`./code-health`). No @angular/*, no fetch, no rxjs, no MCP HTTP. The
// tests in code-health.port.spec.ts rely on the file being framework-
// free so the application layer can exercise the contract without a
// TestBed.

import {
  AnalyzeRepoArgs,
  CodeHealthAnalysis,
  CodeHealthSource,
  CodeHealthIssue,
  CodeHealthSeverity,
  HealthReport,
} from '../models/code-health';

// Re-exports so callers (use case + tests) can import the args from
// either the model or the port without breaking the hexagonal layer.
// Mirrors the practice.use-case.ts pattern where the wire-shape
// arguments are re-exported from the application module.
export type { AnalyzeRepoArgs } from '../models/code-health';

/**
 * Result of `mcp-code-health.analyze_repo`. The server returns the
 * pending `status` synchronously so the client can poll `get_analysis`
 * with the returned `analysis_id`.
 */
export interface AnalyzeRepoResult {
  readonly analysis_id: string;
  readonly status: 'pending';
}

/**
 * Result of `mcp-code-health.get_repo_issues`. The server filters by
 * `severity` + caps at `limit`; the response always carries an issues
 * array (possibly empty).
 */
export interface GetRepoIssuesArgs {
  readonly owner: string;
  readonly name: string;
  readonly source: CodeHealthSource;
  readonly severity?: CodeHealthSeverity;
  readonly limit?: number;
}

export interface GetRepoIssuesResult {
  readonly issues: readonly CodeHealthIssue[];
}

/**
 * Result of `mcp-code-health.generate_health_report`. Today only `md`
 * is implemented; `format: 'pdf'` returns MCP error -32603 from the
 * server (REQ-MCP-CODE-HEALTH-001 + design.md §Tradeoffs).
 */
export interface GenerateHealthReportArgs {
  readonly analysis_id: string;
  readonly format: HealthReport['format'];
}

export interface GenerateHealthReportResult {
  readonly content: string;
}

/**
 * Result of `mcp-code-health.suppress_issue`. The server persists the
 * suppression under the authenticated user and returns the rows' `id`
 * + `created_at` so the UI can show a confirmation toast.
 */
export interface SuppressIssueArgs {
  readonly issue_id: string;
  readonly reason?: string;
}

export interface SuppressIssueResult {
  readonly suppression_id: string;
  readonly suppressed_at: Date;
}

/**
 * Result type aliases for consistency with the rest of the port. The
 * `getAnalysis` payload IS the domain `CodeHealthAnalysis` (the wire
 * shape maps 1:1 with the snake_case → camelCase transformation in the
 * repository) and the `generateHealthReport` payload is plain content.
 */
export type GetAnalysisResult = CodeHealthAnalysis;

/**
 * CodeHealthPort is the application-facing contract for invoking the
 * `mcp-code-health.*` tool group. Implementations MUST inject the
 * authenticated `user_id` into context (consistent with the
 * `mcp-client.port.ts` pattern) so the Go server can scope writes.
 */
export interface CodeHealthPort {
  /**
   * Fire-and-forget analysis request. Returns the `analysis_id`
   * synchronously; the server flips the row through
   * `pending → running → completed | failed` in the background. The
   * `getAnalysis` method polls the row until status is terminal.
   */
  readonly analyzeRepo: (args: AnalyzeRepoArgs) => Promise<AnalyzeRepoResult>;

  /**
   * Poll for an analysis row's current state. The client is expected
   * to back off between calls (spec REQ-CODE-HEALTH-003:
   * `polling backoff 2s→30s`).
   */
  readonly getAnalysis: (analysis_id: string) => Promise<GetAnalysisResult>;

  /**
   * Server-side filter + limit on the issues list. Optional
   * `severity` narrows the union; `limit` defaults server-side.
   */
  readonly getRepoIssues: (args: GetRepoIssuesArgs) => Promise<GetRepoIssuesResult>;

  /**
   * Render the human-readable markdown (or pdf — currently
   * unimplemented) report from the persisted analysis.
   */
  readonly generateHealthReport: (
    args: GenerateHealthReportArgs,
  ) => Promise<GenerateHealthReportResult>;

  /**
   * Mark a single issue as "not an issue" for the authenticated user.
   * The server persists `(user_id, file_path, rule_id, line_number)`
   * so the analyzer skips the same finding on the next analysis
   * (REQ-CODE-HEALTH-008 round-trip).
   */
  readonly suppressIssue: (args: SuppressIssueArgs) => Promise<SuppressIssueResult>;
}
