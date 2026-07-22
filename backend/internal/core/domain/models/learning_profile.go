package models

import "time"

// Preferencias holds the user-facing learning preferences managed by the
// socratic tutor chat. Stored as a JSONB column on the DB side.
type Preferencias struct {
	Idioma             string   `json:"idioma"`
	NivelSocratismo    int      `json:"nivelSocratismo"`
	LongitudMaximaMsg  int      `json:"longitudMaximaMsg"`
	BloqueosTipicos    []string `json:"bloqueosTipicos,omitempty"`
	TopicsQueLeCuestan []string `json:"topicsQueLeCuestan,omitempty"`
	Resumen            string   `json:"resumen,omitempty"`
}

// EstiloAprendizaje captures how the user learns best. Stored as a JSONB
// column on the DB side.
type EstiloAprendizaje struct {
	AprendeMejorCon string `json:"aprendeMejorCon,omitempty"`
}

// LearningProfile is a per-user persistent learning profile consumed by the
// socratic tutor. Upserted with defaults on first read.
type LearningProfile struct {
	UserID              string            `json:"userId"`
	Preferencias        Preferencias      `json:"preferencias"`
	EstiloAprendizaje   EstiloAprendizaje `json:"estiloAprendizaje"`
	UltimaActualizacion time.Time         `json:"ultimaActualizacion"`
}

// DefaultLearningProfile returns a freshly-initialized profile with the
// spec-mandated defaults (idioma=es, nivel_socratismo=2, etc.).
func DefaultLearningProfile(userID string) *LearningProfile {
	return &LearningProfile{
		UserID: userID,
		Preferencias: Preferencias{
			Idioma:             "es",
			NivelSocratismo:    2,
			LongitudMaximaMsg:  200,
			BloqueosTipicos:    []string{},
			TopicsQueLeCuestan: []string{},
			Resumen:            "",
		},
		EstiloAprendizaje: EstiloAprendizaje{
			AprendeMejorCon: "ejemplos",
		},
	}
}
