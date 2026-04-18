package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient_UserAgent(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantUA  string
	}{
		{name: "with version", version: "0.30.0", wantUA: "gemini-cli/gmn-0.30.0"},
		{name: "empty version", version: "", wantUA: "gemini-cli/gmn"},
		{name: "dev version", version: "dev", wantUA: "gemini-cli/gmn-dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(http.DefaultClient, tt.version)
			if c.userAgent != tt.wantUA {
				t.Errorf("userAgent = %q, want %q", c.userAgent, tt.wantUA)
			}
		})
	}
}

func TestUserAgentSentInRequests(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")

		// Return a valid response based on path
		if strings.Contains(r.URL.Path, "loadCodeAssist") {
			json.NewEncoder(w).Encode(LoadCodeAssistResponse{
				CloudAICompanionProject: "test-project",
			})
		} else {
			json.NewEncoder(w).Encode(GenerateResponse{
				Response: &InnerResponse{
					Candidates: []Candidate{{
						Content: Content{
							Role:  "model",
							Parts: []Part{{Text: "hello"}},
						},
						FinishReason: "STOP",
					}},
				},
			})
		}
	}))
	defer server.Close()

	client := NewClient(server.Client(), "1.0.0")
	client.baseURL = server.URL

	ctx := context.Background()

	t.Run("Generate sends User-Agent", func(t *testing.T) {
		receivedUA = ""
		req := &GenerateRequest{
			Model:   "test-model",
			Project: "test",
			Request: InnerRequest{
				Contents: []Content{{
					Role:  "user",
					Parts: []Part{{Text: "hello"}},
				}},
			},
		}
		_, err := client.Generate(ctx, req)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		if receivedUA != "gemini-cli/gmn-1.0.0" {
			t.Errorf("User-Agent = %q, want %q", receivedUA, "gemini-cli/gmn-1.0.0")
		}
	})

	t.Run("LoadCodeAssist sends User-Agent", func(t *testing.T) {
		receivedUA = ""
		_, err := client.LoadCodeAssist(ctx)
		if err != nil {
			t.Fatalf("LoadCodeAssist failed: %v", err)
		}
		if receivedUA != "gemini-cli/gmn-1.0.0" {
			t.Errorf("User-Agent = %q, want %q", receivedUA, "gemini-cli/gmn-1.0.0")
		}
	})
}

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		header  string
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "Retry-After header",
			body:    `{}`,
			header:  "2",
			attempt: 0,
			wantMin: 2 * time.Second,
			wantMax: 2*time.Second + time.Millisecond,
		},
		{
			name:    "retryDelay in body",
			body:    `{"error":{"details":[{"retryDelay":"1.5s"}]}}`,
			header:  "",
			attempt: 0,
			wantMin: 1500 * time.Millisecond,
			wantMax: 1500*time.Millisecond + time.Millisecond,
		},
		{
			name:    "exponential backoff attempt 0",
			body:    `{}`,
			header:  "",
			attempt: 0,
			wantMin: 500 * time.Millisecond,
			wantMax: 500*time.Millisecond + time.Millisecond,
		},
		{
			name:    "exponential backoff attempt 2",
			body:    `{}`,
			header:  "",
			attempt: 2,
			wantMin: 2 * time.Second,
			wantMax: 2*time.Second + time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.header != "" {
				headers.Set("Retry-After", tt.header)
			}
			got := retryDelay([]byte(tt.body), headers, tt.attempt)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("retryDelay() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1.5s", 1500 * time.Millisecond},
		{"420.05163ms", 420051630 * time.Nanosecond},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDuration(tt.input)
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDoRequestWithRetry_429(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.Client(), "test")
	client.baseURL = server.URL

	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL+"/test", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.doRequestWithRetry(context.Background(), req, []byte("{}"))
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoRequestWithRetry_NonRetryableError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer server.Close()

	client := NewClient(server.Client(), "test")
	client.baseURL = server.URL

	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL+"/test", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")

	_, err := client.doRequestWithRetry(context.Background(), req, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("expected status 400 in error, got: %v", err)
	}
}

func TestIsTransientTLSError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "tls error string", err: fmt.Errorf("tls: handshake failure"), want: true},
		{name: "connection reset", err: fmt.Errorf("connection reset by peer"), want: true},
		{name: "EOF", err: fmt.Errorf("unexpected EOF"), want: true},
		{name: "normal error", err: fmt.Errorf("dns lookup failed"), want: false},
		// Upstream ref: 06e7621 - retry additional OpenSSL 3.x BAD_RECORD_MAC errors
		{name: "bad record MAC", err: fmt.Errorf("remote error: bad record MAC"), want: true},
		{name: "bad record MAC uppercase", err: fmt.Errorf("BAD_RECORD_MAC"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientTLSError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientTLSError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid HTTPS", url: "https://api.example.com", wantErr: false},
		{name: "localhost HTTP", url: "http://localhost:8080", wantErr: false},
		{name: "127.0.0.1 HTTP", url: "http://127.0.0.1:8080", wantErr: false},
		{name: "IPv6 localhost HTTP", url: "http://[::1]:8080", wantErr: false},
		{name: "non-local HTTP", url: "http://api.example.com", wantErr: true},
		{name: "invalid URL", url: "://bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBaseURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestNewClient_BaseURLFromEnv(t *testing.T) {
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "https://custom.example.com")
	c := NewClient(http.DefaultClient, "test")
	if c.baseURL != "https://custom.example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://custom.example.com")
	}
}

func TestNewClient_BaseURLFromEnv_Invalid(t *testing.T) {
	t.Setenv("GOOGLE_GEMINI_BASE_URL", "http://remote.example.com")
	c := NewClient(http.DefaultClient, "test")
	if c.baseURL != baseURL {
		t.Errorf("baseURL = %q, want default %q (invalid URL should be ignored)", c.baseURL, baseURL)
	}
}

func TestGenerateResponse_NilHandling(t *testing.T) {
	// Test that empty/nil responses don't cause panics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a response with empty candidates (mimics upstream converter hardening)
		json.NewEncoder(w).Encode(GenerateResponse{
			TraceID: "test-trace",
			Response: &InnerResponse{
				Candidates: nil,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client(), "test")
	client.baseURL = server.URL

	req := &GenerateRequest{
		Model:   "test-model",
		Project: "test",
		Request: InnerRequest{
			Contents: []Content{{
				Role:  "user",
				Parts: []Part{{Text: "hello"}},
			}},
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.TraceID != "test-trace" {
		t.Errorf("TraceID = %q, want %q", resp.TraceID, "test-trace")
	}
	// Iterating over nil response/candidates should be safe
	if resp.Response != nil {
		for range resp.Response.Candidates {
			t.Error("should not iterate over nil candidates")
		}
	}
}
