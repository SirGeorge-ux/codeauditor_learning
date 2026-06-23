package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// FSharpProvider audits F# code by executing it with the `dotnet fsi`
// interactive runner, using the official `mcr.microsoft.com/dotnet/sdk` image
// for Docker execution and the `dotnet` binary for the local healthcheck.
type FSharpProvider struct{}

// NewFSharpProvider returns a ports.LanguageProvider for F#.
func NewFSharpProvider() ports.LanguageProvider { return &FSharpProvider{} }

func (p *FSharpProvider) Language() string      { return "fsharp" }
func (p *FSharpProvider) FileExtension() string { return ".fs" }
func (p *FSharpProvider) DockerImage() string   { return "mcr.microsoft.com/dotnet/sdk:8.0-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	dotnet fsi /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *FSharpProvider) DockerCommand(filename string) []string {
	return []string{"dotnet", "fsi", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("dotnet").
func (p *FSharpProvider) LocalCommand() string { return "dotnet" }

// InstallHint is shown when the local executable is missing.
func (p *FSharpProvider) InstallHint() string {
	return "install .NET SDK from https://dotnet.microsoft.com/download (or apt/brew install dotnet-sdk)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*FSharpProvider)(nil)