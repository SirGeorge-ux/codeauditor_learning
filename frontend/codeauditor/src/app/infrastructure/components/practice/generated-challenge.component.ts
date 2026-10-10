// generated-challenge.component.ts — dumb panel that renders the
// most-recently-generated Challenge from the PracticeUseCase. The
// page passes the challenge + the loading flag via inputs; the
// component emits `openDojo` (with the challenge id) when the user
// clicks "Open in dojo".
//
// Hexagonal contract: zero use case / HTTP access. The page owns
// navigation (router.navigateByUrl); this component only emits the
// intent + the challenge id.
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { Challenge } from '../../../domain/models/challenge';

@Component({
  selector: 'app-generated-challenge',
  standalone: true,
  imports: [],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (loading()) {
      <div class="text-dojo-text opacity-70 text-sm" data-testid="generated-challenge-loading">
        Generating challenge…
      </div>
    } @else if (challenge(); as ch) {
      <div
        class="bg-dojo-surface border border-dojo-border rounded-sm p-4 space-y-3"
        data-testid="generated-challenge-summary"
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-dojo-text">{{ ch.title }}</h3>
            <p class="text-xs text-dojo-text opacity-70 mt-1 leading-relaxed">
              {{ ch.description }}
            </p>
          </div>
          <span
            class="inline-block px-1.5 py-0.5 rounded-sm bg-dojo-base border border-dojo-border text-xs"
          >
            {{ ch.language }} · {{ ch.difficulty }}
          </span>
        </div>

        @if (ch.learningObjectives.length > 0) {
          <div>
            <h4 class="text-xs font-semibold text-dojo-text mb-1">Learning objectives</h4>
            <ul class="text-xs text-dojo-text opacity-80 list-disc list-inside space-y-0.5">
              @for (lo of ch.learningObjectives; track lo) {
                <li>{{ lo }}</li>
              }
            </ul>
          </div>
        }

        <div class="flex justify-end">
          <button
            type="button"
            class="px-3 py-1.5 bg-dojo-accent text-dojo-base text-sm font-semibold rounded-sm hover:opacity-90 transition-opacity"
            [attr.data-testid]="'generated-challenge-open-dojo'"
            (click)="openDojo.emit(ch.id)"
          >
            Open in dojo
          </button>
        </div>
      </div>
    } @else {
      <div class="text-dojo-text opacity-60 text-sm" data-testid="generated-challenge-empty">
        Pick a language, topic, and level to generate a challenge.
      </div>
    }
  `,
})
export class GeneratedChallengeComponent {
  readonly challenge = input<Challenge | null>(null);
  readonly loading = input<boolean>(false);
  readonly openDojo = output<string>();
}
