// Package agent provides the agentic loop for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/k-sub1995/g/internal/api"
	"github.com/k-sub1995/g/internal/mcp"
	"github.com/k-sub1995/g/internal/output"
	"github.com/k-sub1995/g/internal/tools"
)

// SyntheticThoughtSignature is used when a FunctionCall part lacks a thoughtSignature.
// The Gemini API requires this for validation in thinking mode.
const SyntheticThoughtSignature = "skip_thought_signature_validator"

// Config configures the agent loop.
type Config struct {
	MaxTurns  int
	Streaming bool
	Debug     bool
}

// MCPClients maps server names to initialized MCP clients.
type MCPClients map[string]*mcp.Client

// Loop runs the agentic loop.
type Loop struct {
	apiClient  *api.Client
	registry   *tools.Registry
	mcpClients MCPClients
	formatter  output.Formatter
	config     Config
}

// NewLoop creates a new agent loop.
func NewLoop(apiClient *api.Client, registry *tools.Registry,
	mcpClients MCPClients, formatter output.Formatter, config Config) *Loop {
	return &Loop{
		apiClient:  apiClient,
		registry:   registry,
		mcpClients: mcpClients,
		formatter:  formatter,
		config:     config,
	}
}

// Run executes the agent loop with the given request.
func (l *Loop) Run(ctx context.Context, req *api.GenerateRequest) error {
	for turn := 0; turn < l.config.MaxTurns; turn++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if l.config.Debug {
			fmt.Fprintf(os.Stderr, "[agent] turn %d/%d\n", turn+1, l.config.MaxTurns)
		}

		// Step 1: Call the API
		modelParts, err := l.callModel(ctx, req)
		if err != nil {
			return err
		}

		// Step 2: Extract function calls from the response
		var functionCalls []api.FunctionCall
		for _, part := range modelParts {
			if part.FunctionCall != nil {
				functionCalls = append(functionCalls, *part.FunctionCall)
			}
		}

		// Preserve thoughtSignature on all parts
		modelParts = ensureThoughtSignatures(modelParts)

		// Step 3: If no function calls, we're done
		if len(functionCalls) == 0 {
			// Append the model's response to conversation history so it's preserved for future turns
			req.Request.Contents = append(req.Request.Contents, api.Content{
				Role:  "model",
				Parts: modelParts,
			})
			return nil
		}

		// Step 4: Append the model's response to conversation history
		req.Request.Contents = append(req.Request.Contents, api.Content{
			Role:  "model",
			Parts: modelParts,
		})

		// Step 5: Execute all function calls and collect results
		var resultParts []api.Part
		for _, fc := range functionCalls {
			if l.config.Debug {
				fmt.Fprintf(os.Stderr, "[agent] calling tool: %s\n", fc.Name)
			}

			// Write tool call to formatter
			l.formatter.WriteToolCall(fc.Name, fc.Args)

			result, execErr := l.executeTool(ctx, fc)
			if execErr != nil {
				result = map[string]interface{}{"error": execErr.Error()}
			}

			if l.config.Debug {
				fmt.Fprintf(os.Stderr, "[agent] tool %s result keys: ", fc.Name)
				for k := range result {
					fmt.Fprintf(os.Stderr, "%s ", k)
				}
				fmt.Fprintf(os.Stderr, "\n")
			}

			// Write tool result to formatter
			l.formatter.WriteToolResult(fc.Name, result, execErr != nil)

			resultParts = append(resultParts, api.Part{
				FunctionResp: &api.FunctionResp{
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		// Step 6: Append tool results as "user" role (Gemini API convention)
		req.Request.Contents = append(req.Request.Contents, api.Content{
			Role:  "user",
			Parts: resultParts,
		})

		// Loop continues to next turn
	}

	return fmt.Errorf("agent loop: maximum turns (%d) reached", l.config.MaxTurns)
}

// callModel calls the API and returns the model's response parts.
// For streaming mode, text is written to the formatter in real-time.
func (l *Loop) callModel(ctx context.Context, req *api.GenerateRequest) ([]api.Part, error) {
	if l.config.Streaming {
		return l.callModelStreaming(ctx, req)
	}
	return l.callModelNonStreaming(ctx, req)
}

func (l *Loop) callModelStreaming(ctx context.Context, req *api.GenerateRequest) ([]api.Part, error) {
	stream, err := l.apiClient.GenerateStream(ctx, req)
	if err != nil {
		return nil, err
	}

	var parts []api.Part
	var currentText string
	var lastTextSignature string

	for event := range stream {
		switch event.Type {
		case "error":
			return nil, fmt.Errorf("%s", event.Error)
		case "content":
			if event.Text != "" {
				currentText += event.Text
				l.formatter.WriteStreamEvent(&event)
			}
			if event.ThoughtSignature != "" {
				lastTextSignature = event.ThoughtSignature
			}
		case "tool_call":
			// Flush accumulated text as a part before adding tool call
			if currentText != "" {
				parts = append(parts, api.Part{
					Text:             currentText,
					ThoughtSignature: lastTextSignature,
				})
				currentText = ""
				lastTextSignature = ""
			}
			if event.ToolCall != nil {
				part := api.Part{FunctionCall: event.ToolCall}
				if event.ThoughtSignature != "" {
					part.ThoughtSignature = event.ThoughtSignature
				}
				parts = append(parts, part)
			}
		case "done":
			l.formatter.WriteStreamEvent(&event)
		case "start":
			l.formatter.WriteStreamEvent(&event)
		}
	}

	// Flush any remaining accumulated text
	if currentText != "" {
		parts = append(parts, api.Part{
			Text:             currentText,
			ThoughtSignature: lastTextSignature,
		})
	}

	if l.config.Debug {
		fmt.Fprintf(os.Stderr, "[agent] streaming collected %d parts (text=%q truncated to 80)\n", len(parts), truncate(currentText, 80))
		for i, p := range parts {
			if p.Text != "" {
				fmt.Fprintf(os.Stderr, "[agent]   part[%d]: text len=%d sig=%q\n", i, len(p.Text), truncate(p.ThoughtSignature, 20))
			}
			if p.FunctionCall != nil {
				fmt.Fprintf(os.Stderr, "[agent]   part[%d]: functionCall=%s sig=%q\n", i, p.FunctionCall.Name, truncate(p.ThoughtSignature, 20))
			}
		}
	}

	return parts, nil
}

func (l *Loop) callModelNonStreaming(ctx context.Context, req *api.GenerateRequest) ([]api.Part, error) {
	resp, err := l.apiClient.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	var parts []api.Part
	hasFunctionCalls := false

	for _, candidate := range resp.Response.Candidates {
		for _, part := range candidate.Content.Parts {
			parts = append(parts, part)
			if part.FunctionCall != nil {
				hasFunctionCalls = true
			}
		}
	}

	// If no function calls, output the response
	if !hasFunctionCalls {
		l.formatter.WriteResponse(resp)
	}

	return parts, nil
}

// executeTool dispatches to built-in or MCP tools.
func (l *Loop) executeTool(ctx context.Context, fc api.FunctionCall) (map[string]interface{}, error) {
	// Try built-in tools first
	if tool, ok := l.registry.Get(fc.Name); ok {
		result, err := tool.Execute(ctx, fc.Args)
		if err != nil {
			return nil, err
		}
		return result.Content, nil
	}

	// Try MCP tools
	if ref, ok := l.registry.GetMCPRef(fc.Name); ok {
		client, ok := l.mcpClients[ref.ServerName]
		if !ok {
			return nil, fmt.Errorf("MCP server %q not connected", ref.ServerName)
		}
		resultText, err := client.CallTool(ctx, ref.ToolName, fc.Args)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"result": resultText}, nil
	}

	return nil, fmt.Errorf("unknown tool: %s", fc.Name)
}

// ensureThoughtSignatures adds synthetic thought signatures to FunctionCall parts
// that don't already have one. This is required by the Gemini API's thinking mode.
func ensureThoughtSignatures(parts []api.Part) []api.Part {
	for i, part := range parts {
		if part.FunctionCall != nil && part.ThoughtSignature == "" {
			parts[i].ThoughtSignature = SyntheticThoughtSignature
		}
	}
	return parts
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
