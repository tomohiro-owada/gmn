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

	"github.com/k-sub1995/g/internal/api"
)

type ReadManyFilesTool struct {
	opts RegistryOptions
}

func NewReadManyFilesTool(opts RegistryOptions) *ReadManyFilesTool {
	return &ReadManyFilesTool{opts: opts}
}

func (t *ReadManyFilesTool) Name() string { return "read_many_files" }

func (t *ReadManyFilesTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "read_many_files",
		Description: "Reads and returns the content of multiple files simultaneously. More efficient than reading files one by one.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Array of file paths to read.",
				},
			},
			"required": []string{"file_paths"},
		}),
	}
}

func (t *ReadManyFilesTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pathsRaw, ok := args["file_paths"]
	if !ok {
		return errorResult("file_paths is required"), nil
	}

	var paths []string
	switch v := pathsRaw.(type) {
	case []interface{}:
		for _, p := range v {
			if s, ok := p.(string); ok {
				paths = append(paths, s)
			}
		}
	case []string:
		paths = v
	default:
		return errorResult("file_paths must be an array of strings"), nil
	}

	if len(paths) == 0 {
		return errorResult("file_paths must not be empty"), nil
	}

	results := make(map[string]interface{})
	for _, p := range paths {
		absPath := p
		if !filepath.IsAbs(p) {
			absPath = filepath.Join(t.opts.WorkDir, p)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			results[absPath] = map[string]interface{}{"error": fmt.Sprintf("failed to read: %v", err)}
		} else {
			content := string(data)
			if len(content) > 100*1024 { // 100KB per file
				content = content[:100*1024] + "\n... [truncated]"
			}
			results[absPath] = map[string]interface{}{"content": content}
		}
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"files": results,
			"count": len(paths),
		},
	}, nil
}
