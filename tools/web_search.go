// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/k-sub1995/g/internal/api"
)

type WebSearchTool struct {
	opts RegistryOptions
}

func NewWebSearchTool(opts RegistryOptions) *WebSearchTool {
	return &WebSearchTool{opts: opts}
}

func (t *WebSearchTool) Name() string { return "google_web_search" }

func (t *WebSearchTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "google_web_search",
		Description: "Performs a web search using Google Search (via the Gemini API) and returns the results. This tool is useful for finding information on the internet based on a query.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query to find information on the web.",
				},
			},
			"required": []string{"query"},
		}),
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query := stringArg(args, "query", "")
	if query == "" {
		return errorResult("query is required"), nil
	}

	if t.opts.WebSearch == nil {
		return errorResult("web search is not configured"), nil
	}

	text, sources, err := t.opts.WebSearch(ctx, query)
	if err != nil {
		return errorResult(fmt.Sprintf("web search failed: %v", err)), nil
	}

	if text == "" {
		return &ToolResult{
			Content: map[string]interface{}{
				"message": fmt.Sprintf("No search results found for: %q", query),
			},
		}, nil
	}

	// Format sources
	var sourceLines []string
	for i, src := range sources {
		title := src.Title
		if title == "" {
			title = "Untitled"
		}
		sourceLines = append(sourceLines, fmt.Sprintf("[%d] %s (%s)", i+1, title, src.URI))
	}

	result := fmt.Sprintf("Web search results for %q:\n\n%s", query, text)
	if len(sourceLines) > 0 {
		result += "\n\nSources:\n" + strings.Join(sourceLines, "\n")
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"content": result,
			"query":   query,
			"sources": len(sources),
		},
	}, nil
}
