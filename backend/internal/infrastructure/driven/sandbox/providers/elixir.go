package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ElixirProvider audits Elixir code by executing it with the `elixir` runner,
// using the `elixir:alpine` image for Docker execution and the `elixir` binary
// for the local healthcheck.
type ElixirProvider struct{}

// NewElixirProvider returns a ports.LanguageProvider for Elixir.
func NewElixirProvider() ports.LanguageProvider { return &ElixirProvider{} }

func (p *ElixirProvider) Language() string      { return "elixir" }
func (p *ElixirProvider) FileExtension() string { return ".exs" }
func (p *ElixirProvider) DockerImage() string   { return "elixir:alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	elixir /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *ElixirProvider) DockerCommand(filename string) []string {
	return []string{"elixir", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("elixir").
func (p *ElixirProvider) LocalCommand() string { return "elixir" }

// InstallHint is shown when the local executable is missing.
func (p *ElixirProvider) InstallHint() string {
	return "install Elixir from https://elixir-lang.org/install.html (or brew install elixir)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ElixirProvider)(nil)
