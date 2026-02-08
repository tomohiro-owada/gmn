// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/k-sub1995/g/internal/api"
)

const (
	shellTimeout   = 120 * time.Second
	maxOutputBytes = 100 * 1024 // 100KB
)

type ShellTool struct {
	opts RegistryOptions
}

func NewShellTool(opts RegistryOptions) *ShellTool {
	return &ShellTool{opts: opts}
}

func (t *ShellTool) Name() string { return "run_shell_command" }

func (t *ShellTool) Declaration() api.FunctionDecl {
	shellDesc := "This tool executes a given shell command as `bash -c <command>`. Use this to run system commands, build projects, run tests, and perform git operations."
	if runtime.GOOS == "windows" {
		shellDesc = "This tool executes a given shell command as `powershell.exe -NoProfile -Command <command>`. Use this to run system commands, build projects, run tests, and perform git operations."
	}

	return api.FunctionDecl{
		Name:        "run_shell_command",
		Description: shellDesc,
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute.",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Brief description of what the command does.",
				},
				"dir_path": map[string]interface{}{
					"type":        "string",
					"description": "Optional: The directory to run the command in. Defaults to the project root.",
				},
				"is_background": map[string]interface{}{
					"type":        "boolean",
					"description": "Set to true to run in background (e.g. for servers).",
				},
			},
			"required": []string{"command"},
		}),
	}
}

func (t *ShellTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return errorResult("command is required"), nil
	}

	dirPath := stringArg(args, "dir_path", t.opts.WorkDir)
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(t.opts.WorkDir, dirPath)
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, shellTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "powershell.exe", "-NoProfile", "-Command", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", command)
	}
	cmd.Dir = dirPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Truncate output if too large
	if len(stdoutStr) > maxOutputBytes {
		stdoutStr = stdoutStr[:maxOutputBytes] + "\n... [output truncated]"
	}
	if len(stderrStr) > maxOutputBytes {
		stderrStr = stderrStr[:maxOutputBytes] + "\n... [output truncated]"
	}

	result := map[string]interface{}{
		"stdout": stdoutStr,
		"stderr": stderrStr,
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			result["error"] = "command timed out"
		} else {
			result["exit_code"] = cmd.ProcessState.ExitCode()
			result["error"] = err.Error()
		}
		return &ToolResult{Content: result, IsError: true}, nil
	}

	result["exit_code"] = 0
	return &ToolResult{Content: result}, nil
}
