package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a driven adapter for the Ollama LLM API.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// GenerateRequest mirrors Ollama /api/generate request.
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

// GenerateResponse mirrors a single Ollama /api/generate response.
type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewClient creates a new Ollama API client.
func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen2.5-coder:3b"
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Generate sends a prompt to Ollama and returns the complete response.
func (c *Client) Generate(ctx context.Context, system, prompt string) (string, error) {
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: system,
		Stream: false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("ollama marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(msg))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}

	return genResp.Response, nil
}

// StreamGenerate streams tokens from Ollama, calling onToken for each token.
func (c *Client) StreamGenerate(ctx context.Context, system, prompt string, onToken func(string) error) error {
	req := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: system,
		Stream: true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("ollama marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(msg))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var tokenResp GenerateResponse
		if err := json.Unmarshal(scanner.Bytes(), &tokenResp); err != nil {
			continue // skip malformed lines in stream
		}
		if tokenResp.Done {
			break
		}
		if tokenResp.Response != "" {
			if err := onToken(tokenResp.Response); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}
