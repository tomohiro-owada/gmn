// Package cmd provides the CLI commands for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// SchemaFlag describes a single CLI flag for machine consumption.
type SchemaFlag struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// SchemaOutput is the top-level schema document returned by `gmn schema`.
type SchemaOutput struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Usage       string            `json:"usage"`
	Auth        SchemaAuth        `json:"auth"`
	Flags       []SchemaFlag      `json:"flags"`
	ModelAlias  map[string]string `json:"modelAliases"`
	Output      SchemaOutputSpec  `json:"output"`
	AgentNotes  SchemaAgentNotes  `json:"agentNotes"`
}

// SchemaAuth describes authentication requirements.
type SchemaAuth struct {
	Method      string `json:"method"`
	CredPath    string `json:"credPath"`
	Prerequiste string `json:"prerequisite"`
}

// SchemaOutputSpec describes supported output formats and fields.
type SchemaOutputSpec struct {
	Formats       []string `json:"formats"`
	DefaultTTY    string   `json:"defaultTTY"`
	DefaultNonTTY string   `json:"defaultNonTTY"`
	JSONFields    []string `json:"jsonFields"`
	StreamEvents  []string `json:"streamEventTypes"`
}

// SchemaAgentNotes contains operational guidance for AI agents.
type SchemaAgentNotes struct {
	AlwaysUseFields    string   `json:"alwaysUseFieldsWhenParsing"`
	DangerousFlags     []string `json:"dangerousFlags"`
	SafeForAutomation  bool     `json:"safeForAutomation"`
	DestructiveOps     bool     `json:"hasDestructiveOperations"`
	NonInteractive     bool     `json:"nonInteractive"`
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print machine-readable CLI schema (flags, output format, auth requirements)",
	Long:  `Returns a JSON document describing gmn's flags, output formats, model aliases, and agent usage notes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		schema := SchemaOutput{
			Name:        "gmn",
			Version:     version,
			Description: "Lightweight, non-interactive Gemini CLI written in Go. Reuses auth from the official Gemini CLI.",
			Usage:       "gmn [prompt] [flags]",
			Auth: SchemaAuth{
				Method:      "oauth2",
				CredPath:    "~/.gemini/oauth_creds.json",
				Prerequiste: "Run 'gemini' once to authenticate via the official Gemini CLI.",
			},
			Flags: []SchemaFlag{
				{Name: "prompt", Short: "p", Type: "string", Default: "", Description: "Prompt to send to Gemini"},
				{Name: "model", Short: "m", Type: "string", Default: "pro", Description: "Model alias or full model name"},
				{Name: "output-format", Short: "o", Type: "string", Default: "text (stream-json when not a TTY)", Description: "Output format: text, json, stream-json"},
				{Name: "file", Short: "f", Type: "[]string", Default: nil, Description: "Files to include in context (repeatable)"},
				{Name: "timeout", Short: "t", Type: "duration", Default: "5m", Description: "API timeout"},
				{Name: "max-turns", Type: "int", Default: 25, Description: "Maximum agent loop turns"},
				{Name: "yolo", Type: "bool", Default: false, Description: "Auto-approve shell commands (no confirmation) — use with caution"},
				{Name: "sandbox", Type: "bool", Default: false, Description: "Restrict file writes to working directory"},
				{Name: "no-agent", Type: "bool", Default: false, Description: "Disable agent mode (single-turn, no tools)"},
				{Name: "input-json", Type: "string", Default: "", Description: "Structured JSON input overriding individual flags"},
				{Name: "fields", Type: "[]string", Default: nil, Description: "Filter JSON output to specified fields (e.g. --fields response,usage)"},
				{Name: "debug", Type: "bool", Default: false, Description: "Enable debug output to stderr"},
				{Name: "raw-output", Type: "bool", Default: false, Description: "Disable ANSI sanitization of model output"},
			},
			ModelAlias: map[string]string{
				"pro":        "gemini-3-pro-preview (or gemini-2.5-pro without preview access)",
				"flash":      "gemini-3-flash-preview",
				"flash-lite": "gemini-2.5-flash-lite (or gemini-3.1-flash-lite-preview with experiment flag)",
				"auto":       "gemini-3-pro-preview",
			},
			Output: SchemaOutputSpec{
				Formats:       []string{"text", "json", "stream-json"},
				DefaultTTY:    "text",
				DefaultNonTTY: "stream-json",
				JSONFields:    []string{"model", "response", "usage", "finishReason"},
				StreamEvents:  []string{"text", "done", "error", "tool_call", "tool_result"},
			},
			AgentNotes: SchemaAgentNotes{
				AlwaysUseFields:   "When parsing -o json output, prefer --fields to limit token usage.",
				DangerousFlags:    []string{"--yolo"},
				SafeForAutomation: true,
				DestructiveOps:    false,
				NonInteractive:    true,
			},
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(schema); err != nil {
			return fmt.Errorf("failed to encode schema: %w", err)
		}
		return nil
	},
}
