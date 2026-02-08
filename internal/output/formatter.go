// Package output provides output formatting for geminimini.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/acarl005/stripansi"
	"github.com/k-sub1995/g/internal/api"
)

// Formatter is the interface for output formatters
type Formatter interface {
	WriteResponse(resp *api.GenerateResponse) error
	WriteStreamEvent(event *api.StreamEvent) error
	WriteError(err error) error
	WriteToolCall(name string, args map[string]interface{}) error
	WriteToolResult(name string, result map[string]interface{}, isError bool) error
}

// NewFormatter creates a formatter for the given format
func NewFormatter(format string, w io.Writer, errW io.Writer, sanitize bool) (Formatter, error) {
	switch format {
	case "text":
		return &TextFormatter{w: w, errW: errW, sanitize: sanitize}, nil
	case "json":
		return &JSONFormatter{w: w, errW: errW, sanitize: sanitize}, nil
	case "stream-json":
		return &StreamJSONFormatter{w: w, errW: errW, sanitize: sanitize}, nil
	default:
		return nil, fmt.Errorf("unknown output format: %s", format)
	}
}

// sanitizeText strips ANSI escape sequences from text if sanitization is enabled
func sanitizeText(text string, sanitize bool) string {
	if sanitize {
		return stripansi.Strip(text)
	}
	return text
}

// TextFormatter outputs plain text (streaming)
type TextFormatter struct {
	w        io.Writer
	errW     io.Writer
	sanitize bool
}

func (f *TextFormatter) WriteResponse(resp *api.GenerateResponse) error {
	if len(resp.Response.Candidates) > 0 && len(resp.Response.Candidates[0].Content.Parts) > 0 {
		text := sanitizeText(resp.Response.Candidates[0].Content.Parts[0].Text, f.sanitize)
		_, err := fmt.Fprintln(f.w, text)
		return err
	}
	return nil
}

func (f *TextFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	if event.Text != "" {
		text := sanitizeText(event.Text, f.sanitize)
		_, err := fmt.Fprint(f.w, text)
		return err
	}
	if event.Type == "done" {
		// Add final newline
		_, err := fmt.Fprintln(f.w)
		return err
	}
	return nil
}

func (f *TextFormatter) WriteError(err error) error {
	_, writeErr := fmt.Fprintf(f.errW, "Error: %v\n", err)
	return writeErr
}

func (f *TextFormatter) WriteToolCall(name string, args map[string]interface{}) error {
	_, err := fmt.Fprintf(f.errW, "⚡ %s\n", name)
	return err
}

func (f *TextFormatter) WriteToolResult(name string, result map[string]interface{}, isError bool) error {
	if isError {
		if errMsg, ok := result["error"]; ok {
			_, err := fmt.Fprintf(f.errW, "✗ %s: %v\n", name, errMsg)
			return err
		}
	}
	return nil
}

// JSONFormatter outputs structured JSON (non-streaming)
type JSONFormatter struct {
	w        io.Writer
	errW     io.Writer
	sanitize bool
}

// JSONResponse is the JSON output structure
type JSONResponse struct {
	Model        string             `json:"model"`
	Response     string             `json:"response"`
	Usage        *api.UsageMetadata `json:"usage,omitempty"`
	FinishReason string             `json:"finishReason,omitempty"`
}

// JSONError is the JSON error structure
type JSONError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (f *JSONFormatter) WriteResponse(resp *api.GenerateResponse) error {
	out := JSONResponse{}
	if resp.Response.UsageMetadata.TotalTokenCount > 0 {
		out.Usage = &resp.Response.UsageMetadata
	}
	if len(resp.Response.Candidates) > 0 {
		out.FinishReason = resp.Response.Candidates[0].FinishReason
		if len(resp.Response.Candidates[0].Content.Parts) > 0 {
			out.Response = sanitizeText(resp.Response.Candidates[0].Content.Parts[0].Text, f.sanitize)
		}
	}

	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (f *JSONFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	// JSONFormatter collects all events, not used directly
	return nil
}

func (f *JSONFormatter) WriteError(err error) error {
	out := JSONError{}
	out.Error.Message = err.Error()

	enc := json.NewEncoder(f.errW)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (f *JSONFormatter) WriteToolCall(name string, args map[string]interface{}) error {
	return nil // JSON formatter doesn't show intermediate tool calls
}

func (f *JSONFormatter) WriteToolResult(name string, result map[string]interface{}, isError bool) error {
	return nil // JSON formatter doesn't show intermediate tool results
}

// StreamJSONFormatter outputs NDJSON (streaming)
type StreamJSONFormatter struct {
	w        io.Writer
	errW     io.Writer
	sanitize bool
}

func (f *StreamJSONFormatter) WriteResponse(resp *api.GenerateResponse) error {
	// Not used for streaming
	return nil
}

func (f *StreamJSONFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	e := *event
	if e.Text != "" {
		e.Text = sanitizeText(e.Text, f.sanitize)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.w.Write(append(data, '\n'))
	return err
}

func (f *StreamJSONFormatter) WriteError(err error) error {
	event := api.StreamEvent{Type: "error", Error: err.Error()}
	data, _ := json.Marshal(event)
	_, writeErr := f.errW.Write(append(data, '\n'))
	return writeErr
}

func (f *StreamJSONFormatter) WriteToolCall(name string, args map[string]interface{}) error {
	event := map[string]interface{}{
		"type": "tool_call",
		"name": name,
		"args": args,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.w.Write(append(data, '\n'))
	return err
}

func (f *StreamJSONFormatter) WriteToolResult(name string, result map[string]interface{}, isError bool) error {
	event := map[string]interface{}{
		"type":     "tool_result",
		"name":     name,
		"result":   result,
		"is_error": isError,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.w.Write(append(data, '\n'))
	return err
}
