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
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/k-sub1995/g/internal/api"
)

const maxGrepMatches = 100

type GrepTool struct {
	opts RegistryOptions
}

func NewGrepTool(opts RegistryOptions) *GrepTool {
	return &GrepTool{opts: opts}
}

func (t *GrepTool) Name() string { return "grep_search" }

func (t *GrepTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "grep_search",
		Description: "Searches for a regular expression pattern within file contents. Returns matching lines with file paths and line numbers. Max 100 matches.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "The regular expression pattern to search for within file contents.",
				},
				"dir_path": map[string]interface{}{
					"type":        "string",
					"description": "Optional: The directory to search in. Defaults to working directory.",
				},
				"include": map[string]interface{}{
					"type":        "string",
					"description": "Optional: A glob pattern to filter which files are searched (e.g., '*.js', '*.{ts,tsx}').",
				},
			},
			"required": []string{"pattern"},
		}),
	}
}

func (t *GrepTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return errorResult("pattern is required"), nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid regex pattern: %v", err)), nil
	}

	dirPath := stringArg(args, "dir_path", t.opts.WorkDir)
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(t.opts.WorkDir, dirPath)
	}

	include := stringArg(args, "include", "")

	type match struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Content string `json:"content"`
	}
	var matches []match
	truncated := false

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == ".svn" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary/large files
		if info.Size() > 1024*1024 { // 1MB
			return nil
		}

		// Apply include filter
		if include != "" {
			matched, _ := doublestar.Match(include, info.Name())
			if !matched {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, match{
					File:    path,
					Line:    lineNum,
					Content: truncateString(strings.TrimSpace(line), 200),
				})
				if len(matches) >= maxGrepMatches {
					truncated = true
					return fmt.Errorf("max matches reached")
				}
			}
		}
		return nil
	})

	if err != nil && err.Error() != "max matches reached" && ctx.Err() == nil {
		return errorResult(fmt.Sprintf("search error: %v", err)), nil
	}

	// Format results
	var resultLines []string
	for _, m := range matches {
		resultLines = append(resultLines, fmt.Sprintf("%s:%d: %s", m.File, m.Line, m.Content))
	}

	result := map[string]interface{}{
		"matches": strings.Join(resultLines, "\n"),
		"count":   len(matches),
	}
	if truncated {
		result["truncated"] = true
		result["message"] = fmt.Sprintf("Results limited to %d matches. Refine your search.", maxGrepMatches)
	}

	return &ToolResult{Content: result}, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
