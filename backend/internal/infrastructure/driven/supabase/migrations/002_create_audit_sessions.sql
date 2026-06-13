-- Migration: Create audit_sessions table for vault/history
-- Tracks completed audit sessions per user

CREATE TABLE IF NOT EXISTS public.audit_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    challenge_title TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    code_snippet TEXT NOT NULL DEFAULT '',
    findings_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_sessions_user_id ON public.audit_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_sessions_created ON public.audit_sessions(created_at DESC);

-- Enable RLS
ALTER TABLE public.audit_sessions ENABLE ROW LEVEL SECURITY;

-- Users can view their own sessions
CREATE POLICY "Users can view own sessions" ON public.audit_sessions
    FOR SELECT USING (auth.uid() = user_id);

-- Users can insert their own sessions
CREATE POLICY "Users can insert own sessions" ON public.audit_sessions
    FOR INSERT WITH CHECK (auth.uid() = user_id);
