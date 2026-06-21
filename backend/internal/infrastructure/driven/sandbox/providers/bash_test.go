package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestBashProvider(t *testing.T) {
	t.Parallel()

	p := NewBashProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("BashProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "bash"},
		{name: "FileExtension", got: p.FileExtension(), want: ".sh"},
		{name: "DockerImage", got: p.DockerImage(), want: "koalaman/shellcheck-alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "shellcheck"},
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

	// DockerCommand argv: shellcheck <filename>
	// Matches the spec scenario: BashProvider DockerCommand("code.sh")
	// returns ["shellcheck", "code.sh"].
	got := p.DockerCommand("code.sh")
	wantCmd := []string{"shellcheck", "code.sh"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}