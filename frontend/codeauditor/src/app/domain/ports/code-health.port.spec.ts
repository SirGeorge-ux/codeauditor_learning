// code-health.port.spec.ts — compile-time contract tests for the
// CodeHealthPort domain interface. Mirrors the chat.spec.ts +
// tutor-chat.port.spec.ts pattern: a fake implementation is enough to
// prove the interface shape is usable; runtime behavior is tested in
// the repository spec (infrastructure/repositories/code-health.repository.spec.ts)
// and the use-case spec (application/code-health.use-case.spec.ts).
import { describe, it, expect } from 'vitest';

import {
  CodeHealthPort,
  AnalyzeRepoArgs,
  GetRepoIssuesArgs,
  GenerateHealthReportArgs,
  SuppressIssueArgs,
  AnalyzeRepoResult,
  GetAnalysisResult,
  GetRepoIssuesResult,
  GenerateHealthReportResult,
  SuppressIssueResult,
} from './code-health.port';
import { CodeHealthAnalysis, CodeHealthIssue } from '../models/code-health';

describe('CodeHealthPort', () => {
  it('accepts a fake implementation that satisfies the interface', () => {
    const fake: CodeHealthPort = {
      analyzeRepo: async (_args: AnalyzeRepoArgs): Promise<AnalyzeRepoResult> => ({
        analysis_id: 'a-1',
        status: 'pending',
      }),
      getAnalysis: async (_id: string): Promise<GetAnalysisResult> =>
        makeAnalysis({ id: _id, status: 'completed' }),
      getRepoIssues: async (_args: GetRepoIssuesArgs): Promise<GetRepoIssuesResult> => ({
        issues: [],
      }),
      generateHealthReport: async (
        _args: GenerateHealthReportArgs,
      ): Promise<GenerateHealthReportResult> => ({ content: '' }),
      suppressIssue: async (_args: SuppressIssueArgs): Promise<SuppressIssueResult> => ({
        suppression_id: 's-1',
        suppressed_at: new Date(),
      }),
    };
    expect(typeof fake.analyzeRepo).toBe('function');
    expect(typeof fake.getAnalysis).toBe('function');
    expect(typeof fake.getRepoIssues).toBe('function');
    expect(typeof fake.generateHealthReport).toBe('function');
    expect(typeof fake.suppressIssue).toBe('function');
  });

  it('analyzeRepo resolves to {analysis_id, status:"pending"}', async () => {
    const fake: CodeHealthPort = {
      analyzeRepo: async () => ({ analysis_id: 'a-42', status: 'pending' }),
      getAnalysis: async () => makeAnalysis({}),
      getRepoIssues: async () => ({ issues: [] }),
      generateHealthReport: async () => ({ content: '' }),
      suppressIssue: async () => ({
        suppression_id: 's-1',
        suppressed_at: new Date(),
      }),
    };
    const out = await fake.analyzeRepo({
      owner: 'o',
      name: 'n',
      source: 'gogs',
    });
    expect(out.analysis_id).toBe('a-42');
    expect(out.status).toBe('pending');
  });

  it('getRepoIssues resolves to {issues: CodeHealthIssue[]}', async () => {
    const fake: CodeHealthPort = {
      analyzeRepo: async () => ({ analysis_id: 'a', status: 'pending' }),
      getAnalysis: async () => makeAnalysis({}),
      getRepoIssues: async () => ({
        issues: [
          {
            id: 'i-1',
            rule_id: 'RULE-SEC-SECRET-001',
            category: 'security',
            severity: 'critical',
            message: 'm',
            file_path: 'x.ts',
            line_number: 1,
            evidence: 'e',
            suggested_fix: 'f',
            confidence: 1.0,
          },
        ],
      }),
      generateHealthReport: async () => ({ content: '' }),
      suppressIssue: async () => ({
        suppression_id: 's',
        suppressed_at: new Date(),
      }),
    };
    const out = await fake.getRepoIssues({
      owner: 'o',
      name: 'n',
      source: 'gogs',
      severity: 'critical',
      limit: 10,
    });
    expect(out.issues).toHaveLength(1);
    const first: CodeHealthIssue = out.issues[0]!;
    expect(first.rule_id).toBe('RULE-SEC-SECRET-001');
  });

  it('suppressIssue resolves to {suppression_id, suppressed_at}', async () => {
    const ts = new Date('2026-10-10T12:00:00Z');
    const fake: CodeHealthPort = {
      analyzeRepo: async () => ({ analysis_id: 'a', status: 'pending' }),
      getAnalysis: async () => makeAnalysis({}),
      getRepoIssues: async () => ({ issues: [] }),
      generateHealthReport: async () => ({ content: '' }),
      suppressIssue: async () => ({ suppression_id: 's-9', suppressed_at: ts }),
    };
    const out = await fake.suppressIssue({ issue_id: 'i-3' });
    expect(out.suppression_id).toBe('s-9');
    expect(out.suppressed_at).toBe(ts);
  });
});

function makeAnalysis(partial: Partial<CodeHealthAnalysis>): CodeHealthAnalysis {
  return {
    id: 'a-default',
    owner: 'o',
    name: 'n',
    source: 'gogs',
    status: 'pending',
    health_score: 0,
    top_issues: [],
    languages: {},
    summary: '',
    created_at: new Date(),
    ...partial,
  };
}
