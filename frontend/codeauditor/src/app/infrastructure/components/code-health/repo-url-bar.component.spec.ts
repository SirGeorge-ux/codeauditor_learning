// repo-url-bar.component.spec.ts — dumb input bar that lets the user
// paste a gogs:// or github.com URL. Emits {url} on submit so the
// page can drive submitAnalysis. The component holds no service
// references; the page owns the use case.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { RepoUrlBarComponent } from './repo-url-bar.component';

describe('RepoUrlBarComponent', () => {
  let fixture: ComponentFixture<RepoUrlBarComponent>;
  let component: RepoUrlBarComponent;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, FormsModule, RepoUrlBarComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(RepoUrlBarComponent);
    component = fixture.componentInstance;
  });

  it('renders the right placeholder for gogs vs github', () => {
    fixture.componentRef.setInput('source', 'gogs');
    fixture.detectChanges();
    let input = fixture.nativeElement.querySelector(
      '[data-testid="repo-url-input"]',
    ) as HTMLInputElement;
    expect(input.placeholder).toBe('gogs://ggogsmic/repo');

    fixture.componentRef.setInput('source', 'github');
    fixture.detectChanges();
    input = fixture.nativeElement.querySelector(
      '[data-testid="repo-url-input"]',
    ) as HTMLInputElement;
    expect(input.placeholder).toBe('github.com/owner/repo');
  });

  it('emits url when user submits a non-empty input', () => {
    fixture.componentRef.setInput('source', 'gogs');
    let emitted: { url: string } | null = null;
    component.url.subscribe((v) => (emitted = v));
    fixture.detectChanges();
    component.value = 'ggogsmic/academy-mic';
    component.submit();
    expect(emitted).toEqual({ url: 'ggogsmic/academy-mic' });
  });

  it('blocks submit when the input is empty or whitespace', () => {
    fixture.componentRef.setInput('source', 'gogs');
    let emitted = 0;
    component.url.subscribe(() => (emitted += 1));
    fixture.detectChanges();
    component.value = '   ';
    component.submit();
    expect(emitted).toBe(0);
  });

  it('exposes the parsed owner + name as computed signals', () => {
    fixture.componentRef.setInput('source', 'gogs');
    component.value = 'owner/name';
    fixture.detectChanges();
    expect(component.owner()).toBe('owner');
    expect(component.name()).toBe('name');
  });
});
