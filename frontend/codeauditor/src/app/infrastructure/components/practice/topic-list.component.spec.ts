// topic-list.component.spec.ts — dumb list of CurriculumTopic rows.
// The parent page passes the filtered topics + the current value;
// the component emits `topicChange` when the user picks a row.
//
// RED at HEAD~1; GREEN once the implementation lands.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';

import { TopicListComponent } from './topic-list.component';
import { CurriculumTopic } from '../../../domain/models/curriculum';

describe('TopicListComponent', () => {
  let fixture: ComponentFixture<TopicListComponent>;
  let component: TopicListComponent;

  const topics: CurriculumTopic[] = [
    { id: 'variables', title: 'Variables', difficulty: 'F', description: 'let vs const' },
    { id: 'inheritance', title: 'Inheritance', difficulty: 'D', description: 'OOP basics' },
    { id: 'generics', title: 'Generics', difficulty: 'C', description: 'Type params' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TopicListComponent],
    }).compileComponents();
    fixture = TestBed.createComponent(TopicListComponent);
    component = fixture.componentInstance;
  });

  it('renders one row per topic', () => {
    fixture.componentRef.setInput('topics', topics);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const rows = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('[data-testid="topic-list-row"]'),
    );
    expect(rows).toHaveLength(3);
  });

  it('shows the F..S letter as a badge per row', () => {
    fixture.componentRef.setInput('topics', topics);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const html = fixture.nativeElement as HTMLElement;
    expect(
      html.querySelector('[data-testid="topic-list-row"][data-id="variables"]')?.textContent,
    ).toContain('F');
    expect(
      html.querySelector('[data-testid="topic-list-row"][data-id="inheritance"]')?.textContent,
    ).toContain('D');
  });

  it('highlights the selected topic', () => {
    fixture.componentRef.setInput('topics', topics);
    fixture.componentRef.setInput('selected', 'generics');
    fixture.detectChanges();
    const active = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="topic-list-row"][data-active="true"]',
    );
    expect(active?.getAttribute('data-id')).toBe('generics');
  });

  it('emits topicChange when a row is clicked', () => {
    fixture.componentRef.setInput('topics', topics);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const emitted: string[] = [];
    component.topicChange.subscribe((id: string) => emitted.push(id));
    const row = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="topic-list-row"][data-id="inheritance"]',
    ) as HTMLButtonElement;
    row.click();
    expect(emitted).toEqual(['inheritance']);
  });

  it('shows an empty state when no topics are provided', () => {
    fixture.componentRef.setInput('topics', []);
    fixture.componentRef.setInput('selected', null);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="topic-list-empty"]')).toBeTruthy();
  });
});
