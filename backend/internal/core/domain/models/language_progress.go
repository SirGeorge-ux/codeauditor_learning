package models

import "time"

// LanguageProgress tracks a user's per-language mastery.
//
// Two range systems coexist:
//   - Free practice (Rango / Puntos): F, E, D, C, B, A, S
//   - Code health   (RangoCodeHealth / PuntosCodeHealth): Junior, Mid, Senior, Architect
//
// CompletedChallengeIDs is internal (not serialized to API clients) and is used
// by the service layer for anti-gaming deduplication (capped at 500 entries).
type LanguageProgress struct {
	UserID                string     `json:"userId"`
	Language              string     `json:"language"`
	Rango                 string     `json:"rango"`
	Puntos                int        `json:"puntos"`
	ChallengesCompletados int        `json:"challengesCompletados"`
	ChallengesIntentados  int        `json:"challengesIntentados"`
	TasaExito             float64    `json:"tasaExito"`
	TopicsDominados       []string   `json:"topicsDominados"`
	CompletedChallengeIDs []string   `json:"-"` // internal: anti-gaming dedup
	RangoCodeHealth       string     `json:"rangoCodeHealth"`
	PuntosCodeHealth      int        `json:"puntosCodeHealth"`
	ReposAnalizados       int        `json:"reposAnalizados"`
	IssuesEncontrados     int        `json:"issuesEncontrados"`
	UltimoCompletado      *time.Time `json:"ultimoCompletado,omitempty"`
	UltimaActualizacion   time.Time  `json:"ultimaActualizacion"`
}

// RangoFromPuntosFree maps accumulated free-practice puntos to a F-S range.
// Pure function: no side effects.
func RangoFromPuntosFree(puntos int) string {
	switch {
	case puntos < 50:
		return "F"
	case puntos < 150:
		return "E"
	case puntos < 400:
		return "D"
	case puntos < 900:
		return "C"
	case puntos < 2000:
		return "B"
	case puntos < 4000:
		return "A"
	default:
		return "S"
	}
}

// RangoFromPuntosCodeHealth maps accumulated code-health puntos to a
// Junior-Architect range. Pure function: no side effects.
func RangoFromPuntosCodeHealth(puntos int) string {
	switch {
	case puntos < 100:
		return "Junior"
	case puntos < 500:
		return "Mid"
	case puntos < 2000:
		return "Senior"
	default:
		return "Architect"
	}
}
