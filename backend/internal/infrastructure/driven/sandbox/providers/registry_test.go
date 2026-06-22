package providers

import (
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// stubProvider is a minimal LanguageProvider used to exercise registry behavior
// independently of any real language implementation.
type stubProvider struct {
	lang string
}

func (s stubProvider) Language() string                   { return s.lang }
func (s stubProvider) FileExtension() string              { return ".x" }
func (s stubProvider) DockerImage() string                { return "stub:latest" }
func (s stubProvider) DockerCommand(_ string) []string     { return []string{"stub", "check"} }
func (s stubProvider) LocalCommand() string               { return "stub" }
func (s stubProvider) InstallHint() string                 { return "install stub" }

var _ ports.LanguageProvider = stubProvider{}

func TestProviderRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "registered typescript", key: "typescript"},
		{name: "registered javascript", key: "javascript"},
		{name: "registered go", key: "go"},
		{name: "registered python", key: "python"},
		{name: "registered ruby", key: "ruby"},
		{name: "registered php", key: "php"},
		{name: "registered lua", key: "lua"},
		{name: "registered bash", key: "bash"},
		{name: "registered perl", key: "perl"},
		{name: "registered java", key: "java"},
		{name: "registered kotlin", key: "kotlin"},
		{name: "registered scala", key: "scala"},
		{name: "registered groovy", key: "groovy"},
		{name: "registered rust", key: "rust"},
		{name: "registered c", key: "c"},
		{name: "registered cpp", key: "cpp"},
		{name: "registered zig", key: "zig"},
	}

	r := NewDefaultRegistry()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.Get(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for key %q, got nil", tt.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for key %q: %v", tt.key, err)
			}
			if got.Language() != tt.key {
				t.Errorf("Language() = %q, want %q", got.Language(), tt.key)
			}
		})
	}
}

func TestProviderRegistry_Get_UnknownKey(t *testing.T) {
	t.Parallel()

	r := NewDefaultRegistry()
	p, err := r.Get("cobol")
	if err == nil {
		t.Fatalf("expected error for unknown language, got provider %T", p)
	}
	if p != nil {
		t.Errorf("expected nil provider for unknown language, got %T", p)
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("expected 'unsupported language' in error, got: %v", err)
	}
}

func TestProviderRegistry_Register_NilProvider(t *testing.T) {
	t.Parallel()

	r := NewProviderRegistry()
	if err := r.Register(nil); err == nil {
		t.Fatal("expected error registering nil provider, got nil")
	}
}

func TestProviderRegistry_Register_Overwrites(t *testing.T) {
	t.Parallel()

	r := NewProviderRegistry()

	// Register an initial provider under "python".
	first := &distinguishableProvider{}
	if err := r.Register(first); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	// Overwrite with a second, distinguishable provider. Last-write-wins means
	// Register must accept the overwrite and Get must return the new one.
	second := &otherPythonProvider{}
	if err := r.Register(second); err != nil {
		t.Fatalf("overwrite register failed: %v", err)
	}

	got, err := r.Get("python")
	if err != nil {
		t.Fatalf("Get after overwrite failed: %v", err)
	}
	if got != second {
		t.Errorf("expected last-write-wins to return the second provider, got %T", got)
	}
}

// distinguishableProvider is a stub registered first under "python".
type distinguishableProvider struct{}

func (d *distinguishableProvider) Language() string               { return "python" }
func (d *distinguishableProvider) FileExtension() string           { return ".py" }
func (d *distinguishableProvider) DockerImage() string             { return "python:3.12-alpine" }
func (d *distinguishableProvider) DockerCommand(_ string) []string { return []string{"ruff"} }
func (d *distinguishableProvider) LocalCommand() string            { return "ruff" }
func (d *distinguishableProvider) InstallHint() string              { return "pip install ruff" }

// otherPythonProvider is a second stub under "python" used to assert that
// Register overwrites (last-write-wins) by pointer identity.
type otherPythonProvider struct{}

func (o *otherPythonProvider) Language() string               { return "python" }
func (o *otherPythonProvider) FileExtension() string           { return ".py" }
func (o *otherPythonProvider) DockerImage() string             { return "other:latest" }
func (o *otherPythonProvider) DockerCommand(_ string) []string { return []string{"other"} }
func (o *otherPythonProvider) LocalCommand() string            { return "other" }
func (o *otherPythonProvider) InstallHint() string              { return "install other" }

func TestProviderRegistry_Languages(t *testing.T) {
	t.Parallel()

	r := NewDefaultRegistry()
	got := r.Languages()

	// Languages() is required to return all 17 registered keys, sorted.
	want := []string{"bash", "c", "cpp", "go", "groovy", "java", "javascript", "kotlin", "lua", "perl", "php", "python", "ruby", "rust", "scala", "typescript", "zig"}
	if len(got) != len(want) {
		t.Fatalf("Languages() = %v, want %v", got, want)
	}
	for i, lang := range got {
		if lang != want[i] {
			t.Errorf("Languages()[%d] = %q, want %q (must be sorted)", i, lang, want[i])
		}
	}
}