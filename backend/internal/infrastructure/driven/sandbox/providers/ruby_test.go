package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestRubyProvider(t *testing.T) {
	t.Parallel()

	p := NewRubyProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("RubyProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "ruby"},
		{name: "FileExtension", got: p.FileExtension(), want: ".rb"},
		{name: "DockerImage", got: p.DockerImage(), want: "ruby:3.3-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "rubocop"},
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

	// DockerCommand argv: rubocop --format=simple <filename>
	got := p.DockerCommand("code.rb")
	wantCmd := []string{"rubocop", "--format=simple", "code.rb"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}