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

const maxGlobResults = 500

type GlobTool struct {
	opts RegistryOptions
}

func NewGlobTool(opts RegistryOptions) *GlobTool {
	return &GlobTool{opts: opts}
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "glob",
		Description: "Efficiently finds files matching specific glob patterns (e.g., `src/**/*.ts`, `**/*.md`), returning absolute paths sorted by modification time (newest first). Ideal for quickly locating files based on their name or path structure.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The glob pattern to match against (e.g., '**/*.py', 'docs/*.md').",
				},
				"dir_path": map[string]interface{}{
					"type":        "string",
					"description": "Optional: The directory to search within. If omitted, searches from the working directory.",
				},
			},
			"required": []string{"pattern"},
		}),
	}
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return errorResult("pattern is required"), nil
	}

	dirPath := stringArg(args, "dir_path", t.opts.WorkDir)
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(t.opts.WorkDir, dirPath)
	}

	fullPattern := filepath.Join(dirPath, pattern)

	fsys := os.DirFS("/")
	// Strip leading "/" for DirFS
	relPattern := strings.TrimPrefix(fullPattern, "/")

	matches, err := doublestar.Glob(fsys, relPattern)
	if err != nil {
		return errorResult(fmt.Sprintf("glob error: %v", err)), nil
	}

	// Convert back to absolute paths and get mod times
	type fileInfo struct {
		path    string
		modTime int64
	}
	var files []fileInfo
	for _, m := range matches {
		absPath := "/" + m
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		files = append(files, fileInfo{path: absPath, modTime: info.ModTime().UnixNano()})
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	truncated := false
	if len(files) > maxGlobResults {
		files = files[:maxGlobResults]
		truncated = true
	}

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.path
	}

	result := map[string]interface{}{
		"files": paths,
		"count": len(paths),
	}
	if truncated {
		result["truncated"] = true
		result["message"] = fmt.Sprintf("Results limited to %d files. Use a more specific pattern.", maxGlobResults)
	}

	return &ToolResult{Content: result}, nil
}
