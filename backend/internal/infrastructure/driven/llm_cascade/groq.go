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

// Complete is the structured-JSON-mode entry point used by
// LLMClientPort.Complete (S3 PR 1). When req.JSONSchema is set, the
// request body carries `response_format: {type:"json_object"}` so
// Groq enforces a parseable JSON object back; when nil, we omit the
// flag (free text).
//
// The wire shape mirrors StreamChat (OpenAI-compatible) but with
// stream=false and the response_format toggle. The returned
// CompleteResponse carries the parseable JSON content (or the raw
// text body in text mode) plus the canonical provider name and
// observed latency.
func (g *Groq) Complete(ctx context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error) {
	if g.IsDisabled() {
		return ports.CompleteResponse{}, ErrProviderDisabled
	}
	type groqResponseFormat struct {
		Type string `json:"type"`
	}
	type groqRequest struct {
		Model          string                 `json:"model"`
		Messages       []openAIRequestMessage `json:"messages"`
		Stream         bool                   `json:"stream,omitempty"`
		ResponseFormat *groqResponseFormat    `json:"response_format,omitempty"`
	}
	msgs := []openAIRequestMessage{
		{Role: "system", Content: req.System},
		{Role: "user", Content: req.Prompt},
	}
	body, err := json.Marshal(groqRequest{
		Model:    g.model,
		Messages: msgs,
		Stream:   false,
	})
	if err != nil {
		return ports.CompleteResponse{}, fmt.Errorf("groq: marshal: %w", err)
	}
	if req.JSONSchema != nil {
		// Re-marshal with response_format included. Done in two
		// steps to keep the struct concise (the body is the
		// same as StreamChat's plus one optional field).
		body, err = json.Marshal(groqRequest{
			Model:          g.model,
			Messages:       msgs,
			Stream:         false,
			ResponseFormat: &groqResponseFormat{Type: "json_object"},
		})
		if err != nil {
			return ports.CompleteResponse{}, fmt.Errorf("groq: marshal json mode: %w", err)
		}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.chatCompletionURL(), bytes.NewReader(body))
	if err != nil {
		return ports.CompleteResponse{}, fmt.Errorf("groq: new req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	start := time.Now()
	resp, err := g.http.Do(httpReq)
	if err != nil {
		return ports.CompleteResponse{}, fmt.Errorf("groq: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		limited := io.LimitReader(resp.Body, 1024)
		msg, _ := io.ReadAll(limited)
		return ports.CompleteResponse{}, fmt.Errorf("groq: status %d: %s", resp.StatusCode, string(msg))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return ports.CompleteResponse{}, fmt.Errorf("groq: decode: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return ports.CompleteResponse{}, fmt.Errorf("groq: no choices in response")
	}
	return ports.CompleteResponse{
		Content:  parsed.Choices[0].Message.Content,
		Provider: g.Name(),
		Latency:  time.Since(start),
	}, nil
}

// openAIRequest is the body shape Groq's OpenAI-compatible endpoint
// expects. Defined locally so the adapter doesn't import an OpenAI
// SDK (PR 2 keeps the dependency surface stdlib-only).
//
// Stream uses `omitempty` so non-streaming Complete requests omit
// the key entirely (OpenAI defaults to non-streaming); streaming
// requests emit `"stream":true`.
type openAIRequest struct {
	Model     string                 `json:"model"`
	Messages  []openAIRequestMessage `json:"messages"`
	Stream    bool                   `json:"stream,omitempty"`
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
