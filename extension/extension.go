// Package extension provides extension loading for gmn.
// Extensions are discovered from ~/.gemini/extensions/ directory,
// matching the Gemini CLI extension system.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/k-sub1995/g/internal/config"
)

// Manifest represents a gemini-extension.json file.
type Manifest struct {
	Name            string                            `json:"name"`
	Version         string                            `json:"version"`
	ContextFileName interface{}                       `json:"contextFileName"` // string or []string
	MCPServers      map[string]config.MCPServerConfig `json:"mcpServers"`
}

// Extension holds a parsed, variable-hydrated extension ready for use.
type Extension struct {
	Name         string
	Version      string
	Path         string // absolute path to extension directory
	MCPServers   map[string]config.MCPServerConfig
	ContextFiles []string // absolute paths to existing context files
}

// enablementConfig maps extension name to enablement rules.
type enablementConfig map[string]struct {
	Overrides []string `json:"overrides"`
}

// LoadAll discovers and loads all enabled extensions from ~/.gemini/extensions/.
// currentPath is the current working directory, used for enablement matching.
func LoadAll(currentPath string) ([]Extension, error) {
	geminiDir, err := config.GeminiDir()
	if err != nil {
		return nil, err
	}
	extensionsDir := filepath.Join(geminiDir, "extensions")

	info, err := os.Stat(extensionsDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	enablement := loadEnablementConfig(filepath.Join(extensionsDir, "extension-enablement.json"))

	entries, err := os.ReadDir(extensionsDir)
	if err != nil {
		return nil, err
	}

	var extensions []Extension
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		extDir := filepath.Join(extensionsDir, entry.Name())
		ext, err := loadExtension(extDir)
		if err != nil {
			continue // skip broken extensions
		}
		if !isEnabled(ext.Name, currentPath, enablement) {
			continue
		}
		extensions = append(extensions, *ext)
	}
	return extensions, nil
}

func loadExtension(extDir string) (*Extension, error) {
	manifestPath := filepath.Join(extDir, "gemini-extension.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	// Pre-validate name/version before hydration
	var raw struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Name == "" || raw.Version == "" {
		return nil, fmt.Errorf("invalid extension: missing name or version")
	}

	// Hydrate variables on raw JSON string
	absExtDir, _ := filepath.Abs(extDir)
	hydrated := hydrateVariables(string(data), map[string]string{
		"extensionPath": absExtDir,
		"/":             string(filepath.Separator),
		"pathSeparator": string(filepath.Separator),
	})

	var manifest Manifest
	if err := json.Unmarshal([]byte(hydrated), &manifest); err != nil {
		return nil, err
	}

	// Resolve context files
	contextFileNames := getContextFileNames(manifest.ContextFileName)
	var contextFiles []string
	for _, name := range contextFileNames {
		p := filepath.Join(absExtDir, name)
		if _, err := os.Stat(p); err == nil {
			contextFiles = append(contextFiles, p)
		}
	}

	return &Extension{
		Name:         manifest.Name,
		Version:      manifest.Version,
		Path:         absExtDir,
		MCPServers:   manifest.MCPServers,
		ContextFiles: contextFiles,
	}, nil
}

func hydrateVariables(s string, vars map[string]string) string {
	for key, val := range vars {
		s = strings.ReplaceAll(s, "${"+key+"}", val)
	}
	return s
}

func getContextFileNames(v interface{}) []string {
	if v == nil {
		return []string{"GEMINI.md"}
	}
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		var names []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		if len(names) == 0 {
			return []string{"GEMINI.md"}
		}
		return names
	default:
		return []string{"GEMINI.md"}
	}
}

func loadEnablementConfig(path string) enablementConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg enablementConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg
}

// isEnabled checks if an extension is enabled for the given path.
// Last matching rule wins; extensions are enabled by default.
func isEnabled(extName, currentPath string, enablement enablementConfig) bool {
	if enablement == nil {
		return true
	}
	extCfg, ok := enablement[extName]
	if !ok {
		return true
	}

	enabled := true
	normalizedPath := normalizePath(currentPath)
	for _, rule := range extCfg.Overrides {
		isDisable := strings.HasPrefix(rule, "!")
		baseRule := rule
		if isDisable {
			baseRule = rule[1:]
		}
		if matchesPath(baseRule, normalizedPath) {
			enabled = !isDisable
		}
	}
	return enabled
}

func matchesPath(pattern, path string) bool {
	pattern = normalizePath(pattern)
	if strings.HasSuffix(pattern, "*/") {
		prefix := strings.TrimSuffix(pattern, "*/")
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern || strings.HasPrefix(path, pattern)
}

func normalizePath(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}
	return p
}
