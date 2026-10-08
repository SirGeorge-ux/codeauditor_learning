// Package mcpctx owns the canonical context key + helpers that bridge the
// HTTP auth context (stamped by authmiddleware.AuthMiddleware) into MCP
// tool request scopes. Defining it as its own leaf package avoids a
// circular import otherwise between mcpkit.Server and the dictionary tool
// group: both depend on mcpctx, neither depends on the other.
package mcpctx

import (
	"context"
	"errors"
)

// AuthUserIDKey is the canonical, package-private context key.
type AuthUserIDKey struct{}

// ErrInvalidParams is the shared sentinel returned by MCP tools when
// arguments or context fail validation; the dispatcher maps it to the
// JSON-RPC 2.0 -32602 code. Both mcpkit.Server (the dispatcher) and the
// dictionary tool group import this so `errors.Is` matches across the
// boundary.
var ErrInvalidParams = errors.New("invalid params (-32602)")

// ContextWithUserID returns a child context carrying the authenticated
// user id under AuthUserIDKey.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, AuthUserIDKey{}, userID)
}

// UserIDFromContext extracts the user id; "" when none stamped.
func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(AuthUserIDKey{}).(string); ok {
		return v
	}
	return ""
}
