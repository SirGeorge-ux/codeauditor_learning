package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	ollamadriven "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/ollama"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// AuditService orchestrates sandbox execution, Ollama analysis, and SSE streaming.
type AuditService struct {
	sandbox      ports.SandboxExecutor
	ollamaClient *ollamadriven.Client
	progress     *UserProgressService
}

// NewAuditService creates a new AuditService.
func NewAuditService(sandbox ports.SandboxExecutor) *AuditService {
	return &AuditService{sandbox: sandbox}
}

// WithOllama attaches an Ollama client for AI-powered code analysis.
func (s *AuditService) WithOllama(client *ollamadriven.Client) *AuditService {
	s.ollamaClient = client
	return s
}

// WithProgress attaches a UserProgressService for tracking user stats.
func (s *AuditService) WithProgress(p *UserProgressService) *AuditService {
	s.progress = p
	return s
}

// RunAudit executes the audit and streams results via SSE.
func (s *AuditService) RunAudit(ctx context.Context, req models.AuditRequest, streamer ports.SSEStreamer, clientID string) error {
	var output strings.Builder

	reader, err := s.sandbox.Execute(ctx, req.Language, req.Code, 30)
	if err != nil {
		payload := map[string]string{"message": err.Error()}
		data, _ := json.Marshal(payload)
		event := models.AuditEvent{
			Type:      "error",
			Data:      string(data),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		return streamer.StreamEvent(ctx, clientID, "error", event)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")
		payload := map[string]string{"data": line}
		data, _ := json.Marshal(payload)
		event := models.AuditEvent{
			Type:      "stdout",
			Data:      string(data),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		if err := streamer.StreamEvent(ctx, clientID, "stdout", event); err != nil {
			return err
		}
	}

	// Ollama analysis (optional — only if client is configured)
	if s.ollamaClient != nil {
		s.runOllamaAnalysis(ctx, req, output.String(), streamer, clientID)
	}

	// Record user progress (optional — only if service is configured)
	if s.progress != nil && req.UserID != "" {
		if err := s.progress.RecordAuditAttempt(ctx, req.UserID); err != nil {
			// Log but don't fail the audit — progress tracking is non-critical
			log.Printf("Failed to record progress for user %s: %v", req.UserID, err)
		}
	}

	result := models.AuditResult{ExitCode: 0}
	return streamer.StreamEvent(ctx, clientID, "complete", result)
}

// runOllamaAnalysis sends code + lint output to Ollama and streams the response.
func (s *AuditService) runOllamaAnalysis(ctx context.Context, req models.AuditRequest, toolOutput string, streamer ports.SSEStreamer, clientID string) {
	system := "You are a senior code auditor. Analyze code for security vulnerabilities, bugs, performance issues, and code smells. Be concise and specific. Use Spanish when the code or context suggests it, otherwise use English."

	prompt := fmt.Sprintf(
		"Analyze this %s code for security issues, bugs, and code smells.\n\n"+
			"```%s\n%s\n```\n\n"+
			"Tool output:\n%s\n\n"+
			"Provide a concise analysis: what issues did you find? How would you fix them?",
		req.Language, req.Language, req.Code, toolOutput,
	)

	var analysis strings.Builder

	err := s.ollamaClient.StreamGenerate(ctx, system, prompt, func(token string) error {
		analysis.WriteString(token)
		return streamer.BroadcastLLMTokens(ctx, clientID, token)
	})

	if err != nil {
		payload := map[string]string{"message": fmt.Sprintf("LLM analysis failed: %v", err)}
		data, _ := json.Marshal(payload)
		streamer.StreamEvent(ctx, clientID, "llm_error", models.AuditEvent{
			Type:      "llm_error",
			Data:      string(data),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if analysis.Len() > 0 {
		streamer.StreamEvent(ctx, clientID, "llm_analysis", models.AuditEvent{
			Type:      "llm_analysis",
			Data:      analysis.String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}
