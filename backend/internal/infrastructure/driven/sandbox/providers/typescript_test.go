package providers

import (
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func TestTypeScriptProvider(t *testing.T) {
	t.Parallel()

	p := NewTypeScriptProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("TypeScriptProvider does not satisfy ports.LanguageProvider")
	}

	want := struct {
		language      string
		fileExtension string
		dockerImage   string
		localCommand  string
	}{
		language:      "typescript",
		fileExtension: ".ts",
		dockerImage:   "node:22-alpine",
		localCommand:  "npx",
	}

	if got := p.Language(); got != want.language {
		t.Errorf("Language() = %q, want %q", got, want.language)
	}
	if got := p.FileExtension(); got != want.fileExtension {
		t.Errorf("FileExtension() = %q, want %q", got, want.fileExtension)
	}
	if got := p.DockerImage(); got != want.dockerImage {
		t.Errorf("DockerImage() = %q, want %q", got, want.dockerImage)
	}
	if got := p.LocalCommand(); got != want.localCommand {
		t.Errorf("LocalCommand() = %q, want %q", got, want.localCommand)
	}
	if got := p.InstallHint(); got == "" {
		t.Error("InstallHint() must be non-empty")
	}

	// DockerCommand mirrors the previous dockersandbox.go case:
	//   npx eslint --format=unix <filename>
	got := p.DockerCommand("code.ts")
	wantCmd := []string{"npx", "eslint", "--format=unix", "code.ts"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}

func TestJavaScriptProvider(t *testing.T) {
	t.Parallel()

	p := NewJavaScriptProvider()
	if _, ok := p.(ports.LanguageProvider); !ok {
		t.Fatalf("JavaScriptProvider does not satisfy ports.LanguageProvider")
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Language", got: p.Language(), want: "javascript"},
		{name: "FileExtension", got: p.FileExtension(), want: ".js"},
		{name: "DockerImage", got: p.DockerImage(), want: "node:22-alpine"},
		{name: "LocalCommand", got: p.LocalCommand(), want: "npx"},
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

	// JavaScript shares the eslint argv with TypeScript but receives a .js file.
	got := p.DockerCommand("code.js")
	wantCmd := []string{"npx", "eslint", "--format=unix", "code.js"}
	if len(got) != len(wantCmd) {
		t.Fatalf("DockerCommand() = %v, want %v", got, wantCmd)
	}
	for i, arg := range got {
		if arg != wantCmd[i] {
			t.Errorf("DockerCommand()[%d] = %q, want %q", i, arg, wantCmd[i])
		}
	}
}