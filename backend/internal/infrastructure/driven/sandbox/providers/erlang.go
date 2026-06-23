package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErlangProvider audits Erlang code by executing it with `escript`,
// using the official `erlang:latest` image for Docker execution and the
// `escript` binary for the local healthcheck.
type ErlangProvider struct{}

// NewErlangProvider returns a ports.LanguageProvider for Erlang.
func NewErlangProvider() ports.LanguageProvider { return &ErlangProvider{} }

func (p *ErlangProvider) Language() string      { return "erlang" }
func (p *ErlangProvider) FileExtension() string  { return ".erl" }
func (p *ErlangProvider) DockerImage() string    { return "erlang:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	escript /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *ErlangProvider) DockerCommand(filename string) []string {
	return []string{"escript", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("escript").
func (p *ErlangProvider) LocalCommand() string { return "escript" }

// InstallHint is shown when the local executable is missing.
func (p *ErlangProvider) InstallHint() string {
	return "install Erlang/OTP: https://www.erlang.org/downloads"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ErlangProvider)(nil)