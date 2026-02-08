// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k-sub1995/g/internal/api"
)

const defaultReadLimit = 2000

type ReadFileTool struct {
	opts RegistryOptions
}

func NewReadFileTool(opts RegistryOptions) *ReadFileTool {
	return &ReadFileTool{opts: opts}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "read_file",
		Description: "Reads and returns the content of a specified file. If the file is large, the content will be truncated. The tool's response will clearly indicate if truncation has occurred and will provide details on how to read more of the file using the 'offset' and 'limit' parameters. Handles text files. For text files, it can read specific line ranges.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_path": map[string]interface{}{
					"type":        "string",
					"description": "The path to the file to read.",
				},
				"offset": map[string]interface{}{
					"type":        "number",
					"description": "Optional: For text files, the 0-based line number to start reading from. Requires 'limit' to be set. Use for paginating through large files.",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Optional: For text files, maximum number of lines to read. Use with 'offset' to paginate through large files. If omitted, reads up to 2000 lines.",
				},
			},
			"required": []string{"file_path"},
		}),
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return errorResult("file_path is required"), nil
	}

	absPath := t.resolvePath(filePath)

	info, err := os.Stat(absPath)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to access file: %v", err)), nil
	}
	if info.IsDir() {
		return errorResult("path is a directory, not a file. Use list_directory instead."), nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to open file: %v", err)), nil
	}
	defer f.Close()

	offset := intArg(args, "offset", 0)
	limit := intArg(args, "limit", defaultReadLimit)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	var lines []string
	lineNum := 0
	totalLines := 0
	truncated := false

	for scanner.Scan() {
		totalLines++
		if lineNum >= offset && lineNum < offset+limit {
			lines = append(lines, scanner.Text())
		}
		lineNum++
		if lineNum >= offset+limit && len(lines) >= limit {
			truncated = true
		}
	}
	if err := scanner.Err(); err != nil {
		return errorResult(fmt.Sprintf("error reading file: %v", err)), nil
	}

	// Check if there are more lines
	if lineNum > offset+limit {
		truncated = true
	}

	content := strings.Join(lines, "\n")
	result := map[string]interface{}{
		"content":    content,
		"file_path":  absPath,
		"line_count": len(lines),
	}

	if truncated {
		result["truncated"] = true
		result["total_lines"] = totalLines
		result["next_offset"] = offset + limit
		result["message"] = fmt.Sprintf("File truncated. Showing lines %d-%d of %d. Use offset=%d to read more.", offset, offset+len(lines)-1, totalLines, offset+limit)
	}

	return &ToolResult{Content: result}, nil
}

func (t *ReadFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.opts.WorkDir, path)
}

// Helper functions shared across tools

func errorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: map[string]interface{}{"error": msg},
		IsError: true,
	}
}

func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return defaultVal
}

func stringArg(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}

func boolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}
