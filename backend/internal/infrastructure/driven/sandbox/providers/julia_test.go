package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestJuliaProvider(t *testing.T) {
	t.Parallel()

	p := NewJuliaProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("JuliaProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "julia"},
		{name: "FileExtension", got: p.FileExtension(), want: ".jl"},
		{name: "DockerImage", got: p.DockerImage(), want: "julia:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "julia"},
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

	// DockerCommand argv: julia /code/<filename>
	got := p.DockerCommand("code.jl")
	wantCmd := []string{"julia", "/code/code.jl"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}