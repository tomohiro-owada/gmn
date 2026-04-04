package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesRequiredMCPServers(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)

	globalGeminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(globalGeminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `{
  "mcpServers": {
    "local-server": {
      "command": "local-cmd"
    },
    "shared-server": {
      "command": "local-shared"
    }
  },
  "requiredMcpServers": {
    "shared-server": {
      "url": "https://admin.example/shared",
      "type": "http",
      "description": "Admin shared server"
    },
    "required-server": {
      "url": "https://admin.example/required",
      "type": "http",
      "includeTools": ["toolC", "toolA", "toolB"],
      "excludeTools": ["toolZ", "toolX"]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(globalGeminiDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MCPServers["local-server"].Command != "local-cmd" {
		t.Fatalf("local server was unexpectedly modified: %+v", cfg.MCPServers["local-server"])
	}

	shared := cfg.MCPServers["shared-server"]
	if shared.URL != "https://admin.example/shared" || shared.Type != "http" {
		t.Fatalf("required server did not override local config: %+v", shared)
	}
	if shared.Command != "" {
		t.Fatalf("required server should replace stdio fields, got command=%q", shared.Command)
	}
	if !shared.Trust {
		t.Fatalf("required server should default trust=true when omitted")
	}

	required := cfg.MCPServers["required-server"]
	if got, want := required.IncludeTools, []string{"toolA", "toolB", "toolC"}; !equalStrings(got, want) {
		t.Fatalf("IncludeTools = %v, want %v", got, want)
	}
	if got, want := required.ExcludeTools, []string{"toolX", "toolZ"}; !equalStrings(got, want) {
		t.Fatalf("ExcludeTools = %v, want %v", got, want)
	}
}

func TestLoadProjectRequiredMCPServerOverridesGlobalRequiredServer(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "project")
	projectGeminiDir := filepath.Join(projectDir, ".gemini")
	if err := os.MkdirAll(projectGeminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)

	globalGeminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(globalGeminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalSettings := `{
  "requiredMcpServers": {
    "corp-tool": {
      "url": "https://global.example/tool",
      "type": "http"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(globalGeminiDir, "settings.json"), []byte(globalSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	projectSettings := `{
  "requiredMcpServers": {
    "corp-tool": {
      "url": "https://project.example/tool",
      "type": "http",
      "trust": false
    }
  }
}`
	if err := os.WriteFile(filepath.Join(projectGeminiDir, "settings.json"), []byte(projectSettings), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	server := cfg.MCPServers["corp-tool"]
	if server.URL != "https://project.example/tool" {
		t.Fatalf("project required server did not override global one: %+v", server)
	}
	if server.Trust {
		t.Fatalf("explicit trust=false should be preserved")
	}
}

func TestLoadPreservesRequiredMCPServerAuthFields(t *testing.T) {
	home := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)

	globalGeminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(globalGeminiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `{
  "requiredMcpServers": {
    "auth-server": {
      "url": "https://auth.example/tool",
      "type": "http",
      "authProviderType": "google-credentials",
      "oauth": {
        "scopes": ["scope:a", "scope:b"],
        "clientId": "client-id",
        "clientSecret": "client-secret"
      },
      "targetAudience": "aud",
      "targetServiceAccount": "svc@example.iam.gserviceaccount.com",
      "headers": {
        "X-Test": "value"
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(globalGeminiDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	server := cfg.MCPServers["auth-server"]
	if server.AuthProviderType != "google-credentials" {
		t.Fatalf("AuthProviderType = %q, want %q", server.AuthProviderType, "google-credentials")
	}
	if server.OAuth == nil {
		t.Fatalf("OAuth config should be preserved")
	}
	if got, want := server.OAuth.Scopes, []string{"scope:a", "scope:b"}; !equalStrings(got, want) {
		t.Fatalf("OAuth.Scopes = %v, want %v", got, want)
	}
	if server.OAuth.ClientID != "client-id" || server.OAuth.ClientSecret != "client-secret" {
		t.Fatalf("OAuth client fields were not preserved: %+v", server.OAuth)
	}
	if server.TargetAudience != "aud" {
		t.Fatalf("TargetAudience = %q, want %q", server.TargetAudience, "aud")
	}
	if server.TargetServiceAccount != "svc@example.iam.gserviceaccount.com" {
		t.Fatalf("TargetServiceAccount = %q", server.TargetServiceAccount)
	}
	if server.Headers["X-Test"] != "value" {
		t.Fatalf("Headers were not preserved: %+v", server.Headers)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
