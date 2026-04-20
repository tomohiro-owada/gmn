// Package config provides configuration loading for geminimini.
// This file was modified from the original Gemini CLI.
// Copyright 2025 Google LLC
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	geminiDir    = ".gemini"
	settingsFile = "settings.json"
)

// Config is the main configuration structure
type Config struct {
	Security           SecurityConfig                     `json:"security"`
	MCPServers         map[string]MCPServerConfig         `json:"mcpServers"`
	RequiredMCPServers map[string]RequiredMCPServerConfig `json:"requiredMcpServers,omitempty"`
	General            GeneralConfig                      `json:"general"`
	Output             OutputConfig                       `json:"output"`
}

// SecurityConfig holds security-related settings
type SecurityConfig struct {
	Auth AuthConfig `json:"auth"`
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	SelectedType string `json:"selectedType"`
}

// MCPServerConfig holds MCP server configuration
type MCPServerConfig struct {
	// Stdio transport
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	CWD     string            `json:"cwd,omitempty"`

	// HTTP/SSE transport
	URL     string            `json:"url,omitempty"`
	Type    string            `json:"type,omitempty"` // "sse" | "http"
	Headers map[string]string `json:"headers,omitempty"`

	// Common
	Timeout      int      `json:"timeout,omitempty"`
	Trust        bool     `json:"trust,omitempty"`
	Description  string   `json:"description,omitempty"`
	IncludeTools []string `json:"includeTools,omitempty"`
	ExcludeTools []string `json:"excludeTools,omitempty"`

	// Auth and routing metadata for compatibility with newer Gemini CLI config.
	AuthProviderType     string          `json:"authProviderType,omitempty"`
	OAuth                *MCPOAuthConfig `json:"oauth,omitempty"`
	TargetAudience       string          `json:"targetAudience,omitempty"`
	TargetServiceAccount string          `json:"targetServiceAccount,omitempty"`
}

// MCPOAuthConfig holds optional OAuth configuration for HTTP/SSE MCP servers.
type MCPOAuthConfig struct {
	Scopes       []string `json:"scopes,omitempty"`
	ClientID     string   `json:"clientId,omitempty"`
	ClientSecret string   `json:"clientSecret,omitempty"`
}

// RequiredMCPServerConfig matches newer Gemini CLI requiredMcpServers entries.
// It is separate from MCPServerConfig so omitted fields can have different defaults.
type RequiredMCPServerConfig struct {
	URL                  string            `json:"url,omitempty"`
	Type                 string            `json:"type,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"`
	Timeout              int               `json:"timeout,omitempty"`
	Trust                *bool             `json:"trust,omitempty"`
	Description          string            `json:"description,omitempty"`
	IncludeTools         []string          `json:"includeTools,omitempty"`
	ExcludeTools         []string          `json:"excludeTools,omitempty"`
	AuthProviderType     string            `json:"authProviderType,omitempty"`
	OAuth                *MCPOAuthConfig   `json:"oauth,omitempty"`
	TargetAudience       string            `json:"targetAudience,omitempty"`
	TargetServiceAccount string            `json:"targetServiceAccount,omitempty"`
}

// GeneralConfig holds general settings
// Upstream ref: v0.30.0 removed previewFeatures; Gemini 3 is now default.
type GeneralConfig struct{}

// OutputConfig holds output settings
type OutputConfig struct {
	Format string `json:"format"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Security: SecurityConfig{
			Auth: AuthConfig{
				SelectedType: "oauth-personal",
			},
		},
		MCPServers:         make(map[string]MCPServerConfig),
		RequiredMCPServers: make(map[string]RequiredMCPServerConfig),
		General:            GeneralConfig{},
		Output: OutputConfig{
			Format: "text",
		},
	}
}

// GeminiDir returns the path to ~/.gemini
func GeminiDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, geminiDir), nil
}

// Load loads the configuration from ~/.gemini/settings.json
func Load() (*Config, error) {
	geminiPath, err := GeminiDir()
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	// Load global settings
	globalPath := filepath.Join(geminiPath, settingsFile)
	if err := loadFile(globalPath, cfg); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load project settings (optional, overrides global)
	cwd, err := os.Getwd()
	if err == nil {
		projectPath := filepath.Join(cwd, geminiDir, settingsFile)
		if err := loadFile(projectPath, cfg); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	applyRequiredMCPServers(cfg)

	return cfg, nil
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, cfg)
}

func applyRequiredMCPServers(cfg *Config) {
	if len(cfg.RequiredMCPServers) == 0 {
		return
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServerConfig)
	}

	for name, server := range cfg.RequiredMCPServers {
		includeTools := slices.Clone(server.IncludeTools)
		if len(includeTools) > 0 {
			slices.Sort(includeTools)
		}
		excludeTools := slices.Clone(server.ExcludeTools)
		if len(excludeTools) > 0 {
			slices.Sort(excludeTools)
		}

		trust := true
		if server.Trust != nil {
			trust = *server.Trust
		}

		cfg.MCPServers[name] = MCPServerConfig{
			URL:                  server.URL,
			Type:                 server.Type,
			Headers:              server.Headers,
			Timeout:              server.Timeout,
			Trust:                trust,
			Description:          server.Description,
			IncludeTools:         includeTools,
			ExcludeTools:         excludeTools,
			AuthProviderType:     server.AuthProviderType,
			OAuth:                server.OAuth,
			TargetAudience:       server.TargetAudience,
			TargetServiceAccount: server.TargetServiceAccount,
		}
	}
}

// CachedState represents cached state for geminimini
type CachedState struct {
	ProjectID string `json:"projectId,omitempty"`
	UserTier  string `json:"userTier,omitempty"`
}

// LoadCachedState loads the cached state from gmn_state.json
func LoadCachedState() (*CachedState, error) {
	geminiPath, err := GeminiDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(geminiPath, "gmn_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CachedState{}, nil
		}
		return nil, err
	}

	var state CachedState
	if err := json.Unmarshal(data, &state); err != nil {
		return &CachedState{}, nil
	}

	return &state, nil
}

// SaveCachedState saves the cached state to gmn_state.json with retry logic.
// Upstream ref: 8379099e - add exponential backoff for transient file operation failures
func SaveCachedState(state *CachedState) error {
	geminiPath, err := GeminiDir()
	if err != nil {
		return err
	}

	path := filepath.Join(geminiPath, "gmn_state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := path + ".tmp"

	const maxRetries = 5
	const baseDelay = 50 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := os.WriteFile(tmpPath, data, 0600); err != nil {
			if isTransientFileError(err) && attempt < maxRetries {
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * baseDelay)
				continue
			}
			return err
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			if isTransientFileError(err) && attempt < maxRetries {
				time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * baseDelay)
				continue
			}
			return err
		}
		return nil
	}

	return os.WriteFile(path, data, 0600)
}

func isTransientFileError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return false
	}
	msg := err.Error()
	for _, s := range []string{"device or resource busy", "resource temporarily unavailable"} {
		if len(msg) >= len(s) && containsLower(msg, s) {
			return true
		}
	}
	return false
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
