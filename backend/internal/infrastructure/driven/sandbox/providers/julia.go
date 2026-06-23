package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// JuliaProvider audits Julia code by executing it with `julia`,
// using the official `julia:latest` image for Docker execution and the
// `julia` binary for the local healthcheck.
type JuliaProvider struct{}

// NewJuliaProvider returns a ports.LanguageProvider for Julia.
func NewJuliaProvider() ports.LanguageProvider { return &JuliaProvider{} }

func (p *JuliaProvider) Language() string      { return "julia" }
func (p *JuliaProvider) FileExtension() string  { return ".jl" }
func (p *JuliaProvider) DockerImage() string    { return "julia:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	julia /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *JuliaProvider) DockerCommand(filename string) []string {
	return []string{"julia", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("julia").
func (p *JuliaProvider) LocalCommand() string { return "julia" }

// InstallHint is shown when the local executable is missing.
func (p *JuliaProvider) InstallHint() string {
	return "install Julia: https://julialang.org/downloads/"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*JuliaProvider)(nil)