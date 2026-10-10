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
	llmcascade "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/llm_cascade"
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

	// Cascade, when non-nil, is the pre-built multi-provider
	// LLMClientPort used by the tutor service. S3 PR 2
	// (free-practice-mode) introduced this so the same cascade
	// adapter can drive the tutor chat AND the un-stubbed
	// ChallengeService.GenerateChallenge orchestration. When nil,
	// TutorSystem builds one from Providers — this preserves the
	// pre-PR-2 behaviour so existing tutor_wiring_test.go cases
	// keep compiling and asserting the same wiring.
	Cascade *llmcascade.Cascade
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
//
// S3 PR 2 (free-practice-mode) added the optional Cascade dep so the
// caller can pre-build the cascade once + share it between the tutor
// service AND the un-stubbed ChallengeService.GenerateChallenge
// orchestration. When Cascade is nil, TutorSystem builds one from
// Providers — this preserves the pre-PR-2 behaviour so existing
// tutor_wiring_test.go cases keep compiling and asserting the same
// wiring.
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
	var cascade *llmcascade.Cascade
	if deps.Cascade != nil {
		cascade = deps.Cascade
	} else {
		groq := llmcascade.NewGroq(deps.Providers.GroqBaseURL, deps.Providers.GroqAPIKey, deps.Providers.GroqModel)
		deep := llmcascade.NewDeepSeek(deps.Providers.DeepSeekBaseURL, deps.Providers.DeepSeekAPIKey, deps.Providers.DeepSeekModel)
		m3 := llmcascade.NewMiniMax(deps.Providers.MiniMaxBaseURL, deps.Providers.MiniMaxAPIKey, deps.Providers.MiniMaxModel)
		cascade = llmcascade.NewCascade(groq, deep, m3)
	}

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

// BuildTutorCascade is the package-exported helper that constructs
// the 3-provider LLM cascade (Groq → DeepSeek → MiniMax) given the
// same TutorProvidersConfig TutorSystem consumes internally. S3 PR 2
// surfaced it so main.go can build the cascade ONCE and share it
// across the tutor service + the ChallengeService.GenerateChallenge
// orchestration. Exposed here (not as a method on TutorSystem) so the
// dependency direction stays clean: main.go orchestrates, tutor_wiring
// provides the building blocks.
func BuildTutorCascade(cfg TutorProvidersConfig) *llmcascade.Cascade {
	groq := llmcascade.NewGroq(cfg.GroqBaseURL, cfg.GroqAPIKey, cfg.GroqModel)
	deep := llmcascade.NewDeepSeek(cfg.DeepSeekBaseURL, cfg.DeepSeekAPIKey, cfg.DeepSeekModel)
	m3 := llmcascade.NewMiniMax(cfg.MiniMaxBaseURL, cfg.MiniMaxAPIKey, cfg.MiniMaxModel)
	return llmcascade.NewCascade(groq, deep, m3)
}

// LoadTutorPrompts is re-exported so the test suite does not import
// the internal services package for a single constructor.
func LoadTutorPrompts(dir string) (*services.TutorPrompts, error) {
	return services.LoadTutorPrompts(dir)
}
