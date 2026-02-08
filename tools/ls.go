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
	"strings"

	"github.com/k-sub1995/g/internal/api"
)

type LsTool struct {
	opts RegistryOptions
}

func NewLsTool(opts RegistryOptions) *LsTool {
	return &LsTool{opts: opts}
}

func (t *LsTool) Name() string { return "list_directory" }

func (t *LsTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "list_directory",
		Description: "Lists the names of files and subdirectories directly within a specified directory path.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dir_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the directory to list.",
				},
			},
			"required": []string{"dir_path"},
		}),
	}
}

func (t *LsTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	dirPath, _ := args["dir_path"].(string)
	if dirPath == "" {
		dirPath = t.opts.WorkDir
	}
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(t.opts.WorkDir, dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to list directory: %v", err)), nil
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		info, err := entry.Info()
		if err == nil {
			lines = append(lines, fmt.Sprintf("%s  (%d bytes)", name, info.Size()))
		} else {
			lines = append(lines, name)
		}
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"entries":  strings.Join(lines, "\n"),
			"dir_path": dirPath,
			"count":    len(entries),
		},
	}, nil
}
