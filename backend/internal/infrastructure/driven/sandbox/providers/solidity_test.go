package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestSolidityProvider(t *testing.T) {
	t.Parallel()

	p := NewSolidityProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("SolidityProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "solidity"},
		{name: "FileExtension", got: p.FileExtension(), want: ".sol"},
		{name: "DockerImage", got: p.DockerImage(), want: "ethereum/solc:stable"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "solc"},
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

	// DockerCommand argv: solc /code/<filename>
	got := p.DockerCommand("code.sol")
	wantCmd := []string{"solc", "/code/code.sol"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}