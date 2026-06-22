package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// SwiftProvider audits Swift code by executing it with the `swift` runner,
// using the official `swift:latest` image for Docker execution and the `swift`
// binary for the local healthcheck.
type SwiftProvider struct{}

// NewSwiftProvider returns a ports.LanguageProvider for Swift.
func NewSwiftProvider() ports.LanguageProvider { return &SwiftProvider{} }

func (p *SwiftProvider) Language() string      { return "swift" }
func (p *SwiftProvider) FileExtension() string { return ".swift" }
func (p *SwiftProvider) DockerImage() string   { return "swift:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	swift /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *SwiftProvider) DockerCommand(filename string) []string {
	return []string{"swift", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("swift").
func (p *SwiftProvider) LocalCommand() string { return "swift" }

// InstallHint is shown when the local executable is missing.
func (p *SwiftProvider) InstallHint() string {
	return "install Swift from https://www.swift.org/download/ (or apt/brew install swift)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*SwiftProvider)(nil)
