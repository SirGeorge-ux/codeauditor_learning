package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("", "")
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("expected default baseURL, got %s", c.baseURL)
	}
	if c.model != "qwen2.5-coder:3b" {
		t.Errorf("expected default model, got %s", c.model)
	}
}

func TestNewClient_CustomValues(t *testing.T) {
	c := NewClient("http://ollama:11434", "codellama:7b")
	if c.baseURL != "http://ollama:11434" {
		t.Errorf("expected custom baseURL, got %s", c.baseURL)
	}
	if c.model != "codellama:7b" {
		t.Errorf("expected custom model, got %s", c.model)
	}
}

func TestNewClient_TrailingSlash(t *testing.T) {
	c := NewClient("http://ollama:11434/", "test")
	if c.baseURL != "http://ollama:11434" {
		t.Errorf("expected trimmed trailing slash, got %s", c.baseURL)
	}
}

func TestGenerate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"Analysis complete","done":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-model")
	resp, err := c.Generate(context.Background(), "You are an auditor", "Analyze this code")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp != "Analysis complete" {
		t.Errorf("expected 'Analysis complete', got: %s", resp)
	}
}

func TestGenerate_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Ollama is down"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test")
	_, err := c.Generate(context.Background(), "", "test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestGenerate_Unreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:19999", "test")
	_, err := c.Generate(context.Background(), "", "test")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestStreamGenerate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"Token1","done":false}` + "\n"))
		_, _ = w.Write([]byte(`{"response":" Token2","done":false}` + "\n"))
		_, _ = w.Write([]byte(`{"response":"","done":true}` + "\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test")
	var tokens []string
	err := c.StreamGenerate(context.Background(), "", "test", func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0] != "Token1" || tokens[1] != " Token2" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

func TestStreamGenerate_EmptyTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"","done":true}` + "\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test")
	var tokens []string
	err := c.StreamGenerate(context.Background(), "", "test", func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestModel(t *testing.T) {
	c := NewClient("http://localhost", "llama3:8b")
	if c.Model() != "llama3:8b" {
		t.Errorf("expected llama3:8b, got %s", c.Model())
	}
}
