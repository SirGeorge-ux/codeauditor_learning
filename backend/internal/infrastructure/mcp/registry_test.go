package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sashabaranov/go-openai"
)

// fakeTool is a minimal MCPTool used in tests.
type fakeTool struct {
	name string
	desc string
	exec func(ctx context.Context, args json.RawMessage) (any, error)
}

func (f *fakeTool) Name() string        { return f.name }
func (f *fakeTool) Description() string { return f.desc }
func (f *fakeTool) Schema() openai.Tool { return openai.Tool{} }

func (f *fakeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if f.exec == nil {
		return "ok", nil
	}
	return f.exec(ctx, args)
}

func newFakeTool(name string) *fakeTool {
	return &fakeTool{name: name, desc: "fake " + name}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := newFakeTool("mcp-sandbox.run_code")
	r.Register(tool)

	got, err := r.Get("mcp-sandbox.run_code")
	if err != nil {
		t.Fatalf("expected to find registered tool, got error: %v", err)
	}
	if got.Name() != "mcp-sandbox.run_code" {
		t.Errorf("expected name mcp-sandbox.run_code, got %q", got.Name())
	}
}

func TestRegistry_Get_UnknownReturnsErrToolNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("mcp-nonexistent.foo")
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
	if err != ErrToolNotFound {
		t.Errorf("expected ErrToolNotFound, got %v", err)
	}
}

func TestRegistry_Register_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(newFakeTool("mcp-sandbox.run_code"))

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration, got none")
		}
	}()
	r.Register(newFakeTool("mcp-sandbox.run_code"))
}

func TestRegistry_Register_NilPanics(t *testing.T) {
	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil tool, got none")
		}
	}()
	r.Register(nil)
}

func TestRegistry_Register_EmptyNamePanics(t *testing.T) {
	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty name, got none")
		}
	}()
	r.Register(newFakeTool(""))
}

func TestRegistry_List_ReturnsAllRegistered(t *testing.T) {
	r := NewRegistry()
	r.Register(newFakeTool("mcp-a.x"))
	r.Register(newFakeTool("mcp-b.y"))
	r.Register(newFakeTool("mcp-c.z"))

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(list))
	}
	if r.Count() != 3 {
		t.Errorf("expected Count()==3, got %d", r.Count())
	}
}

func TestRegistry_List_EmptyReturnsEmptySlice(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list))
	}
}
