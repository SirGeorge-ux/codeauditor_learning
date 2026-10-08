package mcpkit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// Re-export the canonical auth context helpers for backward compat with
// any caller that imported them directly from this package. New code
// should import mcpctx directly.
type AuthUserIDKey = mcpctx.AuthUserIDKey

// ContextWithUserID injects the authenticated user_id into ctx so MCP
// tools can read it via mcpctx.UserIDFromContext (or UserIDFromContext
// below for backward compatibility). The HTTP bridge in `Server.Router`
// calls this when the auth middleware has already stamped the user id
// from the JWT.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return mcpctx.ContextWithUserID(ctx, userID)
}

// UserIDFromContext extracts the authenticated user id. Returns "" when
// no id is present; tools translate that to ErrInvalidParams (-32602).
func UserIDFromContext(ctx context.Context) string {
	return mcpctx.UserIDFromContext(ctx)
}

// Server is the driving HTTP adapter for the MCP monolith. It mounts three
// JSON-RPC/SSE endpoints under /mcp/ and is protected by the same JWT auth
// middleware used for /api/v1/*.
type Server struct {
	registry   *Registry
	dispatcher *Dispatcher
	limiter    *RateLimiter
}

// NewServer creates an MCP HTTP server bound to the given registry. A default
// dispatcher and rate limiter are created; override with the With* setters.
func NewServer(registry *Registry) *Server {
	return &Server{
		registry:   registry,
		dispatcher: NewDispatcher(registry),
		limiter:    NewRateLimiter(),
	}
}

// WithDispatcher injects a custom dispatcher (useful for tests).
func (s *Server) WithDispatcher(d *Dispatcher) *Server { s.dispatcher = d; return s }

// WithRateLimiter injects a custom rate limiter (useful for tests).
func (s *Server) WithRateLimiter(rl *RateLimiter) *Server { s.limiter = rl; return s }

// Router builds the chi router for the /mcp group. The auth middleware applies
// to every route; the rate limiter wraps only the tool-call route. Callers
// mount this router under "/mcp".
//
// authMW is the project's JWT auth middleware (authmiddleware.AuthMiddleware).
// In addition to its 401-on-missing-token behavior, Router wires a context
// translator that copies the authenticated user_id from
// authmiddleware.UserIDContextKey into the MCP tool's per-request context so
// tools like `mcp-dictionary.search_term` see the same user_id the HTTP
// layer verified. Without this bridge no MCP tool that requires a user_id
// (e.g. all four dictionary tools) would ever be invokable from HTTP.
func (s *Server) Router(authMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(authMW)
	r.Use(httpContextBridge)

	r.Get("/tools/list", s.handleListTools)

	// Rate limit only applies to dispatch, not discovery or SSE.
	r.With(s.limiter.Middleware).Post("/tools/call", s.handleCallTool)

	// SSE streaming for long-running tool results.
	r.Get("/events", s.handleEvents)

	return r
}

// httpContextBridge copies the authenticated user_id from the project's
// auth middleware into the MCP context used by tools that read "user id"
// from request scope. Returning the empty string is a no-op so the bridge
// does not mask the tool's own invalid-params handling.
func httpContextBridge(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := authmiddleware.GetUserID(r.Context())
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := ContextWithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// toolListItem is the public shape of a single entry in GET /mcp/tools/list.
// It mirrors the OpenAI function-calling contract the spec requires.
type toolListItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// handleListTools returns all registered tools with their OpenAI-compatible
// schemas as a JSON array, sorted by name for a stable contract.
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools := s.registry.List()

	items := make([]toolListItem, 0, len(tools))
	for _, t := range tools {
		params := any(nil)
		if sch := t.Schema(); sch.Function != nil {
			params = sch.Function.Parameters
		}
		items = append(items, toolListItem{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  params,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	writeJSON(w, http.StatusOK, items)
}

// handleCallTool parses a JSON-RPC 2.0 tools/call request and dispatches it.
// The rate-limit check already ran in the route-level middleware.
func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeRPCError(w, nil, &RPCError{Code: CodeParseError, Message: "Parse error"})
		return
	}
	resp := s.dispatcher.Dispatch(r.Context(), body)
	writeJSON(w, http.StatusOK, resp)
}

// handleEvents streams long-running tool results over SSE. For PR 1 it emits a
// ready event then heartbeats until the client disconnects, so the transport
// contract is testable before streaming tools exist. PR 2+ replaces the
// heartbeat with per-execution channels.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, _ := w.(http.Flusher)
	emitSSE(w, "message", map[string]any{"status": "ready"})
	if flusher != nil {
		flusher.Flush()
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			emitSSE(w, "ping", map[string]any{"status": "alive"})
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// emitSSE writes one SSE event line to the response writer.
func emitSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = w.Write([]byte("event: " + event + "\n"))
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
}

// writeJSON encodes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeRPCError writes a JSON-RPC 2.0 error envelope.
func writeRPCError(w http.ResponseWriter, id any, e *RPCError) {
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: "2.0", ID: id, Error: e})
}
