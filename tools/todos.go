// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/k-sub1995/g/internal/api"
)

type TodosTool struct {
	opts RegistryOptions
}

func NewTodosTool(opts RegistryOptions) *TodosTool {
	return &TodosTool{opts: opts}
}

func (t *TodosTool) Name() string { return "write_todos" }

func (t *TodosTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "write_todos",
		Description: "Create and manage a todo list for tracking tasks. Use this to plan and track progress on multi-step tasks.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"todos": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Unique identifier for the todo.",
							},
							"title": map[string]interface{}{
								"type":        "string",
								"description": "Brief title of the task.",
							},
							"status": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
								"description": "Current status of the task.",
							},
						},
						"required": []string{"id", "title", "status"},
					},
					"description": "Array of todo items.",
				},
			},
			"required": []string{"todos"},
		}),
	}
}

func (t *TodosTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	todosRaw, ok := args["todos"]
	if !ok {
		return errorResult("todos is required"), nil
	}

	// Marshal and unmarshal to validate structure
	data, err := json.Marshal(todosRaw)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid todos: %v", err)), nil
	}

	var todos []map[string]interface{}
	if err := json.Unmarshal(data, &todos); err != nil {
		return errorResult(fmt.Sprintf("invalid todos structure: %v", err)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"message": fmt.Sprintf("Todo list updated with %d items.", len(todos)),
			"todos":   todos,
		},
	}, nil
}
