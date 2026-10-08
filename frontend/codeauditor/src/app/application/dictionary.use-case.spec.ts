import { describe, it, expect, vi, beforeEach } from 'vitest';
import { DictionaryUseCase } from './dictionary.use-case';
import { DictionaryRepository, ListFilters } from '../domain/ports/dictionary-repository.port';
import { Term, TermCategory, TermInput } from '../domain/models/term';

// --- Test fake ----------------------------------------------------------

class FakeDictionaryRepository implements DictionaryRepository {
  // in-memory state so we can simulate CRUD end-to-end at the use case
  // level.
  terms: Term[] = [];

  async listAll(filters?: ListFilters): Promise<Term[]> {
    let rows = this.terms.slice();
    if (filters?.query) {
      const q = filters.query.toLowerCase();
      rows = rows.filter((t) => `${t.term} ${t.explanation}`.toLowerCase().includes(q));
    }
    if (filters?.category) {
      rows = rows.filter((t) => t.category === filters.category);
    }
    if (filters?.tag) {
      const tag = filters.tag;
      rows = rows.filter((t) => t.tags.includes(tag));
    }
    return rows;
  }

  async get(id: string): Promise<Term | null> {
    return this.terms.find((t) => t.id === id) ?? null;
  }

  async add(input: TermInput): Promise<Term> {
    const now = new Date('2026-01-01');
    if (this.terms.some((t) => t.term === input.term)) {
      throw new Error('term already exists for user');
    }
    const created: Term = {
      id: `t-${this.terms.length + 1}`,
      term: input.term,
      explanation: input.explanation,
      category: input.category,
      example: input.example,
      synonyms: input.synonyms ?? [],
      tags: input.tags ?? [],
      createdAt: now,
      updatedAt: now,
    };
    this.terms.push(created);
    return created;
  }

  async update(id: string, input: TermInput): Promise<Term> {
    const idx = this.terms.findIndex((t) => t.id === id);
    if (idx < 0) throw new Error('not found');
    const updated: Term = {
      ...this.terms[idx],
      term: input.term,
      explanation: input.explanation,
      category: input.category,
      example: input.example,
      synonyms: input.synonyms ?? [],
      tags: input.tags ?? [],
      updatedAt: new Date('2026-01-02'),
    };
    this.terms[idx] = updated;
    return updated;
  }

  async remove(id: string): Promise<void> {
    const before = this.terms.length;
    this.terms = this.terms.filter((t) => t.id !== id);
    if (this.terms.length === before) {
      throw new Error('not found');
    }
  }
}

const validInput = (over: Partial<TermInput> = {}): TermInput => ({
  term: 'closure',
  explanation: 'A function capturing its lexical environment.',
  category: 'Concepto' as TermCategory,
  synonyms: [],
  tags: [],
  ...over,
});

describe('DictionaryUseCase', () => {
  let repo: FakeDictionaryRepository;
  let useCase: DictionaryUseCase;

  beforeEach(() => {
    repo = new FakeDictionaryRepository();
    useCase = new DictionaryUseCase(repo);
  });

  describe('loadAll', () => {
    it('populates terms signal with repo rows', async () => {
      repo.terms.push(makeTerm('t-1', 'closure'));
      repo.terms.push(makeTerm('t-2', 'lambda'));

      await useCase.loadAll();

      expect(useCase.terms().length).toBe(2);
      expect(useCase.loading()).toBe(false);
      expect(useCase.error()).toBeNull();
    });

    it('sets error signal on failure', async () => {
      // Override repo to throw.
      useCase = new DictionaryUseCase({
        listAll: () => Promise.reject(new Error('boom')),
        get: () => Promise.resolve(null),
        add: () => Promise.reject(new Error('not used')),
        update: () => Promise.reject(new Error('not used')),
        remove: () => Promise.reject(new Error('not used')),
      });

      await useCase.loadAll();

      expect(useCase.error()).toBe('boom');
      expect(useCase.terms().length).toBe(0);
    });
  });

  describe('search', () => {
    it('updates terms signal with filtered results', async () => {
      repo.terms.push(makeTerm('t-1', 'closure'));
      repo.terms.push(makeTerm('t-2', 'lambda'));

      await useCase.search('closure');

      expect(useCase.terms().length).toBe(1);
      expect(useCase.terms()[0].term).toBe('closure');
    });
  });

  describe('filterByCategory', () => {
    it('updates terms signal with category-matching results', async () => {
      repo.terms.push({ ...makeTerm('t-1', 'closure'), category: 'Concepto' });
      repo.terms.push({ ...makeTerm('t-2', 'factory'), category: 'Patrón' });

      await useCase.filterByCategory('Patrón');

      expect(useCase.terms().length).toBe(1);
      expect(useCase.terms()[0].term).toBe('factory');
    });
  });

  describe('add', () => {
    it('reloads the list after a successful add', async () => {
      await useCase.add(validInput());

      expect(useCase.terms().length).toBe(1);
      expect(useCase.terms()[0].term).toBe('closure');
    });

    it('validates empty term before hitting the repo', async () => {
      const errSpy = vi.spyOn(repo, 'add');

      await useCase.add(validInput({ term: '' }));

      expect(errSpy).not.toHaveBeenCalled();
      expect(useCase.error()).toMatch(/term is required/);
    });

    it('validates explanation min length', async () => {
      const errSpy = vi.spyOn(repo, 'add');

      await useCase.add(validInput({ explanation: 'short' }));

      expect(errSpy).not.toHaveBeenCalled();
      expect(useCase.error()).toMatch(/explanation/i);
    });

    it('surfaces repo errors via useCase.error', async () => {
      // First, seed the repo with one term via the use case so it
      // lands in the in-memory store.
      await useCase.add(validInput());

      // Now try to add again with the same lowercase term — the in-memory
      // repo refuses it with a duplicate message that surfaces in
      // useCase.error().
      await useCase.add(validInput());

      expect(useCase.error()).toMatch(/duplicate|already/i);
    });
  });

  describe('update', () => {
    it('updates an existing term and reloads the list', async () => {
      await useCase.add(validInput());
      const id = useCase.terms()[0].id;

      await useCase.update(
        id,
        validInput({
          explanation: 'Refined explanation with enough length.',
          category: 'Anti-patrón',
        }),
      );

      expect(useCase.terms()[0].category).toBe('Anti-patrón');
      expect(useCase.terms()[0].explanation).toBe('Refined explanation with enough length.');
    });
  });

  describe('remove', () => {
    it('removes the term and reloads the list', async () => {
      await useCase.add(validInput());
      const id = useCase.terms()[0].id;

      await useCase.remove(id);

      expect(useCase.terms().length).toBe(0);
    });

    it('surfaces repo errors via useCase.error', async () => {
      const spy = vi.spyOn(repo, 'remove').mockRejectedValueOnce(new Error('404'));

      await useCase.remove('missing-id');

      expect(spy).toHaveBeenCalledWith('missing-id');
      expect(useCase.error()).toBe('404');
    });
  });

  describe('selectedTerm signal', () => {
    it('setSelected updates the signal', () => {
      const t = makeTerm('t-1', 'closure');
      useCase.setSelected(t);
      expect(useCase.selectedTerm()).toEqual(t);
    });
  });

  describe('client-side validation', () => {
    it('rejects too-long term (>50 chars)', async () => {
      const longTerm = 'a'.repeat(51);
      await useCase.add(validInput({ term: longTerm }));
      expect(useCase.error()).toMatch(/term/i);
    });
    it('rejects too-long explanation (>500 chars)', async () => {
      const long = 'x'.repeat(501);
      await useCase.add(validInput({ explanation: long }));
      expect(useCase.error()).toMatch(/explanation/i);
    });
    it('rejects invalid category', async () => {
      await useCase.add(validInput({ category: 'Design Pattern' as unknown as TermCategory }));
      expect(useCase.error()).toMatch(/categor/i);
    });
  });
});

function makeTerm(id: string, term: string): Term {
  return {
    id,
    term,
    explanation: `${term} definition with enough length.`,
    category: 'Concepto',
    synonyms: [],
    tags: [],
    createdAt: new Date('2026-01-01'),
    updatedAt: new Date('2026-01-01'),
  };
}
