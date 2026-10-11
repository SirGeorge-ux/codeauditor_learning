-- Migration 010b: code_health_suppressions (S4, PR 1 — task 1.18)
--
-- Adds the `code_health_suppressions` table that backs the
-- "Not an issue" user feedback flow (spec REQ-CODE-HEALTH-008
-- "not an issue" scenario). The composite primary key
-- (user_id, file_path, rule_id, line_number) makes the insert
-- idempotent — the same suppression can be re-applied without
-- a unique-violation error, and the analyzer's
-- IsSuppressed(...) lookup is a single PK fetch.
--
-- Rationale: persistence-required was resolved in the
-- `ask-on-risk` user gate (2026-10-11). Client-side only
-- would lose suppressions across re-analyses and across
-- devices; a server-side table keeps the data durable and
-- scoped to the user.
--
-- See: openspec/changes/code-health-dashboard/tasks.md task
-- 1.18 + the related tasks 1.19 (port + repository extension)
-- and 1.20 (analyzer skip).

CREATE TABLE IF NOT EXISTS public.code_health_suppressions (
    user_id     UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    file_path   TEXT NOT NULL,
    rule_id     TEXT NOT NULL,
    line_number INTEGER NOT NULL,
    -- Optional human reason. The spec doesn't require a value
    -- (the user might just want to dismiss the issue without a
    -- narrative); the field is nullable.
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Composite primary key drives the dedupe contract: the
    -- same (user, file, rule, line) can be re-inserted as a
    -- no-op (Postgres 23505 -> adapter ignores the violation).
    PRIMARY KEY (user_id, file_path, rule_id, line_number)
);

-- Lookup index for the analyzer's per-file IsSuppressed sweep
-- (the analyzer's filter pass checks every (file, rule, line)
-- tuple before emitting an issue).
CREATE INDEX IF NOT EXISTS idx_code_health_suppressions_user_file
    ON public.code_health_suppressions (user_id, file_path);

-- Row Level Security: the user_id PK column is the source of
-- truth; every operation is scoped to the JWT subject.
ALTER TABLE public.code_health_suppressions ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own code health suppressions"
    ON public.code_health_suppressions
    FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own code health suppressions"
    ON public.code_health_suppressions
    FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can delete own code health suppressions"
    ON public.code_health_suppressions
    FOR DELETE
    USING (auth.uid() = user_id);

-- ---------------------------------------------------------------------------
-- DOWN migration (manual, not applied by openspec)
-- ---------------------------------------------------------------------------
--
-- DROP TABLE IF EXISTS public.code_health_suppressions;
--
-- The table has no dependents beyond the FK to public.usuarios
-- (which is ON DELETE CASCADE), so the down is a single DROP.
