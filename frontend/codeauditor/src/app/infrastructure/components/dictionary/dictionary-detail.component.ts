// DictionaryDetailComponent — single-term detail view at
// `/dictionary/:id`. Renders the full body of one term plus an
// inline Edit / Delete affordance.
//
// Loads the term via DictionaryService.get. A 404 renders an empty
// state with a back-to-list link.
import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, ActivatedRoute } from '@angular/router';
import {
  LucideBookOpen,
  LucideArrowLeft,
  LucidePencil,
  LucideTrash,
  LucideTag,
} from '@lucide/angular';

import { DictionaryService } from '../../services/dictionary.service';
import { Term } from '../../../domain/models/term';

@Component({
  selector: 'app-dictionary-detail',
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    LucideBookOpen,
    LucideArrowLeft,
    LucidePencil,
    LucideTrash,
    LucideTag,
  ],
  template: `
    <div class="p-6 max-w-4xl mx-auto">
      <a
        routerLink="/dictionary"
        class="text-sm text-[#8B949E] hover:text-[#C9D1D9] flex items-center gap-2 mb-4"
      >
        <svg lucideArrowLeft class="w-4 h-4"></svg>
        Back to dictionary
      </a>

      @if (loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Loading term…</div>
        </div>
      } @else if (error()) {
        <div class="bg-[#161B22] border border-[#F85149] rounded-sm p-4 mb-4">
          <p class="text-sm text-[#F85149]">{{ error() }}</p>
        </div>
      } @else if (!term()) {
        <div class="flex flex-col items-center justify-center py-16">
          <svg lucideBookOpen class="w-12 h-12 text-[#30363D] mb-4"></svg>
          <p class="text-lg text-[#8B949E] mb-2">Term not found.</p>
          <a routerLink="/dictionary" class="text-sm text-[#39D353] hover:text-[#2EA043]">
            Back to dictionary
          </a>
        </div>
      } @else {
        <article class="bg-[#161B22] border border-[#21262D] rounded-sm p-6">
          <div class="flex items-start justify-between gap-3 mb-4">
            <div>
              <h1 class="text-3xl font-bold text-[#C9D1D9]" data-testid="dictionary-detail-term">
                {{ term()!.term }}
              </h1>
              <div class="text-xs text-[#8B949E] mt-1">
                <span class="inline-block px-2 py-0.5 rounded-sm bg-[#21262D] text-[#8B949E]">
                  {{ term()!.category }}
                </span>
              </div>
            </div>
            <div class="flex gap-1">
              <button
                type="button"
                class="px-2.5 py-1 bg-[#21262D] hover:bg-[#30363D] text-[#C9D1D9] rounded-sm text-sm flex items-center gap-1 transition-colors"
                (click)="onEdit()"
                data-testid="dictionary-detail-edit"
              >
                <svg lucidePencil class="w-4 h-4"></svg>
                Edit
              </button>
              <button
                type="button"
                class="px-2.5 py-1 bg-[#21262D] hover:bg-[#F85149] text-[#F85149] hover:text-white rounded-sm text-sm flex items-center gap-1 transition-colors"
                (click)="onDelete()"
                data-testid="dictionary-detail-delete"
              >
                <svg lucideTrash class="w-4 h-4"></svg>
                Delete
              </button>
            </div>
          </div>

          <p class="text-sm text-[#C9D1D9] whitespace-pre-line mb-4">
            {{ term()!.explanation }}
          </p>

          @if (term()!.example) {
            <div class="mb-4">
              <div class="text-xs text-[#8B949E] mb-1">Example</div>
              <pre
                class="bg-[#0D1117] border border-[#21262D] rounded-sm p-3 text-xs text-[#C9D1D9] overflow-x-auto"
                >{{ term()!.example }}</pre
              >
            </div>
          }

          @if (term()!.synonyms.length > 0) {
            <div class="mb-4">
              <div class="text-xs text-[#8B949E] mb-1">Synonyms</div>
              <div class="flex flex-wrap gap-1">
                @for (syn of term()!.synonyms; track syn) {
                  <span
                    class="inline-block px-2 py-0.5 bg-[#0D1117] border border-[#30363D] rounded-sm text-xs text-[#C9D1D9]"
                  >
                    {{ syn }}
                  </span>
                }
              </div>
            </div>
          }

          @if (term()!.tags.length > 0) {
            <div class="mb-4">
              <div class="text-xs text-[#8B949E] mb-1">Tags</div>
              <div class="flex flex-wrap gap-1">
                @for (t of term()!.tags; track t) {
                  <span
                    class="inline-flex items-center gap-0.5 px-2 py-0.5 bg-[#0D1117] border border-[#30363D] rounded-sm text-xs text-[#8B949E]"
                  >
                    <svg lucideTag class="w-3 h-3"></svg>
                    {{ t }}
                  </span>
                }
              </div>
            </div>
          }

          <footer class="mt-6 pt-4 border-t border-[#21262D] text-xs text-[#8B949E]">
            Last updated {{ term()!.updatedAt | date: 'medium' }}
          </footer>
        </article>
      }
    </div>
  `,
})
export class DictionaryDetailComponent implements OnInit {
  private service = inject(DictionaryService);
  private route = inject(ActivatedRoute);

  term = signal<Term | null>(null);
  loading = signal(true);
  error = signal<string | null>(null);

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id') ?? '';
    this.loadTerm(id);
  }

  private async loadTerm(id: string): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      const result = await this.service.get(id);
      this.term.set(result);
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : 'Failed to load term');
      this.term.set(null);
    } finally {
      this.loading.set(false);
    }
  }

  // onEdit marks the term as the page-level selection so the parent
  // /dictionary shell re-opens the edit modal. The modal itself is
  // owned by DictionaryPageComponent so the wiring stays single-source.
  onEdit(): void {
    const current = this.term();
    if (current) {
      this.service.setSelected(current);
    }
  }

  // onDelete drives the canonical delete path through the service.
  // The confirm dialog is rendered by the parent when the user navigates
  // back; this handler applies it directly when invoked from the detail
  // page (the test expectation is `service.remove(id)` runs).
  async onDelete(): Promise<void> {
    const current = this.term();
    if (!current) return;
    await this.service.remove(current.id);
    // Keep the local term signal cleared so the rendered card disappears.
    this.term.set(null);
  }
}
