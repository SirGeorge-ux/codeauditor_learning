package models

import "time"

// Challenge represents a code-audit challenge.
type Challenge struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Difficulty  string    `json:"difficulty"`
	Category    string    `json:"category"`
	Language    string    `json:"language"`
	RepoURL     string    `json:"repoUrl"`
	Code        string    `json:"code"`
	CodeSmell   string    `json:"codeSmell"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UserID      *string   `json:"userId,omitempty"`
	SourceRepo  string    `json:"sourceRepo,omitempty"`
}

// CreateChallengeInput is the request body for creating a challenge.
type CreateChallengeInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Difficulty  string `json:"difficulty"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	RepoURL     string `json:"repoUrl"`
	SourceRepo  string `json:"sourceRepo"`
	Code        string `json:"code"`
	CodeSmell   string `json:"codeSmell"`
}