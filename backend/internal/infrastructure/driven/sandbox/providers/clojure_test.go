package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestClojureProvider(t *testing.T) {
	t.Parallel()

	p := NewClojureProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("ClojureProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "clojure"},
		{name: "FileExtension", got: p.FileExtension(), want: ".clj"},
		{name: "DockerImage", got: p.DockerImage(), want: "clojure:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "clojure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	if hint := p.InstallHint(); hint == "" {
		t.Error("InstallHint() must be non-empty")
	}

	// DockerCommand runs the source in script mode with clojure -M.
	got := p.DockerCommand("code.clj")
	wantCmd := []string{"clojure", "-M", "/code/code.clj"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
