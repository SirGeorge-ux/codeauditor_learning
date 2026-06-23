package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestPowerShellProvider(t *testing.T) {
	t.Parallel()

	p := NewPowerShellProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("PowerShellProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "powershell"},
		{name: "FileExtension", got: p.FileExtension(), want: ".ps1"},
		{name: "DockerImage", got: p.DockerImage(), want: "mcr.microsoft.com/powershell:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "pwsh"},
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

	// DockerCommand runs the source directly with pwsh -File.
	got := p.DockerCommand("code.ps1")
	wantCmd := []string{"pwsh", "-File", "/code/code.ps1"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}