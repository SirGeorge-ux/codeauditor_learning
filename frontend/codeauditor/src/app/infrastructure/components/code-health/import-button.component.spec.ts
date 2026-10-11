// import-button.component.spec.ts — dumb CTA button that triggers the
// analyze flow. Disables itself when loading or when the daily cap is
// exhausted. Emits `void` on click; the page routes the click to
// submitAnalysis.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';

import { ImportButtonComponent } from './import-button.component';

describe('ImportButtonComponent', () => {
  let fixture: ComponentFixture<ImportButtonComponent>;
  let component: ImportButtonComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, ImportButtonComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ImportButtonComponent);
    component = fixture.componentInstance;
  });

  it('renders enabled when not loading and remainingToday > 0', () => {
    fixture.componentRef.setInput('loading', false);
    fixture.componentRef.setInput('remainingToday', 3);
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="import-button"]',
    ) as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.disabled).toBe(false);
    expect(btn.textContent).toContain('Analyze');
  });

  it('renders disabled while loading', () => {
    fixture.componentRef.setInput('loading', true);
    fixture.componentRef.setInput('remainingToday', 3);
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="import-button"]',
    ) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.textContent).toContain('Analyzing');
  });

  it('renders disabled when remainingToday is 0', () => {
    fixture.componentRef.setInput('loading', false);
    fixture.componentRef.setInput('remainingToday', 0);
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="import-button"]',
    ) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.textContent).toMatch(/Daily limit/i);
  });

  it('emits when clicked in the enabled state', () => {
    fixture.componentRef.setInput('loading', false);
    fixture.componentRef.setInput('remainingToday', 3);
    let emits = 0;
    component.clicked.subscribe(() => (emits += 1));
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="import-button"]',
    ) as HTMLButtonElement;
    btn.click();
    expect(emits).toBe(1);
  });

  it('does NOT emit while disabled (loading OR remaining==0)', () => {
    let emits = 0;
    component.clicked.subscribe(() => (emits += 1));
    fixture.componentRef.setInput('loading', true);
    fixture.componentRef.setInput('remainingToday', 3);
    fixture.detectChanges();
    let btn = fixture.nativeElement.querySelector(
      '[data-testid="import-button"]',
    ) as HTMLButtonElement;
    btn.click();
    expect(emits).toBe(0);

    fixture.componentRef.setInput('loading', false);
    fixture.componentRef.setInput('remainingToday', 0);
    fixture.detectChanges();
    btn = fixture.nativeElement.querySelector('[data-testid="import-button"]') as HTMLButtonElement;
    btn.click();
    expect(emits).toBe(0);
  });
});
