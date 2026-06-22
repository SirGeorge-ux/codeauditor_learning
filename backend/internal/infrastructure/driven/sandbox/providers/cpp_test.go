package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestCppProvider(t *testing.T) {
	t.Parallel()

	p := NewCppProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("CppProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "cpp"},
		{name: "FileExtension", got: p.FileExtension(), want: ".cpp"},
		{name: "DockerImage", got: p.DockerImage(), want: "gcc:15.3.0"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "g++"},
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

	// DockerCommand compiles to /tmp/out and runs it.
	got := p.DockerCommand("main.cpp")
	wantCmd := []string{"sh", "-c", "g++ -o /tmp/out /tmp/code.cpp && /tmp/out"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}