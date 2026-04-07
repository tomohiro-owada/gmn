// Package config provides model constants and resolution logic.
// Ported from upstream packages/core/src/config/models.ts (v0.36.0)
// Copyright 2025 Google LLC
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package config

import "strings"

// Model constants matching upstream Gemini CLI v0.36.0
const (
	PreviewGeminiModel              = "gemini-3-pro-preview"
	PreviewGemini31Model            = "gemini-3.1-pro-preview"
	PreviewGemini31CustomToolsModel = "gemini-3.1-pro-preview-customtools"
	PreviewGeminiFlashModel         = "gemini-3-flash-preview"
	PreviewGemini31FlashLiteModel   = "gemini-3.1-flash-lite-preview"
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
	PreviewGemini31FlashLiteModel:   true,
	DefaultGeminiModel:              true,
	DefaultGeminiFlashModel:         true,
	DefaultGeminiFlashLiteModel:     true,
}

// ResolveModel resolves a model alias to a concrete model name.
// When hasAccessToPreview is false, preview models are downgraded to stable 2.5 equivalents.
// Upstream ref: packages/core/src/config/models.ts resolveModel() (v0.36.0)
// Upstream commit: bbd8483e3 - Add experiment-gated support for gemini flash 3.1 lite (#23794)
func ResolveModel(requestedModel string, useGemini31 bool, useGemini31FlashLite bool, hasAccessToPreview bool) string {
	var resolved string
	switch requestedModel {
	case PreviewGeminiModel, PreviewGeminiModelAuto, GeminiModelAliasAuto, GeminiModelAliasPro:
		if useGemini31 {
			resolved = PreviewGemini31Model
		} else {
			resolved = PreviewGeminiModel
		}
	case DefaultGeminiModelAuto:
		resolved = DefaultGeminiModel
	case GeminiModelAliasFlash:
		resolved = PreviewGeminiFlashModel
	case GeminiModelAliasFlashLt:
		if useGemini31FlashLite {
			resolved = PreviewGemini31FlashLiteModel
		} else {
			resolved = DefaultGeminiFlashLiteModel
		}
	default:
		resolved = requestedModel
	}

	// Upstream ref: 22d962e76 - fallback to 2.5 models when no preview access
	if !hasAccessToPreview && IsPreviewModel(resolved) {
		return fallbackToStable(resolved)
	}
	return resolved
}

// fallbackToStable downgrades a preview model to its stable 2.5 equivalent.
// Upstream ref: bbd8483e3 - add PreviewGemini31FlashLiteModel fallback
func fallbackToStable(model string) string {
	switch model {
	case PreviewGeminiFlashModel:
		return DefaultGeminiFlashModel
	case PreviewGemini31FlashLiteModel:
		return DefaultGeminiFlashLiteModel
	case PreviewGeminiModel, PreviewGemini31Model, PreviewGemini31CustomToolsModel:
		return DefaultGeminiModel
	default:
		// Heuristic fallback for unknown preview models
		lower := strings.ToLower(model)
		if strings.Contains(lower, "flash-lite") {
			return DefaultGeminiFlashLiteModel
		}
		if strings.Contains(lower, "flash") {
			return DefaultGeminiFlashModel
		}
		return DefaultGeminiModel
	}
}

// IsPreviewModel returns true if the model is a preview model.
func IsPreviewModel(model string) bool {
	return model == PreviewGeminiModel ||
		model == PreviewGemini31Model ||
		model == PreviewGemini31CustomToolsModel ||
		model == PreviewGeminiFlashModel ||
		model == PreviewGemini31FlashLiteModel ||
		model == PreviewGeminiModelAuto ||
		model == GeminiModelAliasAuto
}

// IsGemini3Model returns true if the model belongs to the Gemini 3 family.
func IsGemini3Model(model string) bool {
	resolved := ResolveModel(model, false, false, true)
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
