package providers

import (
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestZigProvider(t *testing.T) {
	t.Parallel()

	p := NewZigProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("ZigProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "zig"},
		{name: "FileExtension", got: p.FileExtension(), want: ".zig"},
		{name: "DockerImage", got: p.DockerImage(), want: "alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "zig"},
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

	// DockerCommand must be a sh -c wrapper that copies the source to /tmp,
	// compiles with zig build-exe, and executes the result.
	got := p.DockerCommand("main.zig")
	if len(got) != 3 || got[0] != "sh" || got[1] != "-c" {
		t.Fatalf("DockerCommand() = %v, want [sh -c <wrapper>]", got)
	}
	script := got[2]
	for _, want := range []string{"apk add", "main.zig", "/tmp", "zig build-exe", "./" + "main.zig"} {
		if !strings.Contains(script, want) {
			t.Errorf("DockerCommand script %q missing %q", script, want)
		}
	}
}