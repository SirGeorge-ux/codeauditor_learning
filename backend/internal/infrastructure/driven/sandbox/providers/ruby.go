package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// RubyProvider audits Ruby code with rubocop, using the
// `ruby:3.3-alpine` image for Docker execution and the `rubocop` binary
// for the local healthcheck.
type RubyProvider struct{}

// NewRubyProvider returns a ports.LanguageProvider for Ruby.
func NewRubyProvider() ports.LanguageProvider { return &RubyProvider{} }

func (p *RubyProvider) Language() string      { return "ruby" }
func (p *RubyProvider) FileExtension() string  { return ".rb" }
func (p *RubyProvider) DockerImage() string    { return "ruby:3.3-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   rubocop --format=simple <filename>
func (p *RubyProvider) DockerCommand(filename string) []string {
	return []string{"rubocop", "--format=simple", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("rubocop").
func (p *RubyProvider) LocalCommand() string { return "rubocop" }

// InstallHint is shown when the local executable is missing.
func (p *RubyProvider) InstallHint() string {
	return "gem install rubocop"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*RubyProvider)(nil)