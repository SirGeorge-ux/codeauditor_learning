package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// VaultHandler handles vault/history HTTP endpoints.
type VaultHandler struct {
	historyService *services.AuditHistoryService
}

// NewVaultHandler creates a new VaultHandler.
func NewVaultHandler(historyService *services.AuditHistoryService) *VaultHandler {
	return &VaultHandler{historyService: historyService}
}

// GetHistory handles GET /api/v1/audit/history — returns past audit sessions.
func (h *VaultHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.historyService.GetHistory(r.Context(), userID, 20)
	if err != nil {
		http.Error(w, "Failed to load history", http.StatusInternalServerError)
		return
	}

	if sessions == nil {
		sessions = []services.AuditSession{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// GetStats handles GET /api/v1/audit/stats — returns aggregated audit stats.
func (h *VaultHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	totalAudits, totalFindings, err := h.historyService.GetStats(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"total_audits":   totalAudits,
		"total_findings": totalFindings,
	})
}
