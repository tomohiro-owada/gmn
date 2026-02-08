//go:build !darwin

// Keychain stub for non-macOS platforms
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package auth

import "errors"

// loadFromKeychain is not supported on non-macOS platforms
func (m *Manager) loadFromKeychain() (*Credentials, error) {
	return nil, errors.New("keychain not supported on this platform")
}
