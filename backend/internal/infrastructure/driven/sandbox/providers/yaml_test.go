package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestYamlProvider(t *testing.T) {
	t.Parallel()

	p := NewYamlProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("YamlProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "yaml"},
		{name: "FileExtension", got: p.FileExtension(), want: ".yaml"},
		{name: "DockerImage", got: p.DockerImage(), want: "alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "yq"},
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

	// Matches the spec scenario: YamlProvider DockerCommand("code.yaml")
	// returns sh -c "apk add --no-cache yq && yq . /code/code.yaml".
	got := p.DockerCommand("code.yaml")
	wantCmd := []string{"sh", "-c", "apk add --no-cache yq && yq . /code/code.yaml"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}