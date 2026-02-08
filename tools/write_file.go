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

type WriteFileTool struct {
	opts RegistryOptions
}

func NewWriteFileTool(opts RegistryOptions) *WriteFileTool {
	return &WriteFileTool{opts: opts}
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "write_file",
		Description: "Writes content to a specified file in the local filesystem. Creates the file if it doesn't exist. Creates parent directories as needed.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to write to.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write to the file.",
				},
			},
			"required": []string{"file_path", "content"},
		}),
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	content, _ := args["content"].(string)

	if filePath == "" {
		return errorResult("file_path is required"), nil
	}

	absPath := t.resolvePath(filePath)

	if t.opts.Sandbox {
		if !isPathUnder(absPath, t.opts.WorkDir) {
			return errorResult(fmt.Sprintf("sandbox: cannot write outside working directory %s", t.opts.WorkDir)), nil
		}
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errorResult(fmt.Sprintf("failed to create directory: %v", err)), nil
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return errorResult(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"message":   fmt.Sprintf("Successfully wrote to %s", absPath),
			"file_path": absPath,
			"bytes":     len(content),
		},
	}, nil
}

func (t *WriteFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.opts.WorkDir, path)
}

func isPathUnder(path, base string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !startsWith(rel, "..")
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
