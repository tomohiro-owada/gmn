// Package config provides model constants and resolution logic.
// Ported from upstream packages/core/src/config/models.ts (v0.30.0)
// Copyright 2025 Google LLC
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package config

import "strings"

// Model constants matching upstream Gemini CLI v0.30.0
const (
	PreviewGeminiModel              = "gemini-3-pro-preview"
	PreviewGemini31Model            = "gemini-3.1-pro-preview"
	PreviewGemini31CustomToolsModel = "gemini-3.1-pro-preview-customtools"
	PreviewGeminiFlashModel         = "gemini-3-flash-preview"
	DefaultGeminiModel              = "gemini-2.5-pro"
	DefaultGeminiFlashModel         = "gemini-2.5-flash"
	DefaultGeminiFlashLiteModel     = "gemini-2.5-flash-lite"

	// Auto model aliases
	PreviewGeminiModelAuto  = "auto-gemini-3"
	DefaultGeminiModelAuto  = "auto-gemini-2.5"
	GeminiModelAliasAuto    = "auto"
	GeminiModelAliasPro     = "pro"
	GeminiModelAliasFlash   = "flash"
	GeminiModelAliasFlashLt = "flash-lite"
)

// ValidGeminiModels is the set of known valid Gemini model names.
var ValidGeminiModels = map[string]bool{
	PreviewGeminiModel:              true,
	PreviewGemini31Model:            true,
	PreviewGemini31CustomToolsModel: true,
	PreviewGeminiFlashModel:         true,
	DefaultGeminiModel:              true,
	DefaultGeminiFlashModel:         true,
	DefaultGeminiFlashLiteModel:     true,
}

// ResolveModel resolves a model alias to a concrete model name.
// Upstream ref: packages/core/src/config/models.ts resolveModel()
func ResolveModel(requestedModel string, useGemini31 bool) string {
	switch requestedModel {
	case PreviewGeminiModel, PreviewGeminiModelAuto, GeminiModelAliasAuto, GeminiModelAliasPro:
		if useGemini31 {
			return PreviewGemini31Model
		}
		return PreviewGeminiModel
	case DefaultGeminiModelAuto:
		return DefaultGeminiModel
	case GeminiModelAliasFlash:
		return PreviewGeminiFlashModel
	case GeminiModelAliasFlashLt:
		return DefaultGeminiFlashLiteModel
	default:
		return requestedModel
	}
}

// IsPreviewModel returns true if the model is a preview model.
func IsPreviewModel(model string) bool {
	return model == PreviewGeminiModel ||
		model == PreviewGemini31Model ||
		model == PreviewGemini31CustomToolsModel ||
		model == PreviewGeminiFlashModel ||
		model == PreviewGeminiModelAuto
}

// IsGemini3Model returns true if the model belongs to the Gemini 3 family.
func IsGemini3Model(model string) bool {
	resolved := ResolveModel(model, false)
	return strings.HasPrefix(resolved, "gemini-3")
}

// IsGemini2Model returns true if the model belongs to the Gemini 2 family.
func IsGemini2Model(model string) bool {
	return strings.HasPrefix(model, "gemini-2")
}

// IsProModel returns true if the model name contains "pro".
func IsProModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "pro")
}

// IsAutoModel returns true if the model is an auto-routing model alias.
func IsAutoModel(model string) bool {
	return model == GeminiModelAliasAuto ||
		model == PreviewGeminiModelAuto ||
		model == DefaultGeminiModelAuto
}
