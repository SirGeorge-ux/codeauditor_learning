// level-selector.component.ts — dumb picker for the F..S level band.
// The parent page passes the band via `levels` + the current value
// via `selected`; the component emits `levelChange` on click.
//
// Hexagonal contract: zero use case / HTTP access. The page reads
// PracticeUseCase.F_S_LEVELS and binds it to `levels`.
import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
  selector: 'app-level-selector',
  standalone: true,
  imports: [],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="flex flex-wrap gap-1" role="group" aria-label="Level selector">
      @for (lvl of levels(); track lvl) {
        <button
          type="button"
          class="w-9 h-9 flex items-center justify-center rounded-sm border text-sm font-semibold transition-colors"
          [class.bg-dojo-accent]="selected() === lvl"
          [class.text-dojo-base]="selected() === lvl"
          [class.border-dojo-accent]="selected() === lvl"
          [class.bg-dojo-surface]="selected() !== lvl"
          [class.text-dojo-text]="selected() !== lvl"
          [class.border-dojo-border]="selected() !== lvl"
          [class.hover:border-dojo-text]="selected() !== lvl"
          [attr.data-testid]="'level-selector-row'"
          [attr.data-id]="lvl"
          [attr.data-active]="selected() === lvl"
          (click)="levelChange.emit(lvl)"
          [attr.aria-pressed]="selected() === lvl"
        >
          {{ lvl }}
        </button>
      }
    </div>
  `,
})
export class LevelSelectorComponent {
  readonly levels = input.required<readonly string[]>();
  readonly selected = input<string>('F');
  readonly levelChange = output<string>();
}