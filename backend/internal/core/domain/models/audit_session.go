package models

import "time"

// AuditSession represents a single row in public.audit_sessions — the
// durable record of one completed audit run. The vault HTTP handler
// returns a list of these for the authenticated user, and the
// mcp-user.get_history / record_audit_attempt tools expose the same
// shape to the LLM tutor.
//
// The type lives in domain/models (not in the service or the port) so
// both the port and the service can import it without creating a
// package cycle, and so the MCP transport adapter can refer to a stable
// domain shape without reaching into core/services.
type AuditSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ChallengeTitle string    `json:"challenge_title"`
	Language       string    `json:"language"`
	CodeSnippet    string    `json:"code_snippet"`
	FindingsCount  int       `json:"findings_count"`
	CreatedAt      time.Time `json:"created_at"`
}
