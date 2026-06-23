package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// RacketProvider audits Racket code by executing it with the `racket` runner,
// using the official `racket/racket` image for Docker execution and the
// `racket` binary for the local healthcheck.
type RacketProvider struct{}

// NewRacketProvider returns a ports.LanguageProvider for Racket.
func NewRacketProvider() ports.LanguageProvider { return &RacketProvider{} }

func (p *RacketProvider) Language() string      { return "racket" }
func (p *RacketProvider) FileExtension() string { return ".rkt" }
func (p *RacketProvider) DockerImage() string   { return "racket/racket:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	racket /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *RacketProvider) DockerCommand(filename string) []string {
	return []string{"racket", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("racket").
func (p *RacketProvider) LocalCommand() string { return "racket" }

// InstallHint is shown when the local executable is missing.
func (p *RacketProvider) InstallHint() string {
	return "install Racket from https://racket-lang.org/ (or brew install racket)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*RacketProvider)(nil)