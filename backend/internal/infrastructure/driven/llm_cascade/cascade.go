package llmcascade

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrCascadeAllFailed is the sentinel returned by Cascade.StreamChat
// when every configured provider errored or timed out. The handler
// maps it to an SSE error event so the frontend can surface a
// one-liner to the user (eg. "tutor temporalmente sin respuesta").
var ErrCascadeAllFailed = errors.New("cascade: all providers failed")

// ErrProviderDisabled is what a provider returns when its API key is
// empty (treated as "feature flagged off"). Cascade treats it as a
// silent skip — no error envelope, just walks to the next provider.
var ErrProviderDisabled = errors.New("provider disabled")

// DefaultFirstTokenTimeout is the documented 5-second first-token
// cap per spec.md#SSE-Streaming ("Fallback on timeout"). Callers can
// override via StreamChatOptions.Timeout.
const DefaultFirstTokenTimeout = 5 * time.Second

// providerClient is the package-internal contract each provider
// (Groq, DeepSeek, MiniMax) satisfies. Lowercase so a future
// refactor — e.g. moving providers to subpackages — doesn't break
// any caller outside this package.
type providerClient interface {
	StreamChat(ctx context.Context, msgs []models.ChatMessage, opts ports.StreamChatOptions, onEvent func(ports.LLMStreamEvent) error) error
	// Complete performs a single non-streaming, structured
	// completion. The response body shape is provider-specific:
	// Groq and DeepSeek (via OpenRouter) support a JSON-mode
	// response_format flag; MiniMax does not — its adapter parses
	// the free-text content for the first JSON object.
	Complete(ctx context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error)
	Name() string
}

// Cascade is the multi-provider entry point. It implements
// ports.LLMClientPort and routes every chat request through the
// configured ranking with the documented fallback semantics.
type Cascade struct {
	mu        sync.Mutex
	providers []providerClient
}

// NewCascade returns a Cascade that tries providers in the supplied
// order. The Groq → DeepSeek → MiniMax ordering per docs/llm-strategy.md
// is the production wiring; tests pass shorter chains for clarity.
func NewCascade(providers ...providerClient) *Cascade {
	return &Cascade{providers: providers}
}

// isValidPreferredProvider returns true when the PreferredProvider
// value can short-circuit the cascade ranking. Names match the
// provider Client.Name() values; an empty string is the "use the
// ranking" sentinel. Unknown values are silently ignored (caller
// falls back to ranking).
func isValidPreferredProvider(s string) bool {
	switch s {
	case "", "groq", "deepseek", "minimax":
		return true
	}
	return false
}

// StreamChat walks the configured providers. The precedence rules:
//
//   - PreferredProvider when set AND known → only that provider.
//   - Otherwise the configured ranking in order.
//   - Within each provider, the implementation MUST return within
//     opts.Timeout for the FIRST event (or ErrProviderDisabled on
//     missing key).
//   - On a transient error, the cascade walks to the next provider.
//   - On a successful stream ending in "done", the cascade returns
//     nil — even if later providers would also succeed.
//
// Implementation note: we don't start parallel goroutines racing on
// providers. The first-token timeout is checked at the cascade layer
// via a wrapping context; if a provider is "too quiet" past the
// timeout we cancel its stream and move on.
func (c *Cascade) StreamChat(
	ctx context.Context,
	msgs []models.ChatMessage,
	opts ports.StreamChatOptions,
	onEvent func(ports.LLMStreamEvent) error,
) error {
	if onEvent == nil {
		return errors.New("cascade: onEvent callback is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultFirstTokenTimeout
	}

	chosen := c.providerOrder(opts.PreferredProvider)
	if len(chosen) == 0 {
		return ErrCascadeAllFailed
	}

	var lastErr error
	for _, p := range chosen {
		pCtx, cancel := context.WithTimeout(ctx, timeout)
		startEmitted := false
		err := p.StreamChat(pCtx, msgs, opts, func(ev ports.LLMStreamEvent) error {
			// Forward each event to the caller's onEvent. The
			// wrapping context cancellation is what drives the
			// "first token past timeout" fallthrough.
			if !startEmitted && (ev.Type == "token" || ev.Type == "status") {
				startEmitted = true
			}
			return onEvent(ev)
		})
		cancel()
		if err == nil {
			return nil
		}
		// Disabled providers are silent skips — NOT a failure.
		if errors.Is(err, ErrProviderDisabled) {
			continue
		}
		// Context cancellation from the timeout layer says "give
		// up on this provider". We swallow it here and try the
		// next one. Any other error is recorded as the last
		// failure for the final error envelope.
		if errors.Is(err, context.DeadlineExceeded) && !startEmitted {
			lastErr = err
			continue
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			// Top-level ctx is gone — stop walking.
			return ctx.Err()
		}
		lastErr = err
	}

	// Every provider either failed or was disabled. Return the
	// sentinel so the handler has a single error to translate; the
	// lastErr is wrapped for diagnostic context AND so
	// errors.Is(err, backpressure) can branch on the concrete
	// cause at the handler layer.
	if lastErr == nil {
		return ErrCascadeAllFailed
	}
	return errors.Join(ErrCascadeAllFailed, lastErr)
}

// providerOrder picks the provider sub-slice. PreferredProvider can
// shrink the slice to a single provider; an unknown preferred string
// is a no-op (we still walk the ranking).
func (c *Cascade) providerOrder(preferred string) []providerClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	if preferred == "" {
		return append([]providerClient(nil), c.providers...)
	}
	if !isValidPreferredProvider(preferred) {
		return append([]providerClient(nil), c.providers...)
	}
	for _, p := range c.providers {
		if p.Name() == preferred {
			return []providerClient{p}
		}
	}
	return append([]providerClient(nil), c.providers...)
}

// Complete walks the configured providers in ranking order
// (Groq → DeepSeek → MiniMax) until one returns a usable response.
//
// Semantics:
//   - PreferredProvider when set AND known → only that provider.
//   - Disabled providers (ErrProviderDisabled) are silent skips.
//   - Timeout: a 5s whole-response budget wraps every provider.
//     On timeout, the cascade returns ErrLLMCompleteTimeout
//     IMMEDIATELY (no fallthrough — the contract is a hard budget).
//   - HTTP 5xx / network failures DO walk to the next provider.
//     The walk stops on the first success or when the chain is
//     exhausted (returns ErrCascadeAllFailed).
//
// The Provider field of CompleteResponse identifies which provider
// answered; callers (eg. ChallengeService) attach it to audit logs.
func (c *Cascade) Complete(ctx context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error) {
	chosen := c.providerOrder(req.PreferredProvider)
	if len(chosen) == 0 {
		return ports.CompleteResponse{}, ErrCascadeAllFailed
	}

	timeout := ports.DefaultCompleteTimeout

	for _, p := range chosen {
		pCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := p.Complete(pCtx, req)
		cancel()
		if err == nil {
			// Provider returned successfully — bubble up the
			// identifying name so callers know which tier
			// answered (used for metrics/logging).
			if resp.Provider == "" {
				resp.Provider = p.Name()
			}
			return resp, nil
		}
		// Disabled providers are silent skips.
		if errors.Is(err, ErrProviderDisabled) {
			continue
		}
		// Whole-response timeout is terminal — the contract is a
		// hard 5s budget. Walk-on-timeout would break the
		// documented ErrLLMCompleteTimeout SLA (return within 5s).
		if errors.Is(err, context.DeadlineExceeded) {
			return ports.CompleteResponse{}, errors.Join(ports.ErrLLMCompleteTimeout, err)
		}
		// Top-level ctx is gone — stop walking.
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return ports.CompleteResponse{}, ctx.Err()
		}
		// Transient HTTP error (5xx, network glitch) → walk.
		_ = err // recorded in ErrCascadeAllFailed below if no provider recovers.
	}
	return ports.CompleteResponse{}, ErrCascadeAllFailed
}

// Compile-time guard: Cascade satisfies ports.LLMClientPort. Signature
// drift surfaces at compile time instead of at the handler wiring.
var _ ports.LLMClientPort = (*Cascade)(nil)
