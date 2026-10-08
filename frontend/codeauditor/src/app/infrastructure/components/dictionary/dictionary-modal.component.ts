// DictionaryModalComponent — add/edit modal for a single term.
//
// Emits `submitForm` with a validated input payload on Save; emits
// `cancel` on any dismiss path (X button, Cancel button, Escape key
// when focus is in the modal).
//
// Form-level validation mirrors backend / use-case gating so the user
// sees feedback before the round-trip. The component holds local form
// state via signals — there is no FormsModule/ReactiveFormsManager
// dependency to keep this MVP simple.
//
// Emits `cancelModal` on any dismiss path (X button, Cancel button,
// Escape key when focus is in the modal). The component-level `cancel`
// DOM event name is reserved by Angular — using it as an Output triggers
// `@angular-eslint/no-output-native`, so we use `cancelModal` and document
// the wire here.
import { Component, EventEmitter, Input, Output, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { LucideSave, LucideX } from '@lucide/angular';

import { Term, TermCategory, TERM_CATEGORIES } from '../../../domain/models/term';

export interface TermFormValue {
  term: string;
  explanation: string;
  category: TermCategory;
  example?: string;
  synonyms: string[];
  tags: string[];
}

@Component({
  selector: 'app-dictionary-modal',
  standalone: true,
  imports: [CommonModule, FormsModule, LucideSave, LucideX],
  template: `
    <div
      class="fixed inset-0 bg-black/60 flex items-center justify-center z-40"
      data-testid="dictionary-modal"
    >
      <div
        class="bg-[#161B22] border border-[#30363D] rounded-sm w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto"
        (click)="$event.stopPropagation()"
      >
        <div class="flex items-center justify-between p-4 border-b border-[#21262D]">
          <h2 class="text-lg font-semibold text-[#C9D1D9]">
            {{ isEdit() ? 'Edit term' : 'Add term' }}
          </h2>
          <button
            type="button"
            class="text-[#8B949E] hover:text-[#C9D1D9]"
            (click)="cancelModal.emit()"
            aria-label="Close"
          >
            <svg lucideX class="w-5 h-5"></svg>
          </button>
        </div>

        <div class="p-4 space-y-4">
          <!-- Term -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Term</span>
            <input
              type="text"
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="formValue().term"
              (ngModelChange)="setField('term', $event)"
              placeholder="e.g. closure"
              maxlength="50"
              data-testid="dictionary-modal-term"
            />
            @if (fieldErrors().term) {
              <span class="text-xs text-[#F85149] mt-1">{{ fieldErrors().term }}</span>
            }
          </label>

          <!-- Explanation -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Explanation (10..500 chars)</span>
            <textarea
              rows="4"
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="formValue().explanation"
              (ngModelChange)="setField('explanation', $event)"
              placeholder="A function capturing its lexical environment…"
              maxlength="500"
              data-testid="dictionary-modal-explanation"
            ></textarea>
            @if (fieldErrors().explanation) {
              <span class="text-xs text-[#F85149] mt-1">{{ fieldErrors().explanation }}</span>
            } @else {
              <span class="text-xs text-[#8B949E] mt-1">
                {{ formValue().explanation.length }}/500 chars
              </span>
            }
          </label>

          <!-- Category -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Category</span>
            <select
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="formValue().category"
              (ngModelChange)="setField('category', $event)"
              data-testid="dictionary-modal-category"
            >
              @for (cat of categories; track cat) {
                <option [value]="cat">{{ cat }}</option>
              }
            </select>
            @if (fieldErrors().category) {
              <span class="text-xs text-[#F85149] mt-1">{{ fieldErrors().category }}</span>
            }
          </label>

          <!-- Example -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Example (optional)</span>
            <textarea
              rows="2"
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="formValue().example || ''"
              (ngModelChange)="setField('example', $event)"
              placeholder="const fn = x => x + 1;"
              maxlength="1000"
              data-testid="dictionary-modal-example"
            ></textarea>
          </label>

          <!-- Synonyms -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Synonyms (comma-separated)</span>
            <input
              type="text"
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="synonymsInput()"
              (ngModelChange)="setSynonymsInput($event)"
              placeholder="lexical-closure, scope-binding"
              data-testid="dictionary-modal-synonyms"
            />
          </label>

          <!-- Tags -->
          <label class="block">
            <span class="text-xs text-[#8B949E]">Tags (comma-separated)</span>
            <input
              type="text"
              class="mt-1 w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
              [ngModel]="tagsInput()"
              (ngModelChange)="setTagsInput($event)"
              placeholder="javascript, fundamentals"
              data-testid="dictionary-modal-tags"
            />
          </label>
        </div>

        <div class="flex items-center justify-end gap-2 p-4 border-t border-[#21262D]">
          <button
            type="button"
            class="px-3 py-1.5 bg-[#21262D] hover:bg-[#30363D] text-[#C9D1D9] rounded-sm text-sm transition-colors"
            (click)="cancelModal.emit()"
          >
            Cancel
          </button>
          <button
            type="button"
            class="px-3 py-1.5 bg-[#238636] hover:bg-[#2EA043] text-white rounded-sm text-sm transition-colors flex items-center gap-1 disabled:opacity-50"
            (click)="onSubmit()"
            [disabled]="!canSubmit()"
            data-testid="dictionary-modal-save"
          >
            <svg lucideSave class="w-4 h-4"></svg>
            {{ isEdit() ? 'Save changes' : 'Add term' }}
          </button>
        </div>
      </div>
    </div>
  `,
})
export class DictionaryModalComponent {
  @Input() set term(value: Term | null) {
    if (value) {
      this.editingExisting = true;
      this.formValue.set({
        term: value.term,
        explanation: value.explanation,
        category: value.category,
        example: value.example,
        synonyms: value.synonyms ?? [],
        tags: value.tags ?? [],
      });
      this.synonymsInput.set((value.synonyms ?? []).join(', '));
      this.tagsInput.set((value.tags ?? []).join(', '));
    } else {
      this.editingExisting = false;
      this.formValue.set({
        term: '',
        explanation: '',
        category: 'Concepto',
        example: '',
        synonyms: [],
        tags: [],
      });
      this.synonymsInput.set('');
      this.tagsInput.set('');
    }
  }

  @Output() cancelModal = new EventEmitter<void>();
  @Output() submitForm = new EventEmitter<TermFormValue>();

  categories: TermCategory[] = TERM_CATEGORIES;

  formValue = signal<TermFormValue>({
    term: '',
    explanation: '',
    category: 'Concepto',
    synonyms: [],
    tags: [],
  });
  synonymsInput = signal<string>('');
  tagsInput = signal<string>('');

  editingExisting = false;

  isEdit = computed(() => this.editingExisting);

  // Inline field-level errors. Mirrors the use-case validation surface so
  // the user sees the same wording once the request roundtrips.
  fieldErrors = computed<{ term?: string; explanation?: string; category?: string }>(() => {
    const errs: { term?: string; explanation?: string; category?: string } = {};
    const v = this.formValue();
    const term = (v.term ?? '').trim();
    if (term.length === 0) errs.term = 'Term is required';
    else if (term.length > 50) errs.term = 'Term must be <= 50 chars';
    const e = v.explanation ?? '';
    if (e.length > 0 && e.length < 10) errs.explanation = 'Explanation must be >= 10 chars';
    if (e.length > 500) errs.explanation = 'Explanation must be <= 500 chars';
    if (!TERMS_HAS(v.category)) errs.category = 'Category is required';
    return errs;
  });

  canSubmit = computed(() => {
    const v = this.formValue();
    return (
      (v.term ?? '').trim().length > 0 &&
      (v.term ?? '').trim().length <= 50 &&
      (v.explanation ?? '').length >= 10 &&
      (v.explanation ?? '').length <= 500 &&
      TERMS_HAS(v.category)
    );
  });

  setField<K extends keyof TermFormValue>(field: K, value: TermFormValue[K]): void {
    this.formValue.set({ ...this.formValue(), [field]: value });
  }

  setSynonymsInput(value: string): void {
    this.synonymsInput.set(value);
    this.formValue.set({ ...this.formValue(), synonyms: stringSplit(value) });
  }

  setTagsInput(value: string): void {
    this.tagsInput.set(value);
    this.formValue.set({ ...this.formValue(), tags: stringSplit(value) });
  }

  onSubmit(): void {
    if (!this.canSubmit()) return;
    const v = this.formValue();
    this.submitForm.emit({
      term: v.term.trim(),
      explanation: v.explanation.trim(),
      category: v.category,
      example: v.example?.trim() || undefined,
      synonyms: v.synonyms,
      tags: v.tags,
    });
  }
}

function stringSplit(value: string): string[] {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function TERMS_HAS(value: string): value is TermCategory {
  return (TERMS_CATEGORIES_VALUES as readonly string[]).includes(value);
}

const TERMS_CATEGORIES_VALUES = TERM_CATEGORIES;
