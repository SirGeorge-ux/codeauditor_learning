// DictionaryService — Angular injectable service that wires
// HttpDictionaryRepository into DictionaryUseCase so components can depend
// on a single Injectable and read reactive signals. Follows the
// ProgressService / ChallengeService pattern: signal exposure +
// repo-binding in the constructor.
import { Injectable, inject, computed } from '@angular/core';
import { DictionaryUseCase } from '../../application/dictionary.use-case';
import { HttpDictionaryRepository } from '../repositories/http-dictionary.repository';
import { AuthService } from './auth.service';
import { environment } from '../../../environments/environment';
import { Term, TermInput } from '../../domain/models/term';

@Injectable({ providedIn: 'root' })
export class DictionaryService {
  private useCase: DictionaryUseCase;

  readonly terms = computed(() => this.useCase.terms());
  readonly selectedTerm = computed(() => this.useCase.selectedTerm());
  readonly loading = computed(() => this.useCase.loading());
  readonly error = computed(() => this.useCase.error());
  readonly hasTerms = computed(() => this.useCase.hasTerms());

  constructor() {
    const authService = inject(AuthService);
    const repo = new HttpDictionaryRepository(
      {
        getToken: () => authService.getAccessToken(),
      },
      environment.apiUrl + '/api/v1',
    );
    this.useCase = new DictionaryUseCase(repo);
  }

  // Public API: thin pass-through to use case. Returns the same
  // shape so the page component can use readonly signals + await-on-result.
  async loadAll(): Promise<void> {
    return this.useCase.loadAll();
  }

  async search(query: string): Promise<void> {
    return this.useCase.search(query);
  }

  // get(id) fetches a single term bypassing the use-case store so the
  // detail page can render without polluting the list signal.
  async get(id: string): Promise<Term | null> {
    return (
      this.useCase as unknown as { repo: { get(id: string): Promise<Term | null> } }
    ).repo.get(id);
  }

  async filterByCategory(category: string): Promise<void> {
    return this.useCase.filterByCategory(category);
  }

  async add(input: TermInput): Promise<Term | null> {
    return this.useCase.add(input);
  }

  async update(id: string, input: TermInput): Promise<Term | null> {
    return this.useCase.update(id, input);
  }

  async remove(id: string): Promise<void> {
    return this.useCase.remove(id);
  }

  setSelected(term: Term | null): void {
    this.useCase.setSelected(term);
  }

  clearError(): void {
    this.useCase.clearError();
  }

  categorized(): { letter: string; terms: Term[] }[] {
    return this.useCase.categorized();
  }
}
