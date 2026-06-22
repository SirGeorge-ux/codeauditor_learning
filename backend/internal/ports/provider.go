package ports

// LanguageProvider defines per-language behavior for sandbox execution.
// It is a driven (secondary) port — each language implements this contract so
// the sandbox adapters can delegate command/image generation without inline
// switch statements.
type LanguageProvider interface {
	// Language returns the canonical language key (e.g. "typescript", "go").
	Language() string

	// FileExtension returns the file extension used when writing the code to a
	// temp file (e.g. ".ts", ".go").
	FileExtension() string

	// DockerImage returns the publicly available Alpine-based image used by the
	// DockerSandbox adapter (e.g. "node:22-alpine").
	DockerImage() string

	// DockerCommand returns the argv (without the image) appended after the
	// generic `docker run` flags. The filename is the basename written inside the
	// mounted /code directory.
	DockerCommand(filename string) []string

	// LocalCommand returns the executable run by the LocalSandbox adapter
	// (e.g. "npx", "go"). It must exist in PATH for the local healthcheck.
	LocalCommand() string

	// InstallHint returns actionable installation guidance shown when the local
	// tool required by this provider is missing (e.g. "npm install -g eslint").
	InstallHint() string
}