// language-selector.component.ts — dumb picker for a programming
// language. The parent page passes the catalog via the `languages`
// input; the component emits `languageChange` when a row is clicked.
//
// Dumb by design: no use case injection, no HTTP, no state. The
// page is responsible for reacting to the emit (it calls
// PracticeUseCase.setLanguage + loadTopics).
//
// Hexagonal contract: this component imports ONLY the Angular
// primitives it needs and the local LanguageOption type. Zero use
// case / HTTP / domain-port access.
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

export interface LanguageOption {
  id: string;
  label: string;
}

@Component({
  selector: 'app-language-selector',
  standalone: true,
  imports: [],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (languages().length === 0) {
      <div class="text-dojo-text opacity-60 text-sm" data-testid="language-selector-empty">
        No languages available yet.
      </div>
    } @else {
      <ul class="flex flex-col gap-1" role="list">
        @for (lang of languages(); track lang.id) {
          <li>
            <button
              type="button"
              class="w-full text-left px-3 py-2 rounded-sm border border-dojo-border transition-colors text-sm"
              [class.bg-dojo-accent]="selected() === lang.id"
              [class.text-dojo-base]="selected() === lang.id"
              [class.bg-dojo-surface]="selected() !== lang.id"
              [class.text-dojo-text]="selected() !== lang.id"
              [class.hover:bg-dojo-surface-2]="selected() !== lang.id"
              [attr.data-testid]="'language-selector-row'"
              [attr.data-id]="lang.id"
              [attr.data-active]="selected() === lang.id"
              (click)="languageChange.emit(lang.id)"
            >
              {{ lang.label }}
            </button>
          </li>
        }
      </ul>
    }
  `,
})
export class LanguageSelectorComponent {
  readonly languages = input.required<readonly LanguageOption[]>();
  readonly selected = input<string | null>(null);
  readonly languageChange = output<string>();
}
