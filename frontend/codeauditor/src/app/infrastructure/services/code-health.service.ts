// code-health.service.ts — Angular @Injectable wrapper around the
// framework-free CodeHealthUseCase. Mirrors the PracticeService /
// DictionaryService pattern: the use case stays framework-free, this
// service is the thin Angular adapter that injects the concrete
// MCPClientAdapter and instantiates the repository + use case.
//
// Hexagonal contract: this is the only `@Injectable` on the code-health
// surface. Components depend on this single service via `inject()`;
// the repository + use case are constructed here in one place so a
// future swap to a different MCP port or storage primitive changes one
// file, not every component.
import { Injectable, inject, computed } from '@angular/core';

import { CodeHealthUseCase } from '../../application/code-health.use-case';
import { CodeHealthRepository } from '../repositories/code-health.repository';
import { MCPClientAdapter } from '../adapters/mcp-client.adapter';
import {
  CodeHealthAnalysis,
  CodeHealthIssue,
} from '../../domain/models/code-health';
import {
  AnalyzeRepoArgs,
  GenerateHealthReportArgs,
  GetRepoIssuesArgs,
  SuppressIssueArgs,
} from '../../domain/ports/code-health.port';

@Injectable({ providedIn: 'root' })
export class CodeHealthService {
  private readonly useCase: CodeHealthUseCase;

  // Public signals — components bind to these via readonly computed
  // views so a use case refactor stays contained.
  readonly analysis = computed(() => this.useCase.analysis());
  readonly loading = computed(() => this.useCase.loading());
  readonly error = computed(() => this.useCase.error());
  readonly remainingToday = computed(() => this.useCase.remainingToday());
  readonly topIssues = computed(() => this.useCase.topIssues());
  readonly languages = computed(() => this.useCase.languages());

  constructor() {
    const mcp = inject(MCPClientAdapter);
    const repo = new CodeHealthRepository(mcp);
    this.useCase = new CodeHealthUseCase({ port: repo });
  }

  // -- Page commands ---------------------------------------------------------

  submitAnalysis(args: AnalyzeRepoArgs): Promise<CodeHealthAnalysis | null> {
    return this.useCase.submitAnalysis(args);
  }

  pollAnalysis(analysisId: string): Promise<CodeHealthAnalysis | null> {
    return this.useCase.pollAnalysis(analysisId);
  }

  loadRepoIssues(args: GetRepoIssuesArgs): Promise<CodeHealthIssue[]> {
    return this.useCase.loadRepoIssues(args);
  }

  generateHealthReport(
    args: GenerateHealthReportArgs,
  ): Promise<string | null> {
    return this.useCase.generateHealthReport(args);
  }

  suppressIssue(args: SuppressIssueArgs): Promise<boolean> {
    return this.useCase.suppressIssue(args);
  }

  clearError(): void {
    this.useCase.clearError();
  }
}
