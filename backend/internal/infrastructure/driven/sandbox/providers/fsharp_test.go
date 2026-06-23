package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestFSharpProvider(t *testing.T) {
	t.Parallel()

	p := NewFSharpProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("FSharpProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "fsharp"},
		{name: "FileExtension", got: p.FileExtension(), want: ".fs"},
		{name: "DockerImage", got: p.DockerImage(), want: "mcr.microsoft.com/dotnet/sdk:8.0-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "dotnet"},
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

	// DockerCommand runs the source directly with the dotnet fsi runner.
	got := p.DockerCommand("code.fs")
	wantCmd := []string{"dotnet", "fsi", "/code/code.fs"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}