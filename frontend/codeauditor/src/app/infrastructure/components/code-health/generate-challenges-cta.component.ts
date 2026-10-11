// generate-challenges-cta.component.ts — per-issue CTA that turns a
// code-health issue into a dojo challenge.
//
// Calls `mcp-challenges.generate_challenge({sourceIssueId, language})`
// (per PR 2's `SourceIssueID` wire arg) and navigates to
// `/dojo/<id>` on success. The backend persists the resulting
// Challenge with `origin="code_health"` (per
// openspec/changes/code-health-dashboard/specs/challenges/
// REQ-CHALLENGES-CODE-HEALTH-001/002/003).
//
// Hexagonal contract: standalone component, dumb presentation. Holds
// the MCPClientPort reference (infrastructure concern is OK at the
// CTA level — it's not a service; it's wiring that the page could
// pass through if preferred, but Angular providers make the direct
// inject less verbose for a single-click CTA).
import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { Router } from '@angular/router';
import { LucideSparkles, LucideLoader } from '@lucide/angular';
import { firstValueFrom } from 'rxjs';

import { CodeHealthIssue } from '../../../domain/models/code-health';
import { MCPClientAdapter } from '../../../infrastructure/adapters/mcp-client.adapter';

@Component({
  selector: 'app-generate-challenges-cta',
  standalone: true,
  imports: [LucideSparkles, LucideLoader],
  template: `
    <button
      type="button"
      class="px-3 py-1.5 bg-[#39D353] hover:bg-[#2EA043] disabled:opacity-50 text-[#0D1117] text-sm font-semibold rounded-sm transition-colors flex items-center gap-1"
      [disabled]="loading()"
      (click)="invoke()"
      data-testid="generate-cta"
    >
      @if (loading()) {
        <svg lucideLoader class="w-4 h-4 animate-spin"></svg>
      } @else {
        <svg lucideSparkles class="w-4 h-4"></svg>
      }
      {{ loading() ? 'Generating…' : 'Generate challenge' }}
    </button>
    @if (error()) {
      <span class="text-xs text-[#F85149] ml-2" data-testid="generate-cta-error">
        {{ error() }}
      </span>
    }
  `,
})
export class GenerateChallengesCtaComponent {
  readonly issue = input.required<CodeHealthIssue>();
  readonly language = input.required<string>();

  private readonly mcp = inject(MCPClientAdapter);
  private readonly router = inject(Router);

  readonly loading = signal<boolean>(false);
  readonly error = signal<string | null>(null);

  // Reset error when the issue input changes (the user picks a
  // different issue in the table).
  private readonly issueIdEffect = effect(() => {
    this.issue();
    this.error.set(null);
    this.loading.set(false);
  });

  readonly label = computed(() => (this.loading() ? 'Generating…' : 'Generate challenge'));

  async invoke(): Promise<void> {
    if (this.loading()) {
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    try {
      const out = (await firstValueFrom(
        this.mcp.callTool('mcp-challenges.generate_challenge', {
          sourceIssueId: this.issue().id,
          language: this.language(),
        }),
      )) as { id?: string };
      const id = out?.id;
      if (typeof id === 'string' && id.length > 0) {
        await this.router.navigate(['/dojo', id]);
      } else {
        this.error.set('Backend did not return a challenge id.');
      }
    } catch (e) {
      this.error.set(errMsg(e, 'Could not generate challenge.'));
    } finally {
      this.loading.set(false);
    }
  }
}

function errMsg(e: unknown, fallback: string): string {
  if (e instanceof Error && e.message.length > 0) return e.message;
  return fallback;
}
