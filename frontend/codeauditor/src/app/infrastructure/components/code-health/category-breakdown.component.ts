// category-breakdown.component.ts — dumb horizontal bar chart that
// shows the count of issues per category. The page derives the counts
// from `analysis.top_issues` and passes them in as an input.
//
// The component is dumb: it renders 5 bars, one per CodeHealthCategory,
// with bar widths normalized to the highest count in the set. The
// color tier follows the same convention as the issues table.
import {
  Component,
  computed,
  input,
} from '@angular/core';

import {
  CodeHealthCategory,
  CODE_HEALTH_CATEGORIES,
} from '../../../domain/models/code-health';

@Component({
  selector: 'app-category-breakdown',
  standalone: true,
  template: `
    <div
      class="bg-[#161B22] border border-[#30363D] rounded-sm p-4"
      data-testid="category-breakdown"
    >
      <h3 class="text-xs text-[#8B949E] uppercase tracking-wider mb-3">
        By category
      </h3>
      <div class="space-y-2">
        @for (cat of categories(); track cat) {
          <div
            class="flex items-center gap-2"
            [attr.data-testid]="'category-bar-' + cat"
          >
            <span
              class="w-24 text-xs text-[#C9D1D9] capitalize"
              [attr.data-testid]="'category-label-' + cat"
            >
              {{ cat }}
            </span>
            <div class="flex-1 bg-[#0D1117] rounded-sm h-3 overflow-hidden">
              <div
                class="h-full rounded-sm transition-all"
                [style.width.%]="barWidthPct(cat)"
                [style.backgroundColor]="tierColor(cat)"
                [attr.data-testid]="'category-fill-' + cat"
              ></div>
            </div>
            <span
              class="w-10 text-right text-xs text-[#8B949E]"
              [attr.data-testid]="'category-count-' + cat"
            >
              {{ counts()[cat] }}
            </span>
          </div>
        } @empty {
          <p class="text-xs text-[#8B949E]">No data.</p>
        }
      </div>
    </div>
  `,
})
export class CategoryBreakdownComponent {
  readonly counts = input.required<Readonly<Record<CodeHealthCategory, number>>>();

  // Listing the 5 categories in canonical order (design.md §Issue
  // categories table) keeps the chart stable regardless of input key
  // insertion order.
  readonly categories = computed(() => [...CODE_HEALTH_CATEGORIES]);

  // maxCount is the denominator for bar-width normalization.
  readonly maxCount = computed(() => {
    const map = this.counts();
    let m = 0;
    for (const cat of this.categories()) {
      const v = map[cat] ?? 0;
      if (v > m) m = v;
    }
    return m;
  });

  // Width as a 0..100 percentage relative to the highest count.
  barWidthPct(cat: CodeHealthCategory): number {
    const m = this.maxCount();
    if (m === 0) return 0;
    const v = this.counts()[cat] ?? 0;
    return Math.round((v / m) * 100);
  }

  // Width in pixels (for the legacy API used by the test).
  barWidthPx(cat: CodeHealthCategory): number {
    return this.barWidthPct(cat);
  }

  // Color tier follows the convention from issues-table: each
  // category has a fixed color so the dashboard reads uniformly.
  tierColor(cat: CodeHealthCategory): string {
    switch (cat) {
      case 'security':
        return '#F85149';
      case 'performance':
        return '#D29922';
      case 'refactor':
        return '#58A6FF';
      case 'style':
        return '#8B949E';
      case 'architecture':
        return '#A371F7';
    }
  }
}
