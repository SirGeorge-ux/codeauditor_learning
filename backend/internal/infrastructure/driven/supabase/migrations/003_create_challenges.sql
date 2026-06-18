-- Migration: Create challenges table for code-audit challenges
-- Stores challenge data backed by PostgreSQL instead of frontend mock data

CREATE TABLE IF NOT EXISTS public.challenges (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('junior', 'mid', 'senior', 'architect')),
    category TEXT NOT NULL,
    language TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    code TEXT NOT NULL,
    code_smell TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_challenges_created ON public.challenges(created_at DESC);

-- Enable RLS
ALTER TABLE public.challenges ENABLE ROW LEVEL SECURITY;

-- Authenticated users can view all available challenges
CREATE POLICY "Authenticated users can view challenges" ON public.challenges
    FOR SELECT TO authenticated USING (true);