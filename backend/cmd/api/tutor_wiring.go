// PR 2 tutor-system wiring — additive composition root helper. The
// buildMCPRegistry() function owns the S2 MCP tools; this file
// owns the tutor service + cascade + HTTP handler. Keeping them
// separate lets main.go stay frozen at the mcp-monolith PR 5
// boundary (per the PR 2 brief: "ADDITIVE only, no refactor").
package main

import (
	"errors"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/llm_cascade"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/handlers"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// TutorSystemDeps is the bag TutorSystem needs. It mirrors the
// production env-var wiring; tests use a minimal variant.
type TutorSystemDeps struct {
	// ProfileRepository is the UserProgressRepository used by the
	// rest of the application. The MCP catalog (mcp-user) and the
	// tutor service hit the same row.
	ProfileRepository ports.UserProgressRepository

	// PromptsDir is the directory the loader reads the 5
	// tutor-*.md files from. Production wires ".atl/prompts"
	// (relative to cwd); tests use a temp dir.
	PromptsDir string

	// Providers is the 3-provider config. Empty API keys mark
	// providers disabled; the cascade skips them.
	Providers TutorProvidersConfig
}

// TutorProvidersConfig mirrors the env-var shape so main.go can
// populate it from `os.Getenv(...)` calls without leaking the
// wiring helper to stdlib globals.
type TutorProvidersConfig struct {
	GroqBaseURL     string
	GroqAPIKey      string
	GroqModel       string
	DeepSeekBaseURL string
	DeepSeekAPIKey  string
	DeepSeekModel   string
	MiniMaxBaseURL  string
	MiniMaxAPIKey   string
	MiniMaxModel    string
}

// TutorSystemWiring bundles the constructed TutorService + HTTP
// handler. cmd/api/main.go wires it once at startup.
type TutorSystemWiring struct {
	Service ports.TutorServicePort
	Handler *handlers.TutorHandler
	Prompts *services.TutorPrompts
	Cascade *llmcascade.Cascade
	Cache   ports.CachePort
}

// TutorSystem constructs the TutorService + supporting pieces. It
// returns ErrTutorPromptsMissing if the prompts directory does not
// contain the 5 expected files; degraded mode is the caller's
// choice (production fatals; tests skip the routes).
func TutorSystem(deps TutorSystemDeps) (*TutorSystemWiring, error) {
	if deps.ProfileRepository == nil {
		return nil, errors.New("tutor wiring: ProfileRepository must not be nil")
	}
	if deps.PromptsDir == "" {
		return nil, errors.New("tutor wiring: PromptsDir must not be empty")
	}
	prompts, err := services.LoadTutorPrompts(deps.PromptsDir)
	if err != nil {
		return nil, err
	}
	cache := driven.NewSessionCache()
	groq := llmcascade.NewGroq(deps.Providers.GroqBaseURL, deps.Providers.GroqAPIKey, deps.Providers.GroqModel)
	deep := llmcascade.NewDeepSeek(deps.Providers.DeepSeekBaseURL, deps.Providers.DeepSeekAPIKey, deps.Providers.DeepSeekModel)
	m3 := llmcascade.NewMiniMax(deps.Providers.MiniMaxBaseURL, deps.Providers.MiniMaxAPIKey, deps.Providers.MiniMaxModel)
	cascade := llmcascade.NewCascade(groq, deep, m3)

	tutorService := services.NewTutorService(cascade, cache, deps.ProfileRepository, prompts)
	handler := handlers.NewTutorHandler(tutorService)
	return &TutorSystemWiring{
		Service: tutorService,
		Handler: handler,
		Prompts: prompts,
		Cascade: cascade,
		Cache:   cache,
	}, nil
}

// LoadTutorPrompts is re-exported so the test suite does not import
// the internal services package for a single constructor.
func LoadTutorPrompts(dir string) (*services.TutorPrompts, error) {
	return services.LoadTutorPrompts(dir)
}
