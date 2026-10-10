package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotImplemented is returned by stub service methods that exist for PR 1
// compilability but whose real behavior is delivered in later PRs.
var ErrNotImplemented = errors.New("not implemented in this PR")

// ErrCurriculumInvalidLanguage is returned by CurriculumService when the
// supplied language segment fails path-traversal validation (see the
// threat-matrix row "Path Traversal in Curricula" in design.md). The
// driving layer maps this to ErrInvalidParams (-32602) so the LLM caller
// receives actionable feedback without leaking filesystem paths.
var ErrCurriculumInvalidLanguage = errors.New("invalid language: refusing to traverse outside the curricula root")

// ErrCurriculumTopicNotFound is returned by CurriculumService when a
// (language, topicID) pair is requested but the topic slug does not
// appear in the parsed curriculum document. S3 PR 2 surfaced this as
// its own sentinel so the un-stubbed ChallengeService.GenerateChallenge
// (and the UI) can branch with errors.Is — "Topic not available" is a
// distinct failure mode from "language has no curriculum file" or
// "path traversal".
var ErrCurriculumTopicNotFound = errors.New("curriculum topic not found")

// ErrCurriculumLanguageNotFound is returned by CurriculumService when
// the language key is structurally valid (no traversal) but no
// curriculum file exists for that language. S3 PR 2 surfaced this
// sentinel so the UI can disable generation for the language instead
// of silently returning an empty topic list.
var ErrCurriculumLanguageNotFound = errors.New("curriculum language not found")

// CurriculumService reads the per-language curricula under <baseDir>/<lang>.md
// and serves the mcp-curriculum tool group. PR 3 ships the path-traversal
// guard the spec mandates, plus the markdown extraction logic the four tools
// (get_topics, get_topic, suggest_topic, review_topic) need.
//
// Hexagonal contract: this type imports ONLY core/domain/models (none
// required for the parser yet), ports (none for now), and the standard
// library. The filesystem IS the persistence layer for the curricula —
// .md files are the single source of truth (see
// docs/curricula-format.md). Repo-path resolution lives entirely in main.go
// (defaultBaseDir).
type CurriculumService struct {
	baseDir string
}

// CurriculumServiceOption configures a CurriculumService. Reserved for
// future overrides (custom parser, cache, etc.); keeping it as a function
// option keeps the zero-arg constructor compatible with PR 1 wiring.
type CurriculumServiceOption func(*CurriculumService)

// WithBaseDir overrides the curricula root directory. Tests pass t.TempDir();
// production wires the repo-relative .atl/curricula/ path via main.go.
//
// Setting an empty path returns ErrCurriculumInvalidLanguage on the first
// read; callers (main.go + tests) must supply a non-empty baseDir.
func WithBaseDir(p string) CurriculumServiceOption {
	return func(c *CurriculumService) { c.baseDir = p }
}

// NewCurriculumService creates a CurriculumService rooted at baseDir.
// The legacy PR 1 wiring (no args, empty baseDir) is preserved as
// NewCurriculumService() so the existing main.go compile-target doesn't
// change shape; production callers should migrate to
// NewCurriculumServiceWith(<dir>) or supply WithBaseDir() before first use.
func NewCurriculumService(opts ...CurriculumServiceOption) *CurriculumService {
	c := &CurriculumService{}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Curriculum returns the curricula root directory this service reads from.
// Exposed for test assertions and main.go bootstrap logging.
func (s *CurriculumService) Curriculum() string { return s.baseDir }

// --- Public input/output shapes ---

// GetTopicsInput is the argument shape for GetTopics.
type GetTopicsInput struct {
	Language string `json:"language"`
}

// Topic is a single curriculum topic headline. We intentionally keep the
// domain shape small — only the headline fields the spec lists — so the
// full topic body (description, exercises, etc.) travels via GetTopic.
type Topic struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Difficulty  string `json:"difficulty,omitempty"`
	Description string `json:"description,omitempty"`
}

// TopicDetail is the full topic body GetTopic returns.
type TopicDetail struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Difficulty     string   `json:"difficulty,omitempty"`
	Prerequisites  []string `json:"prerequisites"`
	Category       string   `json:"category,omitempty"`
	Description    string   `json:"description"`
	Exercises      []string `json:"exercises"`
	CommonMistakes []string `json:"commonMistakes"`
	Resources      []string `json:"resources"`
}

// SuggestTopicInput carries the context for topic suggestion.
type SuggestTopicInput struct {
	Language string `json:"language"`
	Level    string `json:"level"`
	Goal     string `json:"goal,omitempty"`
}

// ReviewTopicInput holds a self-assessment for a completed topic.
type ReviewTopicInput struct {
	Language string `json:"language"`
	TopicID  string `json:"topic_id"`
	Rating   int    `json:"rating"`
	Notes    string `json:"notes,omitempty"`
}

// ReviewTopicResult is the outcome of recording a topic review.
type ReviewTopicResult struct {
	Reviewed bool `json:"reviewed"`
}

// --- GetTopics/GetTopic (the path-traversal-safe readers) ---

// GetTopics lists topic headlines for the given language. Returns
// ErrCurriculumInvalidLanguage when the language segment fails the
// traversal guard; returns ErrCurriculumLanguageNotFound when the
// language segment is valid but no curriculum file exists; returns
// an underlying fs error otherwise. The returned slice is empty
// (not nil) when the document parses to zero topics (degenerate
// curriculum file).
func (s *CurriculumService) GetTopics(_ context.Context, in GetTopicsInput) ([]Topic, error) {
	if err := s.validateLanguage(in.Language); err != nil {
		return nil, err
	}
	root, err := s.curriculaPath(in.Language)
	if err != nil {
		return nil, err
	}
	doc, err := s.readCurriculumDoc(root)
	if err != nil {
		if isNotExist(err) {
			return nil, ErrCurriculumLanguageNotFound
		}
		return nil, err
	}
	return doc.Headlines(), nil
}

// GetTopic returns the full topic body for a (language, topicID) pair.
// Returns ErrCurriculumInvalidLanguage on path-traversal attempts,
// ErrCurriculumLanguageNotFound when no curriculum file exists for
// the language, and ErrCurriculumTopicNotFound when the topic slug
// does not appear in the parsed document.
func (s *CurriculumService) GetTopic(_ context.Context, language, topicID string) (TopicDetail, error) {
	var zero TopicDetail
	if err := s.validateLanguage(language); err != nil {
		return zero, err
	}
	if err := validateSegment("topic id", topicID); err != nil {
		return zero, err
	}
	root, err := s.curriculaPath(language)
	if err != nil {
		return zero, err
	}
	doc, err := s.readCurriculumDoc(root)
	if err != nil {
		if isNotExist(err) {
			return zero, ErrCurriculumLanguageNotFound
		}
		return zero, err
	}
	topic, ok := doc.findTopic(topicID)
	if !ok {
		return zero, fmt.Errorf("%w: %q in %s", ErrCurriculumTopicNotFound, topicID, language)
	}
	return topic, nil
}

// SuggestTopic proposes the next topic to study. PR 3 ships a deterministic
// heuristic: pick the FIRST topic that is NOT already in the user's
// dominant topic set (which the calling tool can pass via Goal) AND whose
// difficulty matches the requested F-S level. Real LLM-based suggestions
// land with the S5 cascade — for now the deterministic rule makes the tool
// testable and surfaceable to the tutor without an LLM call.
//
// The function returns ErrCurriculumInvalidLanguage on path-traversal
// attempts (same as GetTopics); otherwise it may return an empty Topic and
// nil when nothing matches.
func (s *CurriculumService) SuggestTopic(_ context.Context, in SuggestTopicInput) (Topic, error) {
	if err := s.validateLanguage(in.Language); err != nil {
		return Topic{}, err
	}
	topics, err := s.GetTopics(context.Background(), GetTopicsInput{Language: in.Language})
	if err != nil {
		return Topic{}, err
	}
	// map: user's "I know these" set from in.Goal (csv) — empty when unset.
	known := make(map[string]struct{})
	for _, k := range strings.Split(in.Goal, ",") {
		if t := strings.TrimSpace(k); t != "" {
			known[t] = struct{}{}
		}
	}
	wantLevel := strings.ToUpper(strings.TrimSpace(in.Level))
	for _, t := range topics {
		if wantLevel != "" && strings.ToUpper(t.Difficulty) != wantLevel {
			continue
		}
		if _, ok := known[t.ID]; ok {
			continue
		}
		return t, nil
	}
	return Topic{}, nil
}

// ReviewTopic records the user's self-assessment of a completed topic.
// PR 3 has no persistent review store — the spec defers that to S3. The
// service validates inputs, then returns a Reviewed=true result so the
// tutor can acknowledge the review. The reviewer-shaped future storage
// (a `reviewed_topics` JSONB column on user_learning_profile, or a
// dedicated review table) lands with the S3 free-practice spec; we
// validate the inputs here to keep the tool's behavior predictable.
func (s *CurriculumService) ReviewTopic(_ context.Context, in ReviewTopicInput) (ReviewTopicResult, error) {
	if err := s.validateLanguage(in.Language); err != nil {
		return ReviewTopicResult{}, err
	}
	if err := validateSegment("topic id", in.TopicID); err != nil {
		return ReviewTopicResult{}, err
	}
	if in.Rating < 1 || in.Rating > 5 {
		return ReviewTopicResult{}, fmt.Errorf("%w: rating must be 1..5", ErrCurriculumInvalidLanguage)
	}
	return ReviewTopicResult{Reviewed: true}, nil
}

// --- Internals ---

// validateLanguage is the canonical guard the threat-matrix scenario
// 'Path Traversal in Curricula' demands. It rejects:
//   - empty languages
//   - segments containing '/' (would escape the language root)
//   - segments containing '..' as a path component (parent traversal)
//   - segments that resolve via filepath.Clean to anything but a single
//     safe token (absolute paths, dotfiles, etc.)
//
// The check is intentionally conservative: even a future runtime convention
// for nested curricula (foo/bar.md) can lift the slash ban by using a
// different separator; the threat-matrix scope is "../etc". We still
// accept the safe single-segment form.
func (s *CurriculumService) validateLanguage(language string) error {
	if err := validateSegment("language", language); err != nil {
		return err
	}
	cleaned := filepath.Clean(language)
	if cleaned != language {
		return fmt.Errorf("%w: %q resolves to %q (not a single segment)", ErrCurriculumInvalidLanguage, language, cleaned)
	}
	// Belt + suspenders: even after filepath.Clean rejects "..", reject any
	// remaining ../appearing-anywhere because some shells / editors normalize
	// without warning. Cheap and explicit.
	if strings.Contains(language, "..") {
		return fmt.Errorf("%w: %q contains parent-traversal segment", ErrCurriculumInvalidLanguage, language)
	}
	return nil
}

// validateSegment applies the rules a single path segment must satisfy.
// The rules match the threat-matrix scope: empty, parent-traversal,
// absolute-path, dotfile, and slash-bearing segments are rejected.
func validateSegment(field, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is required", ErrCurriculumInvalidLanguage, field)
	}
	if strings.ContainsAny(value, "/\\") {
		return fmt.Errorf("%w: %s may not contain a path separator", ErrCurriculumInvalidLanguage, field)
	}
	if strings.Contains(value, "..") {
		return fmt.Errorf("%w: %s may not contain parent-traversal segment", ErrCurriculumInvalidLanguage, field)
	}
	if strings.HasPrefix(value, ".") && !strings.HasPrefix(value, "..") {
		return fmt.Errorf("%w: %s may not start with a dot", ErrCurriculumInvalidLanguage, field)
	}
	if strings.HasPrefix(value, "~") {
		return fmt.Errorf("%w: %s may not use home-expansion", ErrCurriculumInvalidLanguage, field)
	}
	return nil
}

// curriculaPath builds the absolute (language).md path inside baseDir.
// Returning a path that fails filepath.Clean gives the caller a clean
// error to surface; double-checking after Clean is a belt to catch any
// future fix to validateSegment that lets a sneaky segment through.
func (s *CurriculumService) curriculaPath(language string) (string, error) {
	if s.baseDir == "" {
		return "", fmt.Errorf("%w: curricula root not configured", ErrCurriculumInvalidLanguage)
	}
	cleanedBase := filepath.Clean(s.baseDir)
	root := filepath.Join(cleanedBase, language+".md")
	cleaned := filepath.Clean(root)
	// After Clean, the resulting path MUST still live directly under
	// baseDir — a parent-traversal segment that slipped past
	// validateLanguage would make Clean escape the prefix.
	if !strings.HasPrefix(cleaned, cleanedBase+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q escapes the curricula root", ErrCurriculumInvalidLanguage, cleaned)
	}
	return cleaned, nil
}

// readCurriculumDoc opens the .md file at root and returns a parsed
// curriculumDoc. Wrapping the open in os.Open keeps the fs dependency in
// one place so tests can introduce a fs abstraction later without
// touching the parser.
func (s *CurriculumService) readCurriculumDoc(root string) (*curriculumDoc, error) {
	f, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseCurriculum(f, root)
}

// isNotExist reports whether err is or wraps a filesystem not-exist error.
func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return errors.Is(pe.Err, os.ErrNotExist)
	}
	return errors.Is(err, os.ErrNotExist)
}

// --- Markdown parser (intentionally small and tests-friendly) ---

// headingRegex matches a topic heading '### <slug>' (3+ leading #, then
// the slug). The spec uses exactly '###'; we accept 3..6 to be tolerant.
var headingRegex = regexp.MustCompile(`(?m)^#{3,6}\s+([A-Za-z0-9_\-.]+)\s*$`)

// fieldRegex matches a field bullet like '- **slug:** foo' or
// '- **prerequisites:** [a, b]'. The capture group is the field name.
var fieldRegex = regexp.MustCompile(`^\s*-\s*\*\*\s*([A-Za-z0-9_]+)\s*:\*\*\s*(.*)$`)

// curriculumDoc is the in-memory representation of a single .md file.
type curriculumDoc struct {
	path   string
	topics []TopicDetail
}

// lastField labels which body field the parser is currently filling. The
// parser uses it to attribute continuation lines (sub-bullets and prose
// paragraphs) to the correct list/scalar — see parseCurriculum.
type lastField int

const (
	lastFieldNone lastField = iota
	lastFieldDescription
	lastFieldExercises
	lastFieldCommonMistakes
	lastFieldResources
	lastFieldPrerequisites
	lastFieldOther
)

// Headlines returns the topic-id + title slice used by mcp-curriculum.get_topics.
func (d *curriculumDoc) Headlines() []Topic {
	out := make([]Topic, 0, len(d.topics))
	for _, t := range d.topics {
		out = append(out, Topic{
			ID:          t.ID,
			Title:       t.Title,
			Difficulty:  t.Difficulty,
			Description: firstParagraph(t.Description),
		})
	}
	return out
}

// findTopic returns the full TopicDetail for slug, or false if none matches.
func (d *curriculumDoc) findTopic(slug string) (TopicDetail, bool) {
	for _, t := range d.topics {
		if t.ID == slug {
			return t, true
		}
	}
	return TopicDetail{}, false
}

// parseCurriculum reads a .md file (the io.Reader holds the source) and
// returns the parsed curriculum. The parser is purposefully small:
// each '### <slug>' starts a topic, inside which '- **key:** value' lines
// become scalar fields or list fields (depending on key) until the next
// '### <slug>' or EOF. Sub-bullets inside a list field (e.g.
// '  - "exercise text"' after '- **exercises:**') are appended to the
// most-recently-active list using the `lastField` cursor.
func parseCurriculum(r io.Reader, pathForErrors string) (*curriculumDoc, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB lines
	doc := &curriculumDoc{path: pathForErrors}
	var (
		current = (*TopicDetail)(nil)
		last    = lastFieldNone
	)

	flush := func() {
		if current != nil {
			doc.topics = append(doc.topics, *current)
			current = nil
			last = lastFieldNone
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if m := headingRegex.FindStringSubmatch(line); m != nil {
			flush()
			slug := m[1]
			current = &TopicDetail{ID: slug, Title: slug}
			continue
		}
		if current == nil {
			// Pre-heading lines (frontmatter, header blurb) are ignored by
			// design — they are not part of the topic list and the parser
			// would over-attribute them otherwise.
			continue
		}
		m := fieldRegex.FindStringSubmatch(line)
		if m == nil {
			// Not a field bullet: attribute to the most-recent field. We
			// distinguish sub-bullets (start with "- ") from prose
			// continuation lines so list fields accept bullets and
			// description accepts prose.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t") {
				if last == lastFieldExercises || last == lastFieldCommonMistakes ||
					last == lastFieldResources || last == lastFieldPrerequisites {
					// First sub-bullet after `**key:**`: this '- "..."'
					// carries the list entry. Strip leading '-' + whitespace
					// + optional quote characters.
					val := strings.TrimSpace(trimmed[1:])
					if last == lastFieldExercises || last == lastFieldCommonMistakes || last == lastFieldResources {
						val = unquoteBullet(val)
					}
					switch last {
					case lastFieldExercises:
						current.Exercises = appendOrInit(&current.Exercises)
						current.Exercises = append(current.Exercises, val)
					case lastFieldCommonMistakes:
						current.CommonMistakes = appendOrInit(&current.CommonMistakes)
						current.CommonMistakes = append(current.CommonMistakes, val)
					case lastFieldResources:
						current.Resources = appendOrInit(&current.Resources)
						current.Resources = append(current.Resources, val)
					case lastFieldPrerequisites:
						current.Prerequisites = appendOrInit(&current.Prerequisites)
						current.Prerequisites = append(current.Prerequisites, val)
					}
					continue
				}
				// Sub-bullet on a non-list field (rare): fold into the
				// current description as a sub-idea.
				if last == lastFieldDescription || current.Description == "" {
					if current.Description != "" {
						current.Description += "\n"
					}
					current.Description += strings.TrimSpace(trimmed[1:])
				}
				continue
			}
			// Prose-style continuation line (empty / text). Append to the
			// active description field when applicable.
			if last == lastFieldDescription {
				if trimmed != "" {
					current.Description += "\n" + trimmed
				}
			}
			continue
		}
		key := m[1]
		rawValue := strings.TrimSpace(m[2])

		// Field bullet: dispatch by key. The order is the same as the
		// documentation in docs/curricula-format.md §2.4.
		switch key {
		case "slug":
			current.ID = rawValue
			current.Title = rawValue
			last = lastFieldOther
		case "difficulty":
			current.Difficulty = rawValue
			last = lastFieldOther
		case "category":
			current.Category = rawValue
			last = lastFieldOther
		case "prerequisites":
			current.Prerequisites = parseListField(rawValue)
			if current.Prerequisites == nil {
				current.Prerequisites = []string{}
			}
			last = lastFieldPrerequisites
		case "description":
			current.Description = rawValue
			last = lastFieldDescription
		case "exercises":
			current.Exercises = appendOrInit(&current.Exercises)
			last = lastFieldExercises
		case "common_mistakes":
			current.CommonMistakes = appendOrInit(&current.CommonMistakes)
			last = lastFieldCommonMistakes
		case "resources":
			current.Resources = appendOrInit(&current.Resources)
			last = lastFieldResources
		default:
			last = lastFieldOther
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", pathForErrors, err)
	}
	return doc, nil
}

// parseListField interprets a "[a, b, c]" or "a, b, c" inline list. Empty
// input returns a nil slice so callers can distinguish 'unset' from
// 'present and empty'; the assignment paths in parseCurriculum normalise
// the difference where it matters (e.g. prerequisites).
func parseListField(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// unquoteBullet strips the surrounding double quotes a bullet like
//   - "Declare a const for a user's name"
//
// leaves in the field — the spec uses quoted bullets so the markdown
// reader doesn't collapse them. Keeping the quotes would make the LLM
// render literal '"' in tool output.
func unquoteBullet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func appendOrInit(dst *[]string) []string {
	if *dst == nil {
		*dst = []string{}
	}
	return *dst
}

// firstParagraph trims a description to its first paragraph (up to the
// first blank line) so the LLM's get_topics reply stays compact without
// losing the lede.
func firstParagraph(s string) string {
	if s == "" {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' && (i+1 >= len(s) || s[i+1] == '\n') {
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}
