// HttpDictionaryRepository — infrastructure adapter that fetches personal
// tech-dictionary data from the backend API.
//
// Uses native fetch with a Bearer Authorization header (consistent with
// HttpChallengeRepository / HttpProgressRepository). Implements the
// DictionaryRepository port.
//
// API endpoints consumed:
//   GET    /api/v1/dictionary[?q=&category=&tag=] → array of Term
//   GET    /api/v1/dictionary/{id}                → single Term (404 → null)
//   POST   /api/v1/dictionary                     → created Term
//   PUT    /api/v1/dictionary/{id}                → updated Term
//   DELETE /api/v1/dictionary/{id}                → 204 No Content
import { Term, TermInput } from '../../domain/models/term';
import { DictionaryRepository, ListFilters } from '../../domain/ports/dictionary-repository.port';

export interface TokenProvider {
  getToken(): string | null;
}

// camelCase / optional fields / Date string parsing — maps the wire shape
// (Go camelCase JSON tags) onto the TS Term interface the rest of the app
// uses. Defensive defaults ensure a missing optional field never breaks a
// caller that expects `array` over `null`.
function mapTerm(raw: Record<string, unknown>): Term {
  const created = raw['createdAt'];
  const updated = raw['updatedAt'];
  const example = raw['example'];
  const synonyms = Array.isArray(raw['synonyms']) ? (raw['synonyms'] as string[]) : [];
  const tags = Array.isArray(raw['tags']) ? (raw['tags'] as string[]) : [];
  return {
    id: String(raw['id'] ?? ''),
    term: String(raw['term'] ?? ''),
    explanation: String(raw['explanation'] ?? ''),
    category: (raw['category'] as Term['category']) ?? 'Otro',
    example: typeof example === 'string' ? example : undefined,
    synonyms,
    tags,
    createdAt: typeof created === 'string' ? new Date(created) : new Date(),
    updatedAt: typeof updated === 'string' ? new Date(updated) : new Date(),
  };
}

export class HttpDictionaryRepository implements DictionaryRepository {
  private readonly baseUrl: string;
  private readonly tokenProvider: TokenProvider;

  constructor(tokenProvider: TokenProvider, baseUrl: string = '/api/v1') {
    this.baseUrl = baseUrl;
    this.tokenProvider = tokenProvider;
  }

  private authHeaders(): Record<string, string> {
    const token = this.tokenProvider.getToken();
    return token ? { Authorization: `Bearer ${token}` } : {};
  }

  private async throwIfNotOk(resp: Response, action: string): Promise<void> {
    if (resp.ok) return;
    if (resp.status === 401) {
      throw new Error(`${action} failed: 401 Unauthorized — missing or invalid token`);
    }
    let detail = resp.statusText || 'error';
    try {
      const data = await resp.json();
      if (typeof data?.error === 'string') {
        detail = data.error;
      }
    } catch {
      // ignore — fall back to status text
    }
    // Include status code in the message so callers and tests can branch on
    // it without inspecting Response internals. The body error message has
    // first-class status context here (`{action} failed (404): Term not found`).
    throw new Error(`${action} failed (${resp.status}): ${detail}`);
  }

  async listAll(filters: ListFilters = {}): Promise<Term[]> {
    const params = new URLSearchParams();
    if (filters.query) params.set('q', filters.query);
    if (filters.category) params.set('category', filters.category);
    if (filters.tag) params.set('tag', filters.tag);

    const qs = params.toString();
    const url = qs ? `${this.baseUrl}/dictionary?${qs}` : `${this.baseUrl}/dictionary`;

    const resp = await fetch(url, { headers: this.authHeaders() });
    await this.throwIfNotOk(resp, 'list dictionary terms');

    const data = await resp.json();
    if (!Array.isArray(data)) {
      // Defensive: backend contract is `[] not null`, but never trust it.
      return [];
    }
    return data.map((raw: Record<string, unknown>) => mapTerm(raw));
  }

  async get(id: string): Promise<Term | null> {
    const resp = await fetch(`${this.baseUrl}/dictionary/${encodeURIComponent(id)}`, {
      headers: this.authHeaders(),
    });
    if (resp.status === 404) {
      return null;
    }
    await this.throwIfNotOk(resp, 'get dictionary term');

    const raw = await resp.json();
    return mapTerm(raw);
  }

  async add(input: TermInput): Promise<Term> {
    const body = JSON.stringify({
      term: input.term,
      explanation: input.explanation,
      category: input.category,
      ...(input.example ? { example: input.example } : {}),
      synonyms: input.synonyms ?? [],
      tags: input.tags ?? [],
    });

    const resp = await fetch(`${this.baseUrl}/dictionary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...this.authHeaders() },
      body,
    });
    await this.throwIfNotOk(resp, 'add dictionary term');
    const raw = await resp.json();
    return mapTerm(raw);
  }

  async update(id: string, input: TermInput): Promise<Term> {
    const body = JSON.stringify({
      term: input.term,
      explanation: input.explanation,
      category: input.category,
      ...(input.example ? { example: input.example } : {}),
      synonyms: input.synonyms ?? [],
      tags: input.tags ?? [],
    });

    const resp = await fetch(`${this.baseUrl}/dictionary/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...this.authHeaders() },
      body,
    });
    await this.throwIfNotOk(resp, 'update dictionary term');
    const raw = await resp.json();
    return mapTerm(raw);
  }

  async remove(id: string): Promise<void> {
    const resp = await fetch(`${this.baseUrl}/dictionary/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: this.authHeaders(),
    });
    await this.throwIfNotOk(resp, 'remove dictionary term');
  }
}
