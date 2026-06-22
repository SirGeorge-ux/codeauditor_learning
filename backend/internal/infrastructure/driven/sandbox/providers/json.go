package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// JsonProvider audits JSON by validating it with `jq`. alpine:latest does not
// ship jq, so the Docker sandbox installs it on the fly with apk inside a shell
// wrapper and then validates the snippet mounted at /code/code.json.
type JsonProvider struct{}

// NewJsonProvider returns a ports.LanguageProvider for JSON.
func NewJsonProvider() ports.LanguageProvider { return &JsonProvider{} }

func (p *JsonProvider) Language() string     { return "json" }
func (p *JsonProvider) FileExtension() string { return ".json" }
func (p *JsonProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	sh -c "apk add --no-cache jq && jq . /code/code.json"
//
// jq parses the document with `jq .`; invalid JSON (e.g. a missing bracket)
// surfaces a parse error on stderr with a non-zero exit code.
func (p *JsonProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "apk add --no-cache jq && jq . /code/code.json"}
}

// LocalCommand returns the executable run by LocalSandbox ("jq").
func (p *JsonProvider) LocalCommand() string { return "jq" }

// InstallHint is shown when the local executable is missing.
func (p *JsonProvider) InstallHint() string {
	return "Install jq: https://stedolan.github.io/jq/download/ or `apk add jq` / `apt install jq`"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*JsonProvider)(nil)