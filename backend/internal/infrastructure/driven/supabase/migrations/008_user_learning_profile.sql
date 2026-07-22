-- Migration 008: user_learning_profile
-- Stores per-user learning preferences consumed by the socratic tutor chat.

CREATE TABLE IF NOT EXISTS public.user_learning_profile (
    user_id   UUID PRIMARY KEY REFERENCES public.usuarios(id) ON DELETE CASCADE,

    -- Preferencias (JSONB — flexible preferences container)
    preferencias        JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Resumen del estilo de aprendizaje (JSONB — keeps nested fields together)
    estilo_aprendizaje  JSONB NOT NULL DEFAULT '{}'::jsonb,

    ultima_actualizacion TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Auto-update ultima_actualizacion
CREATE OR REPLACE FUNCTION public.update_ulp_profile_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.ultima_actualizacion = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_ulp_profile_updated_at ON public.user_learning_profile;
CREATE TRIGGER update_ulp_profile_updated_at
    BEFORE UPDATE ON public.user_learning_profile
    FOR EACH ROW EXECUTE FUNCTION public.update_ulp_profile_updated_at();

-- Row Level Security
ALTER TABLE public.user_learning_profile ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own learning profile" ON public.user_learning_profile
    FOR SELECT USING (auth.uid() = user_id);

CREATE POLICY "Users can update own learning profile" ON public.user_learning_profile
    FOR UPDATE USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own learning profile" ON public.user_learning_profile
    FOR INSERT WITH CHECK (auth.uid() = user_id);