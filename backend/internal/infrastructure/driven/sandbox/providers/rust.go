package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// RustProvider audits Rust code by compiling with `rustc` and executing the
// resulting binary. In the Docker sandbox the source is mounted at `/code` as
// read-only, so the compilation writes its output to `/tmp`.
type RustProvider struct{}

// NewRustProvider returns a ports.LanguageProvider for Rust.
func NewRustProvider() ports.LanguageProvider { return &RustProvider{} }

func (p *RustProvider) Language() string     { return "rust" }
func (p *RustProvider) FileExtension() string { return ".rs" }
func (p *RustProvider) DockerImage() string   { return "rust:1.96-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`.
// The source lives at /tmp/code.rs in the container and the binary is emitted
// to /tmp/out before execution.
func (p *RustProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "rustc -o /tmp/out /tmp/code.rs && /tmp/out"}
}

// LocalCommand returns the executable run by LocalSandbox ("rustc").
func (p *RustProvider) LocalCommand() string { return "rustc" }

// InstallHint is shown when the local executable is missing.
func (p *RustProvider) InstallHint() string {
	return "rustup: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*RustProvider)(nil)