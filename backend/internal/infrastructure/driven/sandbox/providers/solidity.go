package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// SolidityProvider audits Solidity code by compiling it with `solc`,
// using the `ethereum/solc:stable` image for Docker execution and the
// `solc` binary for the local healthcheck.
type SolidityProvider struct{}

// NewSolidityProvider returns a ports.LanguageProvider for Solidity.
func NewSolidityProvider() ports.LanguageProvider { return &SolidityProvider{} }

func (p *SolidityProvider) Language() string      { return "solidity" }
func (p *SolidityProvider) FileExtension() string  { return ".sol" }
func (p *SolidityProvider) DockerImage() string    { return "ethereum/solc:stable" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	solc /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *SolidityProvider) DockerCommand(filename string) []string {
	return []string{"solc", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("solc").
func (p *SolidityProvider) LocalCommand() string { return "solc" }

// InstallHint is shown when the local executable is missing.
func (p *SolidityProvider) InstallHint() string {
	return "install Solidity compiler: https://docs.soliditylang.org/en/latest/installing-solidity.html"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*SolidityProvider)(nil)