// Package sandbox implements the mcp-sandbox tool group of the MCP monolith.
// It exposes four driving adapters — run_code, list_languages, healthcheck,
// get_install_hint — that wrap the sandbox runtime (Docker + ProviderRegistry)
// for the tutor LLM.
//
// Hexagonal contract: this package imports ONLY the local Service interface
// (and shared mcpkit/mcpctx helpers). It MUST NOT import database/sql,
// net/http, or any external service adapter. The wiring in cmd/api/main.go
// adapts the production *sandbox.DockerSandbox + *providers.ProviderRegistry
// to the local Service so the tools stay swappable for tests.
package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported here as a type alias so tool-local code can
// keep using the short name; the underlying value lives in `mcpkit/mcpctx`
// so the dispatcher's `errors.Is` check (in `mcpkit/dispatcher.go`) actually
// matches.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrSandboxUnavailable is returned by Service.Healthcheck when the underlying
// runtime (Docker daemon, all required images) is not ready. The tool surface
// translates this into a structured error; the dispatcher maps it to -32603.
var ErrSandboxUnavailable = fmt.Errorf("sandbox runtime unavailable")

// ErrUnknownLanguage is returned by Service when the requested language is not
// registered in the ProviderRegistry. The tool maps this to ErrInvalidParams so
// the LLM caller gets actionable feedback.
var ErrUnknownLanguage = fmt.Errorf("unknown language")

// SecurityProfile describes the hardening guarantees of a sandbox backend.
// run_code uses it to refuse any backend that does not provide the spec's
// mandatory flags (--cap-drop=ALL, --network=none, --read-only). The local
// sandbox exposes Backend="local" with no flags, which the tool must reject.
type SecurityProfile struct {
	// Backend is the canonical name of the runtime: "docker" or "local".
	Backend string
	// RequiredFlags is the set of docker-run flags the backend enforces.
	// A non-docker backend returns an empty slice, triggering the tool's
	// rejection path (spec scenario: "mcp-sandbox.run_code enforces
	// --cap-drop=ALL --network=none").
	RequiredFlags []string
}

// LanguageInfo is the public wire shape of ProviderRegistry.Languages(). The
// fields mirror the LanguageProvider contract ports.LanguageProvider exposes
// so the tool can present the catalog to the LLM without leaking internal
// file paths or docker image digests.
type LanguageInfo struct {
	Language       string `json:"language"`
	FileExtension  string `json:"fileExtension"`
	DockerImage    string `json:"dockerImage"`
	LocalCommand   string `json:"localCommand"`
	InstallHint    string `json:"installHint"`
	HasDockerImage bool   `json:"hasDockerImage"`
}

// RunResult is the wire shape returned by Service.RunCode. stdout and stderr
// are merged in the order the underlying reader produced them; Truncated is
// true when the caller pre-trimmed the payload to fit MCP's 50K cap.
type RunResult struct {
	Language    string `json:"language"`
	Output      string `json:"output"`
	DurationMs  int64  `json:"durationMs"`
	Truncated   bool   `json:"truncated,omitempty"`
	SecurityLog string `json:"securityLog,omitempty"`
}

// HealthStatus summarizes the sandbox runtime state at a point in time.
type HealthStatus struct {
	OK            bool     `json:"ok"`
	Backend       string   `json:"backend"`
	Issues        []string `json:"issues"`
	MissingTools  []string `json:"missingTools"`
	BackendHints  []string `json:"backendHints"`
}

// userIDKey is the local context key for the authenticated user_id. The MCP
// server's auth middleware stamps this into the request context before
// invoking any tool; tools read it via userIDFromContext.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID. Production
// callers (the MCP auth bridge) use this; tests use it too so the tool's
// user-isolation contract stays the same across both.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key (ContextWithUserID); if not set, it
// falls back to the canonical MCP bridge key (mcpctx.AuthUserIDKey, used by
// mcpkit.Server.Router when the request came in over HTTP).
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// propertyArg is the standardized JSON envelope produced for LLM tool
// routing. Keeping the property struct local to the package ensures the
// `description` text shows up consistently in the function-calling prompt.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single object
// parameter list. The schema shape matches the dictionary/tutor tools so the
// tutor LLM sees a consistent contract across groups.
func schema(name, description string, props map[string]propertyArg, required []string) openai.Tool {
	return openai.Tool{
		Type: "function",
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
}

// requiredFlags is the canonical set the docker sandbox MUST enforce for
// run_code to be invokable. These mirror the buildDockerRunArgs defaults in
// internal/infrastructure/driven/sandbox/dockersandbox.go.
var requiredFlags = []string{"--cap-drop=ALL", "--network=none", "--read-only"}

// requiredProps materializes required-field and property names. Keeping a
// helper here prevents typo drift between schema() calls.
func requiredProps(names ...string) []string { return names }

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP sandbox tool depends on. The
// production wiring supplies the docker sandbox + provider registry through
// a thin adapter (NewServiceAdapter below); tests substitute a fake without
// touching real docker / provider plumbing.
type Service interface {
	// SecurityProfile describes the runtime guarantees. run_code consults
	// it before every invocation; a profile with Backend != "docker" or
	// missing required flags triggers an ErrInvalidParams rejection.
	SecurityProfile() SecurityProfile
	// RunCode executes the snippet in an isolated runtime. The
	// implementation MUST close the returned reader before returning.
	RunCode(ctx context.Context, userID, language, code string, timeoutSeconds int) (RunResult, error)
	// ListLanguages returns the catalog of providers registered in the
	// sandbox's ProviderRegistry, sorted by language key.
	ListLanguages(ctx context.Context) ([]LanguageInfo, error)
	// Healthcheck probes the runtime; callers should expect a non-nil
	// error or Issues/MissingTools populated on a degraded backend.
	Healthcheck(ctx context.Context) (HealthStatus, error)
	// GetInstallHint returns the provider's InstallHint for the requested
	// language, or an error if the language is unknown.
	GetInstallHint(ctx context.Context, language string) (string, error)
}

// --- mcp-sandbox.run_code -------------------------------------------------

// runCodeTool executes a code snippet in the hardened sandbox. It refuses to
// run on any backend without the spec's security guarantees
// (--cap-drop=ALL, --network=none, --read-only) so the LLM can never trick
// the tool into a less-secure local execution path. The user_id is stamped
// by the auth middleware on the request context; the tool never trusts a
// user_id from the JSON body.
type runCodeTool struct {
	svc Service
}

// NewRunCodeTool registers the run_code action under the mcp-sandbox group.
func NewRunCodeTool(svc Service) *runCodeTool { return &runCodeTool{svc: svc} }

func (t *runCodeTool) Name() string { return "mcp-sandbox.run_code" }

func (t *runCodeTool) Description() string {
	return "Execute a code snippet inside the hardened sandbox runtime. The " +
		"tool REFUSES to run on any backend missing `--cap-drop=ALL`, " +
		"`--network=none`, or `--read-only` (spec scenario: mcp-sandbox.run_code " +
		"enforces security flags). Streams stdout+stderr to the LLM as a single " +
		"string. Timeout is bounded by `timeoutSeconds` (default 30, max 30). " +
		"Requires an authenticated user_id in context."
}

func (t *runCodeTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language":       {Type: "string", Description: "One of the languages returned by mcp-sandbox.list_languages (e.g. typescript, python, go)."},
			"code":           {Type: "string", Description: "Source code to execute. Empty code is rejected."},
			"timeoutSeconds": {Type: "integer", Description: "Maximum execution time in seconds. Defaults to 30; values above 30 are clamped to 30 to honor the spec's 30s budget."},
		},
		requiredProps("language", "code"),
	)
}

// Execute validates the security profile, parses args, clamps the timeout
// to the spec's 30s budget, then delegates to the service.
func (t *runCodeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language       string `json:"language"`
		Code           string `json:"code"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.Language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if strings.TrimSpace(a.Code) == "" {
		return nil, fmt.Errorf("%w: code is required", ErrInvalidParams)
	}
	profile := t.svc.SecurityProfile()
	if profile.Backend != "docker" {
		return nil, fmt.Errorf("%w: sandbox backend %q is not hardened; only docker is accepted", ErrInvalidParams, profile.Backend)
	}
	for _, want := range requiredFlags {
		if !containsString(profile.RequiredFlags, want) {
			return nil, fmt.Errorf("%w: sandbox missing required flag %s", ErrInvalidParams, want)
		}
	}
	if a.TimeoutSeconds <= 0 {
		a.TimeoutSeconds = 30
	}
	if a.TimeoutSeconds > 30 {
		a.TimeoutSeconds = 30
	}
	result, err := t.svc.RunCode(ctx, userID, a.Language, a.Code, a.TimeoutSeconds)
	if err != nil {
		if err == ErrUnknownLanguage {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return result, nil
}

// containsString reports whether list contains the exact element.
func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// readAll is exposed for the adapter wiring to drain the SandboxExecutor's
// stdout stream into a string. Keeping it here lets the production adapter
// stay free of byte-fiddling inline. io.LimitReader enforces the 50KB cap
// so oversized output is truncated before it travels through MCP.
func readAll(r io.Reader, max int) (string, bool, error) {
	if max <= 0 {
		max = 50 * 1000
	}
	limited := io.LimitReader(r, int64(max)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	if len(data) > max {
		return string(data[:max]), true, nil
	}
	return string(data), false, nil
}
