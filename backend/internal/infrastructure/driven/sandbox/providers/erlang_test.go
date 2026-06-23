package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestErlangProvider(t *testing.T) {
	t.Parallel()

	p := NewErlangProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("ErlangProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "erlang"},
		{name: "FileExtension", got: p.FileExtension(), want: ".erl"},
		{name: "DockerImage", got: p.DockerImage(), want: "erlang:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "escript"},
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

	// DockerCommand argv: escript /code/<filename>
	got := p.DockerCommand("code.erl")
	wantCmd := []string{"escript", "/code/code.erl"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}