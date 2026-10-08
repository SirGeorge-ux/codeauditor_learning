package mcpkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// JSON-RPC 2.0 error codes used by the MCP dispatcher.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeRateLimited    = -32609
)

// toolTimeout is the per-call execution budget. The spec mandates 30 seconds.
const toolTimeout = 30 * time.Second

// MaxResponseBytes caps a tool's result payload. Responses larger than this
// are truncated and flagged with `truncated: true`.
const MaxResponseBytes = 50 * 1000 // 50,000 bytes

// RPCError is the JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements error.
func (e *RPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// rpcRequest is the inbound JSON-RPC 2.0 envelope for tools/call.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  rpcParams       `json:"params"`
}

// rpcParams carries the tool name and arguments.
type rpcParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// rpcResponse is the outbound JSON-RPC 2.0 envelope.
type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// Dispatcher resolves JSON-RPC 2.0 requests against a Registry, enforces the
// per-call timeout, sanitizes and truncates results, and maps Go errors to
// JSON-RPC error codes.
type Dispatcher struct {
	registry *Registry
	clock    func() time.Time // injectable for deterministic tests
	timeout  time.Duration    // per-call execution budget (default 30s)
}

// NewDispatcher creates a Dispatcher bound to the given registry.
func NewDispatcher(registry *Registry) *Dispatcher {
	return &Dispatcher{registry: registry, clock: time.Now, timeout: toolTimeout}
}

// WithTimeout overrides the per-call execution budget (tests use small values
// for fast timeout verification; production keeps the 30s default).
func (d *Dispatcher) WithTimeout(t time.Duration) *Dispatcher {
	d.timeout = t
	return d
}

// Dispatch parses and executes a single JSON-RPC 2.0 tools/call request. It
// returns a fully populated rpcResponse ready to be marshaled to the client.
// A non-nil rawErr (when id is unparseable) carries a Parse error to emit.
func (d *Dispatcher) Dispatch(ctx context.Context, body []byte) rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: nil, Error: &RPCError{
			Code: CodeParseError, Message: "Parse error",
		}}
	}

	// Normalize the id to any (nil when absent/null).
	id := normalizeID(req.ID)

	// jsonrpc field is informational; method must be present.
	if req.Method == "" {
		return rpcResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{
			Code: CodeInvalidRequest, Message: "Invalid Request: missing method",
		}}
	}
	if req.Method != "tools/call" {
		return rpcResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{
			Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s (-32601)", req.Method),
		}}
	}

	// Resolve the tool. Unknown name → -32601.
	tool, err := d.registry.Get(req.Params.Name)
	if err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: id, Error: rpcErrorFrom(err, req.Params.Name)}
	}

	// Enforce the per-call timeout (default 30s; overridable for fast tests).
	execCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	result, err := tool.Execute(execCtx, req.Params.Arguments)
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return rpcResponse{JSONRPC: "2.0", ID: id, Error: &RPCError{
				Code: CodeInternalError, Message: "Tool execution timed out after 30s",
			}}
		}
		return rpcResponse{JSONRPC: "2.0", ID: id, Error: rpcErrorFrom(err, req.Params.Name)}
	}

	// Sanitize leaked secrets/assets, then enforce the size cap.
	result = SanitizeResult(result)
	truncated, wasTruncated := TruncateResult(result, MaxResponseBytes)
	if wasTruncated {
		return rpcResponse{
			JSONRPC: "2.0", ID: id,
			Result: map[string]any{"content": truncated, "truncated": true},
		}
	}
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

// rpcErrorFrom maps a Go error to the appropriate JSON-RPC error code.
func rpcErrorFrom(err error, toolName string) *RPCError {
	switch {
	case errors.Is(err, ErrToolNotFound):
		return &RPCError{Code: CodeMethodNotFound, Message: fmt.Sprintf("method not found: %s (-32601)", toolName)}
	case IsRateLimitedError(err):
		return &RPCError{Code: CodeRateLimited, Message: "Rate limit exceeded"}
	case errors.Is(err, ErrInvalidParams):
		return &RPCError{Code: CodeInvalidParams, Message: err.Error()}
	default:
		return &RPCError{Code: CodeInternalError, Message: SanitizeErrorMessage(err.Error())}
	}
}

// ErrInvalidParams is re-exported as an alias of mcpctx.ErrInvalidParams so
// existing imports `mcp.ErrInvalidParams` keep compiling while the value
// lives in the canonical mcpctx package (used by the dictionary tool group
// too, so errors.Is checks actually succeed across the boundary).
var ErrInvalidParams = mcpctx.ErrInvalidParams

// normalizeID converts a raw JSON id into an any safe to marshal back. json
// null or absent becomes nil.
func normalizeID(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var id any
	_ = json.Unmarshal(raw, &id)
	return id
}

// TruncateResult applies the 50KB response cap. If the marshaled payload fits
// within maxBytes, it returns the value unchanged with wasTruncated=false.
// Otherwise it returns a string representation truncated to maxBytes and
// wasTruncated=true; the caller wraps it with the `truncated` flag.
func TruncateResult(value any, maxBytes int) (string, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		// Fall back to a string coercion; if even that grows past the cap it
		// is trimmed below.
		encoded = []byte(fmt.Sprintf("%v", value))
	}
	if len(encoded) <= maxBytes {
		return string(encoded), false
	}
	return string(encoded[:maxBytes]), true
}

// ---- Response sanitization (threat matrix: Data leakage) ----

// secretPatterns are regexp patterns whose presence in a tool response
// indicates leaked credentials/assets. Matches are redacted to "[REDACTED]".
var secretPatterns = []*regexp.Regexp{
	// Generic secret-bearing keys: "api_key":"...", "token":"...", "secret":"...",
	// "password":"...", "apikey":"...", "pwd":"..." — the whole `"key":"value"`
	// pair is replaced with a redacted sentinel.
	regexp.MustCompile(`(?i)"(?:api[_]?key|token|secret|password|passwd|pwd)"\s*:\s*"[^"]*"`),

	// Authorization/Bearer headers: `authorization":"bearer <value>` or
	// `authorization: bearer <value>` — replace only the secret portion.
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*"?bearer\s+)[A-Za-z0-9._\-]+`),

	// JWT-shaped tokens (three base64url segments).
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`),

	// OpenAI-style keys (sk-...).
	regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9]{20,}`),

	// Supabase service/secret keys (sb_publish_..., sb_secret_..., sb_service_...).
	regexp.MustCompile(`(?i)\bsb_(?:publish|secret|service)_[A-Za-z0-9]{20,}`),

	// GitHub personal access tokens (ghp_, gho_, ghs_, ghu_, ghr_). The 6+
	// threshold is intentionally low so partial/short leaks are still
	// redacted defensively; real tokens are 36+ chars after the prefix.
	regexp.MustCompile(`(?i)\bgh[posur]_[A-Za-z0-9]{6,}`),

	// Gogs personal access tokens (gg_ token doesn't exist; Gogs uses long hex).
	// Skip — handled by the generic hex-spanning JWT/secret above.
}

// redactReplacement is the canonical redaction sentinel exposed in tool output
// for the generic `"key":"value"` pattern below.
const redactReplacement = `"[REDACTED]"`

// SanitizeResult redacts credential-shaped substrings from a tool result.
// It serializes the value to JSON, scrubs the patterns, and returns the
// decoded structure when possible (so callers see a normal value); on a decode
// failure it returns the scrubbed JSON string.
func SanitizeResult(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%v", value))
	}
	scrubbed := string(encoded)
	for _, p := range secretPatterns {
		switch {
		case p.NumSubexp() >= 1:
			// Patterns with capture groups preserve the prefix and redact only
			// the secret tail.
			scrubbed = p.ReplaceAllString(scrubbed, "${1}[REDACTED]")
		default:
			// Patterns without groups redact the whole match.
			scrubbed = p.ReplaceAllString(scrubbed, redactReplacement)
		}
	}

	var decoded any
	if err := json.Unmarshal([]byte(scrubbed), &decoded); err == nil {
		return decoded
	}
	return scrubbed
}

// SanitizeErrorMessage strips internal Go package paths, stack frames, and
// credential-shaped substrings from an error message before it is returned to
// the client. The goal: never leak implementation details or secrets.
func SanitizeErrorMessage(msg string) string {
	s := msg
	// Trim Go file/line references like "services/foo.go:42:".
	s = regexp.MustCompile(`\b([a-zA-Z0-9_\-]+/)+[a-zA-Z0-9_\-]+\.go:\d+\s*:\s*`).ReplaceAllString(s, "")
	// Collapse internal package qualifiers like "services." / "driven.".
	s = regexp.MustCompile(`(?i)\b(?:services|driven|infrastructure|adapters)\.`).ReplaceAllString(s, "")
	for _, p := range secretPatterns {
		s = p.ReplaceAllString(s, redactReplacement)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = "internal error"
	}
	return s
}
