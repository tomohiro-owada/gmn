package extension

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHydrateVariables(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		vars   map[string]string
		expect string
	}{
		{
			name:  "extensionPath substitution",
			input: `{"cwd": "${extensionPath}"}`,
			vars:  map[string]string{"extensionPath": "/home/user/.gemini/extensions/my-ext"},
			expect: `{"cwd": "/home/user/.gemini/extensions/my-ext"}`,
		},
		{
			name:  "path separator substitution",
			input: `{"args": ["${extensionPath}${/}dist${/}index.js"]}`,
			vars: map[string]string{
				"extensionPath": "/home/user/.gemini/extensions/my-ext",
				"/":             "/",
			},
			expect: `{"args": ["/home/user/.gemini/extensions/my-ext/dist/index.js"]}`,
		},
		{
			name:  "pathSeparator alias",
			input: `{"args": ["${extensionPath}${pathSeparator}index.js"]}`,
			vars: map[string]string{
				"extensionPath": "/ext",
				"pathSeparator": "/",
			},
			expect: `{"args": ["/ext/index.js"]}`,
		},
		{
			name:   "no variables",
			input:  `{"command": "node"}`,
			vars:   map[string]string{"extensionPath": "/ext"},
			expect: `{"command": "node"}`,
		},
		{
			name:   "unknown variable left as-is",
			input:  `{"val": "${unknown}"}`,
			vars:   map[string]string{"extensionPath": "/ext"},
			expect: `{"val": "${unknown}"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hydrateVariables(tt.input, tt.vars)
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestGetContextFileNames(t *testing.T) {
	tests := []struct {
		name   string
		input  interface{}
		expect []string
	}{
		{
			name:   "nil defaults to GEMINI.md",
			input:  nil,
			expect: []string{"GEMINI.md"},
		},
		{
			name:   "single string",
			input:  "WORKSPACE-Context.md",
			expect: []string{"WORKSPACE-Context.md"},
		},
		{
			name:   "string array",
			input:  []interface{}{"a.md", "b.md"},
			expect: []string{"a.md", "b.md"},
		},
		{
			name:   "empty array defaults to GEMINI.md",
			input:  []interface{}{},
			expect: []string{"GEMINI.md"},
		},
		{
			name:   "unexpected type defaults to GEMINI.md",
			input:  42,
			expect: []string{"GEMINI.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getContextFileNames(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("got %v, want %v", got, tt.expect)
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name       string
		extName    string
		path       string
		enablement enablementConfig
		expect     bool
	}{
		{
			name:       "nil enablement = all enabled",
			extName:    "test",
			path:       "/home/user/project",
			enablement: nil,
			expect:     true,
		},
		{
			name:    "extension not in enablement = enabled",
			extName: "test",
			path:    "/home/user/project",
			enablement: enablementConfig{
				"other": {Overrides: []string{"/home/*"}},
			},
			expect: true,
		},
		{
			name:    "matching wildcard = enabled",
			extName: "test",
			path:    "/Users/towada/projects/foo",
			enablement: enablementConfig{
				"test": {Overrides: []string{"/Users/towada/*"}},
			},
			expect: true,
		},
		{
			name:    "non-matching wildcard = default enabled",
			extName: "test",
			path:    "/other/path",
			enablement: enablementConfig{
				"test": {Overrides: []string{"/Users/towada/*"}},
			},
			expect: true,
		},
		{
			name:    "disable rule",
			extName: "test",
			path:    "/Users/towada/projects/secret",
			enablement: enablementConfig{
				"test": {Overrides: []string{
					"/Users/towada/*",
					"!/Users/towada/projects/secret",
				}},
			},
			expect: false,
		},
		{
			name:    "last rule wins - re-enable",
			extName: "test",
			path:    "/Users/towada/projects/secret/sub",
			enablement: enablementConfig{
				"test": {Overrides: []string{
					"/Users/towada/*",
					"!/Users/towada/projects/secret/*",
					"/Users/towada/projects/secret/sub",
				}},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEnabled(tt.extName, tt.path, tt.enablement)
			if got != tt.expect {
				t.Errorf("isEnabled(%q, %q) = %v, want %v", tt.extName, tt.path, got, tt.expect)
			}
		})
	}
}

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		expect  bool
	}{
		{
			name:    "exact match",
			pattern: "/Users/towada/",
			path:    "/Users/towada/",
			expect:  true,
		},
		{
			name:    "prefix match",
			pattern: "/Users/towada/",
			path:    "/Users/towada/projects/foo/",
			expect:  true,
		},
		{
			name:    "wildcard match",
			pattern: "/Users/towada/*",
			path:    "/Users/towada/projects/foo/",
			expect:  true,
		},
		{
			name:    "no match",
			pattern: "/other/path/",
			path:    "/Users/towada/",
			expect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPath(tt.pattern, tt.path)
			if got != tt.expect {
				t.Errorf("matchesPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.expect)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"/Users/towada", "/Users/towada/"},
		{"/Users/towada/", "/Users/towada/"},
	}
	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expect {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestLoadExtension(t *testing.T) {
	// Create a temp directory with a test extension
	tmpDir := t.TempDir()
	extDir := filepath.Join(tmpDir, "test-ext")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := map[string]interface{}{
		"name":            "test-ext",
		"version":         "1.0.0",
		"contextFileName": "CONTEXT.md",
		"mcpServers": map[string]interface{}{
			"my-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"${extensionPath}${/}dist${/}index.js"},
				"cwd":     "${extensionPath}",
			},
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(extDir, "gemini-extension.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create context file
	if err := os.WriteFile(filepath.Join(extDir, "CONTEXT.md"), []byte("# Test Context"), 0o644); err != nil {
		t.Fatal(err)
	}

	ext, err := loadExtension(extDir)
	if err != nil {
		t.Fatalf("loadExtension failed: %v", err)
	}

	if ext.Name != "test-ext" {
		t.Errorf("Name = %q, want %q", ext.Name, "test-ext")
	}
	if ext.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", ext.Version, "1.0.0")
	}

	// Check MCP server config was hydrated
	srv, ok := ext.MCPServers["my-server"]
	if !ok {
		t.Fatal("MCP server 'my-server' not found")
	}
	if srv.Command != "node" {
		t.Errorf("Command = %q, want %q", srv.Command, "node")
	}

	absExtDir, _ := filepath.Abs(extDir)
	expectedCWD := absExtDir
	if srv.CWD != expectedCWD {
		t.Errorf("CWD = %q, want %q", srv.CWD, expectedCWD)
	}

	expectedArg := absExtDir + string(filepath.Separator) + "dist" + string(filepath.Separator) + "index.js"
	if len(srv.Args) != 1 || srv.Args[0] != expectedArg {
		t.Errorf("Args = %v, want [%q]", srv.Args, expectedArg)
	}

	// Check context file was found
	if len(ext.ContextFiles) != 1 {
		t.Fatalf("ContextFiles len = %d, want 1", len(ext.ContextFiles))
	}
	expectedCtx := filepath.Join(absExtDir, "CONTEXT.md")
	if ext.ContextFiles[0] != expectedCtx {
		t.Errorf("ContextFiles[0] = %q, want %q", ext.ContextFiles[0], expectedCtx)
	}
}

func TestLoadExtension_MissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := loadExtension(tmpDir)
	if err == nil {
		t.Error("expected error for missing manifest, got nil")
	}
}

func TestLoadExtension_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "gemini-extension.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadExtension(tmpDir)
	if err == nil {
		t.Error("expected error for invalid manifest (missing name/version), got nil")
	}
}

func TestLoadExtension_ContextFileDefault(t *testing.T) {
	tmpDir := t.TempDir()
	extDir := filepath.Join(tmpDir, "ext")
	os.MkdirAll(extDir, 0o755)

	manifest := map[string]interface{}{
		"name":    "default-ctx",
		"version": "1.0.0",
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(extDir, "gemini-extension.json"), data, 0o644)

	// Create default GEMINI.md
	os.WriteFile(filepath.Join(extDir, "GEMINI.md"), []byte("# Default"), 0o644)

	ext, err := loadExtension(extDir)
	if err != nil {
		t.Fatalf("loadExtension failed: %v", err)
	}

	if len(ext.ContextFiles) != 1 {
		t.Fatalf("ContextFiles len = %d, want 1", len(ext.ContextFiles))
	}
	absExtDir, _ := filepath.Abs(extDir)
	expected := filepath.Join(absExtDir, "GEMINI.md")
	if ext.ContextFiles[0] != expected {
		t.Errorf("ContextFiles[0] = %q, want %q", ext.ContextFiles[0], expected)
	}
}

func TestLoadExtension_MissingContextFile(t *testing.T) {
	tmpDir := t.TempDir()
	extDir := filepath.Join(tmpDir, "ext")
	os.MkdirAll(extDir, 0o755)

	manifest := map[string]interface{}{
		"name":            "no-ctx",
		"version":         "1.0.0",
		"contextFileName": "DOES-NOT-EXIST.md",
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(extDir, "gemini-extension.json"), data, 0o644)

	ext, err := loadExtension(extDir)
	if err != nil {
		t.Fatalf("loadExtension failed: %v", err)
	}
	if len(ext.ContextFiles) != 0 {
		t.Errorf("ContextFiles len = %d, want 0 (file doesn't exist)", len(ext.ContextFiles))
	}
}

func TestLoadEnablementConfig(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("missing file returns nil", func(t *testing.T) {
		cfg := loadEnablementConfig(filepath.Join(tmpDir, "nonexistent.json"))
		if cfg != nil {
			t.Errorf("expected nil, got %v", cfg)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		data := []byte(`{
			"google-workspace": {
				"overrides": ["/Users/towada/*"]
			}
		}`)
		path := filepath.Join(tmpDir, "enablement.json")
		os.WriteFile(path, data, 0o644)

		cfg := loadEnablementConfig(path)
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		ext, ok := cfg["google-workspace"]
		if !ok {
			t.Fatal("google-workspace not found in config")
		}
		if len(ext.Overrides) != 1 || ext.Overrides[0] != "/Users/towada/*" {
			t.Errorf("unexpected overrides: %v", ext.Overrides)
		}
	})

	t.Run("invalid JSON returns nil", func(t *testing.T) {
		path := filepath.Join(tmpDir, "bad.json")
		os.WriteFile(path, []byte(`{invalid`), 0o644)
		cfg := loadEnablementConfig(path)
		if cfg != nil {
			t.Errorf("expected nil for invalid JSON, got %v", cfg)
		}
	})
}
