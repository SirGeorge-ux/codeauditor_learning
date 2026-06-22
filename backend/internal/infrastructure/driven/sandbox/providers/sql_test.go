package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestSqlProvider(t *testing.T) {
	t.Parallel()

	p := NewSqlProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("SqlProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "sql"},
		{name: "FileExtension", got: p.FileExtension(), want: ".sql"},
		{name: "DockerImage", got: p.DockerImage(), want: "alpine:latest"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "sqlite3"},
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

	// Matches the spec scenario: SqlProvider DockerCommand("code.sql")
	// returns sh -c "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'".
	got := p.DockerCommand("code.sql")
	wantCmd := []string{"sh", "-c", "apk add --no-cache sqlite && sqlite3 :memory: '.read /code/code.sql'"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}