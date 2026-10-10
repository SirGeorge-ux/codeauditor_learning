package services

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrTutorPromptsMissing is the sentinel returned by TutorPrompts
// constructors when one or more of the 5 .atl/prompts/*.md files
// are absent. The cmd/api composition root catches this and either
// fatals (production) or logs+degrades to prompts disabled (test
// harness where the .atl tree does not exist on disk).
var ErrTutorPromptsMissing = errors.New("tutor prompts missing")

// TutorPrompts is the 5 .atl/prompts/*.md file contents in memory.
// Each property holds the raw markdown text; TutorService injects
// these into the ChatContext as a chain of system messages per
// design.md §3 ("Assembly"):
//
//   1. System       — tutor-system.md (identity + command rules).
//   2. Socratic     — tutor-socratic-{N}.md picked by the live level.
//   3. UserSummary  — last LearningProfile.Preferencias.Resumen (built
//                     at call time, not stored on TutorPrompts).
//   4. Location     — current challenge_id/language/code (call time).
//   5. History      — last 10 cache messages + preferred_language
//                     (call time).
//
// Fields 3–5 are NOT cached in TutorPrompts because they vary per
// request; they are owned by the assembleChatContext helper.
type TutorPrompts struct {
	// System is the contents of .atl/prompts/tutor-system.md.
	System string
	// Socratic holds tutor-socratic-{0..3}.md; index == level.
	// Calling code MUST clamp levels to [0..3] before indexing.
	Socratic [4]string
}

// loadTutorPromptsBase is the testable kernel of the loader. It
// reads from an fs.FS so tests can run on a synthetic tree without
// touching disk; production callers go through loadTutorPromptsFromDir.
//
// Returns ErrTutorPromptsMissing with a per-file breakdown when at
// least one file is absent so the error message diagnoses the
// problem without forcing the caller to log the directory.
func loadTutorPromptsBase(fsys fs.FS) (*TutorPrompts, error) {
	required := []string{
		"tutor-system.md",
		"tutor-socratic-0.md",
		"tutor-socratic-1.md",
		"tutor-socratic-2.md",
		"tutor-socratic-3.md",
	}
	system, err := readFile(fsys, "tutor-system.md")
	if err != nil {
		return nil, fmt.Errorf("%w: tutor-system.md: %v", ErrTutorPromptsMissing, err)
	}
	out := &TutorPrompts{System: strings.TrimSpace(system)}
	for i := 0; i < 4; i++ {
		body, err := readFile(fsys, required[i+1])
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrTutorPromptsMissing, required[i+1], err)
		}
		out.Socratic[i] = strings.TrimSpace(body)
	}
	return out, nil
}

// loadTutorPromptsFromDir loads the 5 prompt files from a real
// directory on disk. Used by the production composition root
// (cmd/api/main.go) to read .atl/prompts/.
func loadTutorPromptsFromDir(dir string) (*TutorPrompts, error) {
	if dir == "" {
		return nil, fmt.Errorf("%w: empty directory", ErrTutorPromptsMissing)
	}
	// We synthesize an os.DirFS so the testable kernel above
	// remains the single source of truth for "did this file exist?".
	fsys := os.DirFS(dir)
	return loadTutorPromptsBase(fsys)
}

// SocraticPromptFor returns the prompt body for the requested level,
// clamped to [0..3] so an out-of-range input never panics on array
// indexing. The clamp preserves the design.md §3 invariant: the LLM
// always sees a valid socratic prompt, never an empty one.
func (p *TutorPrompts) SocraticPromptFor(level models.SocraticLevel) string {
	if p == nil {
		return ""
	}
	idx := int(level)
	if idx < 0 {
		idx = 0
	}
	if idx > 3 {
		idx = 3
	}
	return p.Socratic[idx]
}

// readFile wraps fs.ReadFile with a friendlier error so the caller
// can log the filename without re-parsing the wrapped *fs.PathError.
func readFile(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// loadTutorPromptsFromDir is the production-facing loader that
// takes a fully-resolved prompts directory (e.g. ".atl/prompts" or
// the abs path the test temp dir holds).
func loadTutorPromptsFromDirLoader(dir string) (*TutorPrompts, error) {
	return loadTutorPromptsFromDir(dir)
}

// LoadTutorPrompts is the exported form of loadTutorPromptsFromDir.
// The cmd composition root + external tests reach the loader via
// this name. The `dir` parameter is the directory containing the
// 5 tutor-*.md files; the caller is responsible for path
// resolution (e.g. joining ".atl/prompts" onto the project root).
func LoadTutorPrompts(dir string) (*TutorPrompts, error) {
	return loadTutorPromptsFromDirLoader(dir)
}
