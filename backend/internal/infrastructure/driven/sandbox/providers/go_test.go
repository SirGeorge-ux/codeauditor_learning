package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestGoProvider(t *testing.T) {
	t.Parallel()

	p := NewGoProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("GoProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "go"},
		{name: "FileExtension", got: p.FileExtension(), want: ".go"},
		{name: "DockerImage", got: p.DockerImage(), want: "golang:1.23-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "go"},
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

	// DockerCommand mirrors the previous dockersandbox.go case:
	//   go vet ./...
	// The filename arg is ignored because `go vet ./...` operates on the
	// working directory, not a single file — matching the original switch.
	got := p.DockerCommand("main.go")
	wantCmd := []string{"go", "vet", "./..."}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}