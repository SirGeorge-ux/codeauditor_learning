package models

// AuditRequest represents a request to audit code.
type AuditRequest struct {
	Code        string `json:"code"`
	Language    string `json:"language"`
	ChallengeID string `json:"challengeId"`
	UserID      string `json:"-"` // set by server from JWT, not sent by client
}

// AuditEvent represents a single event streamed during audit execution.
type AuditEvent struct {
	Type      string `json:"type"` // stdout, stderr, error, complete
	Data      string `json:"data"`
	Timestamp string `json:"timestamp"`
}

// AuditResult represents the final result of an audit execution.
type AuditResult struct {
	ExitCode int `json:"exitCode"`
}