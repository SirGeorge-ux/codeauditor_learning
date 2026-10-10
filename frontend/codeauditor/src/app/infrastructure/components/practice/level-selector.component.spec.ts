// level-selector.component.spec.ts — dumb component for picking an
// F..S level. The parent page passes the band + the current value;
// the component emits `levelChange` when the user picks a letter.
//
// RED at HEAD~1 (component missing); GREEN when implementation lands.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';

import { LevelSelectorComponent } from './level-selector.component';
import { F_S_LEVELS } from '../../../domain/models/curriculum';

describe('LevelSelectorComponent', () => {
  let fixture: ComponentFixture<LevelSelectorComponent>;
  let component: LevelSelectorComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LevelSelectorComponent],
    }).compileComponents();
    fixture = TestBed.createComponent(LevelSelectorComponent);
    component = fixture.componentInstance;
  });

  it('renders one button per F..S letter', () => {
    fixture.componentRef.setInput('levels', F_S_LEVELS);
    fixture.componentRef.setInput('selected', 'F');
    fixture.detectChanges();
    const buttons = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="level-selector-row"]'),
    );
    expect(buttons).toHaveLength(F_S_LEVELS.length);
    expect(F_S_LEVELS).toEqual(['F', 'E', 'D', 'C', 'B', 'A', 'S']);
  });

  it('highlights the selected level', () => {
    fixture.componentRef.setInput('levels', F_S_LEVELS);
    fixture.componentRef.setInput('selected', 'D');
    fixture.detectChanges();
    const active = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="level-selector-row"][data-active="true"]',
    );
    expect(active?.textContent?.trim()).toBe('D');
  });

  it('emits levelChange when a row is clicked', () => {
    fixture.componentRef.setInput('levels', F_S_LEVELS);
    fixture.componentRef.setInput('selected', 'F');
    fixture.detectChanges();
    const emitted: string[] = [];
    component.levelChange.subscribe((v: string) => emitted.push(v));
    const buttons = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="level-selector-row"]'),
    ) as HTMLButtonElement[];
    buttons[3].click(); // index 3 → 'C'
    expect(emitted).toEqual(['C']);
  });
});
