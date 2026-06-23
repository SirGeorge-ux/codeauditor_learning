package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// DartProvider audits Dart code by executing it with `dart run`,
// using the official `dart:latest` image for Docker execution and the
// `dart` binary for the local healthcheck.
type DartProvider struct{}

// NewDartProvider returns a ports.LanguageProvider for Dart.
func NewDartProvider() ports.LanguageProvider { return &DartProvider{} }

func (p *DartProvider) Language() string      { return "dart" }
func (p *DartProvider) FileExtension() string  { return ".dart" }
func (p *DartProvider) DockerImage() string    { return "dart:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	dart run /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *DartProvider) DockerCommand(filename string) []string {
	return []string{"dart", "run", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("dart").
func (p *DartProvider) LocalCommand() string { return "dart" }

// InstallHint is shown when the local executable is missing.
func (p *DartProvider) InstallHint() string {
	return "install Dart SDK: https://dart.dev/get-dart"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*DartProvider)(nil)