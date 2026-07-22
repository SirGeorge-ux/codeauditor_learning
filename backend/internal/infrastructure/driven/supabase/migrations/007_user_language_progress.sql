-- Migration 007: user_language_progress
-- Tracks per-language mastery with dual range systems (F-S for free practice,
-- Junior-Architect for code health), anti-gaming dedup via completed_challenge_ids JSONB.

CREATE TABLE IF NOT EXISTS public.user_language_progress (
    user_id     UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    language    TEXT NOT NULL,

    -- Free practice (F-S)
    rango                  TEXT     NOT NULL DEFAULT 'F',
    puntos                 INTEGER  NOT NULL DEFAULT 0,
    challenges_completados INTEGER  NOT NULL DEFAULT 0,
    challenges_intentados  INTEGER  NOT NULL DEFAULT 0,
    tasa_exito             REAL     NOT NULL DEFAULT 0,
    topics_dominados       JSONB    NOT NULL DEFAULT '[]'::jsonb,

    -- Dedup / anti-gaming (internal; capped at 500 entries via application logic)
    completed_challenge_ids JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Code health (Junior-Architect)
    rango_code_health   TEXT     NOT NULL DEFAULT 'Junior',
    puntos_code_health  INTEGER  NOT NULL DEFAULT 0,
    repos_analizados    INTEGER  NOT NULL DEFAULT 0,
    issues_encontrados  INTEGER  NOT NULL DEFAULT 0,

    -- Common
    ultimo_completado    TIMESTAMPTZ,
    ultima_actualizacion TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, language)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_ulp_user     ON public.user_language_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_ulp_language ON public.user_language_progress(language);

-- Auto-update ultima_actualizacion
CREATE OR REPLACE FUNCTION public.update_ulp_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.ultima_actualizacion = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_ulp_updated_at ON public.user_language_progress;
CREATE TRIGGER update_ulp_updated_at
    BEFORE UPDATE ON public.user_language_progress
    FOR EACH ROW EXECUTE FUNCTION public.update_ulp_updated_at();

-- Row Level Security
ALTER TABLE public.user_language_progress ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own progress" ON public.user_language_progress
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can update own progress" ON public.user_language_progress
    FOR UPDATE USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own progress" ON public.user_language_progress
    FOR INSERT WITH CHECK (auth.uid() = user_id);