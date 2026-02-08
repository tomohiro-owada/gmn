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

type EditTool struct {
	opts RegistryOptions
}

func NewEditTool(opts RegistryOptions) *EditTool {
	return &EditTool{opts: opts}
}

func (t *EditTool) Name() string { return "replace" }

func (t *EditTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "replace",
		Description: "Replaces text within a file. By default, replaces a single occurrence, but can replace multiple occurrences when `expected_replacements` is specified. Always use the read_file tool to examine the file's current content before attempting a text replacement.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to modify.",
				},
				"old_string": map[string]interface{}{
					"type":        "string",
					"description": "The exact literal text to replace. Include sufficient context to make the match unique.",
				},
				"new_string": map[string]interface{}{
					"type":        "string",
					"description": "The exact literal text to replace old_string with.",
				},
				"expected_replacements": map[string]interface{}{
					"type":        "number",
					"description": "Number of replacements expected. Defaults to 1 if not specified.",
					"minimum":     1,
				},
			},
			"required": []string{"file_path", "old_string", "new_string"},
		}),
	}
}

func (t *EditTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	oldString, _ := args["old_string"].(string)
	newString, _ := args["new_string"].(string)
	expectedReplacements := intArg(args, "expected_replacements", 1)

	if filePath == "" {
		return errorResult("file_path is required"), nil
	}
	if oldString == "" {
		return errorResult("old_string is required"), nil
	}

	absPath := t.resolvePath(filePath)

	if t.opts.Sandbox {
		if !isPathUnder(absPath, t.opts.WorkDir) {
			return errorResult(fmt.Sprintf("sandbox: cannot edit files outside working directory %s", t.opts.WorkDir)), nil
		}
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	content := string(data)
	count := strings.Count(content, oldString)

	if count == 0 {
		return errorResult(fmt.Sprintf("old_string not found in %s. Make sure you have the exact text including whitespace and indentation.", absPath)), nil
	}

	if expectedReplacements > 0 && count != expectedReplacements {
		return errorResult(fmt.Sprintf("expected %d replacement(s) but found %d occurrence(s) of old_string in %s", expectedReplacements, count, absPath)), nil
	}

	newContent := strings.Replace(content, oldString, newString, expectedReplacements)

	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return errorResult(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"message":      fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", expectedReplacements, absPath),
			"file_path":    absPath,
			"replacements": expectedReplacements,
		},
	}, nil
}

func (t *EditTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.opts.WorkDir, path)
}
