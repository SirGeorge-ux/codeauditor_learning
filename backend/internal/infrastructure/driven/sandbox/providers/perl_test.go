package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestPerlProvider(t *testing.T) {
	t.Parallel()

	p := NewPerlProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("PerlProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "perl"},
		{name: "FileExtension", got: p.FileExtension(), want: ".pl"},
		{name: "DockerImage", got: p.DockerImage(), want: "perl:5.38-slim"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "perl"},
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

	// DockerCommand argv: perl -c <filename>
	got := p.DockerCommand("code.pl")
	wantCmd := []string{"perl", "-c", "code.pl"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}