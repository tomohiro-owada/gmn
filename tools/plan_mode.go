// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"

	"github.com/k-sub1995/g/internal/api"
)

// --- enter_plan_mode ---

type EnterPlanModeTool struct {
	opts RegistryOptions
}

func NewEnterPlanModeTool(opts RegistryOptions) *EnterPlanModeTool {
	return &EnterPlanModeTool{opts: opts}
}

func (t *EnterPlanModeTool) Name() string { return "enter_plan_mode" }

func (t *EnterPlanModeTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "enter_plan_mode",
		Description: "Enter plan mode to create a step-by-step plan before making changes. In plan mode, you should outline your approach before executing.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}),
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return &ToolResult{
		Content: map[string]interface{}{
			"message": "Plan mode entered. Outline your step-by-step plan, then call exit_plan_mode when ready to execute.",
		},
	}, nil
}

// --- exit_plan_mode ---

type ExitPlanModeTool struct {
	opts RegistryOptions
}

func NewExitPlanModeTool(opts RegistryOptions) *ExitPlanModeTool {
	return &ExitPlanModeTool{opts: opts}
}

func (t *ExitPlanModeTool) Name() string { return "exit_plan_mode" }

func (t *ExitPlanModeTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "exit_plan_mode",
		Description: "Exit plan mode and begin executing the plan.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}),
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return &ToolResult{
		Content: map[string]interface{}{
			"message": "Plan mode exited. Proceeding with execution.",
		},
	}, nil
}
