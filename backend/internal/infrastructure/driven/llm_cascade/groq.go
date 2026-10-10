package llmcascade

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Groq is the cascade's primary provider. Per docs/llm-strategy.md
// the production model is llama-3.3-70b-versatile on Groq's free
// tier; the constructor exposes the model so a future reviewer can
// flip to the 8b-instant variant without a code-wide grep.
type Groq struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewGroq returns a Groq provider pointed at the OpenAI-compatible
// /chat/completions endpoint. An empty API key marks the provider
// as "feature-flagged off" so the cascade silently skips it instead
// of bombing every tutor request.
func NewGroq(baseURL, apiKey, model string) *Groq {
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	if model == "" {
		model = "llama-3.3-70b-versatile"
	}
	return &Groq{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Name is the canonical provider name used by Cascade.providerOrder
// when matching PreferredProvider.
func (g *Groq) Name() string { return "groq" }

// IsDisabled reports whether the provider should be skipped (no API
// key). The cascade walks past disabled providers instead of
// attempting HTTP and surfacing 401s.
func (g *Groq) IsDisabled() bool { return strings.TrimSpace(g.apiKey) == "" }

// chatCompletionURL is the OpenAI-compatible chat completions path.
// Exposed at the package boundary so a test can lock the URL shape.
func (g *Groq) chatCompletionURL() string {
	return g.baseURL + "/chat/completions"
}

// openAIRequest is the body shape Groq's OpenAI-compatible endpoint
// expects. Defined locally so the adapter doesn't import an OpenAI
// SDK (PR 2 keeps the dependency surface stdlib-only).
type openAIRequest struct {
	Model     string                 `json:"model"`
	Messages  []openAIRequestMessage `json:"messages"`
	Stream    bool                   `json:"stream"`
	MaxTokens int                    `json:"max_tokens,omitempty"`
}

type openAIRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIStreamChunk is the per-line SSE payload emitted by Groq (and
// every OpenAI-compatible provider). The Cascade pipe only needs the
// content delta, so we parse the minimum shape.
type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// StreamChat sends the messages to Groq and pipes the SSE chunks
// back as LLMStreamEvent tokens. The implementation honors
// context cancellation: if the wrapping ctx is canceled (eg. by
// the cascade's first-token timeout), the HTTP request is aborted
// and the next provider is tried.
func (g *Groq) StreamChat(
	ctx context.Context,
	msgs []models.ChatMessage,
	opts ports.StreamChatOptions,
	onEvent func(ports.LLMStreamEvent) error,
) error {
	if g.IsDisabled() {
		return ErrProviderDisabled
	}
	reqMsgs := make([]openAIRequestMessage, 0, len(msgs))
	for _, m := range msgs {
		reqMsgs = append(reqMsgs, openAIRequestMessage{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}
	body, err := json.Marshal(openAIRequest{
		Model:     g.model,
		Messages:  reqMsgs,
		Stream:    true,
		MaxTokens: opts.MaxTokens,
	})
	if err != nil {
		return fmt.Errorf("groq: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.chatCompletionURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("groq: new req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("groq: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Read a small sample so the caller can log server hints.
		limited := io.LimitReader(resp.Body, 1024)
		msg, _ := io.ReadAll(limited)
		return fmt.Errorf("groq: status %d: %s", resp.StatusCode, string(msg))
	}

	// Groq sends SSE: "data: {...}\n\n" per chunk, terminated by
	// "data: [DONE]". Scan lines and forward each delta as a token.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			if onEvent != nil {
				if err := onEvent(ports.LLMStreamEvent{Type: "done"}); err != nil {
					return err
				}
			}
			break
		}
		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content == "" {
				continue
			}
			if onEvent != nil {
				if err := onEvent(ports.LLMStreamEvent{Type: "token", Content: ch.Delta.Content}); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("groq: scan: %w", err)
	}
	return nil
}
