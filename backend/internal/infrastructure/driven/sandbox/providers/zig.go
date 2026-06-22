package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// ZigProvider audits Zig code by compiling with `zig build-exe` and executing
// the resulting binary. Because `zig build-exe` writes the output to the
// current working directory — and the Docker sandbox mounts the user's code at
// `/code` as read-only — the provider wraps the command in `sh -c` to install
// zig, copy the source to `/tmp`, compile there, and run the binary.
type ZigProvider struct{}

// NewZigProvider returns a ports.LanguageProvider for Zig.
func NewZigProvider() ports.LanguageProvider { return &ZigProvider{} }

func (p *ZigProvider) Language() string     { return "zig" }
func (p *ZigProvider) FileExtension() string { return ".zig" }
func (p *ZigProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`.
// It installs zig from the Alpine package index, copies the mounted source
// file to a writable /tmp directory (the /code mount is read-only), compiles it
// with `zig build-exe`, and runs the resulting binary.
func (p *ZigProvider) DockerCommand(filename string) []string {
	return []string{
		"sh", "-c",
		"apk add --no-cache zig && cp /code/" + filename + " /tmp/ && cd /tmp && zig build-exe " + filename + " && ./" + filename,
	}
}

// LocalCommand returns the executable run by LocalSandbox ("zig").
func (p *ZigProvider) LocalCommand() string { return "zig" }

// InstallHint is shown when the local executable is missing.
func (p *ZigProvider) InstallHint() string {
	return "apk add zig (Alpine) or download from ziglang.org"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ZigProvider)(nil)