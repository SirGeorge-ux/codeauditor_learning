package models

import "time"

// Term is a single entry in the user's personal tech dictionary. Mirrors the
// dictionary_terms SQL table (openspec/changes/dictionary/proposal.md).
//
// Hexagonal contract: this type lives in pure Go with stdlib only. JSON tags
// are the wire shape consumed by the Angular frontend and the MCP tutor;
// UserID stays internal (json:"-") so a Term can never leak across users by
// accident — the service layer stamps UserID from the authenticated JWT.
type Term struct {
	ID          string    `json:"id"`
	UserID      string    `json:"-"`
	Term        string    `json:"term"`
	Explanation string    `json:"explanation"`
	Category    string    `json:"category"`
	Example     *string   `json:"example,omitempty"`
	Synonyms    []string  `json:"synonyms"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AllowedTermCategories is the canonical list of dictionary categories
// mandated by the dictionary S2 spec. The service layer rejects writes that
// reference anything outside this list.
var AllowedTermCategories = []string{
	"Concepto",
	"Patrón",
	"Herramienta",
	"Abreviatura",
	"Anti-patrón",
	"Otro",
}

// IsValidTermCategory returns true when category appears in
// AllowedTermCategories. Equality is exact — case, accent, and whitespace
// differences are not normalized; callers must submit the canonical form.
func IsValidTermCategory(category string) bool {
	for _, c := range AllowedTermCategories {
		if c == category {
			return true
		}
	}
	return false
}
