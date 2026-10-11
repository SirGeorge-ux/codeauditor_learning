// issues-table.component.spec.ts — dumb ranked-issues table. Header
// click sorts by severity/file/rule; severity filter narrows rows;
// suppress button emits `{issue_id}` so the page can call the port.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';

import { IssuesTableComponent } from './issues-table.component';
import { CodeHealthIssue, CodeHealthSeverity } from '../../../domain/models/code-health';

function makeIssue(partial: Partial<CodeHealthIssue>): CodeHealthIssue {
  return {
    id: partial.id ?? 'i-1',
    rule_id: partial.rule_id ?? 'RULE-X-001',
    category: partial.category ?? 'security',
    severity: partial.severity ?? 'medium',
    message: partial.message ?? 'message',
    file_path: partial.file_path ?? 'src/a.ts',
    line_number: partial.line_number ?? 1,
    evidence: partial.evidence ?? 'ev',
    suggested_fix: partial.suggested_fix ?? 'fix',
    confidence: partial.confidence ?? 1.0,
    ...partial,
  };
}

describe('IssuesTableComponent', () => {
  let fixture: ComponentFixture<IssuesTableComponent>;
  let component: IssuesTableComponent;

  const samples: CodeHealthIssue[] = [
    makeIssue({
      id: 'i-c',
      rule_id: 'RULE-SEC-SECRET-001',
      category: 'security',
      severity: 'critical',
      message: 'hardcoded secret',
      file_path: 'src/auth.ts',
      line_number: 14,
    }),
    makeIssue({
      id: 'i-h',
      rule_id: 'RULE-PERF-N1-001',
      category: 'performance',
      severity: 'high',
      message: 'n+1 in loop',
      file_path: 'src/db.ts',
      line_number: 32,
    }),
    makeIssue({
      id: 'i-m',
      rule_id: 'RULE-REFAC-GOD-001',
      category: 'refactor',
      severity: 'medium',
      message: 'god method',
      file_path: 'src/util.ts',
      line_number: 7,
    }),
    makeIssue({
      id: 'i-l',
      rule_id: 'RULE-STYLE-MAGIC-001',
      category: 'style',
      severity: 'low',
      message: 'magic number',
      file_path: 'src/math.ts',
      line_number: 99,
    }),
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, IssuesTableComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(IssuesTableComponent);
    component = fixture.componentInstance;
  });

  it('renders every issue row when no filter is applied', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.detectChanges();
    const html = fixture.nativeElement as HTMLElement;
    const rows = html.querySelectorAll('[data-testid^="issues-row-"]');
    expect(rows.length).toBe(4);
  });

  it('sorts by severity desc by default', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.detectChanges();
    expect(component.sortedIssues().map((i) => i.severity)).toEqual([
      'critical',
      'high',
      'medium',
      'low',
    ]);
  });

  it('filter narrows rows by severity', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.componentRef.setInput('severityFilter', 'critical');
    fixture.detectChanges();
    expect(component.visibleIssues().map((i) => i.id)).toEqual(['i-c']);
  });

  it('emits suppress with the issue_id when the row button is clicked', () => {
    fixture.componentRef.setInput('issues', samples);
    let emitted: { issue_id: string } | null = null;
    component.suppress.subscribe((v) => (emitted = v));
    fixture.detectChanges();
    // Click the suppress button for the first sorted row (critical).
    component.requestSuppress('i-c');
    expect(emitted).toEqual({ issue_id: 'i-c' });
  });

  it('re-sorts by line asc when file column header clicked', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.detectChanges();
    component.toggleSort('file');
    expect(component.sortedIssues()[0]?.file_path).toBe('src/auth.ts');
    expect(component.sortedIssues()[3]?.file_path).toBe('src/util.ts');
  });

  it('re-sorts by rule_id asc when rule column header clicked', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.detectChanges();
    component.toggleSort('rule');
    const ids = component.sortedIssues().map((i) => i.rule_id);
    expect(ids).toEqual([...ids].sort());
  });

  it('exposes the severity filter list helper', () => {
    fixture.componentRef.setInput('issues', samples);
    fixture.detectChanges();
    const opts: CodeHealthSeverity[] = component.severityOptions();
    expect(opts).toContain('critical');
    expect(opts).toContain('low');
  });
});
