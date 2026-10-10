// language-selector.component.spec.ts — dumb component for picking a
// programming language. The parent page passes the catalog via
// `languages` input; the component emits `languageChange` when the
// user picks a row.
//
// Per strict_tdd: this file lives at RED at HEAD~1 (the component
// does not exist yet) and flips to GREEN when the implementation
// lands.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core';

import { LanguageSelectorComponent } from './language-selector.component';

describe('LanguageSelectorComponent', () => {
  let fixture: ComponentFixture<LanguageSelectorComponent>;
  let component: LanguageSelectorComponent;

  const langs = [
    { id: 'typescript', label: 'TypeScript' },
    { id: 'python', label: 'Python' },
    { id: 'rust', label: 'Rust' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LanguageSelectorComponent],
    }).compileComponents();
    fixture = TestBed.createComponent(LanguageSelectorComponent);
    component = fixture.componentInstance;
  });

  it('renders one button per language entry', () => {
    fixture.componentRef.setInput('languages', langs);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const buttons = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll(
        '[data-testid="language-selector-row"]',
      ),
    );
    expect(buttons).toHaveLength(3);
    expect(buttons[0].textContent).toContain('TypeScript');
    expect(buttons[1].textContent).toContain('Python');
  });

  it('highlights the selected language', () => {
    fixture.componentRef.setInput('languages', langs);
    fixture.componentRef.setInput('selected', 'python');
    fixture.detectChanges();
    const active = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="language-selector-row"][data-active="true"]',
    );
    expect(active?.textContent).toContain('Python');
  });

  it('emits languageChange when a row is clicked', () => {
    fixture.componentRef.setInput('languages', langs);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const emitted: string[] = [];
    component.languageChange.subscribe((id: string) => emitted.push(id));
    const buttons = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll(
        '[data-testid="language-selector-row"]',
      ),
    ) as HTMLButtonElement[];
    buttons[1].click();
    expect(emitted).toEqual(['python']);
  });

  it('does not crash when languages is empty', () => {
    fixture.componentRef.setInput('languages', []);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="language-selector-empty"]')).toBeTruthy();
  });
});