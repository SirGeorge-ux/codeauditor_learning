// code-health.repository.spec.ts — repository adapter tests for the
// code-health capability. The repository delegates each method to the
// corresponding `mcp-code-health.*` tool via `MCPClientPort.callTool`
// and maps the snake_case wire onto the camelCase TS domain types
// (the only place where the wire conversion lives, per
// design.md §Wire Shapes + the application-layer hexagonal rule).
//
// Hexagonal contract: this spec imports ONLY the repository under test
// + the domain port/model/fake MCP. No @angular/*, no rxjs (the
// repository itself does need rxjs for `firstValueFrom`, but the spec
// asserts on the Promise contract the repository exposes).
import { describe, it, expect, beforeEach } from 'vitest';
import { of, throwError } from 'rxjs';

import { CodeHealthRepository } from './code-health.repository';
import { MCPClientPort, MCPTool } from '../../domain/ports/mcp-client.port';
import { CodeHealthIssue } from '../../domain/models/code-health';

// -- Test doubles ------------------------------------------------------------

interface CapturedCall {
  name: string;
  args: unknown;
}

class FakeMcp implements MCPClientPort {
  captured: CapturedCall[] = [];
  // Default response shapes — per-tool fixtures are set per-test.
  analyzeResult: Record<string, unknown> = {
    analysis_id: 'a-1',
    status: 'pending',
  };
  analysisResult: Record<string, unknown> = {};
  issuesResult: Record<string, unknown> = { issues: [] };
  reportResult: Record<string, unknown> = { content: '' };
  suppressResult: Record<string, unknown> = {
    suppression_id: 's-1',
    suppressed_at: '2026-10-10T12:00:00Z',
  };
  // Per-tool error override (lets a test surface the MCP error path).
  errorFor: { [tool: string]: Error } = {};

  listTools() {
    return of([] as MCPTool[]);
  }
  callTool<T = unknown>(name: string, args: unknown) {
    this.captured.push({ name, args });
    if (this.errorFor[name]) {
      return throwError(() => this.errorFor[name]);
    }
    switch (name) {
      case 'mcp-code-health.analyze_repo':
        return of(this.analyzeResult as unknown as T);
      case 'mcp-code-health.get_analysis':
        return of(this.analysisResult as unknown as T);
      case 'mcp-code-health.get_repo_issues':
        return of(this.issuesResult as unknown as T);
      case 'mcp-code-health.generate_health_report':
        return of(this.reportResult as unknown as T);
      case 'mcp-code-health.suppress_issue':
        return of(this.suppressResult as unknown as T);
      default:
        return of({} as T);
    }
  }
  streamTool<T = unknown>(_name: string, _args: unknown) {
    return of({} as T);
  }
}

// Wire-format fixture builder — use snake_case keys since that is what
// the Go server actually returns on the MCP wire.
function wireAnalysis(partial: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'a-wire-1',
    owner: 'ggogsmic',
    name: 'academy-mic',
    source: 'gogs',
    status: 'completed',
    health_score: 78,
    top_issues: [],
    languages: { go: 0.6, ts: 0.3 },
    summary: 'looks healthy',
    created_at: '2026-10-10T12:00:00Z',
    completed_at: '2026-10-10T12:02:30Z',
    ...partial,
  };
}

function wireIssue(partial: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 'i-1',
    rule_id: 'RULE-SEC-SECRET-001',
    category: 'security',
    severity: 'critical',
    message: 'm',
    file_path: 'src/auth.ts',
    line_number: 14,
    evidence: 'api_key = "x"',
    suggested_fix: 'use env',
    confidence: 1.0,
    ...partial,
  };
}

// -- Tests -------------------------------------------------------------------

describe('CodeHealthRepository', () => {
  let mcp: FakeMcp;
  let repo: CodeHealthRepository;

  beforeEach(() => {
    mcp = new FakeMcp();
    repo = new CodeHealthRepository(mcp);
  });

  describe('analyzeRepo', () => {
    it('calls mcp-code-health.analyze_repo and returns the analysis_id', async () => {
      mcp.analyzeResult = { analysis_id: 'a-99', status: 'pending' };
      const out = await repo.analyzeRepo({
        owner: 'ggogsmic',
        name: 'academy-mic',
        source: 'gogs',
      });
      expect(out.analysis_id).toBe('a-99');
      expect(out.status).toBe('pending');
      expect(mcp.captured[0]?.name).toBe('mcp-code-health.analyze_repo');
      expect(mcp.captured[0]?.args).toEqual({
        owner: 'ggogsmic',
        name: 'academy-mic',
        source: 'gogs',
      });
    });

    it('forwards branch + force when provided', async () => {
      await repo.analyzeRepo({
        owner: 'o',
        name: 'n',
        source: 'github',
        branch: 'main',
        force: true,
      });
      expect(mcp.captured[0]?.args).toEqual({
        owner: 'o',
        name: 'n',
        source: 'github',
        branch: 'main',
        force: true,
      });
    });
  });

  describe('getAnalysis', () => {
    it('maps snake_case wire → camelCase TS', async () => {
      mcp.analysisResult = wireAnalysis({ id: 'a-7', status: 'completed' });
      const out = await repo.getAnalysis('a-7');
      expect(out.id).toBe('a-7');
      expect(out.owner).toBe('ggogsmic');
      expect(out.name).toBe('academy-mic');
      expect(out.source).toBe('gogs');
      expect(out.status).toBe('completed');
      expect(out.health_score).toBe(78);
      expect(out.languages['go']).toBe(0.6);
      expect(out.created_at).toBeInstanceOf(Date);
      expect(out.completed_at).toBeInstanceOf(Date);
      expect(mcp.captured[0]?.name).toBe('mcp-code-health.get_analysis');
      expect(mcp.captured[0]?.args).toEqual({ analysis_id: 'a-7' });
    });

    it('keeps completed_at absent for pending analysis', async () => {
      mcp.analysisResult = wireAnalysis({
        id: 'a-8',
        status: 'pending',
        completed_at: null,
      });
      const out = await repo.getAnalysis('a-8');
      expect(out.completed_at).toBeUndefined();
    });
  });

  describe('getRepoIssues', () => {
    it('maps each issue from wire → domain + forwards filter', async () => {
      const issueWire: Record<string, unknown> = wireIssue({
        id: 'i-1',
        rule_id: 'RULE-SEC-SECRET-001',
        category: 'security',
        severity: 'critical',
        line_number: 17,
      });
      mcp.issuesResult = { issues: [issueWire] };
      const out = await repo.getRepoIssues({
        owner: 'o',
        name: 'n',
        source: 'gogs',
        severity: 'critical',
        limit: 10,
      });
      const first: CodeHealthIssue = out.issues[0]!;
      expect(first.id).toBe('i-1');
      expect(first.rule_id).toBe('RULE-SEC-SECRET-001');
      expect(first.category).toBe('security');
      expect(first.severity).toBe('critical');
      expect(first.line_number).toBe(17);
      expect(mcp.captured[0]?.name).toBe('mcp-code-health.get_repo_issues');
      expect(mcp.captured[0]?.args).toEqual({
        owner: 'o',
        name: 'n',
        source: 'gogs',
        severity: 'critical',
        limit: 10,
      });
    });

    it('returns an empty array when the wire has 0 issues', async () => {
      mcp.issuesResult = { issues: [] };
      const out = await repo.getRepoIssues({
        owner: 'o',
        name: 'n',
        source: 'gogs',
      });
      expect(out.issues).toHaveLength(0);
    });
  });

  describe('generateHealthReport', () => {
    it('forwards analysis_id + format and returns content', async () => {
      mcp.reportResult = { content: '# Code Health\n\nScore: 78' };
      const out = await repo.generateHealthReport({
        analysis_id: 'a-7',
        format: 'md',
      });
      expect(out.content).toContain('Code Health');
      expect(mcp.captured[0]?.name).toBe('mcp-code-health.generate_health_report');
      expect(mcp.captured[0]?.args).toEqual({
        analysis_id: 'a-7',
        format: 'md',
      });
    });
  });

  describe('suppressIssue', () => {
    it('maps the response: suppression_id + suppressed_at → Date', async () => {
      mcp.suppressResult = {
        suppression_id: 's-9',
        suppressed_at: '2026-10-11T08:00:00Z',
      };
      const out = await repo.suppressIssue({ issue_id: 'i-3' });
      expect(out.suppression_id).toBe('s-9');
      expect(out.suppressed_at).toBeInstanceOf(Date);
      expect(out.suppressed_at.toISOString()).toBe('2026-10-11T08:00:00.000Z');
      expect(mcp.captured[0]?.name).toBe('mcp-code-health.suppress_issue');
      expect(mcp.captured[0]?.args).toEqual({ issue_id: 'i-3' });
    });

    it('forwards optional reason', async () => {
      await repo.suppressIssue({ issue_id: 'i-3', reason: 'false positive' });
      expect(mcp.captured[0]?.args).toEqual({
        issue_id: 'i-3',
        reason: 'false positive',
      });
    });
  });

  describe('error propagation', () => {
    it('surfaces MCP errors to the caller', async () => {
      mcp.errorFor['mcp-code-health.analyze_repo'] = new Error('backend 500');
      await expect(repo.analyzeRepo({ owner: 'o', name: 'n', source: 'gogs' })).rejects.toThrow(
        /backend 500/,
      );
    });
  });
});
