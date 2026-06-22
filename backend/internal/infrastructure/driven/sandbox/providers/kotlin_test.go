package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestKotlinProvider(t *testing.T) {
	t.Parallel()

	p := NewKotlinProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("KotlinProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "kotlin"},
		{name: "FileExtension", got: p.FileExtension(), want: ".kt"},
		{name: "DockerImage", got: p.DockerImage(), want: "codeauditor/kotlin-compiler:2.0-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "kotlinc"},
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

	// DockerCommand argv: kotlinc -d /tmp <filename>
	// The -d /tmp directs .class output to the writable tmpfs since
	// DockerSandbox mounts /code as read-only.
	got := p.DockerCommand("App.kt")
	wantCmd := []string{"kotlinc", "-d", "/tmp", "App.kt"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}
