// DictionaryRepository — port interface for the user's personal tech
// dictionary. Implemented by HttpDictionaryRepository. Zero framework
// imports — pure TypeScript contract.
import { Term, TermInput } from '../models/term';

export interface ListFilters {
  query?: string;
  category?: string;
  tag?: string;
}

export interface DictionaryRepository {
  // listAll returns every term the authenticated user owns. Optional
  // filters narrow the result set; empty filters return the entire
  // glossary.
  listAll(filters?: ListFilters): Promise<Term[]>;

  // get returns one term by id; resolves null when the row is missing.
  get(id: string): Promise<Term | null>;

  // add persists a new term; throws on validation conflicts so the
  // calling use case can surface a useful message.
  add(input: TermInput): Promise<Term>;

  // update replaces an existing term; throws on missing id or conflicts.
  update(id: string, input: TermInput): Promise<Term>;

  // remove deletes a term; throws on missing id.
  remove(id: string): Promise<void>;
}
