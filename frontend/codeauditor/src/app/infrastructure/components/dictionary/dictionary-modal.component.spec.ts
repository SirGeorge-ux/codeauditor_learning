import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { DictionaryModalComponent } from './dictionary-modal.component';
import { Term } from '../../../domain/models/term';

describe('DictionaryModalComponent', () => {
  let fixture: ComponentFixture<DictionaryModalComponent>;
  let component: DictionaryModalComponent;

  const seedTerm: Term = {
    id: 't-1',
    term: 'closure',
    explanation: 'A function capturing its lexical environment, originally.',
    category: 'Concepto',
    example: undefined,
    synonyms: ['lexical-closure'],
    tags: ['javascript'],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, FormsModule, DictionaryModalComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(DictionaryModalComponent);
    component = fixture.componentInstance;
  });

  it('renders empty fields for a new term', () => {
    component.term = null;
    fixture.detectChanges();
    const errs = component.fieldErrors();
    expect(errs.term).toBe('Term is required');
    expect(component.canSubmit()).toBe(false);
  });

  it('shows inline error when explanation is too short (spec scenario: Modal inline validation)', () => {
    component.term = null;
    fixture.detectChanges();
    component.setField('term', 'shortcode');
    component.setField('explanation', 'too short');
    component.setField('category', 'Concepto');
    fixture.detectChanges();
    const html = fixture.nativeElement as HTMLElement;
    const errBlock = html.querySelector('[data-testid="dictionary-modal-explanation"] + span');
    expect(errBlock?.textContent ?? '').toMatch(/>= 10/);
    expect(component.canSubmit()).toBe(false);
  });

  it('clears inline error when explanation reaches 10+ chars', () => {
    component.term = null;
    fixture.detectChanges();
    component.setField('term', 'shortcode');
    component.setField('explanation', 'just enough length now');
    component.setField('category', 'Concepto');
    fixture.detectChanges();
    const errs = component.fieldErrors();
    expect(errs.explanation).toBeUndefined();
    expect(component.canSubmit()).toBe(true);
  });

  it('blocks submit until canSubmit() is true', () => {
    component.term = null;
    fixture.detectChanges();
    component.setField('term', 'shortcode');
    component.setField('explanation', 'too short');
    component.setField('category', 'Concepto');
    fixture.detectChanges();
    expect(component.canSubmit()).toBe(false);
    expect(buttonDisabled(fixture, 'dictionary-modal-save')).toBe(true);
  });

  it('enables Save button and emits submitForm when valid', () => {
    component.term = null;
    fixture.detectChanges();
    component.setField('term', 'shortcode');
    component.setField('explanation', 'long enough explanation text');
    component.setField('category', 'Concepto');
    let emitted: { term: string } | null = null;
    component.submitForm.subscribe((v) => (emitted = v));
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="dictionary-modal-save"]',
    ) as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.disabled).toBe(false);
    btn.click();
    expect(emitted).toBeTruthy();
    expect((emitted as unknown as { term: string }).term).toBe('shortcode');
  });

  it('prefills when editing an existing term', () => {
    component.term = seedTerm;
    fixture.detectChanges();
    expect(component.formValue().term).toBe('closure');
    expect(component.formValue().explanation).toContain('lexical');
    expect(component.formValue().category).toBe('Concepto');
    expect(component.formValue().synonyms).toEqual(['lexical-closure']);
    expect(component.isEdit()).toBe(true);
  });

  it('parses comma-separated synonyms and tags', () => {
    component.term = null;
    fixture.detectChanges();
    component.setField('term', 'x');
    component.setField('explanation', 'long enough explanation text');
    component.setSynonymsInput('a, b ,c , d');
    component.setTagsInput('js, fp');
    expect(component.formValue().synonyms).toEqual(['a', 'b', 'c', 'd']);
    expect(component.formValue().tags).toEqual(['js', 'fp']);
  });

  it('emits cancelModal when Cancel button clicked', () => {
    let cancelled = 0;
    component.cancelModal.subscribe(() => (cancelled += 1));
    fixture.detectChanges();
    const btn = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Cancel',
    );
    btn?.click();
    expect(cancelled).toBe(1);
  });
});

function buttonDisabled(
  fixture: ComponentFixture<DictionaryModalComponent>,
  testid: string,
): boolean {
  const el = fixture.nativeElement as HTMLElement;
  const btn = el.querySelector(`[data-testid="${testid}"]`) as HTMLButtonElement | null;
  return !!btn?.disabled;
}
