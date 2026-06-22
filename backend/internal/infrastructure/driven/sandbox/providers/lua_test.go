package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestLuaProvider(t *testing.T) {
	t.Parallel()

	p := NewLuaProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("LuaProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "lua"},
		{name: "FileExtension", got: p.FileExtension(), want: ".lua"},
		{name: "DockerImage", got: p.DockerImage(), want: "nickblah/luacheck:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "luacheck"},
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

	// DockerCommand argv: luacheck <filename>
	got := p.DockerCommand("code.lua")
	wantCmd := []string{"luacheck", "code.lua"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}