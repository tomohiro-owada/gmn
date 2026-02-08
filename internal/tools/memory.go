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

type MemoryTool struct {
	opts RegistryOptions
}

func NewMemoryTool(opts RegistryOptions) *MemoryTool {
	return &MemoryTool{opts: opts}
}

func (t *MemoryTool) Name() string { return "save_memory" }

func (t *MemoryTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "save_memory",
		Description: "Saves important information to a persistent memory file (~/.gemini/GEMINI.md) for future sessions. Use this to remember user preferences, project context, or important facts.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to save to memory.",
				},
			},
			"required": []string{"content"},
		}),
	}
}

func (t *MemoryTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	content, _ := args["content"].(string)
	if content == "" {
		return errorResult("content is required"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get home directory: %v", err)), nil
	}

	memPath := filepath.Join(home, ".gemini", "GEMINI.md")
	if err := os.MkdirAll(filepath.Dir(memPath), 0755); err != nil {
		return errorResult(fmt.Sprintf("failed to create directory: %v", err)), nil
	}

	f, err := os.OpenFile(memPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to open memory file: %v", err)), nil
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n%s\n", content); err != nil {
		return errorResult(fmt.Sprintf("failed to write memory: %v", err)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"message":   "Memory saved successfully.",
			"file_path": memPath,
		},
	}, nil
}
