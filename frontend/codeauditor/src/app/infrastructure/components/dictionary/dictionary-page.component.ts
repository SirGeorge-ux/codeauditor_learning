// DictionaryPageComponent — top-level page for the user's personal tech
// dictionary. Houses the search input, A-Z grid, category chips, list
// panel and add/edit modal. The detail view is rendered separately at
// `/dictionary/:id` (DictionaryDetailComponent) so the user can deep-link
// to a single term.
//
// Hexagonal contract: this component depends ONLY on DictionaryService
// (infrastructure injectable). No HTTP / fetch / signal-direct uses leak
// into the template — every interaction goes through the use case.
import { Component, inject, OnInit, OnDestroy, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import {
  LucideBookOpen,
  LucidePlus,
  LucideSearch,
  LucideTrash,
  LucidePencil,
  LucideTag,
  LucideFilter,
} from '@lucide/angular';

import { DictionaryService } from '../../services/dictionary.service';
import { DictionaryModalComponent } from './dictionary-modal.component';
import { Term, TermCategory, TERM_CATEGORIES } from '../../../domain/models/term';

@Component({
  selector: 'app-dictionary-page',
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    DictionaryModalComponent,
    LucideBookOpen,
    LucidePlus,
    LucideSearch,
    LucideTrash,
    LucidePencil,
    LucideTag,
    LucideFilter,
  ],
  template: `
    <div class="p-6 max-w-6xl mx-auto">
      <!-- Header -->
      <div class="flex items-center gap-3 mb-6">
        <svg lucideBookOpen class="w-7 h-7 text-[#8B949E]"></svg>
        <div class="flex-1">
          <h1 class="text-2xl font-bold text-[#C9D1D9]">Dictionary</h1>
          <p class="text-sm text-[#8B949E]">
            Your private glossary — collect, classify, and recall technical terms.
          </p>
        </div>
        <button
          type="button"
          class="px-3 py-2 bg-[#238636] hover:bg-[#2EA043] text-white text-sm font-semibold rounded-sm transition-colors flex items-center gap-2"
          (click)="addNewTerm()"
          data-testid="dictionary-add-button"
        >
          <svg lucidePlus class="w-4 h-4"></svg>
          Add term
        </button>
      </div>

      <!-- Error banner -->
      @if (service.error()) {
        <div
          class="bg-[#161B22] border border-[#F85149] rounded-sm p-3 mb-4 flex items-center justify-between"
          data-testid="dictionary-error-banner"
        >
          <span class="text-sm text-[#F85149]">{{ service.error() }}</span>
          <button
            type="button"
            class="text-xs text-[#8B949E] hover:text-[#C9D1D9]"
            (click)="service.clearError()"
          >
            Dismiss
          </button>
        </div>
      }

      <!-- Search bar + category filter -->
      <div class="flex flex-col md:flex-row gap-3 mb-6">
        <div class="flex-1 relative">
          <svg
            lucideSearch
            class="w-4 h-4 text-[#8B949E] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none"
          ></svg>
          <input
            type="text"
            placeholder="Search terms, explanations, synonyms, tags…"
            class="w-full bg-[#161B22] border border-[#30363D] rounded-sm pl-9 pr-3 py-2 text-sm text-[#C9D1D9] placeholder:text-[#8B949E] focus:border-[#39D353] focus:outline-none"
            [value]="searchValue"
            (input)="onSearchInput($any($event.target).value)"
            data-testid="dictionary-search-input"
          />
        </div>
        <div class="flex items-center gap-2">
          <svg lucideFilter class="w-4 h-4 text-[#8B949E]"></svg>
          @for (cat of categories; track cat) {
            <button
              type="button"
              class="px-2.5 py-1 rounded-sm text-xs font-medium transition-colors"
              [class.bg-[#238636]]="activeCategory() === cat"
              [class.text-white]="activeCategory() === cat"
              [class.bg-[#21262D]]="activeCategory() !== cat"
              [class.text-[#8B949E]]="activeCategory() !== cat"
              [class.hover:text-[#C9D1D9]]="activeCategory() !== cat"
              (click)="onCategoryChip(activeCategory() === cat ? null : cat)"
              [attr.data-testid]="'dictionary-category-' + cat"
            >
              {{ cat }}
            </button>
          }
        </div>
      </div>

      <!-- A-Z grid nav -->
      <div class="mb-4 flex flex-wrap gap-1">
        @for (letter of aToZ; track letter) {
          @if (letterCounts[letter] && letterCounts[letter] > 0) {
            <button
              type="button"
              class="w-8 h-8 flex items-center justify-center rounded-sm text-sm font-medium transition-colors"
              [class.bg-[#238636]]="activeLetter() === letter"
              [class.text-white]="activeLetter() === letter"
              [class.bg-[#21262D]]="activeLetter() !== letter"
              [class.text-[#C9D1D9]]="activeLetter() !== letter"
              [class.hover:bg-[#30363D]]="activeLetter() !== letter"
              (click)="onLetterClick(letter)"
              [attr.data-testid]="'dictionary-letter-' + letter"
            >
              {{ letter }}
            </button>
          } @else {
            <span
              class="w-8 h-8 flex items-center justify-center text-[#8B949E] text-sm opacity-40"
              [attr.data-testid]="'dictionary-letter-' + letter"
            >
              {{ letter }}
            </span>
          }
        }
      </div>

      <!-- Loading + content -->
      @if (service.loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Loading…</div>
        </div>
      } @else if (service.hasTerms() && flatTerms().length === 0) {
        <div class="flex flex-col items-center justify-center py-12">
          <svg lucideBookOpen class="w-12 h-12 text-[#30363D] mb-4"></svg>
          <p class="text-[#8B949E]">No terms match the current filter.</p>
          <button
            type="button"
            class="mt-3 text-sm text-[#39D353] hover:text-[#2EA043]"
            (click)="clearFiltersAndReload()"
          >
            Clear filters
          </button>
        </div>
      } @else if (!service.hasTerms()) {
        <div class="flex flex-col items-center justify-center py-12">
          <svg lucideBookOpen class="w-12 h-12 text-[#30363D] mb-4"></svg>
          <p class="text-lg text-[#8B949E] mb-2">Your dictionary is empty.</p>
          <button
            type="button"
            class="text-sm text-[#39D353] hover:text-[#2EA043]"
            (click)="addNewTerm()"
          >
            Add your first term
          </button>
        </div>
      } @else {
        <!-- Letter-grouped list -->
        <div class="space-y-6" data-testid="dictionary-list">
          @for (bucket of visibleBuckets(); track bucket.letter) {
            <section>
              <div class="flex items-baseline gap-3 mb-2">
                <h2 class="text-xl font-bold text-[#39D353]">{{ bucket.letter }}</h2>
                <span class="text-xs text-[#8B949E]">{{ bucket.terms.length }} terms</span>
              </div>
              <ul class="space-y-1">
                @for (term of bucket.terms; track term.id) {
                  <li>
                    <a
                      [routerLink]="['/dictionary', term.id]"
                      class="flex items-center justify-between bg-[#161B22] border border-[#21262D] rounded-sm p-3 hover:border-[#30363D] transition-colors"
                      (click)="selectFromList(term)"
                      [attr.data-testid]="'dictionary-row-' + term.id"
                    >
                      <div class="min-w-0 flex-1">
                        <div class="flex items-center gap-2">
                          <span class="text-sm font-semibold text-[#C9D1D9]">{{ term.term }}</span>
                          <span
                            class="inline-block px-1.5 py-0.5 rounded-sm bg-[#21262D] text-[10px] text-[#8B949E]"
                          >
                            {{ term.category }}
                          </span>
                        </div>
                        <div class="text-xs text-[#8B949E] truncate">
                          {{ term.explanation }}
                        </div>
                        @if (term.tags.length > 0) {
                          <div class="flex flex-wrap gap-1 mt-1">
                            @for (tag of term.tags; track tag) {
                              <span
                                class="inline-flex items-center gap-0.5 px-1.5 py-0.5 bg-[#0D1117] border border-[#30363D] rounded-sm text-[10px] text-[#8B949E]"
                              >
                                <svg lucideTag class="w-3 h-3"></svg>
                                {{ tag }}
                              </span>
                            }
                          </div>
                        }
                      </div>
                      <div class="flex gap-1 ml-2">
                        <button
                          type="button"
                          class="p-1.5 rounded-sm text-[#8B949E] hover:bg-[#21262D] hover:text-[#C9D1D9] transition-colors"
                          (click)="
                            editTerm(term); $event.preventDefault(); $event.stopPropagation()
                          "
                          [attr.data-testid]="'dictionary-edit-' + term.id"
                          aria-label="Edit term"
                        >
                          <svg lucidePencil class="w-4 h-4"></svg>
                        </button>
                        <button
                          type="button"
                          class="p-1.5 rounded-sm text-[#8B949E] hover:bg-[#21262D] hover:text-[#F85149] transition-colors"
                          (click)="
                            requestDelete(term); $event.preventDefault(); $event.stopPropagation()
                          "
                          [attr.data-testid]="'dictionary-delete-' + term.id"
                          aria-label="Delete term"
                        >
                          <svg lucideTrash class="w-4 h-4"></svg>
                        </button>
                      </div>
                    </a>
                  </li>
                }
              </ul>
            </section>
          }
        </div>
      }

      <!-- Modal (Add / Edit) -->
      @if (modalOpen()) {
        <app-dictionary-modal
          [term]="editingTerm()"
          (cancelModal)="closeModal()"
          (submitForm)="onModalSubmit($event)"
          data-testid="dictionary-modal"
        ></app-dictionary-modal>
      }

      <!-- Delete confirmation -->
      @if (deleteCandidate()) {
        <div
          class="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
          (click)="cancelDelete()"
          data-testid="dictionary-delete-overlay"
        >
          <div
            class="bg-[#161B22] border border-[#F85149] rounded-sm p-5 max-w-md mx-4"
            (click)="$event.stopPropagation()"
          >
            <h3 class="text-sm font-semibold text-[#C9D1D9] mb-2">Delete term?</h3>
            <p class="text-xs text-[#8B949E] mb-4">
              "{{ deleteCandidate()!.term }}" will be removed permanently.
            </p>
            <div class="flex gap-2 justify-end">
              <button
                type="button"
                class="px-3 py-1.5 bg-[#21262D] hover:bg-[#30363D] text-[#C9D1D9] rounded-sm text-sm transition-colors"
                (click)="cancelDelete()"
              >
                Cancel
              </button>
              <button
                type="button"
                class="px-3 py-1.5 bg-[#F85149] hover:bg-[#DA3633] text-white rounded-sm text-sm transition-colors flex items-center gap-1"
                (click)="confirmDelete()"
                data-testid="dictionary-delete-confirm"
              >
                <svg lucideTrash class="w-4 h-4"></svg>
                Delete
              </button>
            </div>
          </div>
        </div>
      }
    </div>
  `,
})
export class DictionaryPageComponent implements OnInit, OnDestroy {
  service = inject(DictionaryService);
  searchValue = '';

  // Modal state
  modalOpen = signal(false);
  editingTerm = signal<Term | null>(null);

  // Letter / category filter state (UI level only — the actual narrow
  // happens server-side via repo filters).
  activeLetter = signal<string | null>(null);
  activeCategory = signal<string | null>(null);

  // Delete confirmation state
  deleteCandidate = signal<Term | null>(null);

  // Static UI model
  categories: TermCategory[] = TERM_CATEGORIES;
  aToZ = '#ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');

  // Debounce handle for search input.
  private debounceHandle: ReturnType<typeof setTimeout> | null = null;

  ngOnInit(): void {
    this.service.loadAll();
  }

  ngOnDestroy(): void {
    if (this.debounceHandle !== null) {
      clearTimeout(this.debounceHandle);
      this.debounceHandle = null;
    }
  }

  // onSearchInput is wired to the search <input>; debounce 300ms per
  // spec scenario "Search bar filters results in real-time (debounced at
  // 300 ms)".
  onSearchInput(value: string): void {
    this.searchValue = value;
    if (this.debounceHandle !== null) {
      clearTimeout(this.debounceHandle);
    }
    this.debounceHandle = setTimeout(() => {
      this.service.search(this.searchValue);
    }, 300);
  }

  // Letter click narrows the visible buckets locally; does NOT call the
  // repo because the A-Z nav acts as an in-memory filter on the already-
  // loaded terms.
  onLetterClick(letter: string): void {
    this.activeLetter.set(this.activeLetter() === letter ? null : letter);
  }

  // Category chip routes through repo filter for spec scenario "Filter
  // chip selection" (Patrón → only Patrón terms shown).
  async onCategoryChip(category: string | null): Promise<void> {
    this.activeCategory.set(category);
    if (category === null) {
      this.activeLetter.set(null);
      await this.service.loadAll();
    } else {
      await this.service.filterByCategory(category);
    }
  }

  // Clear all filters and reload everything.
  clearFiltersAndReload(): void {
    this.activeLetter.set(null);
    this.activeCategory.set(null);
    this.service.loadAll();
  }

  // flatTerms is the visible list with activeLetter applied. The
  // categorized() helper from the use case provides the unfiltered A-Z
  // grouping; we narrow it here so the A-Z chip stays local.
  flatTerms(): Term[] {
    if (!this.activeLetter()) return this.service.terms();
    const letter = this.activeLetter()!;
    return this.service.terms().filter((t) => {
      const first = (t.term[0] ?? '').toUpperCase();
      return letter === '#' ? !/[A-Z]/.test(first) : first === letter;
    });
  }

  // visibleBuckets groups the filtered terms into A-Z buckets for
  // rendering.
  visibleBuckets(): { letter: string; terms: Term[] }[] {
    if (!this.activeLetter()) return this.service.categorized();
    return [
      {
        letter: this.activeLetter()!,
        terms: this.flatTerms(),
      },
    ];
  }

  // Computed letter counts for the A-Z navigation badges (drives which
  // letters show as enabled vs faded).
  get letterCounts(): Record<string, number> {
    const counts: Record<string, number> = {};
    for (const term of this.service.terms()) {
      const first = (term.term[0] ?? '').toUpperCase();
      const letter = /[A-Z]/.test(first) ? first : '#';
      counts[letter] = (counts[letter] ?? 0) + 1;
    }
    return counts;
  }

  // --- modal handlers ---

  addNewTerm(): void {
    this.editingTerm.set(null);
    this.modalOpen.set(true);
  }

  editTerm(term: Term): void {
    this.editingTerm.set(term);
    this.modalOpen.set(true);
  }

  closeModal(): void {
    this.modalOpen.set(false);
    this.editingTerm.set(null);
  }

  async onModalSubmit(input: {
    term: string;
    explanation: string;
    category: TermCategory;
    example?: string;
    synonyms: string[];
    tags: string[];
  }): Promise<void> {
    const editing = this.editingTerm();
    const result =
      editing === null
        ? await this.service.add(input)
        : await this.service.update(editing.id, input);
    if (result) {
      this.closeModal();
    }
  }

  // --- delete confirmation ---

  requestDelete(term: Term): void {
    this.deleteCandidate.set(term);
  }

  cancelDelete(): void {
    this.deleteCandidate.set(null);
  }

  async confirmDelete(): Promise<void> {
    const candidate = this.deleteCandidate();
    if (!candidate) return;
    await this.service.remove(candidate.id);
    this.deleteCandidate.set(null);
  }

  // --- row selection (router-link target) ---

  selectFromList(term: Term): void {
    this.service.setSelected(term);
  }
}
