package ports

import "context"

// OllamaClient defines the contract for streaming code analysis from a
// local LLM endpoint (Ollama). It is a driven (secondary) port — the
// application use cases call it to push AI-generated tokens to the
// client during the audit flow.
//
// Implementations live in internal/infrastructure/driven/ollama. The
// production wiring (cmd/api/main.go) injects the concrete adapter
// as this interface so core/services never imports infrastructure.
//
// This port exposes ONLY the surface that core currently uses. The
// concrete adapter may expose more methods (e.g. non-streaming Generate,
// Model()) — those remain adapter-internal and are not part of the
// hexagonal contract.
type OllamaClient interface {
	// StreamGenerate streams tokens from the LLM, calling onToken for each
	// emitted token. The callback returns an error if the consumer (e.g.
	// the SSE streamer) fails to deliver the token; StreamGenerate must
	// propagate that error so the audit flow can react.
	StreamGenerate(ctx context.Context, system, prompt string, onToken func(string) error) error
}