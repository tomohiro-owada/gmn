// Package cmd provides MCP command for g.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/k-sub1995/g/internal/config"
	"github.com/k-sub1995/g/internal/extension"
	"github.com/k-sub1995/g/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP (Model Context Protocol) commands",
	Long: `MCP (Model Context Protocol) commands provide functionality for
interacting with external tools and services via the Model Context Protocol.`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available MCP servers and their tools",
	RunE:  runMCPList,
}

var mcpCallCmd = &cobra.Command{
	Use:   "call <server> <tool> [args...]",
	Short: "Call an MCP tool",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runMCPCall,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpCallCmd)
}

func mergeExtensionMCPServers(cfg *config.Config) {
	cwd, _ := os.Getwd()
	extensions, _ := extension.LoadAll(cwd)
	for _, ext := range extensions {
		for serverName, serverCfg := range ext.MCPServers {
			if _, exists := cfg.MCPServers[serverName]; !exists {
				cfg.MCPServers[serverName] = serverCfg
			}
		}
	}
}

func runMCPList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mergeExtensionMCPServers(cfg)

	if len(cfg.MCPServers) == 0 {
		fmt.Println("No MCP servers configured.")
		fmt.Println("Add servers to ~/.gemini/settings.json under 'mcpServers' or install extensions")
		return nil
	}

	ctx := context.Background()

	for name, serverCfg := range cfg.MCPServers {
		fmt.Printf("=== %s ===\n", name)

		if serverCfg.Command == "" {
			fmt.Printf("  (HTTP/SSE transport - not yet supported)\n\n")
			continue
		}

		client, err := mcp.NewClient(serverCfg.Command, serverCfg.Args, serverCfg.Env, serverCfg.CWD)
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		if err := client.Initialize(ctx); err != nil {
			fmt.Printf("  Error initializing: %v\n\n", err)
			client.Close()
			continue
		}

		fmt.Printf("  Server: %s %s\n", client.ServerName, client.ServerVersion)
		fmt.Printf("  Tools:\n")
		for _, tool := range client.Tools {
			fmt.Printf("    - %s", tool.Name)
			if tool.Description != "" {
				fmt.Printf(": %s", tool.Description)
			}
			fmt.Println()
		}
		fmt.Println()

		client.Close()
	}

	return nil
}

func runMCPCall(cmd *cobra.Command, args []string) error {
	serverName := args[0]
	toolName := args[1]

	// Parse tool arguments (key=value pairs)
	toolArgs := make(map[string]interface{})
	for _, arg := range args[2:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			// Try to parse as JSON first
			var val interface{}
			if err := json.Unmarshal([]byte(parts[1]), &val); err != nil {
				// Fall back to string
				val = parts[1]
			}
			toolArgs[parts[0]] = val
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mergeExtensionMCPServers(cfg)

	serverCfg, ok := cfg.MCPServers[serverName]
	if !ok {
		return fmt.Errorf("MCP server '%s' not found in config or extensions", serverName)
	}

	if serverCfg.Command == "" {
		return fmt.Errorf("HTTP/SSE transport not yet supported")
	}

	ctx := context.Background()

	client, err := mcp.NewClient(serverCfg.Command, serverCfg.Args, serverCfg.Env, serverCfg.CWD)
	if err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}
	defer client.Close()

	if err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "Calling %s.%s with args: %v\n", serverName, toolName, toolArgs)
	}

	result, err := client.CallTool(ctx, toolName, toolArgs)
	if err != nil {
		return fmt.Errorf("tool call failed: %w", err)
	}

	fmt.Println(result)
	return nil
}
