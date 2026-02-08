// Package auth provides OAuth authentication for geminimini.
// This file was modified from the original Gemini CLI.
// Copyright 2025 Google LLC
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k-sub1995/g/internal/config"
)

const (
	oauthFile     = "oauth_creds.json"
	tokenEndpoint = "https://oauth2.googleapis.com/token"

	// OAuth credentials from official Gemini CLI
	// It's OK to embed these in source code for installed applications.
	// See: https://developers.google.com/identity/protocols/oauth2#installed
	clientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	clientSecret = "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"
)

// Credentials holds OAuth token information
type Credentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
	ExpiryDate   int64  `json:"expiry_date,omitempty"`
}

// IsExpired checks if the token is expired (with 5 minute margin)
func (c *Credentials) IsExpired() bool {
	if c.ExpiryDate == 0 {
		return false
	}
	// ExpiryDate is in milliseconds
	expiryTime := time.UnixMilli(c.ExpiryDate)
	return time.Now().Add(5 * time.Minute).After(expiryTime)
}

// Manager handles OAuth authentication
type Manager struct {
	geminiDir string
}

// NewManager creates a new auth manager
func NewManager() (*Manager, error) {
	geminiDir, err := config.GeminiDir()
	if err != nil {
		return nil, err
	}
	return &Manager{geminiDir: geminiDir}, nil
}

// LoadCredentials loads OAuth credentials from file or keychain
func (m *Manager) LoadCredentials() (*Credentials, error) {
	// Try keychain first (macOS only)
	creds, err := m.loadFromKeychain()
	if err == nil && creds != nil {
		return creds, nil
	}

	// Fall back to file
	return m.loadFromFile()
}

// loadFromFile reads credentials from oauth_creds.json
func (m *Manager) loadFromFile() (*Credentials, error) {
	path := filepath.Join(m.geminiDir, oauthFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials not found: run 'gemini' to authenticate first")
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// RefreshToken refreshes an expired access token
func (m *Manager) RefreshToken(creds *Credentials) (*Credentials, error) {
	if creds.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available: run 'gemini' to re-authenticate")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", creds.RefreshToken)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	resp, err := http.Post(
		tokenEndpoint,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d): run 'gemini' to re-authenticate", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: creds.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiryDate:   time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UnixMilli(),
	}, nil
}

// HTTPClient returns an HTTP client with the access token
func (m *Manager) HTTPClient(creds *Credentials) *http.Client {
	return &http.Client{
		Transport: &authTransport{
			token: creds.AccessToken,
			base:  http.DefaultTransport,
		},
	}
}

// authTransport adds Authorization header to requests
type authTransport struct {
	token string
	base  http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}
