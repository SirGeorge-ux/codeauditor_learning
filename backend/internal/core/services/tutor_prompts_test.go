package services

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// tpsSystem is a tiny valid prompt body shared by the loader tests.
// Keeping these as constants prevents drift between RED and the
// implementation that relies on the same shape.
const tpsSystem = "# Identity\nTutor persona body.\n"

// tpsSocratic holds the 4 level bodies; index == level. We use
// strings.Join / strings.Split via a slice var instead of a const
// array so the test file is portable across Go versions.
var tpsSocratic = []string{"level 0", "level 1", "level 2", "level 3"}

// newFSYSPrompts builds a synthetic fs.FS that looks like
// .atl/prompts/. Any missing file breaks the loader cleanly with
// a wrapper that still gives ErrTutorPromptsMissing on errors.Is.
func newFSYSPrompts(t *testing.T) fs.FS {
	t.Helper()
	return fstest.MapFS{
		"tutor-system.md":       &fstest.MapFile{Data: []byte(tpsSystem)},
		"tutor-socratic-0.md":   &fstest.MapFile{Data: []byte(tpsSocratic[0])},
		"tutor-socratic-1.md":   &fstest.MapFile{Data: []byte(tpsSocratic[1])},
		"tutor-socratic-2.md":   &fstest.MapFile{Data: []byte(tpsSocratic[2])},
		"tutor-socratic-3.md":   &fstest.MapFile{Data: []byte(tpsSocratic[3])},
	}
}

// TestLoadTutorPrompts_AllFiveFilesHappyPath — el happy path: el FS
// sintético provee los 5 archivos y el loader devuelve la struct
// con todos los campos poblados. La validación de "qué archivo
// falta" vive en el test siguiente.
func TestLoadTutorPrompts_AllFiveFilesHappyPath(t *testing.T) {
	fsys := newFSYSPrompts(t)
	p, err := loadTutorPromptsBase(fsys)
	if err != nil {
		t.Fatalf("loadTutorPromptsBase returned err: %v", err)
	}
	if !strings.Contains(p.System, "Identity") {
		t.Errorf("System stripped contents: %q", p.System)
	}
	for i := 0; i < 4; i++ {
		if p.Socratic[i] != tpsSocratic[i] {
			t.Errorf("Socratic[%d] = %q, want %q", i, p.Socratic[i], tpsSocratic[i])
		}
	}
}

// TestLoadTutorPrompts_MissingSystemFile devuelve ErrTutorPromptsMissing
// cuando el archivo de identidad no está. errors.Is comprueba el
// wrapping — el sentinel debe sobrevivir a través de fmt.Errorf("%w: …").
func TestLoadTutorPrompts_MissingSystemFile(t *testing.T) {
	fsys := fstest.MapFS{
		"tutor-socratic-0.md": &fstest.MapFile{Data: []byte("lvl0")},
		"tutor-socratic-1.md": &fstest.MapFile{Data: []byte("lvl1")},
		"tutor-socratic-2.md": &fstest.MapFile{Data: []byte("lvl2")},
		"tutor-socratic-3.md": &fstest.MapFile{Data: []byte("lvl3")},
	}
	_, err := loadTutorPromptsBase(fsys)
	if err == nil {
		t.Fatalf("expected error for missing tutor-system.md, got nil")
	}
	if !errors.Is(err, ErrTutorPromptsMissing) {
		t.Errorf("err = %v, want ErrTutorPromptsMissing", err)
	}
}

// TestLoadTutorPrompts_MissingSocraticLevelFile — cubre el caso
// "4 archivos cargados pero falta uno de los niveles".
func TestLoadTutorPrompts_MissingSocraticLevelFile(t *testing.T) {
	fsys := fstest.MapFS{
		"tutor-system.md":     &fstest.MapFile{Data: []byte(tpsSystem)},
		"tutor-socratic-0.md": &fstest.MapFile{Data: []byte("lvl0")},
		"tutor-socratic-1.md": &fstest.MapFile{Data: []byte("lvl1")},
		"tutor-socratic-2.md": &fstest.MapFile{Data: []byte("lvl2")},
		// tutor-socratic-3.md is intentionally missing
	}
	_, err := loadTutorPromptsBase(fsys)
	if err == nil || !errors.Is(err, ErrTutorPromptsMissing) {
		t.Fatalf("got %v, want ErrTutorPromptsMissing", err)
	}
}

// TestLoadTutorPrompts_TrimsTrailingNewline valida que el loader
// le hace TrimSpace al contenido para evitar que un archivo
// terminado con \n\n rinda el LLM el "noise" de líneas vacías en
// el primer mensaje del system prompt.
func TestLoadTutorPrompts_TrimsTrailingNewline(t *testing.T) {
	fsys := fstest.MapFS{
		"tutor-system.md":     &fstest.MapFile{Data: []byte(tpsSystem + "\n\n\n")},
		"tutor-socratic-0.md": &fstest.MapFile{Data: []byte("  lvl0 \n\n")},
		"tutor-socratic-1.md": &fstest.MapFile{Data: []byte("lvl1")},
		"tutor-socratic-2.md": &fstest.MapFile{Data: []byte("lvl2")},
		"tutor-socratic-3.md": &fstest.MapFile{Data: []byte("lvl3")},
	}
	p, err := loadTutorPromptsBase(fsys)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.HasSuffix(p.System, "\n") {
		t.Errorf("System has trailing newline: %q", p.System)
	}
	if p.Socratic[0] != "lvl0" {
		t.Errorf("Socratic[0] = %q, want 'lvl0' (TrimSpace)", p.Socratic[0])
	}
}

// TestLoadTutorPromptsFromDir_EmptyDirReturnsErr — el helper basado
// en disk. Un directorio vacío no es una entrada válida y el
// sentinel debe dispararse.
func TestLoadTutorPromptsFromDir_EmptyDirReturnsErr(t *testing.T) {
	_, err := loadTutorPromptsFromDir("")
	if err == nil || !errors.Is(err, ErrTutorPromptsMissing) {
		t.Fatalf("err = %v, want ErrTutorPromptsMissing", err)
	}
}

// TestSocraticPromptFor_ClampsToValidRange cubre el contrato defensivo:
// un nivel fuera de [0..3] debe devolver el prompt del nivel clampleado,
// nunca panicar con index-out-of-range y nunca devolver string vacío.
// La función vive en tutor_prompts.go (caller-side).
func TestSocraticPromptFor_ClampsToValidRange(t *testing.T) {
	p, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	cases := []struct {
		in   models.SocraticLevel
		want string
	}{
		{models.SocraticDirect, "level 0"},
		{models.SocraticGuided, "level 1"},
		{models.SocraticPure, "level 2"},
		{models.SocraticDiscovery, "level 3"},
		{models.SocraticLevel(-1), "level 0"},  // clamp down
		{models.SocraticLevel(99), "level 3"},  // clamp up
	}
	for _, c := range cases {
		t.Run(c.in.String(), func(t *testing.T) {
			got := p.SocraticPromptFor(c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestSocraticPromptFor_NilPointerSafe verifica que el receptor nil
// no panica. El handler puede llamar con un prompts nil bajo feature
// flag (desarrollo local sin .atl/).
func TestSocraticPromptFor_NilPointerSafe(t *testing.T) {
	var p *TutorPrompts
	if got := p.SocraticPromptFor(models.SocraticPure); got != "" {
		t.Errorf("nil receiver returned %q, want empty", got)
	}
}
