-- Migration 009: dictionary_terms (Personal Tech Dictionary — S2)
--
-- Stores the user's private tech glossary. Each row is owned by exactly one
-- user (public.usuarios.id) and isolated via Row Level Security. A
-- generated search_vector column (term | explanation | synonyms | tags)
-- powers ranked Postgres full-text search via a GIN index.
--
-- See: openspec/changes/dictionary/proposal.md and design.md.

CREATE TABLE IF NOT EXISTS public.dictionary_terms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Owner isolation (FK to public.usuarios, NOT to auth.users directly).
    user_id     UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,

    -- The slug itself. Stored as the canonical lowercase form so the unique
    -- index below dedupes "PR" against "pr" instead of allowing both.
    term        TEXT NOT NULL CHECK (char_length(term) BETWEEN 1 AND 50),

    -- Required body: long enough to be useful (>= 10), capped to keep the
    -- list page scannable (<= 500).
    explanation TEXT NOT NULL CHECK (char_length(explanation) BETWEEN 10 AND 500),

    -- Predefined category (validated in the application layer against the
    -- canonical AllowedTermCategories list. The CHECK keeps the DB in sync
    -- even if a future caller bypasses `services.IsValidTermCategory`.
    -- Allowed values match `models.AllowedTermCategories`:
    --   Concepto, Patrón, Herramienta, Abreviatura, Anti-patrón, Otro.
    category    TEXT NOT NULL CHECK (
        category IN (
            'Concepto',
            'Patrón',
            'Herramienta',
            'Abreviatura',
            'Anti-patrón',
            'Otro'
        )
    ),

    -- Optional illustrative example, larger ceiling so code samples fit.
    example     TEXT CHECK (example IS NULL OR char_length(example) <= 1000),

    -- Optional synonym / tag arrays. Application layer caps each entry's
    -- length (50 for synonyms, 30 for tags).
    synonyms    TEXT[] NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',

    -- Generated tsvector: lower(term) || ' ' || lower(explanation) ||
    --                       unnest synonyms  || ' ' || unnest tags
    -- The IMMUTABLE function `to_tsvector('english', ...)` runs at
    -- row-write time so SELECT can simply USE the column with a GIN index.
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(term, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(explanation, '')), 'B') ||
        setweight(
            to_tsvector('english',
                array_to_string(synonyms, ' ') || ' ' ||
                array_to_string(tags, ' ')),
            'C'
        )
    ) STORED,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Per-user dedupe so the same user cannot store two "hardcode" rows.
    CONSTRAINT dictionary_terms_user_term_unique UNIQUE (user_id, term)
);

-- GIN index serves ranked FTS in O(log) lexeme lookups (spec target: <200ms
-- for 1K terms). GIN, not btree, because the indexed column is a tsvector.
CREATE INDEX IF NOT EXISTS idx_dictionary_terms_search_vector
    ON public.dictionary_terms USING GIN (search_vector);

-- Lookup index for the by-user browse path (ListTerms / ExistsByTerm).
CREATE INDEX IF NOT EXISTS idx_dictionary_terms_user_term
    ON public.dictionary_terms (user_id, term);

-- Auto-update updated_at on any row mutation via the existing
-- update_updated_at_column() helper from migration 001.
DROP TRIGGER IF EXISTS update_dictionary_terms_updated_at ON public.dictionary_terms;
CREATE TRIGGER update_dictionary_terms_updated_at
    BEFORE UPDATE ON public.dictionary_terms
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();

-- Row Level Security: every policy matches the JWT subject (auth.uid()) to
-- user_id. This matches the spec's "Row-Level Security" requirement and is
-- the strongest defense against accidental cross-user reads.
ALTER TABLE public.dictionary_terms ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own dictionary terms"
    ON public.dictionary_terms
    FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own dictionary terms"
    ON public.dictionary_terms
    FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update own dictionary terms"
    ON public.dictionary_terms
    FOR UPDATE
    USING (auth.uid() = user_id)
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can delete own dictionary terms"
    ON public.dictionary_terms
    FOR DELETE
    USING (auth.uid() = user_id);
