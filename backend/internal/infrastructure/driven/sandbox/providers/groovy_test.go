package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestGroovyProvider(t *testing.T) {
	t.Parallel()

	p := NewGroovyProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("GroovyProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "groovy"},
		{name: "FileExtension", got: p.FileExtension(), want: ".groovy"},
		{name: "DockerImage", got: p.DockerImage(), want: "groovy:4.0-jdk21-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "groovyc"},
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

	// DockerCommand argv: groovyc -d /tmp <filename>
	// The -d /tmp directs .class output to the writable tmpfs since
	// DockerSandbox mounts /code as read-only.
	got := p.DockerCommand("Script.groovy")
	wantCmd := []string{"groovyc", "-d", "/tmp", "Script.groovy"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
