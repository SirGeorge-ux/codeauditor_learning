-- Migration 010: code_health_* (Code Health Dashboard — S4)
--
-- Adds the three tables that back the S4 code-health dashboard:
--
--   1. code_health_analyses — one row per (user, repo, branch) analysis
--      run; carries the async-job status, the final health_score, the
--      top_issues JSONB (max 10), the per-language byte-share, the
--      human summary, and bookkeeping timestamps.
--   2. code_health_issues   — one row per detected issue; FK-cascades
--      to analyses so a delete cleans up.
--   3. analysis_cache       — per-file result cache keyed
--      (user_id, file_sha) with a 7-day TTL enforced on read via
--      `expires_at > NOW()`. Spec REQ-CODE-HEALTH-009.
--
-- Every table is owned by exactly one user (public.usuarios.id,
-- ON DELETE CASCADE matches the dictionary spec) and isolated via
-- Row Level Security. The application's database/sql pool already
-- scopes user_id in every WHERE clause; the RLS policies are
-- defense-in-depth.
--
-- ALSO adds the nullable `source_issue_id` column to public.challenges
-- so the S4 code-health flow can persist `SourceIssueID` on the
-- resulting Challenge row (challenges spec REQ-CHALLENGES-CODE-HEALTH-001).
-- This is the resolved open question from the spec stage — the column
-- is added in this migration (010_) rather than a follow-up.
--
-- See: openspec/changes/code-health-dashboard/proposal.md and
-- design.md (Database schema section).

-- ---------------------------------------------------------------------------
-- 1. code_health_analyses
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.code_health_analyses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    source        TEXT NOT NULL CHECK (source IN ('gogs','github')),
    owner         TEXT NOT NULL,
    name          TEXT NOT NULL,
    branch        TEXT NOT NULL DEFAULT 'master',
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','running','completed','failed')),
    health_score  INTEGER,
    top_issues    JSONB NOT NULL DEFAULT '[]'::jsonb,
    languages     JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary       TEXT,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    -- 24h job TTL (REQ-CODE-HEALTH-003): a pending/running row past
    -- expires_at with no completed_at is reported as `failed` by
    -- get_analysis ("job expired").
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);

-- History index: dashboard "last 10 per repo" lookup (REQ-CODE-HEALTH-012).
CREATE INDEX IF NOT EXISTS idx_code_health_analyses_history
    ON public.code_health_analyses (user_id, owner, name, source, created_at DESC);

-- Daily-cap index: CountSince(userID, midnightUTC) for REQ-CODE-HEALTH-011.
CREATE INDEX IF NOT EXISTS idx_code_health_analyses_daily_cap
    ON public.code_health_analyses (user_id, created_at DESC);

-- Lookup by id (the primary key covers it) — but explicit is friendlier.
CREATE INDEX IF NOT EXISTS idx_code_health_analyses_id_user
    ON public.code_health_analyses (id, user_id);

-- ---------------------------------------------------------------------------
-- 2. code_health_issues
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.code_health_issues (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id   UUID NOT NULL REFERENCES public.code_health_analyses(id) ON DELETE CASCADE,
    severity      TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low')),
    category      TEXT NOT NULL CHECK (category IN ('security','performance','refactor','style','architecture')),
    file_path     TEXT NOT NULL,
    line          INTEGER,
    message       TEXT NOT NULL,
    evidence      TEXT,
    suggested_fix TEXT,
    rule          TEXT NOT NULL,
    confidence    REAL NOT NULL DEFAULT 1.0,
    sha           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_code_health_issues_analysis
    ON public.code_health_issues (analysis_id);

-- Severity filter (mcp-code-health.get_repo_issues severity=).
CREATE INDEX IF NOT EXISTS idx_code_health_issues_severity
    ON public.code_health_issues (analysis_id, severity);

-- ---------------------------------------------------------------------------
-- 3. analysis_cache
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS public.analysis_cache (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    file_sha   TEXT NOT NULL,
    result     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- 7-day TTL per spec REQ-CODE-HEALTH-009. The adapter reads
    -- `WHERE expires_at > NOW()` so expired rows are silently
    -- skipped; a periodic vacuum reclaims the disk.
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    CONSTRAINT analysis_cache_user_sha_unique UNIQUE (user_id, file_sha)
);

CREATE INDEX IF NOT EXISTS idx_analysis_cache_expires_at
    ON public.analysis_cache (expires_at);

-- ---------------------------------------------------------------------------
-- 4. source_issue_id on challenges (challenges delta)
-- ---------------------------------------------------------------------------

ALTER TABLE public.challenges
    ADD COLUMN IF NOT EXISTS source_issue_id TEXT;

-- ---------------------------------------------------------------------------
-- Row Level Security — mirrors migration 009_'s pattern
-- ---------------------------------------------------------------------------

ALTER TABLE public.code_health_analyses ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.code_health_issues   ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.analysis_cache       ENABLE ROW LEVEL SECURITY;

-- code_health_analyses: user_id = auth.uid() on every operation.
CREATE POLICY "Users can view own code health analyses"
    ON public.code_health_analyses
    FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own code health analyses"
    ON public.code_health_analyses
    FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update own code health analyses"
    ON public.code_health_analyses
    FOR UPDATE
    USING (auth.uid() = user_id)
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can delete own code health analyses"
    ON public.code_health_analyses
    FOR DELETE
    USING (auth.uid() = user_id);

-- code_health_issues: the owning user is the owner of the parent
-- analysis. The application scopes via JOIN to code_health_analyses;
-- RLS does the same via the subquery (Postgres RLS is per-row, the
-- subquery runs under the same RLS context — circular reference is
-- not a problem here because code_health_analyses' SELECT policy
-- already enforces user_id = auth.uid()).
CREATE POLICY "Users can view own code health issues"
    ON public.code_health_issues
    FOR SELECT
    USING (EXISTS (
        SELECT 1 FROM public.code_health_analyses a
        WHERE a.id = code_health_issues.analysis_id
          AND a.user_id = auth.uid()
    ));

CREATE POLICY "Users can insert own code health issues"
    ON public.code_health_issues
    FOR INSERT
    WITH CHECK (EXISTS (
        SELECT 1 FROM public.code_health_analyses a
        WHERE a.id = code_health_issues.analysis_id
          AND a.user_id = auth.uid()
    ));

CREATE POLICY "Users can update own code health issues"
    ON public.code_health_issues
    FOR UPDATE
    USING (EXISTS (
        SELECT 1 FROM public.code_health_analyses a
        WHERE a.id = code_health_issues.analysis_id
          AND a.user_id = auth.uid()
    ))
    WITH CHECK (EXISTS (
        SELECT 1 FROM public.code_health_analyses a
        WHERE a.id = code_health_issues.analysis_id
          AND a.user_id = auth.uid()
    ));

CREATE POLICY "Users can delete own code health issues"
    ON public.code_health_issues
    FOR DELETE
    USING (EXISTS (
        SELECT 1 FROM public.code_health_analyses a
        WHERE a.id = code_health_issues.analysis_id
          AND a.user_id = auth.uid()
    ));

-- analysis_cache: user_id = auth.uid() on every operation.
CREATE POLICY "Users can view own analysis cache"
    ON public.analysis_cache
    FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own analysis cache"
    ON public.analysis_cache
    FOR INSERT
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can update own analysis cache"
    ON public.analysis_cache
    FOR UPDATE
    USING (auth.uid() = user_id)
    WITH CHECK (auth.uid() = user_id);

CREATE POLICY "Users can delete own analysis cache"
    ON public.analysis_cache
    FOR DELETE
    USING (auth.uid() = user_id);

-- ---------------------------------------------------------------------------
-- DOWN migration (manual, not applied by openspec)
-- ---------------------------------------------------------------------------
--
-- The down migration drops in dependency order:
--
--   DROP TABLE IF EXISTS public.analysis_cache;
--   DROP TABLE IF EXISTS public.code_health_issues;
--   DROP TABLE IF EXISTS public.code_health_analyses;
--   ALTER TABLE public.challenges DROP COLUMN IF EXISTS source_issue_id;
--
-- Down is documentation-only here; the project's migration runner
-- is forward-only. To roll back in production, apply the above
-- statements in a `db/migrations/010_create_code_health.down.sql`
-- alongside the rollback runbook.
