// code-health.spec.ts — domain type contract tests for the code-health
// capability. The types under test live in domain/models/code-health.ts
// and must remain framework-free (zero @angular/* imports per AGENTS.md
// §3.2 / §9). The shape mirrors the wire of `mcp-code-health.*` (see
// openspec/changes/code-health-dashboard/design.md §Wire Shapes +
// specs/code-health/spec.md + specs/mcp/spec.md REQ-MCP-CODE-HEALTH-001).
//
// RED state at HEAD~1: this file imports `CodeHealthAnalysis`,
// `CodeHealthIssue`, etc. from a non-existent module — vitest fails to
// load the suite before any assertion fires. GREEN: the module exists
// and every fixture compiles + the union-literal narrowing assertions
// pass.
import { describe, it, expect } from 'vitest';

import {
  CodeHealthAnalysis,
  CodeHealthIssue,
  AnalyzeRepoArgs,
  HealthReport,
  CODE_HEALTH_CATEGORIES,
  CODE_HEALTH_SEVERITIES,
  CODE_HEALTH_SOURCES,
  CODE_HEALTH_STATUSES,
  isCodeHealthCategory,
  isCodeHealthSeverity,
  isCodeHealthSource,
  isCodeHealthStatus,
} from './code-health';

describe('CodeHealthIssue', () => {
  it('accepts a canonical critical security fixture', () => {
    const issue: CodeHealthIssue = {
      id: 'issue-3',
      rule_id: 'RULE-SEC-SECRET-001',
      category: 'security',
      severity: 'critical',
      message: 'Hardcoded API key detected',
      file_path: 'src/api/auth.ts',
      line_number: 14,
      evidence: 'api_key = "abc123"',
      suggested_fix: 'Move to env variable',
      confidence: 1.0,
    };
    expect(issue.severity).toBe('critical');
    expect(issue.category).toBe('security');
    expect(issue.confidence).toBe(1.0);
  });

  it('keeps every severity in {critical, high, medium, low} assignable', () => {
    for (const sev of CODE_HEALTH_SEVERITIES) {
      const issue: CodeHealthIssue = {
        id: `i-${sev}`,
        rule_id: `RULE-X-${sev}`,
        category: 'refactor',
        severity: sev,
        message: `issue ${sev}`,
        file_path: 'x.ts',
        line_number: 1,
        evidence: 'ev',
        suggested_fix: 'fix',
        confidence: 0.5,
      };
      expect(issue.severity).toBe(sev);
    }
  });

  it('keeps every category in the 5-category union', () => {
    for (const cat of CODE_HEALTH_CATEGORIES) {
      const issue: CodeHealthIssue = {
        id: `i-${cat}`,
        rule_id: `RULE-X-${cat}`,
        category: cat,
        severity: 'low',
        message: `issue ${cat}`,
        file_path: 'x.ts',
        line_number: 1,
        evidence: 'ev',
        suggested_fix: 'fix',
        confidence: 0.5,
      };
      expect(issue.category).toBe(cat);
    }
  });
});

describe('CodeHealthAnalysis', () => {
  it('accepts a completed analysis with score + top_issues + languages', () => {
    const top: CodeHealthIssue = {
      id: 'i-1',
      rule_id: 'RULE-SEC-SECRET-001',
      category: 'security',
      severity: 'critical',
      message: 'msg',
      file_path: 'x.ts',
      line_number: 1,
      evidence: 'ev',
      suggested_fix: 'fix',
      confidence: 1.0,
    };
    const analysis: CodeHealthAnalysis = {
      id: 'a-1',
      owner: 'ggogsmic',
      name: 'academy-mic',
      source: 'gogs',
      status: 'completed',
      health_score: 78,
      top_issues: [top],
      languages: { go: 0.6, ts: 0.3 },
      summary: 'Looks healthy overall',
      created_at: new Date('2026-10-10T12:00:00Z'),
      completed_at: new Date('2026-10-10T12:02:30Z'),
    };
    expect(analysis.health_score).toBe(78);
    expect(analysis.top_issues).toHaveLength(1);
    expect(analysis.languages.go).toBe(0.6);
  });

  it('keeps completed_at + error_message optional for pending/running states', () => {
    const pending: CodeHealthAnalysis = {
      id: 'a-2',
      owner: 'ggogsmic',
      name: 'academy-mic',
      source: 'github',
      status: 'pending',
      health_score: 0,
      top_issues: [],
      languages: {},
      summary: '',
      created_at: new Date('2026-10-10T12:00:00Z'),
    };
    expect(pending.completed_at).toBeUndefined();
    expect(pending.error_message).toBeUndefined();
  });

  it('keeps every status in the {pending, running, completed, failed} union', () => {
    for (const status of CODE_HEALTH_STATUSES) {
      const a: CodeHealthAnalysis = {
        id: `a-${status}`,
        owner: 'o',
        name: 'n',
        source: 'gogs',
        status,
        health_score: 0,
        top_issues: [],
        languages: {},
        summary: '',
        created_at: new Date(),
      };
      expect(a.status).toBe(status);
    }
  });
});

describe('AnalyzeRepoArgs', () => {
  it('carries the 4 core fields with branch + force optional', () => {
    const args: AnalyzeRepoArgs = {
      owner: 'ggogsmic',
      name: 'academy-mic',
      source: 'gogs',
    };
    expect(args.branch).toBeUndefined();
    expect(args.force).toBeUndefined();

    const full: AnalyzeRepoArgs = {
      owner: 'anomalyco',
      name: 'codeauditor',
      source: 'github',
      branch: 'master',
      force: true,
    };
    expect(full.branch).toBe('master');
    expect(full.force).toBe(true);
  });
});

describe('HealthReport', () => {
  it('accepts the md + pdf return shape', () => {
    const md: HealthReport = {
      report_id: 'r-1',
      content: '# Code Health\n\nScore: 78',
      format: 'md',
    };
    expect(md.format).toBe('md');
    const pdf: HealthReport = {
      report_id: 'r-2',
      content: '%PDF-1.4',
      format: 'pdf',
    };
    expect(pdf.format).toBe('pdf');
  });
});

describe('narrowing helpers', () => {
  it('isCodeHealthSeverity accepts the 4 severities only', () => {
    for (const sev of CODE_HEALTH_SEVERITIES) {
      expect(isCodeHealthSeverity(sev)).toBe(true);
    }
    for (const bad of ['extreme', 'CRITICAL', 'minor', '']) {
      expect(isCodeHealthSeverity(bad)).toBe(false);
    }
  });

  it('isCodeHealthCategory accepts the 5 categories only', () => {
    for (const cat of CODE_HEALTH_CATEGORIES) {
      expect(isCodeHealthCategory(cat)).toBe(true);
    }
    for (const bad of ['concurrency', 'SECURITY', 'test', '']) {
      expect(isCodeHealthCategory(bad)).toBe(false);
    }
  });

  it('isCodeHealthSource accepts gogs + github only', () => {
    for (const s of CODE_HEALTH_SOURCES) {
      expect(isCodeHealthSource(s)).toBe(true);
    }
    for (const bad of ['gitlab', 'GOGS', 'bitbucket', '']) {
      expect(isCodeHealthSource(bad)).toBe(false);
    }
  });

  it('isCodeHealthStatus accepts the 4 statuses only', () => {
    for (const s of CODE_HEALTH_STATUSES) {
      expect(isCodeHealthStatus(s)).toBe(true);
    }
    for (const bad of ['queued', 'COMPLETED', 'canceled', '']) {
      expect(isCodeHealthStatus(bad)).toBe(false);
    }
  });
});
