package sandbox

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// newTestSandbox returns a LocalSandbox wired with the default registry for tests.
func newTestSandbox(timeout time.Duration) *LocalSandbox {
	return NewLocalSandbox(timeout, providers.NewDefaultRegistry())
}

func TestNewLocalSandbox(t *testing.T) {
	sb := newTestSandbox(30 * time.Second)
	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}
	if sb.timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", sb.timeout)
	}
	if sb.registry == nil {
		t.Error("expected registry to be wired")
	}
}

// TestExecute_UnknownLanguage_RejectsEarly verifies the spec scenario:
// "Unknown language rejects early" — no temp directory or process is created.
func TestExecute_UnknownLanguage_RejectsEarly(t *testing.T) {
	sb := newTestSandbox(30 * time.Second)

	_, err := sb.Execute(context.Background(), "fortran", "print('hi')", 10)
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("expected 'unsupported language' in error, got: %v", err)
	}
}

// TestExecute_Languages build the command for each registered language and
// verifies the resulting exec.Cmd has the expected binary and args (derived
// from the provider's DockerCommand). Tools need not be installed — this
// only exercises command construction, not execution.
func TestExecute_Languages(t *testing.T) {
	registry := providers.NewDefaultRegistry()

	tests := []struct {
		name    string
		lang    string
		ext     string
		wantBin string
		wantArg string // first arg after binary, asserted to include filename
	}{
		{name: "typescript", lang: "typescript", ext: ".ts", wantBin: "npx", wantArg: "eslint"},
		{name: "javascript", lang: "javascript", ext: ".js", wantBin: "npx", wantArg: "eslint"},
		{name: "go", lang: "go", ext: ".go", wantBin: "go", wantArg: "vet"},
		{name: "python", lang: "python", ext: ".py", wantBin: "ruff", wantArg: "check"},
		{name: "ruby", lang: "ruby", ext: ".rb", wantBin: "rubocop", wantArg: "--format=simple"},
		{name: "php", lang: "php", ext: ".php", wantBin: "php", wantArg: "-l"},
		{name: "lua", lang: "lua", ext: ".lua", wantBin: "luacheck", wantArg: "code.lua"},
		{name: "bash", lang: "bash", ext: ".sh", wantBin: "shellcheck", wantArg: "code.sh"},
		{name: "perl", lang: "perl", ext: ".pl", wantBin: "perl", wantArg: "-c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := registry.Get(tt.lang)
			if err != nil {
				t.Fatalf("registry.Get(%q) failed: %v", tt.lang, err)
			}
			if got := provider.FileExtension(); got != tt.ext {
				t.Errorf("FileExtension() = %q, want %q", got, tt.ext)
			}

			filename := "code" + tt.ext
			cmd := buildLocalCommand(context.Background(), provider, filename, "/tmp/fake")
			if cmd.Path != tt.wantBin && !strings.HasSuffix(cmd.Path, "/"+tt.wantBin) {
				t.Errorf("command binary = %q, want %q", cmd.Path, tt.wantBin)
			}
			if len(cmd.Args) < 2 || cmd.Args[1] != tt.wantArg {
				t.Errorf("command argv = %v, want second element %q", cmd.Args, tt.wantArg)
			}
			if cmd.Dir != "/tmp/fake" {
				t.Errorf("cmd.Dir = %q, want /tmp/fake", cmd.Dir)
			}
		})
	}
}

// TestExecute_GoVet_ValidCode runs go vet through the registry path on clean Go
// code. go is guaranteed available in a Go dev environment.
func TestExecute_GoVet_ValidCode(t *testing.T) {
	sb := newTestSandbox(30 * time.Second)

	code := `package main
import "fmt"
func main() {
	fmt.Println("hello")
}
`
	rc, err := sb.Execute(context.Background(), "go", code, 10)
	if err != nil {
		t.Fatalf("expected no error for valid Go code, got: %v", err)
	}
	defer rc.Close()

	output, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// go vet produces no output for clean code
	if len(output) > 0 {
		t.Logf("go vet output (may include warnings): %s", string(output))
	}
}

func TestExecute_ZeroTimeout_Defaults(t *testing.T) {
	sb := newTestSandbox(30 * time.Second)

	code := `package main
func main() {}
`
	rc, err := sb.Execute(context.Background(), "go", code, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer rc.Close()

	if _, err := io.ReadAll(rc); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
}

func TestExecute_NegativeTimeout_Defaults(t *testing.T) {
	sb := newTestSandbox(30 * time.Second)

	code := `package main
func main() {}
`
	rc, err := sb.Execute(context.Background(), "go", code, -5)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer rc.Close()

	if _, err := io.ReadAll(rc); err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
}

// TestHealthcheck verifies the registry-driven healthcheck. Since installing all
// 9 tools is unusual, we test the logic: build a registry with only the go
// provider (go IS installed) and assert healthcheck succeeds. We also verify
// the error path with a registry whose tool is guaranteed missing.
func TestHealthcheck(t *testing.T) {
	t.Run("all tools available", func(t *testing.T) {
		r := providers.NewProviderRegistry()
		_ = r.Register(&goProviderForTest{})
		sb := NewLocalSandbox(30*time.Second, r)
		if err := sb.Healthcheck(context.Background()); err != nil {
			t.Errorf("expected nil for installed go, got: %v", err)
		}
	})

	t.Run("missing tool reported with hint", func(t *testing.T) {
		r := providers.NewProviderRegistry()
		_ = r.Register(&missingToolProvider{})
		sb := NewLocalSandbox(30*time.Second, r)
		err := sb.Healthcheck(context.Background())
		if err == nil {
			t.Fatal("expected error for missing tool, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' in error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "pip install definitely-not-real") {
			t.Errorf("expected install hint in error, got: %v", err)
		}
	})
}

// goProviderForTest is a minimal provider whose LocalCommand is "go", used to
// exercise the healthcheck without registering all 9 providers.
type goProviderForTest struct{}

func (goProviderForTest) Language() string                  { return "go" }
func (goProviderForTest) FileExtension() string             { return ".go" }
func (goProviderForTest) DockerImage() string               { return "golang:1.23-alpine" }
func (goProviderForTest) DockerCommand(_ string) []string   { return []string{"go", "vet", "./..."} }
func (goProviderForTest) LocalCommand() string              { return "go" }
func (goProviderForTest) InstallHint() string               { return "install Go" }

// missingToolProvider is a minimal provider whose LocalCommand is guaranteed
// not to exist in PATH.
type missingToolProvider struct{}

func (missingToolProvider) Language() string                  { return "ghostlang" }
func (missingToolProvider) FileExtension() string             { return ".gh" }
func (missingToolProvider) DockerImage() string              { return "ghost:latest" }
func (missingToolProvider) DockerCommand(_ string) []string  { return []string{"ghost-tool"} }
func (missingToolProvider) LocalCommand() string             { return "definitely-not-real-tool-xyz" }
func (missingToolProvider) InstallHint() string               { return "pip install definitely-not-real" }

// Compile-time guards.
var (
	_ ports.LanguageProvider = goProviderForTest{}
	_ ports.LanguageProvider = missingToolProvider{}
)

func TestCmdReader_Close(t *testing.T) {
	r := &cmdReader{
		Reader: strings.NewReader("output"),
		cmd:    nil,
		tmpDir: "",
	}
	err := r.Close()
	if err != nil {
		t.Errorf("expected no error on Close, got: %v", err)
	}
}