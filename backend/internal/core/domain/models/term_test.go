package models

import "testing"

// TestTermCategoryConstants verifies the canonical category list mandated by
// the dictionary S2 spec (openspec/changes/dictionary/specs/dictionary/spec.md):
//
//	category: enum | Required; one of:
//	  Concepto, Patrón, Herramienta, Abreviatura, Anti-patrón, Otro
//
// Scenario under test (spec): "Invalid category rejection" + "Valid term creation".
func TestTermCategoryConstants(t *testing.T) {
	want := []string{
		"Concepto",
		"Patrón",
		"Herramienta",
		"Abreviatura",
		"Anti-patrón",
		"Otro",
	}
	if len(AllowedTermCategories) != len(want) {
		t.Fatalf("AllowedTermCategories length = %d, want %d (got %v)",
			len(AllowedTermCategories), len(want), AllowedTermCategories)
	}
	for i, c := range want {
		if AllowedTermCategories[i] != c {
			t.Errorf("AllowedTermCategories[%d] = %q, want %q",
				i, AllowedTermCategories[i], c)
		}
	}
}

// TestIsValidTermCategory_Triangulation pairs a truth-table across every
// allowed category plus several invalid inputs to prove the predicate is
// not a tautology.
func TestIsValidTermCategory_Triangulation(t *testing.T) {
	cases := []struct {
		name     string
		category string
		want     bool
	}{
		// Happy path: every spec-mandated category MUST validate true.
		{"concepto", "Concepto", true},
		{"patron", "Patrón", true},
		{"herramienta", "Herramienta", true},
		{"abreviatura", "Abreviatura", true},
		{"antipatron", "Anti-patrón", true},
		{"otro", "Otro", true},

		// Edge cases: invalid categories MUST be rejected so the service can
		// return a 400 Bad Request.
		{"empty string", "", false},
		{"lowercase", "concepto", false},
		{"mixed case", "Concepto ", false},
		{"unknown label", "Design Pattern", false}, // matches spec scenario
		{"whitespace only", "   ", false},
		{"different language", "Concept", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsValidTermCategory(c.category); got != c.want {
				t.Errorf("IsValidTermCategory(%q) = %v, want %v",
					c.category, got, c.want)
			}
		})
	}
}
