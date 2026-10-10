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

// DeepSeek is the OpenRouter-routed fallback. The provider is
// surface-compatible with Groq (OpenAI Chat Completions wire) but
// routed via OpenRouter's `Authorization: Bearer $OPENROUTER_API_KEY`
// header. The model defaults to deepseek/deepseek-chat (free tier)
// and is overridable through NewDeepSeek for future swaps.
type DeepSeek struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewDeepSeek returns a DeepSeek provider. Per docs/llm-strategy.md
// the production model is deepseek-chat via OpenRouter's free tier;
// the constructor exposes the model so a future reviewer can flip
// to deepseek-coder without a code-wide grep.
func NewDeepSeek(baseURL, apiKey, model string) *DeepSeek {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if model == "" {
		model = "deepseek/deepseek-chat"
	}
	return &DeepSeek{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 12 * time.Second},
	}
}

// Name is the canonical provider name used by Cascade.providerOrder.
// Matches the "deepseek" tag the frontend sets in PreferredProvider.
func (d *DeepSeek) Name() string { return "deepseek" }

// IsDisabled reports whether the provider should be skipped.
func (d *DeepSeek) IsDisabled() bool { return strings.TrimSpace(d.apiKey) == "" }

// chatCompletionURL is OpenRouter's chat completions path.
func (d *DeepSeek) chatCompletionURL() string {
	return d.baseURL + "/chat/completions"
}

// StreamChat is the providerClient implementation. Body shape mirrors
// Groq — OpenRouter speaks the same OpenAI wire format.
func (d *DeepSeek) StreamChat(
	ctx context.Context,
	msgs []models.ChatMessage,
	opts ports.StreamChatOptions,
	onEvent func(ports.LLMStreamEvent) error,
) error {
	if d.IsDisabled() {
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
		Model:     d.model,
		Messages:  reqMsgs,
		Stream:    true,
		MaxTokens: opts.MaxTokens,
	})
	if err != nil {
		return fmt.Errorf("deepseek: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.chatCompletionURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("deepseek: new req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	// OpenRouter recommends a referer/title so its dashboard can
	// attribute traffic — surfacing the app name keeps the
	// integration polite.
	httpReq.Header.Set("X-Title", "codeauditor-tutor")

	resp, err := d.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("deepseek: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		limited := io.LimitReader(resp.Body, 1024)
		msg, _ := io.ReadAll(limited)
		return fmt.Errorf("deepseek: status %d: %s", resp.StatusCode, string(msg))
	}

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
		return fmt.Errorf("deepseek: scan: %w", err)
	}
	return nil
}
