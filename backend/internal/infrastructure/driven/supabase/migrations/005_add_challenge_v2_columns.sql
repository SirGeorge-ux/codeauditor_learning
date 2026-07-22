-- Migration: Add v2 JSONB + scalar columns to challenges table
--
-- Adds structured fields for the challenge rebuild (S1): learning objectives,
-- progressive hints, expected findings, test cases, linter rules, solution code,
-- scoring parameters, and origin tracking.
--
-- All new columns are nullable or have safe defaults so existing rows do not break.
-- The deprecated v1 columns (repo_url, code_smell) are relaxed to nullable in
-- preparation for their eventual removal in a later migration. They are NOT dropped
-- here so the existing ChallengeService SQL queries continue to work.

-- Relax v1 NOT NULL constraints (prep for eventual removal)
ALTER TABLE public.challenges ALTER COLUMN repo_url DROP NOT NULL;
ALTER TABLE public.challenges ALTER COLUMN code_smell DROP NOT NULL;

-- v2 JSONB columns (document-style arrays stored as JSONB)
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS learning_objectives    JSONB DEFAULT '[]'::jsonb;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS common_mistakes         JSONB DEFAULT '[]'::jsonb;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS hints                  JSONB DEFAULT '[]'::jsonb;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS expected_findings      JSONB DEFAULT '[]'::jsonb;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS test_cases             JSONB DEFAULT '[]'::jsonb;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS linter_rules           JSONB DEFAULT '[]'::jsonb;

-- v2 text columns
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS solution_code         TEXT DEFAULT '';
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS solution_explanation   TEXT DEFAULT '';

-- v2 scalar scoring columns
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS base_points           INT     DEFAULT 100;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS bonus_points          INT     DEFAULT 50;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS penalty_per_hint      INT     DEFAULT 0;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS time_bonus            BOOLEAN DEFAULT false;
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS estimated_time_minutes INT     DEFAULT 15;

-- v2 origin tracking
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS origin                 VARCHAR(50) DEFAULT 'curated';
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS source_path            VARCHAR(255);
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS generated_by          VARCHAR(50);
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS created_by            VARCHAR(100);