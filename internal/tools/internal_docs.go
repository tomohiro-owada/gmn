// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/k-sub1995/g/internal/api"
)

type InternalDocsTool struct {
	opts RegistryOptions
}

func NewInternalDocsTool(opts RegistryOptions) *InternalDocsTool {
	return &InternalDocsTool{opts: opts}
}

func (t *InternalDocsTool) Name() string { return "get_internal_docs" }

func (t *InternalDocsTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "get_internal_docs",
		Description: "Returns the content of Gemini CLI internal documentation files. If no path is provided, returns a list of all available documentation paths.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The relative path to the documentation file (e.g., 'cli/commands.md'). If omitted, lists all available documentation.",
				},
			},
		}),
	}
}

func (t *InternalDocsTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	docPath := stringArg(args, "path", "")

	docsRoot, err := findDocsRoot(t.opts.WorkDir)
	if err != nil {
		return errorResult(fmt.Sprintf("could not find documentation directory: %v", err)), nil
	}

	if docPath == "" {
		// List all .md files
		pattern := filepath.Join(docsRoot, "**", "*.md")
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to list docs: %v", err)), nil
		}

		var relPaths []string
		for _, m := range matches {
			rel, err := filepath.Rel(docsRoot, m)
			if err != nil {
				continue
			}
			relPaths = append(relPaths, rel)
		}
		sort.Strings(relPaths)

		var lines []string
		for _, p := range relPaths {
			lines = append(lines, "- "+p)
		}

		return &ToolResult{
			Content: map[string]interface{}{
				"content": fmt.Sprintf("Available documentation files:\n\n%s", strings.Join(lines, "\n")),
				"count":   len(relPaths),
			},
		}, nil
	}

	// Read a specific file — prevent path traversal
	resolved := filepath.Join(docsRoot, filepath.Clean(docPath))
	resolved, _ = filepath.Abs(resolved)
	absRoot, _ := filepath.Abs(docsRoot)

	if !strings.HasPrefix(resolved, absRoot+string(filepath.Separator)) && resolved != absRoot {
		return errorResult("access denied: requested path is outside the documentation directory"), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to read documentation: %v", err)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"content": string(data),
			"path":    docPath,
		},
	}, nil
}

// findDocsRoot looks for a docs directory in several locations.
func findDocsRoot(workDir string) (string, error) {
	candidates := []string{
		filepath.Join(workDir, "docs"),
	}

	// Also check relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "docs"))
		candidates = append(candidates, filepath.Join(exeDir, "..", "docs"))
	}

	// Check ~/.gemini/docs
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".gemini", "docs"))
	}

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			return dir, nil
		}
	}

	return "", fmt.Errorf("no docs directory found")
}
