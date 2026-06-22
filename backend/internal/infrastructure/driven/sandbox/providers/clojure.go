package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ClojureProvider audits Clojure code by executing it with the Clojure CLI in
// script mode (`clojure -M`), using the official `clojure:latest` image for
// Docker execution and the `clojure` binary for the local healthcheck.
type ClojureProvider struct{}

// NewClojureProvider returns a ports.LanguageProvider for Clojure.
func NewClojureProvider() ports.LanguageProvider { return &ClojureProvider{} }

func (p *ClojureProvider) Language() string      { return "clojure" }
func (p *ClojureProvider) FileExtension() string { return ".clj" }
func (p *ClojureProvider) DockerImage() string   { return "clojure:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	clojure -M /code/<filename>
//
// The -M flag runs the file as a script (main-mode) rather than an AOT
// compilation. The source path uses the read-only /code mount.
func (p *ClojureProvider) DockerCommand(filename string) []string {
	return []string{"clojure", "-M", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("clojure").
func (p *ClojureProvider) LocalCommand() string { return "clojure" }

// InstallHint is shown when the local executable is missing.
func (p *ClojureProvider) InstallHint() string {
	return "install Clojure CLI: https://clojure.org/guides/install_clojure (or brew install clojure/tools/clojure)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ClojureProvider)(nil)
