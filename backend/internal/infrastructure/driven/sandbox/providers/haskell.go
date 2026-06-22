package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// HaskellProvider audits Haskell code by executing it with the `runhaskell`
// runner, using the official `haskell:latest` image for Docker execution and
// the `runhaskell` binary for the local healthcheck.
type HaskellProvider struct{}

// NewHaskellProvider returns a ports.LanguageProvider for Haskell.
func NewHaskellProvider() ports.LanguageProvider { return &HaskellProvider{} }

func (p *HaskellProvider) Language() string      { return "haskell" }
func (p *HaskellProvider) FileExtension() string { return ".hs" }
func (p *HaskellProvider) DockerImage() string   { return "haskell:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	runhaskell /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *HaskellProvider) DockerCommand(filename string) []string {
	return []string{"runhaskell", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("runhaskell").
func (p *HaskellProvider) LocalCommand() string { return "runhaskell" }

// InstallHint is shown when the local executable is missing.
func (p *HaskellProvider) InstallHint() string {
	return "install GHC/Haskell: apt install ghc (or brew install ghc)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*HaskellProvider)(nil)
