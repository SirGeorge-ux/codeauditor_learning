// Hexagonal purity assertion (PR 2, task 2.14).
//
// Scope: verifies the mcp-code-health tool group has zero
// database/sql + zero net/http imports. The test reads the
// package's source files and asserts no forbidden imports
// exist. A regression that introduces a DB / HTTP call from
// the MCP layer surfaces here first.
//
// This is a read-only assertion; the test does not boot a
// database or HTTP server.
package codehealth

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoForbiddenImports_InfrastructureLayer asserts the
// mcp-code-health package does not import database/sql or
// net/http. The MCP tool layer is a driving adapter and must
// stay free of persistence + transport primitives (those
// belong in the driven adapters in infrastructure/driven).
func TestNoForbiddenImports_InfrastructureLayer(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// dir is the package's directory (the test runner
	// invokes the test from there).
	forbidden := []string{
		"database/sql",
		"net/http",
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		imports, err := parseImports(path)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range imports {
			for _, f := range forbidden {
				if imp == f {
					t.Errorf("%s imports %q; the mcp-code-health layer is a driving adapter and must not import database/sql or net/http (use ports.CodeHealthRepository + ports.AnalysisCache + the mcpkit transport instead)", path, f)
				}
			}
		}
	}
}

// parseImports reads a Go source file and returns the set of
// imported package paths.
func parseImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		// Strip the surrounding quotes from the import
		// path (the AST stores them literally).
		s := strings.Trim(imp.Path.Value, `"`)
		out = append(out, s)
	}
	return out, nil
}
