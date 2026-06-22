package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestRustProvider(t *testing.T) {
	t.Parallel()

	p := NewRustProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("RustProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "rust"},
		{name: "FileExtension", got: p.FileExtension(), want: ".rs"},
		{name: "DockerImage", got: p.DockerImage(), want: "rust:1.96-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "rustc"},
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

	// DockerCommand compiles to /tmp/out and runs it.
	got := p.DockerCommand("main.rs")
	wantCmd := []string{"sh", "-c", "rustc -o /tmp/out /tmp/code.rs && /tmp/out"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}