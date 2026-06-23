package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestNimProvider(t *testing.T) {
	t.Parallel()

	p := NewNimProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("NimProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "nim"},
		{name: "FileExtension", got: p.FileExtension(), want: ".nim"},
		{name: "DockerImage", got: p.DockerImage(), want: "nimlang/nim:alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "nim"},
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

	// DockerCommand argv: nim c -r --hints:off /code/<filename>
	got := p.DockerCommand("code.nim")
	wantCmd := []string{"nim", "c", "-r", "--hints:off", "/code/code.nim"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}