-- Migration: Add user ownership and source tracking to challenges
-- Enables persistent imports from Gogs/GitHub with deduplication

ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES public.usuarios(id);
ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS source_repo TEXT;

CREATE INDEX IF NOT EXISTS idx_challenges_source_repo_user ON public.challenges(source_repo, user_id);

-- RLS INSERT policy (defense-in-depth: authenticated users can only insert their own challenges)
CREATE POLICY "Users can insert own challenges" ON public.challenges
    FOR INSERT WITH CHECK (auth.uid() = user_id);