// code-health.ts — domain types for the code-health capability (S4).
//
// Pure TypeScript entities mirroring the wire shapes of
// `mcp-code-health.*` (see openspec/changes/code-health-dashboard/
// specs/mcp/spec.md REQ-MCP-CODE-HEALTH-001 +
// specs/code-health/spec.md REQ-CODE-HEALTH-004..013).
//
// Hexagonal contract: zero @angular/* imports. The repository
// (`infrastructure/repositories/code-health.repository.ts`) is the only
// place that translates the wire shapes onto these domain types.
//
// The union literals + readonly tuples stay narrow so a flag of an
// invalid wire value (e.g. `severity: "extreme"`) fails compile-time
// instead of silently propagating into the dashboard render.

/**
 * CodeHealthSeverity ranks the 4 severity bands used by every
 * `mcp-code-health.*` finding. Aligned with REQ-CODE-HEALTH-004
 * (weights: critical=15, high=8, medium=3, low=1).
 */
export type CodeHealthSeverity = 'critical' | 'high' | 'medium' | 'low';

/**
 * CodeHealthCategory partitions findings into the 5 buckets required
 * by REQ-CODE-HEALTH-006 (one per seeded rule per the design table).
 */
export type CodeHealthCategory =
  | 'security'
  | 'performance'
  | 'refactor'
  | 'style'
  | 'architecture';

/**
 * CodeHealthStatus is the async-lifecycle state machine for an
 * analysis job — per design.md §Async job lifecycle
 * (pending → running → completed | failed).
 */
export type CodeHealthStatus = 'pending' | 'running' | 'completed' | 'failed';

/**
 * CodeHealthSource is the supported git host. The MCP tool
 * `mcp-code-health.analyze_repo` rejects anything outside this set
 * with `-32602` (REQ-MCP-CODE-HEALTH-001 + design.md §Wire Shapes).
 */
export type CodeHealthSource = 'gogs' | 'github';

/**
 * Readonly tuples exported alongside the union types so callers and
 * tests can iterate or build constraint checks without re-listing the
 * literals. Match the order in the design.md + spec.md canonical
 * tables.
 */
export const CODE_HEALTH_SEVERITIES: readonly CodeHealthSeverity[] = [
  'critical',
  'high',
  'medium',
  'low',
] as const;

export const CODE_HEALTH_CATEGORIES: readonly CodeHealthCategory[] = [
  'security',
  'performance',
  'refactor',
  'style',
  'architecture',
] as const;

export const CODE_HEALTH_STATUSES: readonly CodeHealthStatus[] = [
  'pending',
  'running',
  'completed',
  'failed',
] as const;

export const CODE_HEALTH_SOURCES: readonly CodeHealthSource[] = [
  'gogs',
  'github',
] as const;

/**
 * CodeHealthIssue is the finding shape returned by
 * `mcp-code-health.get_repo_issues` + the `top_issues` field of
 * `get_analysis`. The wire carries `line_number` + `rule_id` in
 * snake_case; the repository maps them onto the camelCase TS surface.
 *
 * Field reference (mirrors
 * `backend/internal/core/domain/models/code_health_issue.go`):
 *   - id:           tool-emitted UUID for the issue row.
 *   - rule_id:      stable registry key (e.g. RULE-SEC-SECRET-001).
 *   - confidence:   1.0 for regex hits; ∈ [0,1] for LLM findings.
 */
export interface CodeHealthIssue {
  readonly id: string;
  readonly rule_id: string;
  readonly category: CodeHealthCategory;
  readonly severity: CodeHealthSeverity;
  readonly message: string;
  readonly file_path: string;
  readonly line_number: number;
  readonly evidence: string;
  readonly suggested_fix: string;
  readonly confidence: number;
}

/**
 * CodeHealthAnalysis is the async-job payload returned by
 * `mcp-code-health.get_analysis`. While the goroutine is still
 * processing, `health_score` is 0, `top_issues` is empty, and
 * `completed_at` + `error_message` are absent.
 *
 * `languages` is a normalized 0..1 map keyed by language name
 * (matches spec REQ-CODE-HEALTH-003 language breakdown column).
 */
export interface CodeHealthAnalysis {
  readonly id: string;
  readonly owner: string;
  readonly name: string;
  readonly source: CodeHealthSource;
  readonly status: CodeHealthStatus;
  readonly health_score: number;
  readonly top_issues: readonly CodeHealthIssue[];
  readonly languages: Readonly<Record<string, number>>;
  readonly summary: string;
  readonly created_at: Date;
  readonly completed_at?: Date;
  readonly error_message?: string;
}

/**
 * AnalyzeRepoArgs is the input the page sends to
 * `mcp-code-health.analyze_repo`. Branch defaults to "master" server-
 * side; `force` bypasses the cache read (REQ-CODE-HEALTH-009).
 */
export interface AnalyzeRepoArgs {
  readonly owner: string;
  readonly name: string;
  readonly source: CodeHealthSource;
  readonly branch?: string;
  readonly force?: boolean;
}

/**
 * HealthReport is the artifact returned by
 * `mcp-code-health.generate_health_report`. Today only `md` is
 * implemented; `pdf` returns MCP error -32603 from the server
 * (REQ-MCP-CODE-HEALTH-001).
 */
export interface HealthReport {
  readonly report_id: string;
  readonly content: string;
  readonly format: 'md' | 'pdf';
}

/**
 * Narrowing helper for the severity union. Booleans, numbers and
 * unknown strings collapse to false so callers (the use case, the
 * filter component) can guard a user-supplied value before passing it
 * to MCP tools.
 */
export function isCodeHealthSeverity(value: unknown): value is CodeHealthSeverity {
  return typeof value === 'string' && (CODE_HEALTH_SEVERITIES as readonly string[]).includes(value);
}

/**
 * Narrowing helper for the category union. The spec mandates exactly
 * the 5 categories (REQ-CODE-HEALTH-006); unknown strings fail fast
 * rather than silently dropping a detection.
 */
export function isCodeHealthCategory(value: unknown): value is CodeHealthCategory {
  return typeof value === 'string' && (CODE_HEALTH_CATEGORIES as readonly string[]).includes(value);
}

/**
 * Narrowing helper for the source union. Used by the `repo-url-bar`
 * component to reject malformed input before the call goes out.
 */
export function isCodeHealthSource(value: unknown): value is CodeHealthSource {
  return typeof value === 'string' && (CODE_HEALTH_SOURCES as readonly string[]).includes(value);
}

/**
 * Narrowing helper for the status union. The polling loop reads the
 * status signal and stops at `completed` | `failed`; an unrecognized
 * value is treated as "keep polling".
 */
export function isCodeHealthStatus(value: unknown): value is CodeHealthStatus {
  return typeof value === 'string' && (CODE_HEALTH_STATUSES as readonly string[]).includes(value);
}
