package config

import "testing"

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		useGemini31 bool
		want        string
	}{
		// Alias resolution (Gemini 3 default, useGemini31=false)
		{name: "pro alias → gemini-3-pro-preview", input: "pro", useGemini31: false, want: PreviewGeminiModel},
		{name: "auto alias → gemini-3-pro-preview", input: "auto", useGemini31: false, want: PreviewGeminiModel},
		{name: "auto-gemini-3 → gemini-3-pro-preview", input: "auto-gemini-3", useGemini31: false, want: PreviewGeminiModel},
		{name: "flash alias → gemini-3-flash-preview", input: "flash", useGemini31: false, want: PreviewGeminiFlashModel},
		{name: "flash-lite alias → gemini-2.5-flash-lite", input: "flash-lite", useGemini31: false, want: DefaultGeminiFlashLiteModel},
		{name: "auto-gemini-2.5 → gemini-2.5-pro", input: "auto-gemini-2.5", useGemini31: false, want: DefaultGeminiModel},

		// Concrete model names pass through
		{name: "concrete gemini-2.5-pro", input: "gemini-2.5-pro", useGemini31: false, want: "gemini-2.5-pro"},
		{name: "concrete gemini-2.5-flash", input: "gemini-2.5-flash", useGemini31: false, want: "gemini-2.5-flash"},
		{name: "unknown model passes through", input: "custom-model-v1", useGemini31: false, want: "custom-model-v1"},

		// Gemini 3.1 mode
		{name: "pro alias with 3.1 → gemini-3.1-pro-preview", input: "pro", useGemini31: true, want: PreviewGemini31Model},
		{name: "auto alias with 3.1 → gemini-3.1-pro-preview", input: "auto", useGemini31: true, want: PreviewGemini31Model},
		{name: "auto-gemini-3 with 3.1 → gemini-3.1-pro-preview", input: "auto-gemini-3", useGemini31: true, want: PreviewGemini31Model},
		{name: "gemini-3-pro-preview with 3.1 → gemini-3.1-pro-preview", input: PreviewGeminiModel, useGemini31: true, want: PreviewGemini31Model},

		// flash and flash-lite are not affected by useGemini31
		{name: "flash alias with 3.1 still → gemini-3-flash-preview", input: "flash", useGemini31: true, want: PreviewGeminiFlashModel},
		{name: "flash-lite alias with 3.1 still → gemini-2.5-flash-lite", input: "flash-lite", useGemini31: true, want: DefaultGeminiFlashLiteModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveModel(tt.input, tt.useGemini31)
			if got != tt.want {
				t.Errorf("ResolveModel(%q, %v) = %q, want %q", tt.input, tt.useGemini31, got, tt.want)
			}
		})
	}
}

func TestIsPreviewModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{PreviewGeminiModel, true},
		{PreviewGemini31Model, true},
		{PreviewGemini31CustomToolsModel, true},
		{PreviewGeminiFlashModel, true},
		{PreviewGeminiModelAuto, true},
		{DefaultGeminiModel, false},
		{DefaultGeminiFlashModel, false},
		{"custom-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := IsPreviewModel(tt.model)
			if got != tt.want {
				t.Errorf("IsPreviewModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsGemini3Model(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{PreviewGeminiModel, true},
		{"gemini-3-flash-preview", true},
		{"gemini-3.1-pro-preview", true},
		{DefaultGeminiModel, false},
		{DefaultGeminiFlashModel, false},
		{"custom-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := IsGemini3Model(tt.model)
			if got != tt.want {
				t.Errorf("IsGemini3Model(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsGemini2Model(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{DefaultGeminiModel, true},
		{DefaultGeminiFlashModel, true},
		{DefaultGeminiFlashLiteModel, true},
		{PreviewGeminiModel, false},
		{"custom-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := IsGemini2Model(tt.model)
			if got != tt.want {
				t.Errorf("IsGemini2Model(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsProModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{PreviewGeminiModel, true},
		{PreviewGemini31Model, true},
		{DefaultGeminiModel, true},
		{PreviewGeminiFlashModel, false},
		{DefaultGeminiFlashModel, false},
		{"custom-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := IsProModel(tt.model)
			if got != tt.want {
				t.Errorf("IsProModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsAutoModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{GeminiModelAliasAuto, true},
		{PreviewGeminiModelAuto, true},
		{DefaultGeminiModelAuto, true},
		{PreviewGeminiModel, false},
		{"custom-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := IsAutoModel(tt.model)
			if got != tt.want {
				t.Errorf("IsAutoModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}
