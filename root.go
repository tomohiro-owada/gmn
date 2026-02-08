// Package cmd provides the CLI commands for g.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"
	"github.com/k-sub1995/g/internal/agent"
	"github.com/k-sub1995/g/internal/api"
	"github.com/k-sub1995/g/internal/auth"
	"github.com/k-sub1995/g/internal/config"
	"github.com/k-sub1995/g/internal/extension"
	"github.com/k-sub1995/g/internal/input"
	"github.com/k-sub1995/g/internal/mcp"
	"github.com/k-sub1995/g/internal/output"
	"github.com/k-sub1995/g/internal/prompt"
	"github.com/k-sub1995/g/internal/tools"
	"github.com/spf13/cobra"
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
	Use:   "g [prompt]",
	Short: "A lightweight, non-interactive Gemini CLI",
	Long: `g is a lightweight reimplementation of Google's Gemini CLI
focused on non-interactive and TUI use cases. It reuses authentication from
the official Gemini CLI (~/.gemini/).

Examples:
  g "Hello, world"
  g "Explain Go generics" -m gemini-2.5-pro
  cat file.go | g "Review this code"
  g "Add error handling" -f main.go
  g "Fix the tests" --yolo`,
	RunE: run,

	Args: cobra.MaximumNArgs(1),
}

func init() {
	rootCmd.Flags().StringVarP(&prompt_, "prompt", "p", "", "Prompt to send to Gemini (required)")
	rootCmd.Flags().StringVarP(&model, "model", "m", "gemini-2.5-flash", "Model to use")
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
	// No need to set rootCmd.Version directly here, as we'll use a dedicated version command.
}

func run(cmd *cobra.Command, args []string) error {
	// Handle positional argument as prompt
	if len(args) > 0 {
		prompt_ = args[0]
	}
	// Setup context with timeout and signal handling
	// Note: In REPL mode, this timeout applies to the GLOBAL session, which might not be what we want.
	// We might want to use background context for REPL and timeout per turn.
	// For now, let's keep it simple and use a long timeout or background for REPL main loop.

	ctx, cancel := context.WithCancel(context.Background())
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

	// Determine mode: REPL if no input and no files provided
	isREPL := inputText == "" && len(files) == 0

	if !isREPL && inputText == "" {
		err := fmt.Errorf("no input provided")
		formatter.WriteError(err)
		return err
	}

	// State for lazy initialization
	var (
		apiClient  *api.Client
		projectID  string
		agentLoop  *agent.Loop
		mcpClients agent.MCPClients
		registry   *tools.Registry
		isInit     bool
		req        *api.GenerateRequest
	)

	// Lazy initialization function
	initialize := func(ctx context.Context) error {
		if isInit {
			return nil // Already initialized
		}

		if debug {
			fmt.Fprintln(os.Stderr, "Initializing backend...")
		}

		// Create API client
		httpClient := authMgr.HTTPClient(creds)
		apiClient = api.NewClient(httpClient)

		// Try to load cached project ID first
		cachedState, _ := config.LoadCachedState()
		projectID = cachedState.ProjectID

		// If no cached project ID, fetch from API
		if projectID == "" {
			if debug {
				fmt.Fprintln(os.Stderr, "Loading Code Assist status...")
			}
			loadResp, err := apiClient.LoadCodeAssist(ctx)
			if err != nil {
				return fmt.Errorf("failed to load Code Assist: %w", err)
			}
			projectID = loadResp.CloudAICompanionProject

			if projectID == "" {
				if len(loadResp.IneligibleTiers) > 0 {
					var reasons []string
					for _, tier := range loadResp.IneligibleTiers {
						if tier.ReasonMessage != "" {
							reasons = append(reasons, tier.ReasonMessage)
						}
					}
					if len(reasons) > 0 {
						return fmt.Errorf("unable to use Gemini: %s", strings.Join(reasons, ", "))
					}
				}
				return fmt.Errorf("unable to use Gemini: no project ID available. Please run 'gemini' to set up your account")
			}

			// Cache the project ID
			userTier := ""
			if loadResp.CurrentTier != nil {
				userTier = loadResp.CurrentTier.ID
			}
			_ = config.SaveCachedState(&config.CachedState{
				ProjectID: projectID,
				UserTier:  userTier,
			})
			if debug {
				fmt.Fprintf(os.Stderr, "Project ID: %s (cached)\n", projectID)
			}
		} else if debug {
			fmt.Fprintf(os.Stderr, "Using cached Project ID: %s\n", projectID)
		}

		// --- Agent Setup ---
		if !noAgent {
			// Web search callback
			webSearchFn := func(ctx context.Context, query string) (string, []tools.WebSource, error) {
				resp, err := apiClient.WebSearch(ctx, projectID, model, query)
				if err != nil {
					return "", nil, err
				}
				var text string
				var sources []tools.WebSource
				if len(resp.Response.Candidates) > 0 {
					cand := resp.Response.Candidates[0]
					for _, part := range cand.Content.Parts {
						text += part.Text
					}
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

			// Get working directory for extensions
			workDir, _ := os.Getwd()

			// Load extensions
			extensions, extErr := extension.LoadAll(workDir)
			if extErr != nil && debug {
				fmt.Fprintf(os.Stderr, "[ext] failed to load extensions: %v\n", extErr)
			}
			if cfg != nil {
				for _, ext := range extensions {
					for serverName, serverCfg := range ext.MCPServers {
						if _, exists := cfg.MCPServers[serverName]; !exists {
							cfg.MCPServers[serverName] = serverCfg
						}
					}
				}
			}

			// Registry
			registry = tools.NewRegistry(tools.RegistryOptions{
				WorkDir:     workDir,
				AutoApprove: yolo,
				Sandbox:     sandbox,
				Debug:       debug,
				WebSearch:   webSearchFn,
			})

			// MCP Clients
			mcpClients = make(agent.MCPClients)
			var mcpDecls []api.FunctionDecl

			if cfg != nil {
				for serverName, serverCfg := range cfg.MCPServers {
					if serverCfg.Command == "" {
						continue
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
					// We can't defer close here easily, so we rely on process exit or explicit close if we add shutdown logic

					for _, tool := range client.Tools {
						prefixedName := serverName + "__" + tool.Name
						registry.RegisterMCPTool(serverName, prefixedName)
						mcpDecls = append(mcpDecls, api.FunctionDecl{
							Name:        prefixedName,
							Description: tool.Description,
							Parameters:  json.RawMessage(tool.InputSchema),
						})
					}
				}
			}

			// Extension contexts
			var extContextFiles []string
			for _, ext := range extensions {
				extContextFiles = append(extContextFiles, ext.ContextFiles...)
			}

			// System Instruction
			req.Request.SystemInstruction = prompt.BuildSystemInstruction(prompt.Options{
				WorkDir:           workDir,
				ExtensionContexts: extContextFiles,
			})

			// Tools
			allDecls := registry.AllDeclarations()
			allDecls = append(allDecls, mcpDecls...)
			req.Request.Tools = []api.Tool{{FunctionDeclarations: allDecls}}

			// Agent Loop
			streaming := outputFormat != "json"
			agentLoop = agent.NewLoop(apiClient, registry, mcpClients, formatter, agent.Config{
				MaxTurns:  maxTurns,
				Streaming: streaming,
				Debug:     debug,
			})
		}

		isInit = true
		return nil
	}

	// Generate a simple user prompt ID
	userPromptID := fmt.Sprintf("g-%d", time.Now().UnixNano())

	// Build base request
	req = &api.GenerateRequest{
		Model:        model,
		Project:      "", // Filled in initialize
		UserPromptID: userPromptID,
		Request: api.InnerRequest{
			Contents: []api.Content{}, // populated later
			Config: api.GenerationConfig{
				Temperature:     1.0,
				TopP:            0.95,
				MaxOutputTokens: 65536,
			},
		},
	}

	// Execution Logic
	runTurn := func(ctx context.Context) error {
		// Ensure initialized
		if !isInit {
			if err := initialize(ctx); err != nil {
				return err
			}
			// Update project ID in request
			req.Project = projectID
		}

		if !noAgent {
			return agentLoop.Run(ctx, req)
		}

		// Legacy mode
		switch outputFormat {
		case "json":
			return runNonStreaming(ctx, apiClient, req, formatter)
		default:
			return runStreaming(ctx, apiClient, req, formatter)
		}
	}

	if isREPL {
		// Check home directory warning
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cwd, _ := os.Getwd()
			if cwd == homeDir {
				// Use yellow color logic if possible, or just plain text
				fmt.Fprintln(os.Stderr, "Warning: you are running Gemini CLI in your home directory.")
				fmt.Fprintln(os.Stderr, "This warning can be disabled in (not implemented)")
				fmt.Fprintln(os.Stderr, "")
			}
		}

		// File count message
		if len(files) > 0 {
			fileLabel := "file"
			if len(files) > 1 {
				fileLabel = "files"
			}
			fmt.Fprintf(os.Stderr, "%d %s\n\n", len(files), fileLabel)
		}

		// use readline
		rl, err := readline.NewEx(&readline.Config{
			Prompt:          "> ",
			HistoryFile:     filepath.Join(os.TempDir(), "gmn_history"),
			InterruptPrompt: "^C",
			EOFPrompt:       "exit",
		})
		if err != nil {
			return err
		}
		defer rl.Close()

		// Placeholder hint (simulated)
		// readline doesn't support placeholder text easily without prompt manipulation,
		// but we can print a dim instruction once
		fmt.Fprintln(os.Stderr, "\033[2mType your message or @path/to/file\033[0m")

		for {
			line, err := rl.Readline()
			if err != nil {
				// EOF or Ctrl+C
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "exit" || line == "quit" {
				break
			}

			// Add user input to context
			req.Request.Contents = append(req.Request.Contents, api.Content{
				Role:  "user",
				Parts: []api.Part{{Text: line}},
			})

			// Create a per-turn context with timeout
			turnCtx, turnCancel := context.WithTimeout(context.Background(), timeout)
			err = runTurn(turnCtx)
			turnCancel()

			if err != nil {
				formatter.WriteError(err)
				// Don't exit REPL on error
			}

			// Newline handled by formatter usually, but REPL might need one
			// agent loop prints newlines
		}
		return nil
	}

	// Single turn mode
	// Determine if initial input was provided via files/prompt
	if inputText != "" {
		req.Request.Contents = append(req.Request.Contents, api.Content{
			Role:  "user",
			Parts: []api.Part{{Text: inputText}},
		})
	} else {
		// Fallback if no input and not isREPL (should be caught above)
		return fmt.Errorf("no input provided")
	}

	return runTurn(ctx)
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
