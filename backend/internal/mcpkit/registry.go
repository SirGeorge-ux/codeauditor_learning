package mcpkit

import (
	"fmt"
	"sync"
)

// ErrToolNotFound is returned by Registry.Get when the requested tool name is
// not registered. The Dispatcher maps this to JSON-RPC error code -32601
// (Method not found).
var ErrToolNotFound = fmt.Errorf("method not found (-32601)")

// Registry is a thread-safe, write-once/read-many store of MCP tools.
// Tools are registered entirely at startup; reads happen continuously per
// request. sync.RWMutex is the right primitive for this read-heavy workload.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]MCPTool
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]MCPTool)}
}

// Register adds a tool to the registry. It panics if a tool with the same
// Name() is already registered — duplicates are a programming error caught
// at startup, never a runtime condition.
func (r *Registry) Register(tool MCPTool) {
	if tool == nil {
		panic("mcp: cannot register a nil tool")
	}
	name := tool.Name()
	if name == "" {
		panic("mcp: cannot register a tool with an empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		panic("mcp: tool already registered: " + name)
	}
	r.tools[name] = tool
}

// Get returns the tool registered under name, or ErrToolNotFound (mapped to
// JSON-RPC -32601) when the name is unknown.
func (r *Registry) Get(name string) (MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	if !exists {
		return nil, ErrToolNotFound
	}
	return tool, nil
}

// List returns a snapshot of all registered tools. The order is not
// guaranteed; callers that need stable ordering must sort the result.
func (r *Registry) List() []MCPTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]MCPTool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
