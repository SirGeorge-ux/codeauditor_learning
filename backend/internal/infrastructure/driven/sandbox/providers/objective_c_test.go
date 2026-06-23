package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestObjectiveCProvider(t *testing.T) {
	t.Parallel()

	p := NewObjectiveCProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("ObjectiveCProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "objective-c"},
		{name: "FileExtension", got: p.FileExtension(), want: ".m"},
		{name: "DockerImage", got: p.DockerImage(), want: "gcc:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "gcc"},
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

	// DockerCommand compiles objective-c to /tmp/out and runs it.
	got := p.DockerCommand("code.m")
	wantCmd := []string{"sh", "-c", "gcc -x objective-c -o /tmp/out /code/code.m -lobjc && /tmp/out"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}