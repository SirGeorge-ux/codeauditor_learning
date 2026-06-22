package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestCssProvider(t *testing.T) {
	t.Parallel()

	p := NewCssProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("CssProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "css"},
		{name: "FileExtension", got: p.FileExtension(), want: ".css"},
		{name: "DockerImage", got: p.DockerImage(), want: "alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "cat"},
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

	// Matches the spec scenario: CssProvider DockerCommand("code.css")
	// returns cat /code/code.css.
	got := p.DockerCommand("code.css")
	wantCmd := []string{"cat", "/code/code.css"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}