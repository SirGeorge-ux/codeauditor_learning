// category-breakdown.component.spec.ts — dumb horizontal bar chart
// that shows the count of issues per category (security / performance
// / refactor / style / architecture). Reads counts from the parent.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';

import { CategoryBreakdownComponent } from './category-breakdown.component';
import { CodeHealthCategory } from '../../../domain/models/code-health';

describe('CategoryBreakdownComponent', () => {
  let fixture: ComponentFixture<CategoryBreakdownComponent>;
  let component: CategoryBreakdownComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, CategoryBreakdownComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(CategoryBreakdownComponent);
    component = fixture.componentInstance;
  });

  it('renders a bar per category with the count', () => {
    const counts: Record<CodeHealthCategory, number> = {
      security: 3,
      performance: 5,
      refactor: 0,
      style: 2,
      architecture: 1,
    };
    fixture.componentRef.setInput('counts', counts);
    fixture.detectChanges();
    const bars = fixture.nativeElement.querySelectorAll('[data-testid^="category-bar-"]');
    expect(bars.length).toBe(5);
    const sec = fixture.nativeElement.querySelector('[data-testid="category-bar-security"]');
    expect(sec?.textContent).toContain('3');
  });

  it('normalizes bar widths to the highest count', () => {
    const counts: Record<CodeHealthCategory, number> = {
      security: 10,
      performance: 2,
      refactor: 0,
      style: 0,
      architecture: 0,
    };
    fixture.componentRef.setInput('counts', counts);
    fixture.detectChanges();
    expect(component.maxCount()).toBe(10);
    expect(component.barWidthPx('performance')).toBeLessThan(component.barWidthPx('security'));
  });

  it('returns 0 width when the counts are all zero', () => {
    const counts: Record<CodeHealthCategory, number> = {
      security: 0,
      performance: 0,
      refactor: 0,
      style: 0,
      architecture: 0,
    };
    fixture.componentRef.setInput('counts', counts);
    fixture.detectChanges();
    expect(component.maxCount()).toBe(0);
    expect(component.barWidthPx('security')).toBe(0);
  });

  it('lists the 5 categories in the canonical order', () => {
    const counts: Record<CodeHealthCategory, number> = {
      security: 0,
      performance: 0,
      refactor: 0,
      style: 0,
      architecture: 0,
    };
    fixture.componentRef.setInput('counts', counts);
    fixture.detectChanges();
    expect(component.categories()).toEqual([
      'security',
      'performance',
      'refactor',
      'style',
      'architecture',
    ]);
  });
});
