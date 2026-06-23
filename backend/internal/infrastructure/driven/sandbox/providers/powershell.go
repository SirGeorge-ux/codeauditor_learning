package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// PowerShellProvider audits PowerShell code by executing it with the `pwsh`
// runner, using the official `mcr.microsoft.com/powershell` image for Docker
// execution and the `pwsh` binary for the local healthcheck.
type PowerShellProvider struct{}

// NewPowerShellProvider returns a ports.LanguageProvider for PowerShell.
func NewPowerShellProvider() ports.LanguageProvider { return &PowerShellProvider{} }

func (p *PowerShellProvider) Language() string      { return "powershell" }
func (p *PowerShellProvider) FileExtension() string { return ".ps1" }
func (p *PowerShellProvider) DockerImage() string  { return "mcr.microsoft.com/powershell:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	pwsh -File /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *PowerShellProvider) DockerCommand(filename string) []string {
	return []string{"pwsh", "-File", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("pwsh").
func (p *PowerShellProvider) LocalCommand() string { return "pwsh" }

// InstallHint is shown when the local executable is missing.
func (p *PowerShellProvider) InstallHint() string {
	return "install PowerShell: https://github.com/PowerShell/PowerShell (or brew install powershell/tap/powershell)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*PowerShellProvider)(nil)