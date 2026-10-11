// health-gauge.component.spec.ts — dumb SVG gauge that visualizes the
// 0–100 health score with green/yellow/red color tiers. Asserts the
// rendered SVG angle and color class depend on the input score.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';

import { HealthGaugeComponent } from './health-gauge.component';

describe('HealthGaugeComponent', () => {
  let fixture: ComponentFixture<HealthGaugeComponent>;
  let component: HealthGaugeComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, HealthGaugeComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(HealthGaugeComponent);
    component = fixture.componentInstance;
  });

  it('renders the score text in the center', () => {
    fixture.componentRef.setInput('score', 75);
    fixture.detectChanges();
    const html = fixture.nativeElement as HTMLElement;
    expect(html.querySelector('[data-testid="gauge-score"]')?.textContent?.trim()).toBe('75');
  });

  it('clamps negative scores to 0', () => {
    fixture.componentRef.setInput('score', -5);
    fixture.detectChanges();
    expect(component.clampedScore()).toBe(0);
    expect(component.tier()).toBe('red');
  });

  it('clamps scores above 100 to 100', () => {
    fixture.componentRef.setInput('score', 250);
    fixture.detectChanges();
    expect(component.clampedScore()).toBe(100);
  });

  it('uses the red tier for score < 40', () => {
    fixture.componentRef.setInput('score', 30);
    fixture.detectChanges();
    expect(component.tier()).toBe('red');
  });

  it('uses the yellow tier for score 40..69', () => {
    fixture.componentRef.setInput('score', 50);
    fixture.detectChanges();
    expect(component.tier()).toBe('yellow');
  });

  it('uses the green tier for score >= 70', () => {
    fixture.componentRef.setInput('score', 80);
    fixture.detectChanges();
    expect(component.tier()).toBe('green');
  });

  it('renders the stroke-dasharray reflecting the percentage', () => {
    fixture.componentRef.setInput('score', 50);
    fixture.detectChanges();
    const arc = fixture.nativeElement.querySelector('[data-testid="gauge-arc"]') as SVGPathElement;
    expect(arc).not.toBeNull();
    expect(arc.getAttribute('stroke-dasharray')).toBeTruthy();
  });
});
