// practice-page.component.ts — top-level page for Free Practice Mode.
//
// Replaces the pre-S3 stub with the real flow:
//   - Language selector (dumb component) → emits languageChange →
//     PracticeService.setLanguage + loadTopics.
//   - Topic list (dumb component, filtered by language) → emits
//     topicChange → PracticeService.setTopic.
//   - Level selector (dumb component, F..S) → emits levelChange →
//     PracticeService.setLevel.
//   - Generate button → PracticeService.generateChallenge() → shows
//     the summary in generated-challenge.component.
//   - Daily-cap notice (rendered when remainingToday == 0).
//   - Error banner (renders service.error()).
//   - Socratic tutor chat panel on the right (already present in the
//     stub; mounted on every page that uses it).
//
// Hexagonal contract: this component depends ONLY on PracticeService
// (Injectable wrapper) and the four dumb components. No HTTP / fetch /
// direct use case access. New control flow (@if / @for) throughout —
// zero *ngIf / *ngFor per AGENTS.md §2.2.
import { ChangeDetectionStrategy, Component, inject, OnInit, signal, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';

import { PracticeService } from '../../services/practice.service';
import { LanguageSelectorComponent, LanguageOption } from './language-selector.component';
import { LevelSelectorComponent } from './level-selector.component';
import { TopicListComponent } from './topic-list.component';
import { GeneratedChallengeComponent } from './generated-challenge.component';
import { TutorChatPanelComponent } from '../tutor-chat/tutor-chat-panel.component';

// Hard-coded language catalog for v1. The canonical source lives in
// `.atl/curricula/<lang>.md`; the frontend can fetch the catalog via
// `mcp-curriculum.list_languages` later. PR 3 keeps the list small
// and explicit so the UI is shippable without a new MCP roundtrip.
const SUPPORTED_LANGUAGES: readonly LanguageOption[] = [{ id: 'typescript', label: 'TypeScript' }];

@Component({
  selector: 'app-practice-page',
  standalone: true,
  imports: [
    CommonModule,
    LanguageSelectorComponent,
    LevelSelectorComponent,
    TopicListComponent,
    GeneratedChallengeComponent,
    TutorChatPanelComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="h-full flex flex-col bg-dojo-base">
      <!-- Top bar -->
      <div
        class="h-10 flex items-center justify-between px-4 bg-dojo-surface border-b border-dojo-border"
      >
        <span class="text-xs text-dojo-text">Free Practice</span>
        <span class="text-xs text-dojo-text opacity-70" data-testid="practice-remaining-label">
          {{ service.remainingToday() }}/{{ DAILY_CAP }} generations today
        </span>
      </div>

      <div class="flex-1 flex overflow-hidden">
        <!-- Left: selectors + generate -->
        <div
          class="w-80 flex-shrink-0 overflow-y-auto p-4 border-r border-dojo-border space-y-6 bg-dojo-surface"
          data-testid="practice-selector-panel"
        >
          <!-- Daily cap notice -->
          @if (service.remainingToday() <= 0) {
            <div
              class="bg-dojo-base border border-dojo-warning rounded-sm p-3 text-xs text-dojo-warning leading-relaxed"
              data-testid="practice-daily-cap-notice"
            >
              Daily limit reached. The counter resets at UTC midnight.
            </div>
          }

          <!-- Error banner -->
          @if (service.error(); as err) {
            <div
              class="bg-dojo-base border border-dojo-error rounded-sm p-3 text-xs text-dojo-error flex items-center justify-between"
              data-testid="practice-error-banner"
            >
              <span>{{ err }}</span>
              <button
                type="button"
                class="text-dojo-text opacity-60 hover:opacity-100 text-xs"
                (click)="onClearError()"
                aria-label="Dismiss error"
              >
                ×
              </button>
            </div>
          }

          <!-- Language -->
          <section>
            <h2 class="text-xs font-semibold text-dojo-text mb-2">Language</h2>
            <app-language-selector
              [languages]="languages"
              [selected]="service.language()"
              (languageChange)="onLanguageChange($event)"
            ></app-language-selector>
          </section>

          <!-- Level (F..S) -->
          <section>
            <h2 class="text-xs font-semibold text-dojo-text mb-2">Level</h2>
            <app-level-selector
              [levels]="service.levels"
              [selected]="service.level()"
              (levelChange)="onLevelChange($event)"
            ></app-level-selector>
          </section>

          <!-- Topic -->
          <section>
            <h2 class="text-xs font-semibold text-dojo-text mb-2">Topic</h2>
            <app-topic-list
              [topics]="service.topics()"
              [selected]="service.topic()"
              (topicChange)="onTopicChange($event)"
            ></app-topic-list>
          </section>

          <!-- Generate button -->
          <section>
            <button
              type="button"
              class="w-full px-3 py-2 rounded-sm bg-dojo-accent text-dojo-base text-sm font-semibold transition-opacity disabled:opacity-50"
              [disabled]="!service.canGenerate()"
              (click)="onGenerate()"
              data-testid="practice-generate-button"
            >
              @if (service.loading()) {
                Generating…
              } @else {
                Generate challenge
              }
            </button>
          </section>
        </div>

        <!-- Center: generated challenge summary -->
        <div class="flex-1 overflow-y-auto p-6" data-testid="practice-result-panel">
          <app-generated-challenge
            [challenge]="service.generatedChallenge()"
            [loading]="service.loading()"
            (openDojo)="onOpenDojo($event)"
          ></app-generated-challenge>
        </div>

        <!-- Right: tutor chat panel -->
        <div class="w-80 flex-shrink-0">
          <app-tutor-chat-panel
            [routeContext]="{ route: '/practice/free' }"
            (navigateTo)="onTutorNavigate($event)"
          ></app-tutor-chat-panel>
        </div>
      </div>
    </div>
  `,
})
export class PracticePageComponent implements OnInit {
  readonly service = inject(PracticeService);
  private readonly router = inject(Router);

  // Re-export the cap so the template can render the counter label
  // without reaching into the service static.
  readonly DAILY_CAP = 20;
  readonly languages = SUPPORTED_LANGUAGES;

  // selectionVersion is a write-only signal that triggers the effect
  // below to refresh the topic catalog when language changes. We keep
  // it as a signal so the effect can subscribe; the setLanguage
  // call inside the service already updates the language() signal.
  private readonly _selectionTick = signal(0);

  constructor() {
    // When language changes (and the user hasn't already picked a
    // topic), refresh the topic catalog. The effect runs in the
    // injection context so no manual lifecycle hook is needed.
    effect(() => {
      const lang = this.service.language();
      this._selectionTick();
      if (lang.length > 0) {
        void this.service.loadTopics(lang);
      }
    });
  }

  ngOnInit(): void {
    // Pre-select the F..S level at F so the user starts at the
    // foundation. The use case defaults to F anyway; this is just a
    // nudge for templates that read the level signal directly.
    if (this.service.level().length === 0) {
      this.service.setLevel('F');
    }
  }

  // onLanguageChange forwards the dumb-component emit into the
  // service; the effect above then refreshes the topic catalog.
  onLanguageChange(value: string): void {
    this.service.setLanguage(value);
    this._selectionTick.update((v) => v + 1);
  }

  onTopicChange(value: string): void {
    this.service.setTopic(value);
  }

  onLevelChange(value: string): void {
    this.service.setLevel(value);
  }

  async onGenerate(): Promise<void> {
    await this.service.generateChallenge();
  }

  onClearError(): void {
    this.service.clearError();
  }

  // onOpenDojo navigates to the dojo with the generated challenge
  // loaded. The ChallengeService.selectChallenge(id) call is the same
  // path curated-mode uses, so the dojo renders the generated
  // challenge without any change to the dojo page itself.
  onOpenDojo(challengeId: string): void {
    void this.router.navigateByUrl(`/dojo/${challengeId}`);
  }

  onTutorNavigate(path: string): void {
    void this.router.navigateByUrl(path);
  }
}
