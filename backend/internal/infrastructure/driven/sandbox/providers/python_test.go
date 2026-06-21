package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestPythonProvider(t *testing.T) {
	t.Parallel()

	p := NewPythonProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("PythonProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "python"},
		{name: "FileExtension", got: p.FileExtension(), want: ".py"},
		{name: "DockerImage", got: p.DockerImage(), want: "python:3.12-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "ruff"},
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

	// DockerCommand argv: ruff check --output-format=text <filename>
	got := p.DockerCommand("code.py")
	wantCmd := []string{"ruff", "check", "--output-format=text", "code.py"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}