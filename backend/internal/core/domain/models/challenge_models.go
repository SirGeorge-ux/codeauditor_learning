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
}