// Package tools provides tool implementations used by the Gemini agent.
// Copyright 2025 Tomohiro Owada
// Copyright 2026 k-sub1995
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/k-sub1995/g/internal/api"
)

const (
	webFetchTimeout = 30 * time.Second
	maxFetchBytes   = 512 * 1024 // 512KB
)

type WebFetchTool struct {
	opts RegistryOptions
}

func NewWebFetchTool(opts RegistryOptions) *WebFetchTool {
	return &WebFetchTool{opts: opts}
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Declaration() api.FunctionDecl {
	return api.FunctionDecl{
		Name:        "web_fetch",
		Description: "Fetches content from a specified URL and returns the text content.",
		Parameters: mustMarshalJSON(map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "The URL to fetch content from.",
				},
			},
			"required": []string{"url"},
		}),
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return errorResult("url is required"), nil
	}

	// Ensure HTTPS
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}
	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	client := &http.Client{Timeout: webFetchTimeout}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errorResult(fmt.Sprintf("invalid URL: %v", err)), nil
	}
	req.Header.Set("User-Agent", "gmn/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return errorResult(fmt.Sprintf("fetch failed: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	if err != nil {
		return errorResult(fmt.Sprintf("read failed: %v", err)), nil
	}

	content := string(body)

	// Simple HTML tag stripping for basic readability
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		content = stripHTMLTags(content)
	}

	return &ToolResult{
		Content: map[string]interface{}{
			"content":      content,
			"url":          url,
			"status":       resp.StatusCode,
			"content_type": resp.Header.Get("Content-Type"),
		},
	}, nil
}

// stripHTMLTags is a simple HTML tag remover for basic text extraction.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	// Collapse whitespace
	text := result.String()
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}
