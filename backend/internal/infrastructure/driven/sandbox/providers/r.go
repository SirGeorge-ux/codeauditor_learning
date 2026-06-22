package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// RProvider audits R code by executing it with `Rscript`, using the official
// `r-base:latest` image for Docker execution and the `Rscript` binary for the
// local healthcheck.
type RProvider struct{}

// NewRProvider returns a ports.LanguageProvider for R.
func NewRProvider() ports.LanguageProvider { return &RProvider{} }

func (p *RProvider) Language() string      { return "r" }
func (p *RProvider) FileExtension() string { return ".r" }
func (p *RProvider) DockerImage() string   { return "r-base:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	Rscript /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *RProvider) DockerCommand(filename string) []string {
	return []string{"Rscript", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("Rscript").
func (p *RProvider) LocalCommand() string { return "Rscript" }

// InstallHint is shown when the local executable is missing.
func (p *RProvider) InstallHint() string {
	return "install R: apt install r-base (or brew install r)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*RProvider)(nil)
