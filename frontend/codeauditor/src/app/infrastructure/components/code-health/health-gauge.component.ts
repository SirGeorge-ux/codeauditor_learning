// health-gauge.component.ts — dumb SVG gauge that visualizes the 0–100
// health score produced by `mcp-code-health.get_analysis`. Three color
// tiers (red/yellow/green) match the design.md §Health-score formula
// thresholds (`< 40`, `40..69`, `>= 70`).
//
// Hexagonal contract: standalone component, single `input.required`
// `score`. No service reference. The SVG is hand-rolled (a stroked
// arc) so the component doesn't pull in a charting library.
import { Component, computed, input } from '@angular/core';

type Tier = 'red' | 'yellow' | 'green';

@Component({
  selector: 'app-health-gauge',
  standalone: true,
  template: `
    <div
      class="flex flex-col items-center bg-[#161B22] border border-[#30363D] rounded-sm p-4"
      data-testid="health-gauge"
    >
      <svg
        viewBox="0 0 120 120"
        class="w-32 h-32"
        [attr.aria-label]="'Health score ' + score()"
      >
        <!-- Track -->
        <circle
          cx="60"
          cy="60"
          r="50"
          fill="none"
          stroke="#21262D"
          stroke-width="10"
          data-testid="gauge-track"
        />
        <!-- Arc -->
        <circle
          cx="60"
          cy="60"
          r="50"
          fill="none"
          [attr.stroke]="tierColor()"
          stroke-width="10"
          stroke-linecap="round"
          [attr.stroke-dasharray]="circumference + ' ' + circumference"
          [attr.stroke-dashoffset]="offset()"
          transform="rotate(-90 60 60)"
          data-testid="gauge-arc"
        />
        <text
          x="60"
          y="66"
          text-anchor="middle"
          [attr.fill]="tierColor()"
          font-size="22"
          font-weight="700"
          data-testid="gauge-score"
        >
          {{ clampedScore() }}
        </text>
      </svg>
      <div class="mt-2 text-xs text-[#8B949E] uppercase tracking-wider">
        Health score
      </div>
    </div>
  `,
})
export class HealthGaugeComponent {
  readonly score = input.required<number>();

  // Clamp the input into the 0..100 visualization range. The
  // display is purely cosmetic — the underlying analysis carries
  // the same value to the table + report.
  readonly clampedScore = computed(() => {
    const v = this.score();
    if (v < 0) return 0;
    if (v > 100) return 100;
    return Math.round(v);
  });

  // Tier reflects the design.md §Health-score formula thresholds.
  readonly tier = computed<Tier>(() => {
    const v = this.clampedScore();
    if (v < 40) return 'red';
    if (v < 70) return 'yellow';
    return 'green';
  });

  readonly tierColor = computed(() => {
    switch (this.tier()) {
      case 'red':
        return '#F85149';
      case 'yellow':
        return '#D29922';
      case 'green':
        return '#39D353';
    }
  });

  // circumference + offset drive the SVG arc length.
  readonly circumference = 2 * Math.PI * 50; // 314.16…
  readonly offset = computed(
    () => this.circumference * (1 - this.clampedScore() / 100),
  );
}
