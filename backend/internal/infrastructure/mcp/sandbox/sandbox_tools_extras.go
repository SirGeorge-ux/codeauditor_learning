package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
)

// NewServiceAdapter returns the production wiring for the mcp-sandbox tool
// group. The adapter requires a hardened *sandbox.DockerSandbox (not the
// SandboxExecutor port) so the SecurityProfile contract is structurally
// enforced: the run_code tool can never be wired to a backend that lacks the
// spec-required flags. The provider registry is shared with the rest of the
// app (see internal/infrastructure/driven/sandbox/providers.NewDefaultRegistry).
func NewServiceAdapter(sb *sandbox.DockerSandbox, registry *providers.ProviderRegistry) Service {
	return &serviceAdapter{sb: sb, registry: registry}
}

// serviceAdapter bridges the production sandbox runtime to the local Service
// interface. It owns the SecurityProfile truth (docker backend + the 3
// required flags mirrored from buildDockerRunArgs) so any unauthorized
// change there will surface as a failing run_code rejection.
type serviceAdapter struct {
	sb       *sandbox.DockerSandbox
	registry *providers.ProviderRegistry
}

// SecurityProfile is the source of truth the run_code tool consults. Keeping
// it here — not in dockersandbox.go — makes the dependency explicit at the
// MCP boundary and lets tests pin down the flag set without booting docker.
func (a *serviceAdapter) SecurityProfile() SecurityProfile {
	// Mirror the buildDockerRunArgs defaults from dockersandbox.go.
	// If those ever change, update this list AND the run_code tool's
	// requiredFlags — a divergence is a real bug, not a typo.
	return SecurityProfile{
		Backend: "docker",
		RequiredFlags: []string{
			"--cap-drop=ALL",
			"--network=none",
			"--read-only",
		},
	}
}

// RunCode executes the snippet via the docker sandbox and gathers the
// streamed output into a RunResult. Duration is measured end-to-end so the
// LLM can correlate slow runs with code complexity.
func (a *serviceAdapter) RunCode(ctx context.Context, _, language, code string, timeoutSeconds int) (RunResult, error) {
	if _, err := a.registry.Get(language); err != nil {
		return RunResult{}, ErrUnknownLanguage
	}
	start := time.Now()
	reader, err := a.sb.Execute(ctx, language, code, timeoutSeconds)
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox execute: %w", err)
	}
	defer reader.Close()
	output, truncated, err := readAll(reader, 50*1000)
	if err != nil {
		return RunResult{}, fmt.Errorf("sandbox read: %w", err)
	}
	return RunResult{
		Language:    language,
		Output:      output,
		DurationMs:  time.Since(start).Milliseconds(),
		Truncated:   truncated,
		SecurityLog: "docker: --cap-drop=ALL, --network=none, --read-only",
	}, nil
}

// ListLanguages iterates the provider registry and projects each provider
// into LanguageInfo. The catalog is intentionally reproducible: the registry
// sorts keys, so the LLM sees stable output across calls.
func (a *serviceAdapter) ListLanguages(_ context.Context) ([]LanguageInfo, error) {
	out := make([]LanguageInfo, 0, len(a.registry.Languages()))
	for _, lang := range a.registry.Languages() {
		provider, err := a.registry.Get(lang)
		if err != nil {
			continue
		}
		out = append(out, LanguageInfo{
			Language:       provider.Language(),
			FileExtension:  provider.FileExtension(),
			DockerImage:    provider.DockerImage(),
			LocalCommand:   provider.LocalCommand(),
			InstallHint:    provider.InstallHint(),
			HasDockerImage: true,
		})
	}
	return out, nil
}

// Healthcheck probes the runtime. A non-nil error or populated Issues
// surfaces a degraded backend to the LLM so it can warn the user BEFORE
// every subsequent tool call.
func (a *serviceAdapter) Healthcheck(ctx context.Context) (HealthStatus, error) {
	status := HealthStatus{Backend: "docker"}
	if err := a.sb.Healthcheck(ctx); err != nil {
		status.Issues = append(status.Issues, err.Error())
		status.OK = false
		return status, ErrSandboxUnavailable
	}
	status.OK = true
	return status, nil
}

// GetInstallHint returns the provider's hint or ErrUnknownLanguage when the
// language key is not in the registry. The tool maps the latter to
// ErrInvalidParams so the LLM gets a structured rejection.
func (a *serviceAdapter) GetInstallHint(_ context.Context, language string) (string, error) {
	provider, err := a.registry.Get(language)
	if err != nil {
		return "", ErrUnknownLanguage
	}
	return provider.InstallHint(), nil
}

// --- mcp-sandbox.list_languages ------------------------------------------

// listLanguagesTool returns the catalog of supported languages. It exposes
// the languages keyed by their canonical name (typescript, python, etc.)
// so the tutor LLM can pick a value for run_code without trial-and-error.
type listLanguagesTool struct {
	svc Service
}

// NewListLanguagesTool registers the list_languages action under the
// mcp-sandbox group.
func NewListLanguagesTool(svc Service) *listLanguagesTool {
	return &listLanguagesTool{svc: svc}
}

func (t *listLanguagesTool) Name() string { return "mcp-sandbox.list_languages" }

func (t *listLanguagesTool) Description() string {
	return "List every language the sandbox can execute, with file extension, " +
		"docker image, local command, and install hint. Use this BEFORE " +
		"mcp-sandbox.run_code to pick a language the runtime supports. " +
		"Requires an authenticated user_id in context."
}

func (t *listLanguagesTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{},
		[]string{},
	)
}

func (t *listLanguagesTool) Execute(ctx context.Context, _ json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	langs, err := t.svc.ListLanguages(ctx)
	if err != nil {
		return nil, err
	}
	if langs == nil {
		return []LanguageInfo{}, nil
	}
	return langs, nil
}

// --- mcp-sandbox.healthcheck ---------------------------------------------

// healthcheckTool returns the runtime status. Errors are surfaced verbatim
// so the LLM can present the install hint the backend reports.
type healthcheckTool struct {
	svc Service
}

// NewHealthcheckTool registers the healthcheck action under the mcp-sandbox
// group.
func NewHealthcheckTool(svc Service) *healthcheckTool {
	return &healthcheckTool{svc: svc}
}

func (t *healthcheckTool) Name() string { return "mcp-sandbox.healthcheck" }

func (t *healthcheckTool) Description() string {
	return "Probe the sandbox runtime: docker daemon reachability, image " +
		"presence, and per-provider local-command presence. Returns a " +
		"HealthStatus object; ok=false means the run_code tool will refuse " +
		"calls until the listed Issues are resolved. Requires an authenticated " +
		"user_id in context."
}

func (t *healthcheckTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{},
		[]string{},
	)
}

func (t *healthcheckTool) Execute(ctx context.Context, _ json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	status, err := t.svc.Healthcheck(ctx)
	if err != nil {
		// We still return the status object so the LLM can read Issues.
		// The error is mapped to -32603 by the dispatcher, which is the
		// right semantic for "runtime unavailable".
		return status, err
	}
	return status, nil
}

// --- mcp-sandbox.get_install_hint ----------------------------------------

// getInstallHintTool returns the install command the local sandbox would
// need for the given language. The tool is intentionally read-only — it
// never modifies the user's PATH.
type getInstallHintTool struct {
	svc Service
}

// NewGetInstallHintTool registers the get_install_hint action under the
// mcp-sandbox group.
func NewGetInstallHintTool(svc Service) *getInstallHintTool {
	return &getInstallHintTool{svc: svc}
}

func (t *getInstallHintTool) Name() string { return "mcp-sandbox.get_install_hint" }

func (t *getInstallHintTool) Description() string {
	return "Return the install command for a language the LOCAL sandbox needs. " +
		"Use when the user wants to switch off docker (e.g. on a laptop without " +
		"docker installed) and instead use a locally-resolved interpreter. " +
		"Requires an authenticated user_id in context."
}

func (t *getInstallHintTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Canonical language key (must appear in mcp-sandbox.list_languages)."},
		},
		requiredProps("language"),
	)
}

func (t *getInstallHintTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.Language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	hint, err := t.svc.GetInstallHint(ctx, a.Language)
	if err != nil {
		if err == ErrUnknownLanguage {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return map[string]any{"language": a.Language, "installHint": hint}, nil
}
