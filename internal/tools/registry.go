// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/k-sub1995/g/internal/api"
)

// ToolResult is the standard return value from tool execution.
type ToolResult struct {
	Content map[string]interface{}
	IsError bool
}

// Tool is the interface all built-in tools must implement.
type Tool interface {
	Name() string
	Declaration() api.FunctionDecl
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
}

// WebSearchFunc is a callback for performing web searches via the API.
type WebSearchFunc func(ctx context.Context, query string) (text string, sources []WebSource, err error)

// WebSource represents a web search result source.
type WebSource struct {
	Title string
	URI   string
}

// RegistryOptions configures tool behavior.
type RegistryOptions struct {
	WorkDir     string
	AutoApprove bool
	Sandbox     bool
	Debug       bool
	WebSearch   WebSearchFunc
}

// MCPToolRef tracks which MCP server owns a tool.
type MCPToolRef struct {
	ServerName string
	ToolName   string
}

// Registry holds all available tools (built-in + MCP).
type Registry struct {
	builtins map[string]Tool
	mcp      map[string]MCPToolRef
	order    []string // insertion order for deterministic output
}

// NewRegistry creates a registry populated with all built-in tools.
func NewRegistry(opts RegistryOptions) *Registry {
	r := &Registry{
		builtins: make(map[string]Tool),
		mcp:      make(map[string]MCPToolRef),
	}
	r.registerBuiltins(opts)
	return r
}

func (r *Registry) registerBuiltins(opts RegistryOptions) {
	tools := []Tool{
		NewReadFileTool(opts),
		NewWriteFileTool(opts),
		NewEditTool(opts),
		NewShellTool(opts),
		NewGlobTool(opts),
		NewGrepTool(opts),
		NewLsTool(opts),
		NewReadManyFilesTool(opts),
		NewWebSearchTool(opts),
		NewWebFetchTool(opts),
		NewMemoryTool(opts),
		NewTodosTool(opts),
		NewAskUserTool(opts),
		NewEnterPlanModeTool(opts),
		NewExitPlanModeTool(opts),
		NewActivateSkillTool(opts),
		NewInternalDocsTool(opts),
	}
	for _, t := range tools {
		r.builtins[t.Name()] = t
		r.order = append(r.order, t.Name())
	}
}

// RegisterMCPTool adds an MCP-backed tool to the registry.
func (r *Registry) RegisterMCPTool(serverName, toolName string) {
	r.mcp[toolName] = MCPToolRef{ServerName: serverName, ToolName: toolName}
}

// Get returns a built-in tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.builtins[name]
	return t, ok
}

// GetMCPRef returns the MCP reference for a tool.
func (r *Registry) GetMCPRef(name string) (MCPToolRef, bool) {
	ref, ok := r.mcp[name]
	return ref, ok
}

// AllDeclarations returns FunctionDeclarations for API request.
func (r *Registry) AllDeclarations() []api.FunctionDecl {
	decls := make([]api.FunctionDecl, 0, len(r.order))
	for _, name := range r.order {
		if t, ok := r.builtins[name]; ok {
			decls = append(decls, t.Declaration())
		}
	}
	return decls
}

// AllMCPDeclarations returns FunctionDeclarations for MCP tools.
// The caller must provide the actual MCP tool schemas.
func (r *Registry) AllMCPDeclarations(schemas map[string]api.FunctionDecl) []api.FunctionDecl {
	var decls []api.FunctionDecl
	for name := range r.mcp {
		if schema, ok := schemas[name]; ok {
			decls = append(decls, schema)
		}
	}
	return decls
}

// mustMarshalJSON marshals v to json.RawMessage, panicking on error.
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return json.RawMessage(data)
}
