package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// GoProvider audits Go code with `go vet`, mirroring the previous inline switch
// in both LocalSandbox (`go vet ./...` with cmd.Dir = tmpDir) and DockerSandbox
// (`golang:1.23-alpine go vet ./...`).
type GoProvider struct{}

// NewGoProvider returns a ports.LanguageProvider for Go.
func NewGoProvider() ports.LanguageProvider { return &GoProvider{} }

func (p *GoProvider) Language() string      { return "go" }
func (p *GoProvider) FileExtension() string  { return ".go" }
func (p *GoProvider) DockerImage() string    { return "golang:1.23-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`.
// It mirrors the previous dockersandbox.go case: go vet ./...
func (p *GoProvider) DockerCommand(_ string) []string {
	return []string{"go", "vet", "./..."}
}

// LocalCommand returns the executable run by LocalSandbox ("go").
func (p *GoProvider) LocalCommand() string { return "go" }

// InstallHint is shown when the local executable is missing.
func (p *GoProvider) InstallHint() string {
	return "install Go from https://go.dev/doc/install"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*GoProvider)(nil)