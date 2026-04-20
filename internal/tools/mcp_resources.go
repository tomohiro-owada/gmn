// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
// Upstream ref: f16f1cce - add tools to list and read MCP resources
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/tomohiro-owada/gmn/internal/api"
)

// MCPResourceInfo represents a resource exposed by an MCP server.
type MCPResourceInfo struct {
	ServerName  string
	URI         string
	Name        string
	Description string
}

// ListMCPResourcesFunc returns all available MCP resources.
type ListMCPResourcesFunc func() []MCPResourceInfo

// ReadMCPResourceFunc reads a resource from an MCP server.
type ReadMCPResourceFunc func(ctx context.Context, serverName, uri string) (string, error)

// --- list_mcp_resources ---

type ListMCPResourcesTool struct {
	opts RegistryOptions
}

func NewListMCPResourcesTool(opts RegistryOptions) *ListMCPResourcesTool {
	return &ListMCPResourcesTool{opts: opts}
}

func (t *ListMCPResourcesTool) Name() string { return "list_mcp_resources" }

func (t *ListMCPResourcesTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "list_mcp_resources",
		Description: "Lists resources available from connected MCP servers. Returns the server name, URI, name, and description for each resource. Optionally filter by server name.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"serverName": map[string]interface{}{
					"type":        "string",
					"description": "Optional server name to filter resources by.",
				},
			},
		}),
	}
}

func (t *ListMCPResourcesTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.opts.ListMCPResources == nil {
		return errorResult("MCP resource listing is not available"), nil
	}

	resources := t.opts.ListMCPResources()

	serverName := stringArg(args, "serverName", "")
	if serverName != "" {
		var filtered []MCPResourceInfo
		for _, r := range resources {
			if r.ServerName == serverName {
				filtered = append(filtered, r)
			}
		}
		resources = filtered
	}

	if len(resources) == 0 {
		msg := "No MCP resources found."
		if serverName != "" {
			msg = fmt.Sprintf("No resources found for server: %s", serverName)
		}
		return &ToolResult{
			Content: map[string]interface{}{"message": msg},
		}, nil
	}

	var lines []string
	for _, r := range resources {
		line := fmt.Sprintf("- %s:%s", r.ServerName, r.URI)
		if r.Name != "" {
			line += " | " + r.Name
		}
		if r.Description != "" {
			line += " | " + r.Description
		}
		lines = append(lines, line)
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"content": "Available MCP Resources:\n" + strings.Join(lines, "\n"),
			"count":   len(resources),
		},
	}, nil
}

// --- read_mcp_resource ---

type ReadMCPResourceTool struct {
	opts RegistryOptions
}

func NewReadMCPResourceTool(opts RegistryOptions) *ReadMCPResourceTool {
	return &ReadMCPResourceTool{opts: opts}
}

func (t *ReadMCPResourceTool) Name() string { return "read_mcp_resource" }

func (t *ReadMCPResourceTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "read_mcp_resource",
		Description: "Reads the content of a specific MCP resource by its URI. The URI should be in the format 'serverName:resourceUri' as shown by list_mcp_resources.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "The resource URI in the format 'serverName:resourceUri'.",
				},
			},
			"required": []string{"uri"},
		}),
	}
}

func (t *ReadMCPResourceTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.opts.ReadMCPResource == nil {
		return errorResult("MCP resource reading is not available"), nil
	}

	uri := stringArg(args, "uri", "")
	if uri == "" {
		return errorResult("uri is required"), nil
	}

	colonIdx := strings.Index(uri, ":")
	if colonIdx <= 0 {
		return errorResult("invalid URI format: expected 'serverName:resourceUri'"), nil
	}

	serverName := uri[:colonIdx]
	resourceURI := uri[colonIdx+1:]

	content, err := t.opts.ReadMCPResource(ctx, serverName, resourceURI)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to read resource: %v", err)), nil
	}

	if content == "" {
		content = "No content returned from resource."
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"content":    content,
			"serverName": serverName,
			"uri":        resourceURI,
		},
	}, nil
}
