// Package cmd provides the CLI commands for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tomohiro-owada/gmn/internal/agent"
	"github.com/tomohiro-owada/gmn/internal/api"
	"github.com/tomohiro-owada/gmn/internal/auth"
	"github.com/tomohiro-owada/gmn/internal/config"
	"github.com/tomohiro-owada/gmn/internal/extension"
	"github.com/tomohiro-owada/gmn/internal/input"
	"github.com/tomohiro-owada/gmn/internal/mcp"
	"github.com/tomohiro-owada/gmn/internal/output"
	"github.com/tomohiro-owada/gmn/internal/prompt"
	"github.com/tomohiro-owada/gmn/internal/tools"
)

var (
	version = "dev"

	prompt_             string
	model               string
	outputFormat        string
	files               []string
	timeout             time.Duration
	debug               bool
	rawOutput           bool
	acceptRawOutputRisk bool
	maxTurns            int
	yolo                bool
	sandbox             bool
	noAgent             bool
)

var rootCmd = &cobra.Command{
	Use:   "gmn [prompt]",
	Short: "A lightweight, non-interactive Gemini CLI",
	Long: `gmn is a lightweight reimplementation of Google's Gemini CLI
focused on non-interactive use cases. It reuses authentication from
the official Gemini CLI (~/.gemini/).

Examples:
  gmn "Hello, world"
  gmn "Explain Go generics" -m gemini-2.5-pro
  cat file.go | gmn "Review this code"
  gmn "Add error handling" -f main.go
  gmn "Fix the tests" --yolo`,
	RunE:    run,
	Version: version,
	Args:    cobra.MaximumNArgs(1),
}

func init() {
	rootCmd.Flags().StringVarP(&prompt_, "prompt", "p", "", "Prompt to send to Gemini (required)")
	rootCmd.Flags().StringVarP(&model, "model", "m", "pro", "Model to use (aliases: auto, pro, flash, flash-lite)")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text, json, stream-json")
	rootCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
	rootCmd.Flags().BoolVar(&rawOutput, "raw-output", false, "Disable sanitization of model output (allow ANSI escape sequences)")
	rootCmd.Flags().BoolVar(&acceptRawOutputRisk, "accept-raw-output-risk", false, "Suppress security warning when using --raw-output")
	rootCmd.Flags().IntVar(&maxTurns, "max-turns", 25, "Maximum agent loop turns")
	rootCmd.Flags().BoolVar(&yolo, "yolo", false, "Auto-approve shell commands (no confirmation)")
	rootCmd.Flags().BoolVar(&sandbox, "sandbox", false, "Restrict file writes to working directory")
	rootCmd.Flags().BoolVar(&noAgent, "no-agent", false, "Disable agent mode (single-turn, no tools)")
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func run(cmd *cobra.Command, args []string) error {
	// Handle positional argument as prompt
	if len(args) > 0 {
		prompt_ = args[0]
	}
	// Setup context with timeout and signal handling
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Upstream ref: 799007354 - sanitize ANSI escape sequences in non-interactive output
	sanitize := !rawOutput && !acceptRawOutputRisk
	if rawOutput && !acceptRawOutputRisk && outputFormat == "text" {
		fmt.Fprintln(os.Stderr, "[WARNING] --raw-output is enabled. Model output is not sanitized and may contain harmful ANSI sequences (e.g. for phishing or command injection). Use --accept-raw-output-risk to suppress this warning.")
	}

	// Create formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout, os.Stderr, sanitize)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		formatter.WriteError(fmt.Errorf("failed to load config: %w", err))
		return err
	}

	// Load credentials
	authMgr, err := auth.NewManager()
	if err != nil {
		formatter.WriteError(fmt.Errorf("failed to initialize auth: %w", err))
		return err
	}

	creds, err := authMgr.LoadCredentials()
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	// Refresh if expired
	if creds.IsExpired() {
		if debug {
			fmt.Fprintln(os.Stderr, "Token expired, refreshing...")
		}
		creds, err = authMgr.RefreshToken(creds)
		if err != nil {
			formatter.WriteError(err)
			return err
		}
	}

	// Prepare input
	inputText, err := input.PrepareInput(prompt_, files)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	if inputText == "" {
		err := fmt.Errorf("no input provided")
		formatter.WriteError(err)
		return err
	}

	// Resolve model alias to concrete model name
	// Upstream ref: 61d92c4a2 - Remove previewFeatures and default to Gemini 3
	resolvedModel := config.ResolveModel(model, false, false, true)
	if debug && resolvedModel != model {
		fmt.Fprintf(os.Stderr, "Model resolved: %s → %s\n", model, resolvedModel)
	}
	model = resolvedModel

	// Create API client
	httpClient := authMgr.HTTPClient(creds)
	apiClient := api.NewClient(httpClient, version)

	// Try to load cached project ID first
	cachedState, _ := config.LoadCachedState()
	projectID := cachedState.ProjectID

	// If no cached project ID, fetch from API
	if projectID == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "Loading Code Assist status...")
		}
		loadResp, err := apiClient.LoadCodeAssist(ctx)
		if err != nil {
			formatter.WriteError(fmt.Errorf("failed to load Code Assist: %w", err))
			return err
		}
		projectID = loadResp.CloudAICompanionProject

		// Upstream ref: f4e73191d - fix tier eligibility for unlicensed users
		if projectID == "" {
			if len(loadResp.IneligibleTiers) > 0 {
				var reasons []string
				for _, tier := range loadResp.IneligibleTiers {
					if tier.ReasonMessage != "" {
						reasons = append(reasons, tier.ReasonMessage)
					}
				}
				if len(reasons) > 0 {
					errMsg := fmt.Errorf("unable to use Gemini: %s", strings.Join(reasons, ", "))
					formatter.WriteError(errMsg)
					return errMsg
				}
			}
			errMsg := fmt.Errorf("unable to use Gemini: no project ID available. Please run 'gemini' to set up your account")
			formatter.WriteError(errMsg)
			return errMsg
		}

		// Cache the project ID for next time
		// Upstream ref: e70978906 - fallback to STANDARD when tier ID is missing
		userTier := "STANDARD"
		if loadResp.PaidTier != nil && loadResp.PaidTier.ID != "" {
			userTier = loadResp.PaidTier.ID
		} else if loadResp.CurrentTier != nil && loadResp.CurrentTier.ID != "" {
			userTier = loadResp.CurrentTier.ID
		}
		_ = config.SaveCachedState(&config.CachedState{
			ProjectID: projectID,
			UserTier:  userTier,
		})

		if debug {
			fmt.Fprintf(os.Stderr, "Project ID: %s (cached)\n", projectID)
			if loadResp.CurrentTier != nil {
				fmt.Fprintf(os.Stderr, "Tier: %s\n", loadResp.CurrentTier.ID)
			}
		}
	} else if debug {
		fmt.Fprintf(os.Stderr, "Using cached Project ID: %s\n", projectID)
	}

	// Generate a simple user prompt ID
	userPromptID := fmt.Sprintf("gmn-%d", time.Now().UnixNano())

	// Get working directory
	workDir, _ := os.Getwd()

	// Build request (Code Assist API format)
	req := &api.GenerateRequest{
		Model:        model,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request: api.InnerRequest{
			Contents: []api.Content{{
				Role:  "user",
				Parts: []api.Part{{Text: inputText}},
			}},
			Config: api.GenerationConfig{
				Temperature:     1.0,
				TopP:            0.95,
				MaxOutputTokens: 65536,
			},
		},
	}

	// --- Agent mode ---
	if !noAgent {
		// Create web search callback
		webSearchFn := func(ctx context.Context, query string) (string, []tools.WebSource, error) {
			resp, err := apiClient.WebSearch(ctx, projectID, model, query)
			if err != nil {
				return "", nil, err
			}
			// Extract text from response
			var text string
			var sources []tools.WebSource
			if resp.Response != nil && len(resp.Response.Candidates) > 0 {
				cand := resp.Response.Candidates[0]
				for _, part := range cand.Content.Parts {
					text += part.Text
				}
				// Extract grounding sources
				if cand.GroundingMetadata != nil {
					for _, chunk := range cand.GroundingMetadata.GroundingChunks {
						if chunk.Web != nil {
							sources = append(sources, tools.WebSource{
								Title: chunk.Web.Title,
								URI:   chunk.Web.URI,
							})
						}
					}
				}
			}
			return text, sources, nil
		}

		// Load extensions and merge MCP servers
		extensions, extErr := extension.LoadAll(workDir)
		if extErr != nil && debug {
			fmt.Fprintf(os.Stderr, "[ext] failed to load extensions: %v\n", extErr)
		}
		if cfg != nil {
			for _, ext := range extensions {
				for serverName, serverCfg := range ext.MCPServers {
					if _, exists := cfg.MCPServers[serverName]; !exists {
						cfg.MCPServers[serverName] = serverCfg
						if debug {
							fmt.Fprintf(os.Stderr, "[ext] loaded MCP server %q from extension %q\n", serverName, ext.Name)
						}
					} else if debug {
						fmt.Fprintf(os.Stderr, "[ext] MCP server %q from extension %q skipped (already configured)\n", serverName, ext.Name)
					}
				}
			}
		}

		// Create tool registry
		registry := tools.NewRegistry(tools.RegistryOptions{
			WorkDir:     workDir,
			AutoApprove: yolo,
			Sandbox:     sandbox,
			Debug:       debug,
			WebSearch:   webSearchFn,
		})

		// Initialize MCP clients and register tools
		mcpClients := make(agent.MCPClients)
		var mcpDecls []api.FunctionDecl

		if cfg != nil {
			for serverName, serverCfg := range cfg.MCPServers {
				if serverCfg.Command == "" {
					continue // Skip HTTP/SSE (not yet supported)
				}
				client, err := mcp.NewClient(serverCfg.Command, serverCfg.Args, serverCfg.Env, serverCfg.CWD)
				if err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] failed to create client for %s: %v\n", serverName, err)
					}
					continue
				}
				if err := client.Initialize(ctx); err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] failed to initialize %s: %v\n", serverName, err)
					}
					client.Close()
					continue
				}
				mcpClients[serverName] = client
				defer client.Close()

				for _, tool := range client.Tools {
					prefixedName := serverName + "__" + tool.Name
					registry.RegisterMCPTool(serverName, prefixedName, tool.Name)
					mcpDecls = append(mcpDecls, api.FunctionDecl{
						Name:        prefixedName,
						Description: tool.Description,
						Parameters:  sanitizeSchema(tool.InputSchema),
					})
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] registered tool: %s\n", prefixedName)
					}
				}
			}
		}

		// Collect extension context files
		var extContextFiles []string
		for _, ext := range extensions {
			extContextFiles = append(extContextFiles, ext.ContextFiles...)
		}

		// Set system instruction
		req.Request.SystemInstruction = prompt.BuildSystemInstruction(prompt.Options{
			WorkDir:           workDir,
			ExtensionContexts: extContextFiles,
		})

		// Set tools
		allDecls := registry.AllDeclarations()
		allDecls = append(allDecls, mcpDecls...)
		req.Request.Tools = []api.Tool{{FunctionDeclarations: allDecls}}

		if debug {
			fmt.Fprintf(os.Stderr, "[agent] %d built-in tools, %d MCP tools registered\n",
				len(registry.AllDeclarations()), len(mcpDecls))
		}

		// Run agent loop
		streaming := outputFormat != "json"
		loop := agent.NewLoop(apiClient, registry, mcpClients, formatter, agent.Config{
			MaxTurns:  maxTurns,
			Streaming: streaming,
			Debug:     debug,
		})

		if err := loop.Run(ctx, req); err != nil {
			formatter.WriteError(err)
			return err
		}
		return nil
	}

	// --- Legacy single-turn mode (--no-agent) ---
	switch outputFormat {
	case "json":
		return runNonStreaming(ctx, apiClient, req, formatter)
	default:
		return runStreaming(ctx, apiClient, req, formatter)
	}
}

func runNonStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	resp, err := client.Generate(ctx, req)
	if err != nil {
		formatter.WriteError(err)
		return err
	}
	return formatter.WriteResponse(resp)
}

func runStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	stream, err := client.GenerateStream(ctx, req)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	for event := range stream {
		if event.Type == "error" {
			formatter.WriteError(fmt.Errorf(event.Error))
			return fmt.Errorf(event.Error)
		}
		if err := formatter.WriteStreamEvent(&event); err != nil {
			return err
		}
	}

	return nil
}

// sanitizeSchema removes fields from MCP tool inputSchema that the Gemini API
// does not accept (e.g. "$schema"). The upstream Gemini CLI performs equivalent
// sanitization in its converter layer.
func sanitizeSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(raw, &schema); err != nil {
		return raw
	}
	delete(schema, "$schema")
	sanitized, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return sanitized
}
