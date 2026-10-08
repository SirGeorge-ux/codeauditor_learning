package sandbox

// This file owns the production-side glue of the mcp-sandbox tool group:
// the NewServiceAdapter constructor, the serviceAdapter type, and the
// SecurityProfile source of truth run_code consults. Tool definitions
// live in sandbox_tool.go (run_code) and sandbox_tools_extras.go
// (list_languages, healthcheck, get_install_hint) — the split mirrors
// the challenges/adapter.go isolation so the production wiring is
// reviewable on its own and tests can pin the SecurityProfile/registry
// wiring without re-reading the tool layer.

import (
	"context"
	"fmt"
	"time"

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
