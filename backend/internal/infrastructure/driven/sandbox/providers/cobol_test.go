package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestCobolProvider(t *testing.T) {
	t.Parallel()

	p := NewCobolProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("CobolProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "cobol"},
		{name: "FileExtension", got: p.FileExtension(), want: ".cbl"},
		{name: "DockerImage", got: p.DockerImage(), want: "alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "cobc"},
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

	// DockerCommand installs gnucobol, compiles to /tmp/out and runs it.
	got := p.DockerCommand("code.cbl")
	wantCmd := []string{"sh", "-c", "apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/code.cbl && /tmp/out"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}