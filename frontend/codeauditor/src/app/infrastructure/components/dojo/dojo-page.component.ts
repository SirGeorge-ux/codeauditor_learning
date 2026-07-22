import { Component, computed, inject, OnInit, signal, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { ContextPanelComponent } from '../shared/context-panel.component';
import { CodePanelComponent } from '../shared/code-panel.component';
import { TerminalPanelComponent } from '../shared/terminal-panel.component';
import { ResizeDirective } from '../shared/resize.directive';
import { AuditService } from '../../services/audit.service';
import { ChallengeService } from '../../services/challenge.service';
import { Challenge, Hint } from '../../domain/models/challenge';
import { ScoreBreakdown } from '../../domain/models/score-breakdown';

@Component({
  selector: 'app-dojo-page',
  standalone: true,
  imports: [
    CommonModule,
    ContextPanelComponent,
    CodePanelComponent,
    TerminalPanelComponent,
    ResizeDirective,
  ],
  template: `
    <!-- Top bar -->
    <div
      class="h-10 flex items-center justify-between px-4 bg-dojo-surface border-b border-dojo-border"
    >
      <span class="text-xs text-dojo-text">
        @if (challengeService.selectedChallengeSignal()) {
          Auditando: {{ challengeService.selectedChallengeSignal()?.title }}
        } @else {
          No challenge selected
        }
      </span>
      <button
        (click)="auditChallenge()"
        [disabled]="!challengeService.selectedChallengeSignal() || isAuditing()"
        class="px-3 py-1 text-xs rounded-sm transition-colors"
        [class.bg-blue-600]="!isAuditing()"
        [class.bg-blue-800]="isAuditing()"
        [class.text-white]="true"
        [class.opacity-50]="!challengeService.selectedChallengeSignal()"
        [class.cursor-not-allowed]="!challengeService.selectedChallengeSignal()"
      >
        @if (isAuditing()) {
          Auditando...
        } @else {
          Auditar
        }
      </button>
    </div>

    <div class="h-full flex bg-dojo-base">
      @if (challengeService.loadingSignal()) {
        <div class="flex-1 flex items-center justify-center">
          <div class="text-dojo-text opacity-70">Loading challenge...</div>
        </div>
      } @else if (challengeService.selectedChallengeSignal() === null && hasId) {
        <div class="flex-1 flex items-center justify-center">
          <div class="text-dojo-error">Challenge not found</div>
        </div>
      } @else {
        <!-- Left: Context + Hints Panel (resizable) -->
        <div
          class="overflow-hidden border-r border-dojo-border flex flex-col"
          appResize
          [minWidth]="250"
          [maxWidth]="500"
          [initialWidth]="320"
        >
          <div class="flex-1 overflow-y-auto">
            <app-context-panel
              [challenge]="challengeService.selectedChallengeSignal()"
            ></app-context-panel>

            <!-- Progressive Hints Panel -->
            @if (hints().length > 0) {
              <div class="border-t border-dojo-border p-4 space-y-3">
                <h3 class="text-sm font-semibold text-dojo-text">Hints</h3>

                <p class="text-xs text-dojo-text opacity-70">
                  Progressive hints — reveal only what you need. Each hint costs points.
                </p>

                @for (hint of hints(); track hint.level) {
                  @if (canRevealHint(hint.level)) {
                    <!-- Revealable hint card -->
                    <div class="rounded-sm border border-dojo-border bg-dojo-surface p-3">
                      @if (isHintRevealed(hint.level)) {
                        <!-- Revealed content -->
                        <div class="space-y-1">
                          <div class="flex items-center justify-between">
                            <span class="text-xs font-semibold text-dojo-accent">
                              Hint {{ hint.level }}
                            </span>
                            <span class="text-xs text-dojo-text opacity-70">
                              @if (hint.costPoints > 0) {
                                -{{ hint.costPoints }} pts
                              } @else {
                                free
                              }
                            </span>
                          </div>
                          <p class="text-xs text-dojo-text">{{ hint.content }}</p>
                        </div>
                      } @else {
                        <!-- Locked — reveal button -->
                        <button
                          (click)="revealHint(hint.level)"
                          class="w-full text-left flex items-center justify-between text-xs text-dojo-text hover:text-dojo-accent transition-colors"
                        >
                          <span>Reveal hint {{ hint.level }}</span>
                          @if (hint.costPoints > 0) {
                            <span class="text-dojo-warning font-medium">
                              -{{ hint.costPoints }} pts
                            </span>
                          } @else {
                            <span class="text-dojo-accent font-medium">free</span>
                          }
                        </button>
                      }
                    </div>
                  }
                }

                <!-- Running penalty tally -->
                @if (hintsPenalty() > 0) {
                  <div class="text-xs text-dojo-warning pt-1">
                    Total hint penalty: -{{ hintsPenalty() }} pts
                  </div>
                }
              </div>
            }
          </div>
        </div>

        <!-- Right: Code + Terminal split -->
        <div class="flex-1 flex flex-col overflow-hidden">
          <div class="flex-1 overflow-hidden border-b border-dojo-border">
            <app-code-panel
              [code]="challengeService.selectedChallengeSignal()?.code ?? ''"
              [language]="challengeService.selectedChallengeSignal()?.language ?? 'typescript'"
              [readOnly]="true"
            ></app-code-panel>
          </div>
          <div class="h-48 overflow-hidden">
            <app-terminal-panel #termPanel></app-terminal-panel>
          </div>
        </div>
      }
    </div>

    <!-- Score Breakdown Overlay — appears after audit completes -->
    @if (scoreBreakdown(); as breakdown) {
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-dojo-base/80"
        (click)="dismissScoreBreakdown()"
      >
        <div
          class="bg-dojo-surface border border-dojo-border rounded-sm p-6 w-full max-w-sm shadow-lg"
          (click)="$event.stopPropagation()"
        >
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-base font-semibold text-dojo-text">Score Breakdown</h2>
            <button
              (click)="dismissScoreBreakdown()"
              class="text-dojo-text opacity-60 hover:opacity-100 text-sm"
            >
              ×
            </button>
          </div>

          <div class="space-y-2 font-mono text-sm">
            <div class="flex justify-between text-dojo-text">
              <span>Base points</span>
              <span class="text-dojo-accent">+{{ breakdown.basePoints }}</span>
            </div>
            <div class="flex justify-between text-dojo-text">
              <span
                >Test points
                <span class="text-dojo-text opacity-50 text-xs">(Σ pass × weight × 30)</span></span
              >
              <span class="text-dojo-accent">+{{ breakdown.testPoints }}</span>
            </div>
            <div class="flex justify-between text-dojo-text">
              <span
                >Lint points
                <span class="text-dojo-text opacity-50 text-xs">(Σ clean × 20)</span></span
              >
              <span class="text-dojo-accent">+{{ breakdown.lintPoints }}</span>
            </div>
            <div class="flex justify-between text-dojo-text">
              <span
                >Findings matched
                <span class="text-dojo-text opacity-50 text-xs">(Σ × 10)</span></span
              >
              <span class="text-dojo-accent">+{{ breakdown.findingsMatched }}</span>
            </div>
            <div class="flex justify-between text-dojo-text">
              <span
                >Hints penalty
                <span class="text-dojo-text opacity-50 text-xs">(Σ used × cost)</span></span
              >
              <span class="text-dojo-error">-{{ breakdown.hintsPenalty }}</span>
            </div>
            <div class="flex justify-between text-dojo-text">
              <span>Time bonus</span>
              <span class="text-dojo-accent">+{{ breakdown.timeBonus }}</span>
            </div>

            <div class="border-t border-dojo-border my-3"></div>

            <div class="flex justify-between text-base font-semibold">
              <span class="text-dojo-text">TOTAL</span>
              <span class="text-dojo-accent">{{ breakdown.total }}</span>
            </div>
          </div>

          @if (!scoringFromBackend()) {
            <p class="text-xs text-dojo-text opacity-50 mt-4 leading-relaxed">
              Test, lint, and findings points require server-side scoring (not yet wired into the
              audit SSE stream). Base, hints penalty, and time bonus are computed locally.
            </p>
          }
        </div>
      </div>
    }
  `,
})
export class DojoPageComponent implements OnInit {
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  challengeService = inject(ChallengeService);
  private auditService = inject(AuditService);

  @ViewChild('termPanel', { static: false }) terminalPanel!: TerminalPanelComponent;

  hasId = false;
  isAuditing = signal(false);

  // Progressive hints state
  private revealedHints = signal<Set<number>>(new Set());

  // Audit timing — captured locally for the time bonus computation
  private auditStartedAt: number | null = null;
  private auditElapsedMinutes = 0;

  // Score breakdown shown after audit completes
  scoreBreakdown = signal<ScoreBreakdown | null>(null);

  // Whether the breakdown came from the backend (true) or was
  // computed locally (false — partial). Used to inform the user.
  scoringFromBackend = signal(false);

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
    if (id) {
      this.hasId = true;
      this.challengeService.selectChallenge(id);
    }
  }

  // Visible hints array (only the levels unlocked so far are visible)
  hints = computed<Hint[]>(() => {
    const challenge = this.challengeService.selectedChallengeSignal();
    return challenge?.hints ?? [];
  });

  hintsPenalty = computed<number>(() => {
    const used = this.revealedHints();
    return this.hints()
      .filter((h) => used.has(h.level))
      .reduce((sum, h) => sum + h.costPoints, 0);
  });

  hintsUsedCount = computed<number>(() => this.revealedHints().size);

  // Progressive unlock: a hint level N is revealable when N === 1 OR
  // level N-1 has already been revealed. Already-revealed hints stay
  // visible (handled by the template via isHintRevealed).
  canRevealHint(level: number): boolean {
    if (level === 1) return true;
    return this.revealedHints().has(level - 1);
  }

  isHintRevealed(level: number): boolean {
    return this.revealedHints().has(level);
  }

  revealHint(level: number): void {
    // Enforce sequential unlock — never reveal N before N-1
    if (!this.canRevealHint(level)) return;
    if (this.isHintRevealed(level)) return;
    const next = new Set(this.revealedHints());
    next.add(level);
    this.revealedHints.set(next);
  }

  auditChallenge(): void {
    const challenge = this.challengeService.selectedChallengeSignal();
    if (!challenge || this.isAuditing()) return;

    this.isAuditing.set(true);
    this.terminalPanel?.clear();
    this.terminalPanel?.write('\x1b[36mStarting audit...\x1b[0m\n\r');
    this.auditStartedAt = Date.now();
    this.scoreBreakdown.set(null);

    this.auditService.runAudit(challenge.code, challenge.language, challenge.id).subscribe({
      next: (event) => {
        if (event.type === 'stdout') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write((payload.data ?? event.data) + '\n\r');
          } catch {
            this.terminalPanel?.write(event.data + '\n\r');
          }
        } else if (event.type === 'stderr') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write('\x1b[33m' + (payload.data ?? event.data) + '\x1b[0m\n\r');
          } catch {
            this.terminalPanel?.write('\x1b[33m' + event.data + '\x1b[0m\n\r');
          }
        } else if (event.type === 'error') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write(
              '\x1b[31mError: ' + (payload.message ?? event.data) + '\x1b[0m\n\r',
            );
          } catch {
            this.terminalPanel?.write('\x1b[31mError: ' + event.data + '\x1b[0m\n\r');
          }
        } else if (event.type === 'llm_token') {
          this.terminalPanel?.write('\x1b[35m' + event.data + '\x1b[0m');
        } else if (event.type === 'llm_analysis') {
          this.terminalPanel?.write('\n\r\x1b[35m━━━ AI Analysis ━━━\x1b[0m\n\r');
          this.terminalPanel?.write('\x1b[35m' + event.data + '\x1b[0m\n\r');
          this.terminalPanel?.write('\x1b[35m━━━━━━━━━━━━━━━━━━\x1b[0m\n\r');
        } else if (event.type === 'llm_error') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write(
              '\x1b[33mAI: ' + (payload.message ?? 'LLM unavailable') + '\x1b[0m\n\r',
            );
          } catch {
            this.terminalPanel?.write('\x1b[33mAI: ' + event.data + '\x1b[0m\n\r');
          }
        } else if (event.type === 'complete') {
          // The backend score event is not yet wired into the audit SSE
          // stream — compute a local partial breakdown from current state.
          this.computeLocalScoreBreakdown(challenge);
        }
      },
      error: (err) => {
        this.terminalPanel?.write('\x1b[31mError: ' + err.message + '\x1b[0m\n\r');
        this.isAuditing.set(false);
      },
      complete: () => {
        this.terminalPanel?.write('\n\r\x1b[32mAudit complete.\x1b[0m\n\r');
        this.isAuditing.set(false);
      },
    });
  }

  dismissScoreBreakdown(): void {
    this.scoreBreakdown.set(null);
  }

  // Compute what we can from local state. Test, lint, and findings
  // points require server-side scoring — those values stay at 0 until
  // the audit SSE stream emits a score event (planned for a later PR).
  private computeLocalScoreBreakdown(challenge: Challenge): void {
    if (this.auditStartedAt !== null) {
      this.auditElapsedMinutes = (Date.now() - this.auditStartedAt) / 60_000;
    }

    const basePoints = challenge.basePoints;
    const hintsPenalty = this.hintsPenalty();
    const timeBonus = this.computeTimeBonus(challenge);
    const testPoints = 0;
    const lintPoints = 0;
    const findingsMatched = 0;

    const total = Math.max(
      0,
      basePoints + testPoints + lintPoints + findingsMatched - hintsPenalty + timeBonus,
    );

    this.scoringFromBackend.set(false);
    this.scoreBreakdown.set({
      basePoints,
      testPoints,
      lintPoints,
      findingsMatched,
      hintsPenalty,
      timeBonus,
      total,
    });
  }

  // Time bonus = 10% of basePoints when challenge.timeBonus is enabled
  // and the user finished under the estimated time. Matches the Go
  // ScoringService formula.
  private computeTimeBonus(challenge: Challenge): number {
    if (!challenge.timeBonus) return 0;
    if (
      challenge.estimatedTimeMinutes > 0 &&
      this.auditElapsedMinutes >= challenge.estimatedTimeMinutes
    ) {
      return 0;
    }
    return Math.floor(challenge.basePoints * 0.1);
  }
}
