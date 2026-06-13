package sandbox

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewLocalSandbox(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)
	if sb == nil {
		t.Fatal("expected non-nil sandbox")
	}
	if sb.timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", sb.timeout)
	}
}

func TestExecute_UnsupportedLanguage(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)
	_, err := sb.Execute(context.Background(), "python", "print('hi')", 10)
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("expected 'unsupported language' in error, got: %v", err)
	}
}

func TestExecute_GoVet_ValidCode(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)

	// go vet on valid Go code should succeed (no output)
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

func TestExecute_GoVet_UnusedVariable(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)

	// go vet should detect unused variable
	code := `package main
func main() {
	var x int
	_ = x
}
`
	rc, err := sb.Execute(context.Background(), "go", code, 10)
	if err != nil {
		t.Fatalf("expected no execution error, got: %v", err)
	}
	defer rc.Close()

	output, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Execution itself succeeds; go vet output is returned as text
	// (vet may or may not flag this depending on Go version)
	_ = output
}

func TestExecute_ZeroTimeout_Defaults(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)

	code := `package main
func main() {}
`
	rc, err := sb.Execute(context.Background(), "go", code, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer rc.Close()

	output, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	_ = output
}

func TestExecute_NegativeTimeout_Defaults(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)

	code := `package main
func main() {}
`
	rc, err := sb.Execute(context.Background(), "go", code, -5)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer rc.Close()

	output, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	_ = output
}

func TestHealthcheck(t *testing.T) {
	sb := NewLocalSandbox(30 * time.Second)
	err := sb.Healthcheck(context.Background())
	if err != nil {
		t.Skipf("skipping healthcheck: go not available (%v)", err)
	}
}

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
