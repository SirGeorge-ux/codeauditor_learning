package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestHtmlProvider(t *testing.T) {
	t.Parallel()

	p := NewHtmlProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("HtmlProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "html"},
		{name: "FileExtension", got: p.FileExtension(), want: ".html"},
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

	// Matches the spec scenario: HtmlProvider DockerCommand("code.html")
	// returns cat /code/code.html.
	got := p.DockerCommand("code.html")
	wantCmd := []string{"cat", "/code/code.html"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}