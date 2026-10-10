// generated-challenge.component.spec.ts — dumb panel that renders the
// most recently generated Challenge from the PracticeUseCase. The
// page passes the challenge + loading flag via inputs; the
// component emits `openDojo` (with the challenge id) when the user
// clicks "Open in dojo".
//
// RED at HEAD~1; GREEN once the implementation lands.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';

import { GeneratedChallengeComponent } from './generated-challenge.component';
import { Challenge } from '../../../domain/models/challenge';

describe('GeneratedChallengeComponent', () => {
  let fixture: ComponentFixture<GeneratedChallengeComponent>;
  let component: GeneratedChallengeComponent;

  const challenge: Challenge = {
    id: 'gen-42',
    title: 'Inheritance chain',
    description: 'Refactor the broken inheritance chain.',
    difficulty: 'mid',
    category: 'oop',
    language: 'typescript',
    code: 'class Animal {}',
    status: 'available',
    createdAt: new Date('2026-10-10'),
    createdBy: 'user-1',
    learningObjectives: ['Inheritance', 'Polymorphism'],
    hints: [],
    commonMistakes: [],
    estimatedTimeMinutes: 30,
    expectedFindings: [],
    testCases: [],
    linterRules: [],
    basePoints: 100,
    bonusPoints: 20,
    penaltyPerHint: 5,
    timeBonus: true,
    origin: 'generated',
    generatedBy: 'free-practice-v1',
    solutionCode: 'class Animal extends Object {}',
    solutionExplanation: '...',
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [GeneratedChallengeComponent],
    }).compileComponents();
    fixture = TestBed.createComponent(GeneratedChallengeComponent);
    component = fixture.componentInstance;
  });

  it('shows an empty state when challenge is null and not loading', () => {
    fixture.componentRef.setInput('challenge', null);
    fixture.componentRef.setInput('loading', false);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="generated-challenge-empty"]')).toBeTruthy();
    expect(root.querySelector('[data-testid="generated-challenge-loading"]')).toBeFalsy();
  });

  it('shows a loading state while loading', () => {
    fixture.componentRef.setInput('challenge', null);
    fixture.componentRef.setInput('loading', true);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="generated-challenge-loading"]')).toBeTruthy();
  });

  it('renders the challenge summary when provided', () => {
    fixture.componentRef.setInput('challenge', challenge);
    fixture.componentRef.setInput('loading', false);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.textContent).toContain('Inheritance chain');
    expect(root.textContent).toContain('Refactor the broken inheritance chain');
    expect(root.textContent).toContain('typescript');
    expect(root.querySelector('[data-testid="generated-challenge-summary"]')).toBeTruthy();
  });

  it('exposes an "Open in dojo" button', () => {
    fixture.componentRef.setInput('challenge', challenge);
    fixture.componentRef.setInput('loading', false);
    fixture.detectChanges();
    const btn = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="generated-challenge-open-dojo"]',
    ) as HTMLButtonElement;
    expect(btn).toBeTruthy();
  });

  it('emits openDojo with the challenge id when the button is clicked', () => {
    fixture.componentRef.setInput('challenge', challenge);
    fixture.componentRef.setInput('loading', false);
    fixture.detectChanges();
    const emitted: string[] = [];
    component.openDojo.subscribe((id: string) => emitted.push(id));
    const btn = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="generated-challenge-open-dojo"]',
    ) as HTMLButtonElement;
    btn.click();
    expect(emitted).toEqual(['gen-42']);
  });
});
