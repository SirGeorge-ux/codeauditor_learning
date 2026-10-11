// code-health.repository.ts — infrastructure adapter that delegates to
// the `mcp-code-health.*` MCP tool group (see
// openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001) via the existing `MCPClientPort`.
//
// Why MCP and not REST: per the design.md §Tradeoffs + proposal.md
// (backend Go MCP-only for S4), the `mcp-code-health.*` group is the
// only transport for this capability — there is no `/api/v1/code-health/*`
// handler in S4. The adapter is the only place that knows the
// snake_case ↔ camelCase mapping for the wire shapes
// (design.md §Wire Shapes + REQ-MCP-CODE-HEALTH-001).
//
// Hexagonal contract: PLAIN class — does NOT use `@Injectable`. The
// project's existing convention (see HttpDictionaryRepository /
// HttpChallengeRepository / HttpProgressRepository in this same folder)
// keeps repositories as plain classes and instantiates them from the
// `@Injectable` Service (CodeHealthService in
// infrastructure/services/code-health.service.ts). This lets the use
// case + repository stay framework-free and direct-instantiation in
// the spec without spinning up a TestBed.
import { firstValueFrom } from 'rxjs';

import {
  AnalyzeRepoArgs,
  AnalyzeRepoResult,
  CodeHealthPort,
  GenerateHealthReportArgs,
  GenerateHealthReportResult,
  GetAnalysisResult,
  GetRepoIssuesArgs,
  GetRepoIssuesResult,
  SuppressIssueArgs,
  SuppressIssueResult,
} from '../../domain/ports/code-health.port';
import { MCPClientPort } from '../../domain/ports/mcp-client.port';
import {
  CodeHealthAnalysis,
  CodeHealthIssue,
  CodeHealthSeverity,
  CodeHealthStatus,
} from '../../domain/models/code-health';

// Tool name constants — single source of truth in the application
// layer (mirrors practice.use-case.ts). Updated when the Go tool
// surface changes; see REQ-MCP-CODE-HEALTH-001 for the canonical
// names.
export const TOOL_ANALYZE_REPO = 'mcp-code-health.analyze_repo';
export const TOOL_GET_ANALYSIS = 'mcp-code-health.get_analysis';
export const TOOL_GET_REPO_ISSUES = 'mcp-code-health.get_repo_issues';
export const TOOL_GENERATE_HEALTH_REPORT = 'mcp-code-health.generate_health_report';
export const TOOL_SUPPRESS_ISSUE = 'mcp-code-health.suppress_issue';

/**
 * CodeHealthRepository is the only place that translates between the
 * MCP wire (snake_case) and the domain types (camelCase). The use case
 * stays framework-free; tests in code-health.repository.spec.ts mock
 * the MCPClientPort so the wire mapping is exercise-able without HTTP.
 */
export class CodeHealthRepository implements CodeHealthPort {
  constructor(private readonly mcp: MCPClientPort) {}

  async analyzeRepo(args: AnalyzeRepoArgs): Promise<AnalyzeRepoResult> {
    const result = await firstValueFrom(
      this.mcp.callTool<AnalyzeRepoResult>(TOOL_ANALYZE_REPO, args),
    );
    return result;
  }

  async getAnalysis(analysis_id: string): Promise<GetAnalysisResult> {
    const wire = await firstValueFrom(
      this.mcp.callTool<Record<string, unknown>>(
        TOOL_GET_ANALYSIS,
        { analysis_id },
      ),
    );
    return mapAnalysis(wire);
  }

  async getRepoIssues(args: GetRepoIssuesArgs): Promise<GetRepoIssuesResult> {
    const wire = await firstValueFrom(
      this.mcp.callTool<{ issues: readonly Record<string, unknown>[] }>(
        TOOL_GET_REPO_ISSUES,
        args,
      ),
    );
    const issues: CodeHealthIssue[] = Array.isArray(wire?.issues)
      ? wire.issues.map((raw) => mapIssue(raw))
      : [];
    return { issues };
  }

  async generateHealthReport(
    args: GenerateHealthReportArgs,
  ): Promise<GenerateHealthReportResult> {
    const wire = await firstValueFrom(
      this.mcp.callTool<GenerateHealthReportResult>(
        TOOL_GENERATE_HEALTH_REPORT,
        args,
      ),
    );
    return { content: String(wire?.content ?? '') };
  }

  async suppressIssue(args: SuppressIssueArgs): Promise<SuppressIssueResult> {
    const wire = await firstValueFrom(
      this.mcp.callTool<Record<string, unknown>>(
        TOOL_SUPPRESS_ISSUE,
        args,
      ),
    );
    return {
      suppression_id: String(wire['suppression_id'] ?? ''),
      suppressed_at: toDate(wire['suppressed_at']),
    };
  }
}

// -- Wire mapping helpers ----------------------------------------------------

/**
 * mapAnalysis translates the `mcp-code-health.get_analysis` payload
 * (snake_case) onto the TS CodeHealthAnalysis surface. Field names that
 * already match in camelCase are unchanged; the only conversions are
 * the ISO-string → Date transforms for the two timestamp columns and
 * the absent-vs-present logic for `completed_at` + `error_message`.
 */
function mapAnalysis(raw: Record<string, unknown>): CodeHealthAnalysis {
  return {
    id: String(raw['id'] ?? ''),
    owner: String(raw['owner'] ?? ''),
    name: String(raw['name'] ?? ''),
    source: (raw['source'] as CodeHealthAnalysis['source']) ?? 'gogs',
    status: (raw['status'] as CodeHealthStatus) ?? 'pending',
    health_score: numberOr(raw['health_score'], 0),
    top_issues: mapIssueList(raw['top_issues']),
    languages: mapStringNumberMap(raw['languages']),
    summary: String(raw['summary'] ?? ''),
    created_at: toDate(raw['created_at']),
    completed_at: optionalDate(raw['completed_at']),
    error_message: optionalString(raw['error_message']),
  };
}

/**
 * mapIssue translates the `mcp-code-health.get_repo_issues` wire (and
 * each entry of `get_analysis.top_issues`) onto the TS domain. Every
 * field is required in the domain so the mapper fills defensive
 * defaults instead of throwing — a malformed finding should not crash
 * the dashboard.
 */
function mapIssue(raw: Record<string, unknown>): CodeHealthIssue {
  return {
    id: String(raw['id'] ?? ''),
    rule_id: String(raw['rule_id'] ?? raw['rule'] ?? ''),
    category: (raw['category'] as CodeHealthIssue['category']) ?? 'refactor',
    severity: (raw['severity'] as CodeHealthSeverity) ?? 'low',
    message: String(raw['message'] ?? ''),
    file_path: String(raw['file_path'] ?? ''),
    line_number: numberOr(raw['line_number'] ?? raw['line'], 0),
    evidence: String(raw['evidence'] ?? ''),
    suggested_fix: String(raw['suggested_fix'] ?? ''),
    confidence: numberOr(raw['confidence'], 1.0),
  };
}

function mapIssueList(raw: unknown): CodeHealthIssue[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .filter((item): item is Record<string, unknown> => item !== null && typeof item === 'object')
    .map(mapIssue);
}

function mapStringNumberMap(raw: unknown): Readonly<Record<string, number>> {
  if (raw === null || typeof raw !== 'object') return {};
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof v === 'number' && Number.isFinite(v)) {
      out[k] = v;
    }
  }
  return out;
}

function toDate(raw: unknown): Date {
  if (raw instanceof Date) return raw;
  if (typeof raw === 'string') {
    const d = new Date(raw);
    if (!Number.isNaN(d.getTime())) return d;
  }
  return new Date();
}

function optionalDate(raw: unknown): Date | undefined {
  if (raw === null || raw === undefined || raw === '') return undefined;
  return toDate(raw);
}

function optionalString(raw: unknown): string | undefined {
  if (raw === null || raw === undefined || raw === '') return undefined;
  return String(raw);
}

function numberOr(raw: unknown, fallback: number): number {
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw;
  if (typeof raw === 'string') {
    const parsed = Number(raw);
    if (Number.isFinite(parsed)) return parsed;
  }
  return fallback;
}
