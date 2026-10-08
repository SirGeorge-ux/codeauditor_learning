import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HttpDictionaryRepository } from './http-dictionary.repository';
import { isValidCategory, TermInput } from '../../domain/models/term';

// Mock global fetch — the repo uses native fetch for parity with the
// existing HttpChallengeRepository / HttpProgressRepository patterns.
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const mockTokenProvider = {
  getToken: vi.fn().mockReturnValue('valid-token'),
};

// Mock term as it would appear on the wire (camelCase, no user_id field).
const mockApiTerm = {
  id: 't-1',
  term: 'closure',
  explanation: 'A function capturing its lexical environment.',
  category: 'Concepto',
  example: null,
  synonyms: ['lexical-closure'],
  tags: ['javascript'],
  createdAt: '2026-01-01T12:00:00.000Z',
  updatedAt: '2026-01-01T12:00:00.000Z',
};

describe('HttpDictionaryRepository', () => {
  let repo: HttpDictionaryRepository;

  beforeEach(() => {
    vi.clearAllMocks();
    mockTokenProvider.getToken.mockReturnValue('valid-token');
    repo = new HttpDictionaryRepository(mockTokenProvider, 'http://localhost:8080/api/v1');
  });

  describe('listAll', () => {
    it('should map API terms without a query', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [mockApiTerm],
      });

      const result = await repo.listAll();

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('t-1');
      expect(result[0].term).toBe('closure');
      expect(result[0].category).toBe('Concepto');
      expect(result[0].createdAt).toBeInstanceOf(Date);
      // Bearer auth header present on the request.
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/dictionary',
        expect.objectContaining({
          headers: expect.objectContaining({ Authorization: 'Bearer valid-token' }),
        }),
      );
    });

    it('should set ?q= when query filter is present', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await repo.listAll({ query: 'closure' });

      const calledURL = mockFetch.mock.calls[0][0] as string;
      expect(calledURL).toContain('?q=closure');
    });

    it('should set ?category= when category filter is present', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await repo.listAll({ category: 'Patrón' });

      const calledURL = mockFetch.mock.calls[0][0] as string;
      expect(calledURL).toContain('category=');
      // URL encoding turns ó into %C3%B3
      expect(calledURL.toLowerCase()).toMatch(/patr(%c3%b3n|ón)/);
    });

    it('should return [] when API returns empty array', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      const result = await repo.listAll();
      expect(result).toEqual([]);
    });

    it('should return [] when API returns null (defensive)', async () => {
      // Prevents the frontend from breaking if a backend code path leaks
      // `null` instead of `[]`.
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => null,
      });

      const result = await repo.listAll();
      expect(result).toEqual([]);
    });

    it('should throw on 4xx', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        json: async () => ({ error: 'Missing authorization header' }),
      });

      await expect(repo.listAll()).rejects.toThrow(/401|Unauthorized/);
    });
  });

  describe('get', () => {
    it('should return mapped term on success', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => mockApiTerm,
      });

      const result = await repo.get('t-1');
      expect(result).not.toBeNull();
      expect(result!.id).toBe('t-1');
      expect(result!.createdAt).toBeInstanceOf(Date);
      expect(mockFetch.mock.calls[0][0]).toBe('http://localhost:8080/api/v1/dictionary/t-1');
    });

    it('should resolve null on 404', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'Term not found' }),
      });

      const result = await repo.get('missing');
      expect(result).toBeNull();
    });

    it('should throw on non-404 errors', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: async () => ({ error: 'boom' }),
      });

      await expect(repo.get('t-1')).rejects.toThrow(/500/);
    });
  });

  describe('add', () => {
    it('should POST the term and return mapped created row', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => mockApiTerm,
      });

      const input: TermInput = {
        term: 'closure',
        explanation: 'A function capturing its lexical environment.',
        category: 'Concepto',
        example: undefined,
        synonyms: ['lexical-closure'],
        tags: ['javascript'],
      };

      const result = await repo.add(input);
      expect(result.id).toBe('t-1');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/dictionary',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            term: 'closure',
            explanation: 'A function capturing its lexical environment.',
            category: 'Concepto',
            synonyms: ['lexical-closure'],
            tags: ['javascript'],
          }),
        }),
      );
    });

    it('should include optional example when provided', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => mockApiTerm,
      });

      const input: TermInput = {
        term: 'hardcode',
        explanation: 'Writing literal values in code.',
        category: 'Anti-patrón',
        example: 'if(x===0) y=x;',
        synonyms: [],
        tags: [],
      };

      await repo.add(input);
      const body = JSON.parse((mockFetch.mock.calls[0][1] as RequestInit).body as string);
      expect(body.example).toBe('if(x===0) y=x;');
    });

    it('should throw on 400 with validation detail', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
        json: async () => ({ error: 'explanation must be >= 10 chars' }),
      });

      const input: TermInput = {
        term: 'x',
        explanation: 'too short',
        category: 'Concepto',
        synonyms: [],
        tags: [],
      };

      await expect(repo.add(input)).rejects.toThrow(/explanation must be >= 10 chars/);
    });

    it('should throw on 409 duplicate', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 409,
        statusText: 'Conflict',
        json: async () => ({ error: 'term already exists for user' }),
      });

      const input: TermInput = {
        term: 'closure',
        explanation: 'A function capturing its lexical environment.',
        category: 'Concepto',
        synonyms: [],
        tags: [],
      };

      await expect(repo.add(input)).rejects.toThrow(/already|duplicate|conflict/);
    });
  });

  describe('update', () => {
    it('should PUT and return updated term', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          ...mockApiTerm,
          explanation: 'Refined explanation with enough length now.',
          category: 'Anti-patrón',
        }),
      });

      const input: TermInput = {
        term: 'hardcode',
        explanation: 'Refined explanation with enough length now.',
        category: 'Anti-patrón',
        synonyms: [],
        tags: [],
      };

      const result = await repo.update('t-1', input);
      expect(result.category).toBe('Anti-patrón');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/dictionary/t-1',
        expect.objectContaining({ method: 'PUT' }),
      );
    });

    it('should throw on 404', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'Term not found' }),
      });

      const input: TermInput = {
        term: 'x',
        explanation: 'A function capturing environment.',
        category: 'Concepto',
        synonyms: [],
        tags: [],
      };

      await expect(repo.update('missing', input)).rejects.toThrow(/404|Not [Ff]ound/);
    });
  });

  describe('remove', () => {
    it('should DELETE and resolve on 204', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        statusText: 'No Content',
        json: async () => '',
      });

      await expect(repo.remove('t-1')).resolves.toBeUndefined();
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/dictionary/t-1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('should throw on 404 cross-user delete', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'Term not found' }),
      });

      await expect(repo.remove('other-id')).rejects.toThrow(/404|Not [Ff]ound/);
    });
  });

  describe('isValidCategory (sanity)', () => {
    it('accepts canonical names', () => {
      expect(isValidCategory('Concepto')).toBe(true);
      expect(isValidCategory('Anti-patrón')).toBe(true);
    });
    it('rejects unknown', () => {
      expect(isValidCategory('Design Pattern')).toBe(false);
      expect(isValidCategory('')).toBe(false);
    });
  });
});
