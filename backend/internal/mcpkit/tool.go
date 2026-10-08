// Package mcpkit implements the MCP (Model Context Protocol) runtime: a
// JSON-RPC 2.0 server that exposes tool calls as driving adapters over the
// hexagonal core/services/ layer.
//
// Hexagonal contract: everything in this package imports ONLY ports, core
// services, and the standard library (plus the openai schema contract for
// tool definitions). It MUST NOT import database/sql, net/http clients, or
// external IO SDKs. HTTP wiring lives in server.go which uses the chi router
// but relies on the delegated auth middleware for user identity.
package mcpkit

import (
	"context"
	"encoding/json"

	"github.com/sashabaranov/go-openai"
)

// MCPTool is the contract every MCP tool implements. Tools are driving
// adapters: they parse JSON arguments, validate them, delegate to a
// core/services/ method, and return a JSON-serializable result.
type MCPTool interface {
	// Name returns the globally unique tool name in the form
	// "mcp-<group>.<action>" (e.g. "mcp-sandbox.run_code").
	Name() string

	// Description returns a human-readable summary including Args, Returns,
	// and Example sections for LLM function-calling context.
	Description() string

	// Schema returns an OpenAI-compatible function definition used by the
	// tutor LLM for tool routing.
	Schema() openai.Tool

	// Execute parses the raw JSON arguments, validates them, delegates to the
	// backing core service, and returns a JSON-serializable result.
	Execute(ctx context.Context, args json.RawMessage) (any, error)
}

// Compile-time guard that a minimal stub tool satisfies the interface.
var _ MCPTool = (*stubTool)(nil)

type stubTool struct {
	name string
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return "stub" }
func (s *stubTool) Schema() openai.Tool { return openai.Tool{} }

func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (any, error) { return nil, nil }
