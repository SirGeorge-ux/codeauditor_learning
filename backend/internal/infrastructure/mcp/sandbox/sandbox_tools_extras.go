package sandbox

// This file owns the secondary mcp-sandbox tool definitions:
// list_languages, healthcheck, and get_install_hint. The production
// adapter wiring (serviceAdapter + SecurityProfile + RunCode) was
// moved to adapter.go as part of the mcp-monolith PR 2 amendment so
// the production-side glue is reviewable in isolation, mirroring the
// challenges/adapter.go split. The run_code tool itself lives in
// sandbox_tool.go because it predates the amendment and gates on the
// SecurityProfile contract declared in adapter.go.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

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
