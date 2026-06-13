package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// AuditHandler handles the audit SSE endpoint.
type AuditHandler struct {
	auditService *services.AuditService
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditService *services.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// HandleSSE handles POST /api/v1/audit — streams audit results as SSE.
func (h *AuditHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush to establish SSE connection early
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Parse request body
	var req models.AuditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"data\":\"Invalid request\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	// Extract user ID from JWT context (set by auth middleware)
	req.UserID = authmiddleware.GetUserID(r.Context())

	// Create SSE writer (satisfies SSEStreamer port)
	sseWriter := NewSSEWriter(w, r)

	clientID := fmt.Sprintf("audit-%d", time.Now().UnixNano())
	h.auditService.RunAudit(r.Context(), req, sseWriter, clientID)
}