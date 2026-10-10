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

// MiniMax is the opt-in premium tier (M3). The cascade only routes
// requests here when the user has flagged use_minimax=true on their
// LearningProfile OR set PreferredProvider=="minimax" in the
// request body. The provider reuses the OpenAI wire shape so the
// adapter is structurally identical to Groq/DeepSeek — only the
// URL and Authorization header differ.
type MiniMax struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewMiniMax returns a MiniMax provider. The default baseURL
// (api.MiniMax.ai) and model (MiniMax-M3) follow docs/llm-strategy.md;
// both are overridable.
func NewMiniMax(baseURL, apiKey, model string) *MiniMax {
	if baseURL == "" {
		baseURL = "https://api.MiniMax.ai/v1"
	}
	if model == "" {
		model = "MiniMax-M3"
	}
	return &MiniMax{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Name is the canonical provider name used by Cascade.providerOrder
// when matching PreferredProvider=="minimax".
func (m *MiniMax) Name() string { return "minimax" }

// IsDisabled reports whether the provider should be skipped.
// MiniMax is feature-flagged via the api key; an empty key
// naturally produces a "minimax off" state.
func (m *MiniMax) IsDisabled() bool { return strings.TrimSpace(m.apiKey) == "" }

// chatCompletionURL is MiniMax's chat completions endpoint.
func (m *MiniMax) chatCompletionURL() string {
	return m.baseURL + "/chat/completions"
}

// StreamChat is the providerClient implementation. Wire shape
// mirrors Groq/DeepSeek.
func (m *MiniMax) StreamChat(
	ctx context.Context,
	msgs []models.ChatMessage,
	opts ports.StreamChatOptions,
	onEvent func(ports.LLMStreamEvent) error,
) error {
	if m.IsDisabled() {
		return ErrProviderDisabled
	}
	reqMsgs := make([]openAIRequestMessage, 0, len(msgs))
	for _, msg := range msgs {
		reqMsgs = append(reqMsgs, openAIRequestMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	body, err := json.Marshal(openAIRequest{
		Model:     m.model,
		Messages:  reqMsgs,
		Stream:    true,
		MaxTokens: opts.MaxTokens,
	})
	if err != nil {
		return fmt.Errorf("minimax: marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.chatCompletionURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("minimax: new req: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := m.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("minimax: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		limited := io.LimitReader(resp.Body, 1024)
		msg, _ := io.ReadAll(limited)
		return fmt.Errorf("minimax: status %d: %s", resp.StatusCode, string(msg))
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
		return fmt.Errorf("minimax: scan: %w", err)
	}
	return nil
}
