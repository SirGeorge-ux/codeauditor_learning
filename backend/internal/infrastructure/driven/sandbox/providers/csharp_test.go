package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestCSharpProvider(t *testing.T) {
	t.Parallel()

	p := NewCSharpProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("CSharpProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "csharp"},
		{name: "FileExtension", got: p.FileExtension(), want: ".cs"},
		{name: "DockerImage", got: p.DockerImage(), want: "mono:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "mcs"},
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

	// DockerCommand compiles to /tmp/out.exe and runs it with mono.
	got := p.DockerCommand("code.cs")
	wantCmd := []string{"sh", "-c", "mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
