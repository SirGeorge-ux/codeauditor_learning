// import-button.component.ts — dumb CTA button for the analyze flow.
// Three render branches:
//   - loading: disables the button + shows "Analyzing…" label.
//   - remainingToday === 0: disables + shows the daily-limit message.
//   - happy path: enables + emits `void` on click.
//
// Hexagonal contract: standalone component with input signal + output
// signal. Holds no service reference.
import { Component, computed, input, output } from '@angular/core';

@Component({
  selector: 'app-import-button',
  standalone: true,
  template: `
    <button
      type="button"
      class="px-4 py-2 text-sm font-semibold rounded-sm transition-colors"
      [class.bg-\\[\\#238636\\]]="!disabled()"
      [class.hover\\:bg-\\[\\#2EA043\\]]="!disabled()"
      [class.text-white]="true"
      [class.opacity-50]="disabled()"
      [class.cursor-not-allowed]="disabled()"
      [disabled]="disabled()"
      (click)="onClick()"
      data-testid="import-button"
    >
      {{ label() }}
    </button>
  `,
})
export class ImportButtonComponent {
  readonly loading = input.required<boolean>();
  readonly remainingToday = input.required<number>();
  readonly clicked = output<void>();

  // disabled reflects either the loading or the cap-hit state.
  readonly disabled = computed(() => this.loading() || this.remainingToday() <= 0);

  readonly label = computed(() => {
    if (this.loading()) return 'Analyzing…';
    if (this.remainingToday() <= 0) return 'Daily limit reached (resets 00:00 UTC)';
    return `Analyze (${this.remainingToday()} left today)`;
  });

  onClick(): void {
    if (this.disabled()) {
      return;
    }
    this.clicked.emit();
  }
}
