// code-health-page.component.ts — top-level page for the S4 code-
// health dashboard (PR 3 of `code-health-dashboard` change).
//
// Composes the 5 dumb components (repo-url-bar + import-button +
// health-gauge + issues-table + category-breakdown) + the
// generate-challenges-cta per issue. Reads `:owner` / `:name` from
// the activated route and triggers CodeHealthService.submitAnalysis
// when the URL bar emits.
//
// Route:  /repo/:owner/:name/code-health
// Hexagonal contract: stateful page component that owns the
// presentation glue. Holds no MCP / use-case references directly —
// the CodeHealthService @Injectable is the seam.
import {
  Component,
  computed,
  effect,
  inject,
  OnDestroy,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import {
  LucideRefreshCw,
  LucideArrowLeft,
  LucideActivity,
} from '@lucide/angular';

import { CodeHealthService } from '../../services/code-health.service';
import { RepoUrlBarComponent } from './repo-url-bar.component';
import { ImportButtonComponent } from './import-button.component';
import { HealthGaugeComponent } from './health-gauge.component';
import { IssuesTableComponent } from './issues-table.component';
import { CategoryBreakdownComponent } from './category-breakdown.component';
import { GenerateChallengesCtaComponent } from './generate-challenges-cta.component';
import {
  AnalyzeRepoArgs,
  CodeHealthPort,
  GenerateHealthReportArgs,
  GetRepoIssuesArgs,
  SuppressIssueArgs,
} from '../../../domain/ports/code-health.port';
import {
  CodeHealthAnalysis,
  CodeHealthCategory,
  CodeHealthIssue,
} from '../../../domain/models/code-health';

@Component({
  selector: 'app-code-health-page',
  standalone: true,
  imports: [
    CommonModule,
    LucideRefreshCw,
    LucideArrowLeft,
    LucideActivity,
    RepoUrlBarComponent,
    ImportButtonComponent,
    HealthGaugeComponent,
    IssuesTableComponent,
    CategoryBreakdownComponent,
    GenerateChallengesCtaComponent,
  ],
  template: `
    <div class="p-6 max-w-6xl mx-auto" data-testid="code-health-page">
      <!-- Header -->
      <div class="flex items-center gap-3 mb-4">
        <svg lucideActivity class="w-7 h-7 text-[#39D353]"></svg>
        <div class="flex-1">
          <h1 class="text-2xl font-bold text-[#C9D1D9]">Code Health</h1>
          <p
            class="text-sm text-[#8B949E]"
            data-testid="owner-name"
          >
            {{ ownerName() }}
          </p>
        </div>
        <button
          type="button"
          class="px-3 py-2 bg-[#21262D] hover:bg-[#30363D] text-[#C9D1D9] text-sm font-medium rounded-sm transition-colors flex items-center gap-1"
          (click)="refresh()"
          data-testid="refresh-button"
        >
          <svg lucideRefreshCw class="w-4 h-4"></svg>
          Re-analyze
        </button>
        <a
          href="/dashboard"
          class="px-3 py-2 bg-[#21262D] hover:bg-[#30363D] text-[#C9D1D9] text-sm rounded-sm transition-colors"
        >
          <svg lucideArrowLeft class="w-4 h-4 inline"></svg>
          Back
        </a>
      </div>

      <!-- Input row -->
      <div class="flex items-stretch gap-3 mb-6">
        <app-repo-url-bar
          class="flex-1"
          [source]="source()"
          (url)="onUrlSubmit($event)"
          data-testid="repo-url-bar-host"
        ></app-repo-url-bar>
        <app-import-button
          [loading]="service.loading()"
          [remainingToday]="service.remainingToday()"
          (clicked)="onImportClicked()"
          data-testid="import-button-host"
        ></app-import-button>
      </div>

      <!-- Error banner -->
      @if (service.error(); as err) {
        <div
          class="bg-[#161B22] border border-[#F85149] rounded-sm p-3 mb-4 flex items-center justify-between"
          data-testid="code-health-error-banner"
        >
          <span class="text-sm text-[#F85149]">{{ err }}</span>
          <button
            type="button"
            class="text-xs text-[#8B949E] hover:text-[#C9D1D9]"
            (click)="service.clearError()"
          >
            Dismiss
          </button>
        </div>
      }

      <!-- Result card row -->
      @if (analysis()) {
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
          <app-health-gauge [score]="analysis()!.health_score"></app-health-gauge>
          <app-category-breakdown [counts]="categoryCounts()"></app-category-breakdown>
        </div>

        <!-- Issues table -->
        <div class="mb-6">
          <app-issues-table
            [issues]="analysis()!.top_issues"
            (suppress)="onSuppress($event)"
            data-testid="issues-table-host"
          ></app-issues-table>
        </div>

        <!-- Per-issue CTAs (rendered inside each row; keep this section
        so the spec wires at least one CTA to the visible issues). -->
        <div
          class="bg-[#161B22] border border-[#30363D] rounded-sm p-4 mb-6"
          data-testid="generate-challenges-cta-row"
        >
          <h3 class="text-xs text-[#8B949E] uppercase tracking-wider mb-3">
            Generate dojo challenges from these issues
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            @for (issue of analysis()!.top_issues.slice(0, 4); track issue.id) {
              <div
                class="flex items-center justify-between bg-[#0D1117] border border-[#21262D] rounded-sm p-2"
                [attr.data-testid]="'cta-row-' + issue.id"
              >
                <span class="text-xs text-[#C9D1D9] truncate pr-2">
                  {{ issue.rule_id }} · {{ issue.file_path }}:{{ issue.line_number }}
                </span>
                <app-generate-challenges-cta
                  [issue]="issue"
                  [language]="primaryLanguage()"
                ></app-generate-challenges-cta>
              </div>
            }
          </div>
        </div>

        <!-- Markdown report -->
        @if (reportContent(); as report) {
          <div
            class="bg-[#161B22] border border-[#30363D] rounded-sm p-4"
            data-testid="code-health-report"
          >
            <h3 class="text-xs text-[#8B949E] uppercase tracking-wider mb-3">
              Markdown report
            </h3>
            <pre class="text-xs text-[#C9D1D9] whitespace-pre-wrap">{{ report }}</pre>
          </div>
        }
      } @else if (service.loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]" data-testid="loading-banner">
            Analyzing repository… (up to 5 min)
          </div>
        </div>
      } @else {
        <div
          class="flex flex-col items-center justify-center py-12 bg-[#161B22] border border-[#30363D] rounded-sm"
          data-testid="empty-state"
        >
          <svg lucideActivity class="w-12 h-12 text-[#30363D] mb-3"></svg>
          <p class="text-sm text-[#8B949E] mb-2">
            Pick a Gogs or GitHub repo to analyze.
          </p>
          <p class="text-xs text-[#8B949E]">
            You have {{ service.remainingToday() }} analyses left today.
          </p>
        </div>
      }
    </div>
  `,
})
export class CodeHealthPageComponent implements OnDestroy {
  service = inject(CodeHealthService);
  private readonly route = inject(ActivatedRoute);

  // Read :owner + :name from ActivatedRoute.paramMap. The convertTo-
  // ParamMap snapshot is read on init; the toSignal() bridge would
  // also work for navigation events.
  private readonly params = toSignal(this.route.paramMap, {
    initialValue: this.route.snapshot.paramMap,
  });

  // Defaults to 'gogs' — the page lets the user toggle source via
  // the URL bar; for an MVP we default to the agent's primary host.
  readonly source = signal<'gogs' | 'github'>('gogs');
  readonly reportContent = signal<string | null>(null);

  readonly owner = computed(() => this.params().get('owner') ?? '');
  readonly name = computed(() => this.params().get('name') ?? '');
  readonly ownerName = computed(
    () => `${this.owner()}/${this.name()}`,
  );

  readonly analysis = this.service.analysis;

  // categoryCounts rolls up the top_issues by category. Drives the
  // <app-category-breakdown> bar chart.
  readonly categoryCounts = computed<
    Readonly<Record<CodeHealthCategory, number>>
  >(() => {
    const a = this.analysis();
    const counts: Record<CodeHealthCategory, number> = {
      security: 0,
      performance: 0,
      refactor: 0,
      style: 0,
      architecture: 0,
    };
    if (!a) return counts;
    for (const issue of a.top_issues) {
      counts[issue.category] = (counts[issue.category] ?? 0) + 1;
    }
    return counts;
  });

  // primaryLanguage surfaces the language with the highest
  // representation in the language breakdown. Used as the default
  // for the generate-challenge CTA buttons.
  readonly primaryLanguage = computed<string>(() => {
    const langs = this.service.languages() ?? {};
    let best = 'typescript';
    let bestN = 0;
    for (const [lang, n] of Object.entries(langs)) {
      if (typeof n === 'number' && n > bestN) {
        best = lang;
        bestN = n;
      }
    }
    return best;
  });

  // Stop polling if the user navigates away mid-analysis.
  private readonly stopOnDestroy = effect(() => {
    // Touch the analysis signal so the effect re-runs on every
    // update — required by Angular's effect() signature.
    this.analysis();
  });

  ngOnDestroy(): void {
    // CodeHealthUseCase exposes a stop helper; if it doesn't, the
    // in-flight promise simply resolves on its own.
    const maybe = (this.service as unknown as { useCase?: {
      stopPolling?: () => void;
    } }).useCase;
    if (maybe && typeof maybe.stopPolling === 'function') {
      maybe.stopPolling();
    }
  }

  // -- Page-level commands ----------------------------------------------------

  onUrlSubmit(event: { url: string }): void {
    this.submit({ ...this.parseUrl(event.url) });
  }

  onImportClicked(): void {
    if (this.owner().length === 0 || this.name().length === 0) {
      return;
    }
    this.submit({
      owner: this.owner(),
      name: this.name(),
      source: this.source(),
    });
  }

  onSuppress(event: { issue_id: string }): void {
    void this.service.suppressIssue(event);
  }

  refresh(): void {
    if (this.owner().length === 0 || this.name().length === 0) {
      return;
    }
    this.submit({
      owner: this.owner(),
      name: this.name(),
      source: this.source(),
      force: true,
    });
  }

  // -- Internals --------------------------------------------------------------

  private submit(args: AnalyzeRepoArgs): void {
    this.reportContent.set(null);
    void this.service.submitAnalysis(args).then(async (final) => {
      if (final) {
        const md = await this.service.generateHealthReport({
          analysis_id: final.id,
          format: 'md',
        });
        this.reportContent.set(md ?? null);
      }
    });
  }

  // parseUrl extracts owner/name from a typed URL.
  private parseUrl(url: string): AnalyzeRepoArgs {
    const trimmed = url.trim();
    if (this.source() === 'github') {
      const stripped = trimmed.replace(/^https?:\/\//, '').replace(
        /^github\.com\//,
        '',
      );
      const [owner, name] = stripped.split('/');
      return {
        owner: owner ?? '',
        name: name ?? '',
        source: 'github',
      };
    }
    // gogs://owner/name OR owner/name
    const stripped = trimmed.replace(/^gogs:\/\//, '');
    const [owner, name] = stripped.split('/');
    return {
      owner: owner ?? '',
      name: name ?? '',
      source: 'gogs',
    };
  }
}
