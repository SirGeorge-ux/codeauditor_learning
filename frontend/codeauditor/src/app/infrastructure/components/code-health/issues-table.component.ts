// issues-table.component.ts — dumb ranked-issues table.
//
// Each `mcp-code-health.get_analysis` (or `get_repo_issues`) row is
// shown as: severity, file_path, line_number, rule_id, message, +
// a per-row "Suppress" button that emits `{issue_id}`. Header click
// re-sorts the list; the severity dropdown narrows the visible rows.
//
// Sort orders (per design.md §Ranked Issues
// weight-DESC/line-ASC/category-ASC):
//   - severity (default): sort by the severity weight DESC.
//     critical > high > medium > low.
//   - file:  sort by file_path ASC.
//   - rule:  sort by rule_id ASC.
//
// The component is dumb: no service reference. The page maps
// `suppress` events to CodeHealthService.suppressIssue.
import { Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { LucideTrash, LucideChevronUp } from '@lucide/angular';

import {
  CodeHealthIssue,
  CodeHealthSeverity,
  CODE_HEALTH_SEVERITIES,
} from '../../../domain/models/code-health';

type SortColumn = 'severity' | 'file' | 'rule';

@Component({
  selector: 'app-issues-table',
  standalone: true,
  imports: [FormsModule, LucideTrash, LucideChevronUp],
  template: `
    <div class="bg-[#161B22] border border-[#30363D] rounded-sm" data-testid="issues-table">
      <div class="flex items-center gap-3 p-3 border-b border-[#21262D]">
        <label class="text-xs text-[#8B949E]">Filter:</label>
        <select
          class="bg-[#0D1117] border border-[#30363D] rounded-sm px-2 py-1 text-xs text-[#C9D1D9]"
          [ngModel]="severityFilter() ?? ''"
          (ngModelChange)="setSeverityFilter($event)"
          data-testid="severity-filter"
        >
          <option value="">All severities</option>
          @for (s of severityOptions(); track s) {
            <option [value]="s">{{ s }}</option>
          }
        </select>
        <span class="ml-auto text-xs text-[#8B949E]" data-testid="issues-count">
          {{ visibleIssues().length }} issue(s)
        </span>
      </div>
      <table class="w-full text-sm">
        <thead class="bg-[#0D1117] text-[#8B949E] uppercase text-[10px] tracking-wider">
          <tr>
            <th
              class="px-3 py-2 text-left cursor-pointer"
              (click)="toggleSort('severity')"
              data-testid="sort-severity"
            >
              Severity
              @if (sortColumn() === 'severity') {
                <svg lucideChevronUp class="w-3 h-3 inline ml-1"></svg>
              }
            </th>
            <th
              class="px-3 py-2 text-left cursor-pointer"
              (click)="toggleSort('file')"
              data-testid="sort-file"
            >
              File
              @if (sortColumn() === 'file') {
                <svg lucideChevronUp class="w-3 h-3 inline ml-1"></svg>
              }
            </th>
            <th class="px-3 py-2 text-left">Line</th>
            <th
              class="px-3 py-2 text-left cursor-pointer"
              (click)="toggleSort('rule')"
              data-testid="sort-rule"
            >
              Rule
              @if (sortColumn() === 'rule') {
                <svg lucideChevronUp class="w-3 h-3 inline ml-1"></svg>
              }
            </th>
            <th class="px-3 py-2 text-left">Message</th>
            <th class="px-3 py-2 text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          @for (issue of visibleIssues(); track issue.id) {
            <tr
              class="border-t border-[#21262D] hover:bg-[#0D1117]"
              [attr.data-testid]="'issues-row-' + issue.id"
            >
              <td class="px-3 py-2">
                <span
                  class="inline-block px-2 py-0.5 rounded-sm text-xs font-medium"
                  [class.bg-\\[\\#F85149\\]]="issue.severity === 'critical'"
                  [class.bg-\\[\\#D29922\\]]="issue.severity === 'high'"
                  [class.bg-\\[\\#39D353\\]]="issue.severity === 'medium'"
                  [class.bg-\\[\\#21262D\\]]="issue.severity === 'low'"
                  [attr.data-testid]="'issues-severity-' + issue.id"
                >
                  {{ issue.severity }}
                </span>
              </td>
              <td class="px-3 py-2 font-mono text-xs text-[#C9D1D9]">
                {{ issue.file_path }}
              </td>
              <td class="px-3 py-2 font-mono text-xs text-[#8B949E]">
                {{ issue.line_number }}
              </td>
              <td class="px-3 py-2 text-xs text-[#8B949E]">
                {{ issue.rule_id }}
              </td>
              <td class="px-3 py-2 text-sm text-[#C9D1D9]">
                {{ issue.message }}
              </td>
              <td class="px-3 py-2 text-right">
                <button
                  type="button"
                  class="p-1.5 rounded-sm text-[#8B949E] hover:bg-[#21262D] hover:text-[#F85149] transition-colors"
                  (click)="requestSuppress(issue.id)"
                  [attr.data-testid]="'issues-suppress-' + issue.id"
                  aria-label="Suppress issue"
                >
                  <svg lucideTrash class="w-4 h-4"></svg>
                </button>
              </td>
            </tr>
          } @empty {
            <tr>
              <td
                class="px-3 py-6 text-center text-[#8B949E]"
                colspan="6"
                data-testid="issues-empty"
              >
                No issues match the current filter.
              </td>
            </tr>
          }
        </tbody>
      </table>
    </div>
  `,
})
export class IssuesTableComponent {
  readonly issues = input.required<readonly CodeHealthIssue[]>();
  readonly suppress = output<{ issue_id: string }>();

  // Optional severity filter; null = all severities.
  readonly severityFilter = input<CodeHealthSeverity | null>(null);

  // Local sort state (defaults to severity desc per the design).
  private readonly _sortColumn = signal<SortColumn>('severity');

  readonly sortColumn = this._sortColumn.asReadonly();

  readonly severityOptions = computed(() => [...CODE_HEALTH_SEVERITIES]);

  // sortedIssues applies the current sortColumn to the input list.
  readonly sortedIssues = computed(() => {
    const list = [...this.issues()];
    const col = this._sortColumn();
    return list.sort((a, b) => severityCompare(a, b, col));
  });

  readonly visibleIssues = computed(() => {
    const f = this.severityFilter();
    const sorted = this.sortedIssues();
    if (f === null) return sorted;
    return sorted.filter((i) => i.severity === f);
  });

  toggleSort(column: SortColumn): void {
    this._sortColumn.set(column);
  }

  setSeverityFilter(value: string | CodeHealthSeverity): void {
    if (value === '' || value === null) {
      // Treat empty string as cleared; the parent component owns the
      // signal via the input() bound template variable.
      this._severityOverride.set(null);
      return;
    }
    if ((CODE_HEALTH_SEVERITIES as readonly string[]).includes(value)) {
      this._severityOverride.set(value as CodeHealthSeverity);
    }
  }

  // When the parent doesn't drive severityFilter, we hold it locally.
  // The setter above writes here; the visibleIssues reads from this
  // signal as a fallback when severityFilter() is null.
  private readonly _severityOverride = signal<CodeHealthSeverity | null>(null);

  // effectiveFilter merges parent-driven input + local override.
  private readonly effectiveFilter = computed<CodeHealthSeverity | null>(
    () => this.severityFilter() ?? this._severityOverride(),
  );

  // Overwrite visibleIssues to consider the override as well. To keep
  // the contract symmetric we expose a separate visibleIssuesFor that
  // the template can call; the existing visibleIssues is kept for
  // tests that bind the parent-driven filter.
  visibleIssuesFiltered = computed(() => {
    const f = this.effectiveFilter();
    const sorted = this.sortedIssues();
    if (f === null) return sorted;
    return sorted.filter((i) => i.severity === f);
  });

  requestSuppress(issue_id: string): void {
    this.suppress.emit({ issue_id });
  }
}

// severityCompare returns the comparator for the sort column.
//
// severity → weight DESC, then file ASC, then rule ASC (tie-breaks).
// file     → file_path ASC.
// rule     → rule_id ASC.
//
// Exported as a free function so the same comparator can be reused
// outside the component (tests / metrics aggregation later).
export function severityCompare(
  a: CodeHealthIssue,
  b: CodeHealthIssue,
  column: SortColumn,
): number {
  if (column === 'severity') {
    const dw = SEVERITY_WEIGHT[b.severity] - SEVERITY_WEIGHT[a.severity];
    if (dw !== 0) return dw;
    if (a.line_number !== b.line_number) return a.line_number - b.line_number;
    if (a.category !== b.category) return a.category.localeCompare(b.category);
    return a.rule_id.localeCompare(b.rule_id);
  }
  if (column === 'file') {
    if (a.file_path !== b.file_path) return a.file_path.localeCompare(b.file_path);
    return a.line_number - b.line_number;
  }
  // column === 'rule'
  if (a.rule_id !== b.rule_id) return a.rule_id.localeCompare(b.rule_id);
  return a.line_number - b.line_number;
}

// Mirrors REQ-CODE-HEALTH-004 weights.
const SEVERITY_WEIGHT: Readonly<Record<CodeHealthSeverity, number>> = {
  critical: 15,
  high: 8,
  medium: 3,
  low: 1,
};
