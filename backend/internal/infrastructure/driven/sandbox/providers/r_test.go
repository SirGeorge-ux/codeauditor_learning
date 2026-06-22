package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestRProvider(t *testing.T) {
	t.Parallel()

	p := NewRProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("RProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "r"},
		{name: "FileExtension", got: p.FileExtension(), want: ".r"},
		{name: "DockerImage", got: p.DockerImage(), want: "r-base:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "Rscript"},
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

	// DockerCommand runs the source directly with the Rscript runner.
	got := p.DockerCommand("code.r")
	wantCmd := []string{"Rscript", "/code/code.r"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
