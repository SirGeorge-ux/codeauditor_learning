package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestHaskellProvider(t *testing.T) {
	t.Parallel()

	p := NewHaskellProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("HaskellProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "haskell"},
		{name: "FileExtension", got: p.FileExtension(), want: ".hs"},
		{name: "DockerImage", got: p.DockerImage(), want: "haskell:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "runhaskell"},
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

	// DockerCommand runs the source directly with the runhaskell runner.
	got := p.DockerCommand("code.hs")
	wantCmd := []string{"runhaskell", "/code/code.hs"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
