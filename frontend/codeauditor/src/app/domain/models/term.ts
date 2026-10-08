// Term — domain entity mirroring backend models.Term for the user's
// personal tech dictionary.
//
// Field names match the backend's JSON-shaped wire (Go camelCase). The
// user_id field is intentionally present in the TS surface so the
// repository can serialize the same payload; the backend omits it on the
// wire (json:"-") and the HTTP repo therefore never surfaces it on the
// client. See HttpDictionaryRepository for the mapping.
export type TermCategory =
  | 'Concepto'
  | 'Patrón'
  | 'Herramienta'
  | 'Abreviatura'
  | 'Anti-patrón'
  | 'Otro';

export const TERM_CATEGORIES: TermCategory[] = [
  'Concepto',
  'Patrón',
  'Herramienta',
  'Abreviatura',
  'Anti-patrón',
  'Otro',
];

export interface Term {
  id: string;
  term: string;
  explanation: string;
  category: TermCategory;
  example?: string;
  synonyms: string[];
  tags: string[];
  createdAt: Date;
  updatedAt: Date;
}

// Input shape for creating / updating terms. The repository serializes
// this into the HTTP body. Field validation happens in the backend
// service; the form component performs parallel client-side gating so
// users see feedback before the round-trip.
export interface TermInput {
  term: string;
  explanation: string;
  category: TermCategory;
  example?: string;
  synonyms: string[];
  tags: string[];
}

// isValidCategory narrows the can-call-it-a-TermCategory check used by the
// use case / form component. Mirrors models.IsValidTermCategory in Go.
export function isValidCategory(value: string): value is TermCategory {
  return (TERM_CATEGORIES as readonly string[]).includes(value);
}
