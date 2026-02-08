//go:build darwin

// Keychain support for macOS
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package auth

import (
	"encoding/json"
	"os/exec"
)

const keychainService = "gemini-cli-oauth"
const keychainAccount = "main-account"

// loadFromKeychain loads credentials from macOS Keychain
func (m *Manager) loadFromKeychain() (*Credentials, error) {
	cmd := exec.Command(
		"security",
		"find-generic-password",
		"-s", keychainService,
		"-a", keychainAccount,
		"-w",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// The keychain stores the token in a specific format
	var stored struct {
		Token struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			TokenType    string `json:"tokenType"`
			ExpiresAt    int64  `json:"expiresAt"`
		} `json:"token"`
	}

	if err := json.Unmarshal(output, &stored); err != nil {
		return nil, err
	}

	return &Credentials{
		AccessToken:  stored.Token.AccessToken,
		RefreshToken: stored.Token.RefreshToken,
		TokenType:    stored.Token.TokenType,
		ExpiryDate:   stored.Token.ExpiresAt,
	}, nil
}
