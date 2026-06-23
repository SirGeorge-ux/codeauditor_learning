package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestDartProvider(t *testing.T) {
	t.Parallel()

	p := NewDartProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("DartProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "dart"},
		{name: "FileExtension", got: p.FileExtension(), want: ".dart"},
		{name: "DockerImage", got: p.DockerImage(), want: "dart:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "dart"},
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

	// DockerCommand argv: dart run /code/<filename>
	got := p.DockerCommand("code.dart")
	wantCmd := []string{"dart", "run", "/code/code.dart"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}