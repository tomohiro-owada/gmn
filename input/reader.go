// Package input provides input handling for geminimini.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package input

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadStdin reads from stdin if available
func ReadStdin() (string, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", nil
	}

	// Check if stdin is a pipe or has data
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}

// ReadFiles reads content from multiple files
func ReadFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", path, err)
		}
		builder.WriteString(fmt.Sprintf("=== %s ===\n", path))
		builder.Write(content)
		builder.WriteString("\n\n")
	}

	return builder.String(), nil
}

// PrepareInput combines stdin, files, and prompt into a single input
func PrepareInput(prompt string, files []string) (string, error) {
	var parts []string

	// Read stdin
	stdin, err := ReadStdin()
	if err != nil {
		return "", err
	}
	if stdin != "" {
		parts = append(parts, stdin)
	}

	// Read files
	filesContent, err := ReadFiles(files)
	if err != nil {
		return "", err
	}
	if filesContent != "" {
		parts = append(parts, filesContent)
	}

	// Add prompt
	if prompt != "" {
		parts = append(parts, prompt)
	}

	return strings.Join(parts, "\n\n"), nil
}
