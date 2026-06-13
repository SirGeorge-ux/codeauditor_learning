package services

import (
	"bufio"
	"context"
	"encoding/json"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// AuditService orchestrates sandbox execution and SSE streaming.
type AuditService struct {
	sandbox ports.SandboxExecutor
}

// NewAuditService creates a new AuditService.
func NewAuditService(sandbox ports.SandboxExecutor) *AuditService {
	return &AuditService{sandbox: sandbox}
}

// RunAudit executes the audit and streams results to the client via SSE.
func (s *AuditService) RunAudit(ctx context.Context, req models.AuditRequest, streamer ports.SSEStreamer, clientID string) error {
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

	result := models.AuditResult{ExitCode: 0}
	return streamer.StreamEvent(ctx, clientID, "complete", result)
}