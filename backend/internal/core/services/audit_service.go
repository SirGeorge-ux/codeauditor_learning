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
	sandbox          ports.SandboxExecutor
	ollamaClient     *ollamadriven.Client
	progress         *UserProgressService
	history          *AuditHistoryService
	challengeService *ChallengeService
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

// WithHistory attaches an AuditHistoryService for saving session history.
func (s *AuditService) WithHistory(h *AuditHistoryService) *AuditService {
	s.history = h
	return s
}

// WithChallengeService attaches a ChallengeService so that AuditService can
// look up challenge learning objectives and base points when recording audit
// completions via UserProgressService.
func (s *AuditService) WithChallengeService(cs *ChallengeService) *AuditService {
	s.challengeService = cs
	return s
}

// RunAudit executes the audit and streams results via SSE.
func (s *AuditService) RunAudit(ctx context.Context, req models.AuditRequest, streamer ports.SSEStreamer, clientID string) error {
	var output strings.Builder

	// recordProgress records the audit attempt against the user's per-language
	// progress. completed mirrors the spec's AuditSession.status: true ==
	// "completed" (full scoring path, anti-gaming dedup, topics); false ==
	// "failed" (challenges_intentados only). It is a no-op when the progress
	// service is unconfigured or the request is not tied to a specific challenge.
	recordProgress := func(completed bool) {
		if s.progress == nil || req.UserID == "" || req.ChallengeID == "" {
			return
		}
		var score int
		var learningObjectives []string
		// Only look up challenge metadata for successful audits; the failed
		// path ignores score and learningObjectives entirely.
		if completed && s.challengeService != nil {
			challenge, err := s.challengeService.GetByID(ctx, req.ChallengeID, req.UserID)
			if err != nil {
				log.Printf("Failed to look up challenge %s for progress: %v", req.ChallengeID, err)
			} else {
				score = challenge.BasePoints
				learningObjectives = challenge.LearningObjectives
			}
		}
		if err := s.progress.RecordAuditCompletion(ctx, req.UserID, req.Language, req.ChallengeID, score, learningObjectives, completed); err != nil {
			log.Printf("Failed to record progress for user %s: %v", req.UserID, err)
		}
	}

	reader, err := s.sandbox.Execute(ctx, req.Language, req.Code, 30)
	if err != nil {
		// The sandbox could not run the audit — record the attempt as a
		// failure (challenges_intentados only) before signaling the client.
		recordProgress(false)
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

	// Record user progress (optional — only if service is configured and
	// the audit is tied to a specific challenge). On the happy path the
	// audit completed successfully, so the full scoring path applies.
	recordProgress(true)

	// Save to audit history (optional)
	if s.history != nil && req.UserID != "" {
		findingsCount := strings.Count(output.String(), "\n")
		if err := s.history.SaveSession(ctx, req.UserID, req, findingsCount); err != nil {
			log.Printf("Failed to save audit session for user %s: %v", req.UserID, err)
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
		_ = streamer.StreamEvent(ctx, clientID, "llm_error", models.AuditEvent{
			Type:      "llm_error",
			Data:      string(data),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if analysis.Len() > 0 {
		_ = streamer.StreamEvent(ctx, clientID, "llm_analysis", models.AuditEvent{
			Type:      "llm_analysis",
			Data:      analysis.String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}
