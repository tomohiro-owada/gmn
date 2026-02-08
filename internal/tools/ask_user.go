// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"

	"github.com/k-sub1995/g/internal/api"
)

type AskUserTool struct {
	opts RegistryOptions
}

func NewAskUserTool(opts RegistryOptions) *AskUserTool {
	return &AskUserTool{opts: opts}
}

func (t *AskUserTool) Name() string { return "ask_user" }

func (t *AskUserTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "ask_user",
		Description: "Ask the user a question and get their response. In non-interactive mode, this tool will indicate that user input is not available.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"question": map[string]interface{}{
					"type":        "string",
					"description": "The question to ask the user.",
				},
			},
			"required": []string{"question"},
		}),
	}
}

func (t *AskUserTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return errorResult("question is required"), nil
	}

	// Non-interactive mode: cannot ask questions
	return &ToolResult{
		Content: map[string]interface{}{
			"message":  "Running in non-interactive mode. Cannot ask user questions. Please make your best judgment and proceed, or explain what you need in the output.",
			"question": question,
		},
		IsError: true,
	}, nil
}
