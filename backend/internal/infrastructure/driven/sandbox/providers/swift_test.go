package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestSwiftProvider(t *testing.T) {
	t.Parallel()

	p := NewSwiftProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("SwiftProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "swift"},
		{name: "FileExtension", got: p.FileExtension(), want: ".swift"},
		{name: "DockerImage", got: p.DockerImage(), want: "swift:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "swift"},
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

	// DockerCommand runs the source directly with the swift runner.
	got := p.DockerCommand("code.swift")
	wantCmd := []string{"swift", "/code/code.swift"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
