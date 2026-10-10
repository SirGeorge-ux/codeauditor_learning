// topic-list.component.ts — dumb list of CurriculumTopic rows. The
// parent page passes the (already-filtered) topics + the current
// selection; the component emits `topicChange` on click.
//
// Hexagonal contract: zero use case / HTTP access. The page passes
// the filtered catalog (filtered by language) — this component is
// agnostic to where the catalog comes from.
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

import { CurriculumTopic } from '../../../domain/models/curriculum';

@Component({
  selector: 'app-topic-list',
  standalone: true,
  imports: [],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (topics().length === 0) {
      <div class="text-dojo-text opacity-60 text-sm" data-testid="topic-list-empty">
        No topics for this language yet.
      </div>
    } @else {
      <ul class="flex flex-col gap-1" role="list">
        @for (topic of topics(); track topic.id) {
          <li>
            <button
              type="button"
              class="w-full text-left px-3 py-2 rounded-sm border border-dojo-border transition-colors text-sm"
              [class.bg-dojo-accent]="selected() === topic.id"
              [class.text-dojo-base]="selected() === topic.id"
              [class.bg-dojo-surface]="selected() !== topic.id"
              [class.text-dojo-text]="selected() !== topic.id"
              [class.hover:border-dojo-text]="selected() !== topic.id"
              [attr.data-testid]="'topic-list-row'"
              [attr.data-id]="topic.id"
              [attr.data-active]="selected() === topic.id"
              (click)="topicChange.emit(topic.id)"
            >
              <div class="flex items-center justify-between gap-3">
                <span class="font-semibold">{{ topic.title }}</span>
                <span
                  class="inline-block px-1.5 py-0.5 rounded-sm bg-dojo-base border border-dojo-border text-xs"
                  [attr.data-testid]="'topic-list-difficulty'"
                  >{{ topic.difficulty }}</span
                >
              </div>
              @if (topic.description) {
                <p class="text-xs text-dojo-text opacity-70 mt-1 leading-snug">
                  {{ topic.description }}
                </p>
              }
            </button>
          </li>
        }
      </ul>
    }
  `,
})
export class TopicListComponent {
  readonly topics = input.required<readonly CurriculumTopic[]>();
  readonly selected = input<string | null>(null);
  readonly topicChange = output<string>();
}