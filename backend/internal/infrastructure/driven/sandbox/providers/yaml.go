package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// YamlProvider audits YAML by validating it with `yq`. alpine:latest does not
// ship yq, so the Docker sandbox installs it on the fly with apk inside a shell
// wrapper and then validates the snippet mounted at /code/code.yaml.
type YamlProvider struct{}

// NewYamlProvider returns a ports.LanguageProvider for YAML.
func NewYamlProvider() ports.LanguageProvider { return &YamlProvider{} }

func (p *YamlProvider) Language() string     { return "yaml" }
func (p *YamlProvider) FileExtension() string { return ".yaml" }
func (p *YamlProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	sh -c "apk add --no-cache yq && yq . /code/code.yaml"
//
// yq parses the document with `yq .`; invalid YAML surfaces a parse error on
// stderr with a non-zero exit code.
func (p *YamlProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "apk add --no-cache yq && yq . /code/code.yaml"}
}

// LocalCommand returns the executable run by LocalSandbox ("yq").
func (p *YamlProvider) LocalCommand() string { return "yq" }

// InstallHint is shown when the local executable is missing.
func (p *YamlProvider) InstallHint() string {
	return "Install yq: https://github.com/mikefarah/yq/#install or `apk add yq` / go install github.com/mikefarah/yq/v4@latest"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*YamlProvider)(nil)