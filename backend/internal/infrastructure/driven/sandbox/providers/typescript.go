package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// TypeScriptProvider audits TypeScript code with eslint (the same tool the
// previous inline switch used for both "typescript" and "javascript").
type TypeScriptProvider struct{}

// NewTypeScriptProvider returns a ports.LanguageProvider for TypeScript.
func NewTypeScriptProvider() ports.LanguageProvider { return &TypeScriptProvider{} }

func (p *TypeScriptProvider) Language() string      { return "typescript" }
func (p *TypeScriptProvider) FileExtension() string  { return ".ts" }
func (p *TypeScriptProvider) DockerImage() string    { return "node:22-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`.
// It mirrors the previous dockersandbox.go case:
//   npx eslint --format=unix {filename}
func (p *TypeScriptProvider) DockerCommand(filename string) []string {
	return []string{"npx", "eslint", "--format=unix", filename}
}

// LocalCommand returns the executable run by LocalSandbox. The local flow uses
// `npx eslint --format=unix --stdin`, so the binary is "npx".
func (p *TypeScriptProvider) LocalCommand() string { return "npx" }

// InstallHint is shown when the local executable is missing.
func (p *TypeScriptProvider) InstallHint() string {
	return "npm install -g eslint (npx ships with npm)"
}

// JavaScriptProvider audits JavaScript code. It shares the eslint toolchain with
// TypeScript but is registered under its own key with a .js extension.
type JavaScriptProvider struct{}

// NewJavaScriptProvider returns a ports.LanguageProvider for JavaScript.
func NewJavaScriptProvider() ports.LanguageProvider { return &JavaScriptProvider{} }

func (p *JavaScriptProvider) Language() string      { return "javascript" }
func (p *JavaScriptProvider) FileExtension() string { return ".js" }
func (p *JavaScriptProvider) DockerImage() string    { return "node:22-alpine" }

// DockerCommand mirrors the TypeScript argv — eslint accepts .js files unchanged.
func (p *JavaScriptProvider) DockerCommand(filename string) []string {
	return []string{"npx", "eslint", "--format=unix", filename}
}

func (p *JavaScriptProvider) LocalCommand() string { return "npx" }

func (p *JavaScriptProvider) InstallHint() string {
	return "npm install -g eslint (npx ships with npm)"
}

// Compile-time guard: both providers satisfy the port.
var (
	_ ports.LanguageProvider = (*TypeScriptProvider)(nil)
	_ ports.LanguageProvider = (*JavaScriptProvider)(nil)
)