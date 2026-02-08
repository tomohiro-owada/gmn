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

	"github.com/k-sub1995/g/internal/api"
)

type ActivateSkillTool struct {
	opts RegistryOptions
}

func NewActivateSkillTool(opts RegistryOptions) *ActivateSkillTool {
	return &ActivateSkillTool{opts: opts}
}

func (t *ActivateSkillTool) Name() string { return "activate_skill" }

func (t *ActivateSkillTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "activate_skill",
		Description: "Activates a skill by name, loading its instructions from the project's .gemini/skills/ directory.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to activate.",
				},
			},
			"required": []string{"skill_name"},
		}),
	}
}

func (t *ActivateSkillTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	skillName, _ := args["skill_name"].(string)
	if skillName == "" {
		return errorResult("skill_name is required"), nil
	}

	// Search for SKILL.md in .gemini/skills/<skill_name>/
	skillPath := filepath.Join(t.opts.WorkDir, ".gemini", "skills", skillName, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return errorResult(fmt.Sprintf("skill '%s' not found at %s", skillName, skillPath)), nil
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"skill_name":   skillName,
			"instructions": string(data),
			"message":      fmt.Sprintf("Skill '%s' activated. Follow the instructions above.", skillName),
		},
	}, nil
}
