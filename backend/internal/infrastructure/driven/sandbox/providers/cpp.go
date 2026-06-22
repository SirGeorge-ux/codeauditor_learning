package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// CppProvider audits C++ code by compiling with `g++` and executing the
// resulting binary. In the Docker sandbox the source is mounted at `/code` as
// read-only, so the compilation writes its output to `/tmp`.
type CppProvider struct{}

// NewCppProvider returns a ports.LanguageProvider for C++.
func NewCppProvider() ports.LanguageProvider { return &CppProvider{} }

func (p *CppProvider) Language() string     { return "cpp" }
func (p *CppProvider) FileExtension() string { return ".cpp" }
func (p *CppProvider) DockerImage() string   { return "gcc:15.3.0" }

// DockerCommand returns the argv appended after the image in `docker run`.
// The source lives at /tmp/code.cpp in the container and the binary is emitted
// to /tmp/out before execution.
func (p *CppProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "g++ -o /tmp/out /tmp/code.cpp && /tmp/out"}
}

// LocalCommand returns the executable run by LocalSandbox ("g++").
func (p *CppProvider) LocalCommand() string { return "g++" }

// InstallHint is shown when the local executable is missing.
func (p *CppProvider) InstallHint() string {
	return "apt install g++ (or brew install gcc)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*CppProvider)(nil)