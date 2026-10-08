// DictionaryUseCase — application service orchestrating the user's personal
// tech dictionary. Uses signals for reactive state. Delegates storage to
// the DictionaryRepository port.
//
// Responsibilities:
//   - Load all terms (with optional filters)
//   - Search by free-text query
//   - Filter by category
//   - Add / update / remove terms with client-side validation
//   - Track the currently selected term for the detail view
import { signal, computed } from '@angular/core';
import { Term, TermInput, isValidCategory } from '../domain/models/term';
import { DictionaryRepository, ListFilters } from '../domain/ports/dictionary-repository.port';

// client-side error sentinel mirroring backend ErrDictionaryValidation. We
// reuse this Error type at the throw site so the same `error` signal shape
// carries both backend and client validation details.
export const ERR_DICTIONARY_VALIDATION = new Error('dictionary input validation failed');

// client-side validation mirrors services.validateTerm so failures feel
// instantaneous in the form. The backend re-validates authoritatively; if
// either side rejects the same error wording flows back via the error
// signal.
function validateInput(input: TermInput): string | null {
  const term = input.term?.trim() ?? '';
  if (term.length === 0) {
    return 'term is required';
  }
  if (term.length > 50) {
    return 'term must be <= 50 chars';
  }
  const explanation = input.explanation ?? '';
  if (explanation.length < 10) {
    return 'explanation must be >= 10 chars';
  }
  if (explanation.length > 500) {
    return 'explanation must be <= 500 chars';
  }
  if (!isValidCategory(input.category)) {
    return `category "${input.category}" is not in the allowed list`;
  }
  for (const syn of input.synonyms ?? []) {
    if (syn.length > 50) {
      return `synonym "${syn}" must be <= 50 chars`;
    }
  }
  for (const tag of input.tags ?? []) {
    if (tag.length > 30) {
      return `tag "${tag}" must be <= 30 chars`;
    }
  }
  return null;
}

// -- internal helper: wrap a repo op so loading + error signals stay
// consistent across every public method. The helper closure-less
// implementation avoids the cross-instance private-access check.
class OperationContext {
  constructor(
    private readonly setLoading: (v: boolean) => void,
    private readonly setError: (msg: string | null) => void,
  ) {}
  async run<T>(fn: () => Promise<T>): Promise<T | undefined> {
    this.setLoading(true);
    this.setError(null);
    try {
      return await fn();
    } catch (e) {
      this.setError(e instanceof Error ? e.message : String(e));
      return undefined;
    } finally {
      this.setLoading(false);
    }
  }
}

export class DictionaryUseCase {
  // Reactive state
  private readonly _terms = signal<Term[]>([]);
  private readonly _selectedTerm = signal<Term | null>(null);
  private readonly _loading = signal(false);
  private readonly _error = signal<string | null>(null);

  // Public readonly signals
  readonly terms = this._terms.asReadonly();
  readonly selectedTerm = this._selectedTerm.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();
  readonly hasTerms = computed(() => this._terms().length > 0);

  // Internal: shared error/loading context for every public method so
  // progress and error signals stay consistent regardless of which
  // operation just completed.
  private readonly ctx: OperationContext;

  constructor(private readonly repo: DictionaryRepository) {
    this.ctx = new OperationContext(
      (v) => this._loading.set(v),
      (msg) => this._error.set(msg),
    );
  }

  // loadAll reloads the entire glossary (no filters).
  async loadAll(): Promise<void> {
    await this.ctx.run(async () => {
      const list = await this.repo.listAll();
      this._terms.set(list);
    });
  }

  // search routes through the repo's query filter — backend uses
  // Postgres tsvector + GIN for ranked matching.
  async search(query: string): Promise<void> {
    await this.ctx.run(async () => {
      const list = await this.repo.listAll({ query });
      this._terms.set(list);
    });
  }

  // filterByCategory narrows by category; backend CHECK enforces
  // allowed set but we mirror the gate client-side so the list pane
  // toggles without a round-trip.
  async filterByCategory(category: string): Promise<void> {
    await this.ctx.run(async () => {
      const list = await this.repo.listAll({ category });
      this._terms.set(list);
    });
  }

  // add validates, persists, and reloads the list. On validation
  // failure the repo is never called.
  async add(input: TermInput): Promise<Term | null> {
    const detail = validateInput(input);
    if (detail) {
      this._error.set(detail);
      return null;
    }
    let created: Term | null = null;
    await this.ctx.run(async () => {
      created = await this.repo.add(input);
      const next = [...this._terms(), created!];
      this._terms.set(next);
      // Select the freshly-created term so the detail page can open it
      // directly after the modal closes.
      this._selectedTerm.set(created);
      return created;
    });
    return created;
  }

  // update validates + replaces a term in place, then reloads the
  // list so alphabetical order + last-updated are reflected.
  async update(id: string, input: TermInput): Promise<Term | null> {
    const detail = validateInput(input);
    if (detail) {
      this._error.set(detail);
      return null;
    }
    let updated: Term | null = null;
    await this.ctx.run(async () => {
      const result = await this.repo.update(id, input);
      updated = result;
      const next = this._terms().map((t) => (t.id === id ? result : t));
      this._terms.set(next);
      if (this._selectedTerm()?.id === id) {
        this._selectedTerm.set(result);
      }
    });
    return updated;
  }

  // remove deletes a term and reloads the list. If the removed term
  // was selected, deselect it so the detail panel doesn't show a
  // stale row.
  async remove(id: string): Promise<void> {
    await this.ctx.run(async () => {
      await this.repo.remove(id);
      this._terms.set(this._terms().filter((t) => t.id !== id));
      if (this._selectedTerm()?.id === id) {
        this._selectedTerm.set(null);
      }
    });
  }

  // setSelected is the page-side hook for selecting a row from the
  // list. Detail view panel reacts to the signal.
  setSelected(term: Term | null): void {
    this._selectedTerm.set(term);
  }

  // clearError is exposed for the page so the user can dismiss the
  // error banner without reloading.
  clearError(): void {
    this._error.set(null);
  }

  // categorizeByFirstLetter is a helper for the A-Z grid navigation.
  // It groups terms by lowercased first character (non-A-Z → '#').
  categorized(): { letter: string; terms: Term[] }[] {
    const buckets = new Map<string, Term[]>();
    for (const term of this._terms()) {
      const first = (term.term[0] ?? '').toLowerCase();
      const letter = /[a-z]/.test(first) ? first.toUpperCase() : '#';
      if (!buckets.has(letter)) buckets.set(letter, []);
      buckets.get(letter)!.push(term);
    }
    const out: { letter: string; terms: Term[] }[] = [];
    for (const letter of ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')]) {
      const t = buckets.get(letter);
      if (t && t.length > 0) out.push({ letter, terms: t });
    }
    return out;
  }

  // applyFilters accepts a filters object (UI state) and reloads
  // the list. Centralizes the "what we're showing" decision so the
  // page component can drive it from a single event.
  async applyFilters(filters: ListFilters): Promise<void> {
    await this.ctx.run(async () => {
      const list = await this.repo.listAll(filters);
      this._terms.set(list);
    });
  }
}
