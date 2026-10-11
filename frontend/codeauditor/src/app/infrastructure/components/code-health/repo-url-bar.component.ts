// repo-url-bar.component.ts — dumb input bar that lets the user paste
// a gogs://<owner>/<name> or github.com/<owner>/<name> URL.
//
// Emits `url` (the raw value) on submit. The page component parses
// owner/name and calls CodeHealthService.submitAnalysis. The component
// holds no service references; it's a presentation-only input.
//
// Hexagonal contract: standalone component, no `inject()` of any use
// case / port. Inputs via the new `input()` signal API (Angular 21);
// outputs via `output()`.
import { Component, computed, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { LucideLink, LucideArrowRight } from '@lucide/angular';

import { CodeHealthSource } from '../../../domain/models/code-health';

@Component({
  selector: 'app-repo-url-bar',
  standalone: true,
  imports: [FormsModule, LucideLink, LucideArrowRight],
  template: `
    <div
      class="flex items-center gap-2 bg-[#161B22] border border-[#30363D] rounded-sm p-3"
      data-testid="repo-url-bar"
    >
      <span class="text-[#8B949E] text-xs uppercase" data-testid="repo-url-source">
        {{ source() }}
      </span>
      <svg lucideLink class="w-4 h-4 text-[#8B949E]"></svg>
      <input
        type="text"
        class="flex-1 bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
        [placeholder]="placeholder()"
        [ngModel]="value"
        (ngModelChange)="onInput($event)"
        (keyup.enter)="submit()"
        data-testid="repo-url-input"
      />
      <button
        type="button"
        class="px-3 py-2 bg-[#238636] hover:bg-[#2EA043] disabled:opacity-50 text-white text-sm font-semibold rounded-sm transition-colors flex items-center gap-1"
        [disabled]="value.trim().length === 0"
        (click)="submit()"
        data-testid="repo-url-submit"
      >
        <svg lucideArrowRight class="w-4 h-4"></svg>
        Analyze
      </button>
    </div>
  `,
})
export class RepoUrlBarComponent {
  readonly source = input.required<CodeHealthSource>();

  // output() signal — emits the raw URL string on submit. The page
  // parses owner + name.
  readonly url = output<{ url: string }>();

  // Internal form state; local signal mirrors the input value via
  // (ngModelChange).
  private readonly _value = signal<string>('');

  get value(): string {
    return this._value();
  }

  set value(v: string) {
    this._value.set(v);
  }

  onInput(value: string): void {
    this._value.set(value);
  }

  placeholder = computed(() =>
    this.source() === 'gogs' ? 'gogs://ggogsmic/repo' : 'github.com/owner/repo',
  );

  // Cheap "owner/name" parse — used by the test to confirm the page
  // can derive these. Lives in the component as a convenience; the
  // page may override parsing for slugged GitHub URLs.
  owner = computed(() => {
    const segments = this._value()
      .split('/')
      .filter((s) => s.length > 0);
    if (this.source() === 'github') {
      // strip "github.com" prefix if present
      if (segments[0] === 'github.com') {
        return segments[1] ?? '';
      }
    }
    return segments[0] ?? '';
  });

  name = computed(() => {
    const segments = this._value()
      .split('/')
      .filter((s) => s.length > 0);
    if (this.source() === 'github') {
      if (segments[0] === 'github.com') {
        return segments[2] ?? '';
      }
      return segments[1] ?? '';
    }
    return segments[1] ?? '';
  });

  submit(): void {
    const trimmed = this._value().trim();
    if (trimmed.length === 0) {
      return;
    }
    this.url.emit({ url: trimmed });
  }
}
