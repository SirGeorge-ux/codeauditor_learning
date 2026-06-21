package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestPhpProvider(t *testing.T) {
	t.Parallel()

	p := NewPhpProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("PhpProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "php"},
		{name: "FileExtension", got: p.FileExtension(), want: ".php"},
		{name: "DockerImage", got: p.DockerImage(), want: "php:8.3-cli-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "php"},
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

	// DockerCommand argv: php -l <filename>
	got := p.DockerCommand("code.php")
	wantCmd := []string{"php", "-l", "code.php"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}