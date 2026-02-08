# 詳細設計

## 1. ファイル構成詳細

```
geminimini/
├── main.go
├── cmd/
│   └── root.go
├── internal/
│   ├── auth/
│   │   ├── auth.go
│   │   ├── oauth.go
│   │   ├── keychain.go
│   │   ├── keychain_darwin.go
│   │   ├── keychain_other.go
│   │   └── refresh.go
│   ├── api/
│   │   ├── client.go
│   │   ├── request.go
│   │   ├── response.go
│   │   ├── stream.go
│   │   └── errors.go
│   ├── mcp/
│   │   ├── client.go
│   │   ├── manager.go
│   │   ├── protocol.go
│   │   ├── types.go
│   │   └── transport/
│   │       ├── transport.go
│   │       ├── stdio.go
│   │       ├── sse.go
│   │       └── http.go
│   ├── config/
│   │   ├── config.go
│   │   ├── loader.go
│   │   └── defaults.go
│   ├── input/
│   │   ├── reader.go
│   │   └── file.go
│   └── output/
│       ├── formatter.go
│       ├── text.go
│       ├── json.go
│       └── stream.go
├── go.mod
├── go.sum
├── Makefile
└── docs/
```

## 2. 型定義詳細

### 2.1 認証 (internal/auth)

```go
// auth.go
package auth

import (
    "net/http"
    "time"
)

// Credentials はOAuth認証情報を表す
type Credentials struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    Scope        string    `json:"scope,omitempty"`
    ExpiresAt    time.Time `json:"expiry_date,omitempty"`
}

// IsExpired はトークンが期限切れかどうかを返す
func (c *Credentials) IsExpired() bool {
    if c.ExpiresAt.IsZero() {
        return false
    }
    // 5分のマージンを持たせる
    return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// Manager は認証を管理するインターフェース
type Manager interface {
    // LoadCredentials は保存された認証情報を読み込む
    LoadCredentials() (*Credentials, error)

    // RefreshIfNeeded は必要に応じてトークンをリフレッシュする
    RefreshIfNeeded(creds *Credentials) (*Credentials, error)

    // HTTPClient は認証済みのHTTPクライアントを返す
    HTTPClient(creds *Credentials) *http.Client
}

// oauth.go
package auth

import (
    "encoding/json"
    "os"
    "path/filepath"
)

const (
    geminiDir     = ".gemini"
    oauthFile     = "oauth_creds.json"
    accountsFile  = "google_accounts.json"
    settingsFile  = "settings.json"
)

// OAuthManager はOAuth認証を管理する
type OAuthManager struct {
    homeDir string
}

// NewOAuthManager は新しいOAuthManagerを作成する
func NewOAuthManager() (*OAuthManager, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }
    return &OAuthManager{homeDir: home}, nil
}

// LoadCredentials は認証情報を読み込む
// 優先順位: 1. Keychain 2. oauth_creds.json
func (m *OAuthManager) LoadCredentials() (*Credentials, error) {
    // 1. Keychainから試行
    creds, err := loadFromKeychain()
    if err == nil && creds != nil {
        return creds, nil
    }

    // 2. ファイルから読み込み
    return m.loadFromFile()
}

func (m *OAuthManager) loadFromFile() (*Credentials, error) {
    path := filepath.Join(m.homeDir, geminiDir, oauthFile)
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, &AuthError{
            Code:    ErrNoCredentials,
            Message: "credentials not found",
            Hint:    "Run 'gemini' to authenticate first",
        }
    }

    var creds Credentials
    if err := json.Unmarshal(data, &creds); err != nil {
        return nil, err
    }

    // expiry_date はミリ秒なので変換
    if creds.ExpiresAt.Unix() > 1e12 {
        creds.ExpiresAt = time.UnixMilli(creds.ExpiresAt.UnixMilli())
    }

    return &creds, nil
}

// refresh.go
package auth

import (
    "encoding/json"
    "net/http"
    "net/url"
    "strings"
)

const (
    tokenEndpoint = "https://oauth2.googleapis.com/token"

    // 公式Gemini CLIのクライアントID/シークレット（ソースからハードコード）
    // 参照: gemini-cli/packages/core/src/code_assist/oauth2.ts
    //
    // Note: インストールアプリケーションの場合、client_secret をソースに
    // 埋め込むことは Google が許可している。
    // https://developers.google.com/identity/protocols/oauth2#installed
    // "The process results in a client ID and, in some cases, a client secret,
    // which you embed in the source code of your application. (In this context,
    // the client secret is obviously not treated as a secret.)"
    clientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
    clientSecret = "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"

    // OAuth スコープ
    oauthScopes = []string{
        "https://www.googleapis.com/auth/cloud-platform",
        "https://www.googleapis.com/auth/userinfo.email",
        "https://www.googleapis.com/auth/userinfo.profile",
    }
)

// RefreshToken はアクセストークンをリフレッシュする
func (m *OAuthManager) RefreshToken(creds *Credentials) (*Credentials, error) {
    if creds.RefreshToken == "" {
        return nil, &AuthError{
            Code:    ErrRefreshFailed,
            Message: "no refresh token available",
            Hint:    "Run 'gemini' to re-authenticate",
        }
    }

    data := url.Values{}
    data.Set("grant_type", "refresh_token")
    data.Set("refresh_token", creds.RefreshToken)
    data.Set("client_id", clientID)
    data.Set("client_secret", clientSecret)

    resp, err := http.Post(tokenEndpoint,
        "application/x-www-form-urlencoded",
        strings.NewReader(data.Encode()))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, &AuthError{
            Code:    ErrRefreshFailed,
            Message: "token refresh failed",
            Hint:    "Run 'gemini' to re-authenticate",
        }
    }

    var tokenResp struct {
        AccessToken string `json:"access_token"`
        ExpiresIn   int    `json:"expires_in"`
        TokenType   string `json:"token_type"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &Credentials{
        AccessToken:  tokenResp.AccessToken,
        RefreshToken: creds.RefreshToken,
        TokenType:    tokenResp.TokenType,
        ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
    }, nil
}
```

### 2.2 API クライアント (internal/api)

```go
// client.go
package api

import (
    "context"
    "net/http"
)

const (
    baseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Client はGemini APIクライアント
type Client struct {
    httpClient *http.Client
    baseURL    string
}

// NewClient は新しいAPIクライアントを作成する
func NewClient(httpClient *http.Client) *Client {
    return &Client{
        httpClient: httpClient,
        baseURL:    baseURL,
    }
}

// Generate はコンテンツを生成する
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    endpoint := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, req.Model)
    // ... 実装
}

// GenerateStream はストリーミングでコンテンツを生成する
func (c *Client) GenerateStream(ctx context.Context, req *GenerateRequest) (<-chan StreamEvent, error) {
    endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent", c.baseURL, req.Model)
    // ... 実装
}

// request.go
package api

// GenerateRequest は生成リクエスト
type GenerateRequest struct {
    Model    string           `json:"-"`
    Contents []Content        `json:"contents"`
    Config   GenerationConfig `json:"generationConfig,omitempty"`
    Tools    []Tool           `json:"tools,omitempty"`
}

// Content はコンテンツ
type Content struct {
    Role  string `json:"role"`
    Parts []Part `json:"parts"`
}

// Part はコンテンツの一部
type Part struct {
    Text         string        `json:"text,omitempty"`
    FunctionCall *FunctionCall `json:"functionCall,omitempty"`
    FunctionResp *FunctionResp `json:"functionResponse,omitempty"`
}

// FunctionCall はツール呼び出し
type FunctionCall struct {
    Name string                 `json:"name"`
    Args map[string]interface{} `json:"args"`
}

// FunctionResp はツール応答
type FunctionResp struct {
    Name     string                 `json:"name"`
    Response map[string]interface{} `json:"response"`
}

// GenerationConfig は生成設定
type GenerationConfig struct {
    Temperature     float64 `json:"temperature,omitempty"`
    TopP            float64 `json:"topP,omitempty"`
    TopK            int     `json:"topK,omitempty"`
    MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// Tool はツール定義
type Tool struct {
    FunctionDeclarations []FunctionDecl `json:"functionDeclarations"`
}

// FunctionDecl は関数宣言
type FunctionDecl struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`
}

// response.go
package api

// GenerateResponse は生成レスポンス
type GenerateResponse struct {
    Candidates    []Candidate   `json:"candidates"`
    UsageMetadata UsageMetadata `json:"usageMetadata"`
}

// Candidate は候補
type Candidate struct {
    Content      Content `json:"content"`
    FinishReason string  `json:"finishReason"`
}

// UsageMetadata は使用量メタデータ
type UsageMetadata struct {
    PromptTokenCount     int `json:"promptTokenCount"`
    CandidatesTokenCount int `json:"candidatesTokenCount"`
    TotalTokenCount      int `json:"totalTokenCount"`
}

// stream.go
package api

// StreamEventType はストリームイベントタイプ
type StreamEventType string

const (
    EventStart      StreamEventType = "start"
    EventContent    StreamEventType = "content"
    EventToolCall   StreamEventType = "tool_call"
    EventToolResult StreamEventType = "tool_result"
    EventDone       StreamEventType = "done"
    EventError      StreamEventType = "error"
)

// StreamEvent はストリームイベント
type StreamEvent struct {
    Type       StreamEventType `json:"type"`
    Model      string          `json:"model,omitempty"`
    Text       string          `json:"text,omitempty"`
    ToolCall   *FunctionCall   `json:"tool_call,omitempty"`
    ToolResult *ToolResult     `json:"tool_result,omitempty"`
    Usage      *UsageMetadata  `json:"usage,omitempty"`
    Error      string          `json:"error,omitempty"`
}

// ToolResult はツール実行結果
type ToolResult struct {
    Name   string      `json:"name"`
    Result interface{} `json:"result"`
}
```

### 2.3 MCP クライアント (internal/mcp)

```go
// types.go
package mcp

import "encoding/json"

// Tool はMCPツール
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description,omitempty"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

// JSONRPCRequest はJSON-RPCリクエスト
type JSONRPCRequest struct {
    JSONRPC string      `json:"jsonrpc"`
    ID      int         `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse はJSON-RPCレスポンス
type JSONRPCResponse struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      int             `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError はJSON-RPCエラー
type JSONRPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// client.go
package mcp

import (
    "context"
    "sync"
)

// Client はMCPクライアント
type Client struct {
    name      string
    transport Transport
    tools     []Tool
    prompts   []Prompt
    resources []Resource
    mu        sync.RWMutex
    requestID int
}

// NewClient は新しいMCPクライアントを作成する
func NewClient(name string, transport Transport) *Client {
    return &Client{
        name:      name,
        transport: transport,
    }
}

// Connect はサーバーに接続する
func (c *Client) Connect(ctx context.Context) error {
    if err := c.transport.Connect(ctx); err != nil {
        return err
    }

    // Initialize
    if err := c.initialize(ctx); err != nil {
        return err
    }

    // Discover tools
    return c.discover(ctx)
}

// CallTool はツールを実行する
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
    req := &JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      c.nextID(),
        Method:  "tools/call",
        Params: map[string]interface{}{
            "name":      name,
            "arguments": args,
        },
    }

    resp, err := c.transport.Send(ctx, req)
    if err != nil {
        return nil, err
    }

    if resp.Error != nil {
        return nil, &MCPError{
            Code:    ErrToolExecution,
            Message: resp.Error.Message,
        }
    }

    return resp.Result, nil
}

// ListPrompts はプロンプト一覧を取得する
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
    // prompts/list を呼び出し
}

// GetPrompt はプロンプトを引数で展開して取得する
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*PromptResult, error) {
    // prompts/get を呼び出し
}

// ListResources はリソース一覧を取得する
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
    // resources/list を呼び出し
}

// ReadResource はリソースを読み取る
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceContent, error) {
    // resources/read を呼び出し
}

// transport/transport.go
package transport

import (
    "context"
    "github.com/towada/geminimini/internal/mcp"
)

// Transport はMCPトランスポートインターフェース
type Transport interface {
    Connect(ctx context.Context) error
    Send(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error)
    Close() error
}

// transport/stdio.go
package transport

import (
    "bufio"
    "context"
    "encoding/json"
    "os/exec"
)

// StdioTransport はStdioトランスポート
type StdioTransport struct {
    command string
    args    []string
    env     []string
    cwd     string

    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout *bufio.Reader
}

// NewStdioTransport は新しいStdioトランスポートを作成する
func NewStdioTransport(cfg *config.MCPServerConfig) *StdioTransport {
    return &StdioTransport{
        command: cfg.Command,
        args:    cfg.Args,
        env:     mapToEnv(cfg.Env),
        cwd:     cfg.CWD,
    }
}

// Connect はプロセスを起動して接続する
func (t *StdioTransport) Connect(ctx context.Context) error {
    t.cmd = exec.CommandContext(ctx, t.command, t.args...)
    t.cmd.Env = append(os.Environ(), t.env...)
    if t.cwd != "" {
        t.cmd.Dir = t.cwd
    }

    var err error
    t.stdin, err = t.cmd.StdinPipe()
    if err != nil {
        return err
    }

    stdout, err := t.cmd.StdoutPipe()
    if err != nil {
        return err
    }
    t.stdout = bufio.NewReader(stdout)

    return t.cmd.Start()
}

// Send はリクエストを送信してレスポンスを受信する
func (t *StdioTransport) Send(ctx context.Context, req *mcp.JSONRPCRequest) (*mcp.JSONRPCResponse, error) {
    data, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }

    if _, err := t.stdin.Write(append(data, '\n')); err != nil {
        return nil, err
    }

    line, err := t.stdout.ReadBytes('\n')
    if err != nil {
        return nil, err
    }

    var resp mcp.JSONRPCResponse
    if err := json.Unmarshal(line, &resp); err != nil {
        return nil, err
    }

    return &resp, nil
}

// Close はプロセスを終了する
func (t *StdioTransport) Close() error {
    if t.stdin != nil {
        t.stdin.Close()
    }
    if t.cmd != nil && t.cmd.Process != nil {
        return t.cmd.Process.Kill()
    }
    return nil
}
```

### 2.4 設定 (internal/config)

```go
// config.go
package config

// Config は設定
type Config struct {
    Security   SecurityConfig            `json:"security"`
    MCPServers map[string]MCPServerConfig `json:"mcpServers"`
    General    GeneralConfig             `json:"general"`
    Output     OutputConfig              `json:"output"`
}

// SecurityConfig はセキュリティ設定
type SecurityConfig struct {
    Auth AuthConfig `json:"auth"`
}

// AuthConfig は認証設定
type AuthConfig struct {
    SelectedType string `json:"selectedType"`
}

// MCPServerConfig はMCPサーバー設定
type MCPServerConfig struct {
    // Stdio
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    CWD     string            `json:"cwd,omitempty"`

    // HTTP/SSE
    URL     string            `json:"url,omitempty"`
    Type    string            `json:"type,omitempty"` // "sse" | "http"
    Headers map[string]string `json:"headers,omitempty"`

    // Common
    Timeout      int      `json:"timeout,omitempty"`
    Trust        bool     `json:"trust,omitempty"`
    IncludeTools []string `json:"includeTools,omitempty"`
    ExcludeTools []string `json:"excludeTools,omitempty"`
}

// GeneralConfig は一般設定
type GeneralConfig struct {
    PreviewFeatures bool `json:"previewFeatures"`
}

// OutputConfig は出力設定
type OutputConfig struct {
    Format string `json:"format"`
}

// loader.go
package config

import (
    "encoding/json"
    "os"
    "path/filepath"
)

const (
    geminiDir    = ".gemini"
    settingsFile = "settings.json"
)

// Load は設定を読み込む
func Load() (*Config, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return nil, err
    }

    // デフォルト設定
    cfg := DefaultConfig()

    // グローバル設定
    globalPath := filepath.Join(home, geminiDir, settingsFile)
    if err := loadFile(globalPath, cfg); err != nil && !os.IsNotExist(err) {
        return nil, err
    }

    // プロジェクト設定（オプション）
    cwd, _ := os.Getwd()
    projectPath := filepath.Join(cwd, geminiDir, settingsFile)
    if err := loadFile(projectPath, cfg); err != nil && !os.IsNotExist(err) {
        return nil, err
    }

    return cfg, nil
}

func loadFile(path string, cfg *Config) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, cfg)
}

// defaults.go
package config

// DefaultConfig はデフォルト設定を返す
func DefaultConfig() *Config {
    return &Config{
        Security: SecurityConfig{
            Auth: AuthConfig{
                SelectedType: "oauth-personal",
            },
        },
        MCPServers: make(map[string]MCPServerConfig),
        General: GeneralConfig{
            PreviewFeatures: false,
        },
        Output: OutputConfig{
            Format: "text",
        },
    }
}
```

### 2.5 出力 (internal/output)

```go
// formatter.go
package output

import (
    "io"
    "github.com/towada/geminimini/internal/api"
)

// Formatter は出力フォーマッタインターフェース
type Formatter interface {
    // WriteResponse はレスポンスを出力する
    WriteResponse(resp *api.GenerateResponse) error

    // WriteStreamEvent はストリームイベントを出力する
    WriteStreamEvent(event *api.StreamEvent) error

    // WriteError はエラーを出力する
    WriteError(err error) error
}

// NewFormatter は指定された形式のフォーマッタを作成する
func NewFormatter(format string, w io.Writer) (Formatter, error) {
    switch format {
    case "text":
        return NewTextFormatter(w), nil
    case "json":
        return NewJSONFormatter(w), nil
    case "stream-json":
        return NewStreamJSONFormatter(w), nil
    default:
        return nil, fmt.Errorf("unknown format: %s", format)
    }
}

// text.go
package output

import (
    "fmt"
    "io"
)

// TextFormatter はテキストフォーマッタ
type TextFormatter struct {
    w io.Writer
}

func NewTextFormatter(w io.Writer) *TextFormatter {
    return &TextFormatter{w: w}
}

func (f *TextFormatter) WriteResponse(resp *api.GenerateResponse) error {
    if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
        text := resp.Candidates[0].Content.Parts[0].Text
        _, err := fmt.Fprintln(f.w, text)
        return err
    }
    return nil
}

func (f *TextFormatter) WriteStreamEvent(event *api.StreamEvent) error {
    if event.Text != "" {
        _, err := fmt.Fprint(f.w, event.Text)
        return err
    }
    return nil
}

func (f *TextFormatter) WriteError(err error) error {
    _, writeErr := fmt.Fprintf(f.w, "Error: %v\n", err)
    return writeErr
}

// json.go
package output

import (
    "encoding/json"
    "io"
)

// JSONFormatter はJSONフォーマッタ
type JSONFormatter struct {
    w   io.Writer
    enc *json.Encoder
}

func NewJSONFormatter(w io.Writer) *JSONFormatter {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return &JSONFormatter{w: w, enc: enc}
}

// OutputResponse はJSON出力用レスポンス
type OutputResponse struct {
    Model        string             `json:"model"`
    Response     string             `json:"response"`
    Usage        *api.UsageMetadata `json:"usage,omitempty"`
    FinishReason string             `json:"finish_reason,omitempty"`
}

func (f *JSONFormatter) WriteResponse(resp *api.GenerateResponse) error {
    out := OutputResponse{
        Usage: &resp.UsageMetadata,
    }
    if len(resp.Candidates) > 0 {
        out.FinishReason = resp.Candidates[0].FinishReason
        if len(resp.Candidates[0].Content.Parts) > 0 {
            out.Response = resp.Candidates[0].Content.Parts[0].Text
        }
    }
    return f.enc.Encode(out)
}

// stream.go
package output

import (
    "encoding/json"
    "io"
)

// StreamJSONFormatter はストリームJSONフォーマッタ
type StreamJSONFormatter struct {
    w io.Writer
}

func NewStreamJSONFormatter(w io.Writer) *StreamJSONFormatter {
    return &StreamJSONFormatter{w: w}
}

func (f *StreamJSONFormatter) WriteStreamEvent(event *api.StreamEvent) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    _, err = f.w.Write(append(data, '\n'))
    return err
}
```

## 3. 処理フロー詳細

### 3.1 メイン処理フロー

```go
func run(cmd *cobra.Command, args []string) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // シグナルハンドリング
    ctx = setupSignalHandler(ctx, cancel)

    // 1. 設定読み込み
    cfg, err := config.Load()
    if err != nil {
        return exitError(ErrConfig, err)
    }

    // 2. 認証
    authMgr, err := auth.NewOAuthManager()
    if err != nil {
        return exitError(ErrAuth, err)
    }

    creds, err := authMgr.LoadCredentials()
    if err != nil {
        return exitError(ErrAuth, err)
    }

    if creds.IsExpired() {
        creds, err = authMgr.RefreshToken(creds)
        if err != nil {
            return exitError(ErrAuth, err)
        }
    }

    // 3. APIクライアント作成
    httpClient := authMgr.HTTPClient(creds)
    apiClient := api.NewClient(httpClient)

    // 4. MCP初期化（設定がある場合）
    var mcpManager *mcp.Manager
    if len(cfg.MCPServers) > 0 {
        mcpManager = mcp.NewManager(cfg.MCPServers)
        if err := mcpManager.ConnectAll(ctx); err != nil {
            // MCPエラーは警告として続行
            log.Warn("MCP connection failed:", err)
        }
        defer mcpManager.CloseAll()
    }

    // 5. 入力準備
    input, err := prepareInput(prompt, files, dirs)
    if err != nil {
        return exitError(ErrInput, err)
    }

    // 6. フォーマッタ作成
    formatter, err := output.NewFormatter(outputFormat, os.Stdout)
    if err != nil {
        return exitError(ErrConfig, err)
    }

    // 7. リクエスト実行
    req := &api.GenerateRequest{
        Model: model,
        Contents: []api.Content{{
            Role:  "user",
            Parts: []api.Part{{Text: input}},
        }},
        Config: api.GenerationConfig{
            Temperature:     1.0,
            TopP:            0.95,
            MaxOutputTokens: 8192,
        },
    }

    // MCPツールがあれば追加
    if mcpManager != nil {
        req.Tools = mcpManager.GetToolDeclarations()
    }

    // 8. 生成実行
    switch outputFormat {
    case "stream-json":
        return runStreaming(ctx, apiClient, mcpManager, req, formatter)
    default:
        return runNonStreaming(ctx, apiClient, mcpManager, req, formatter)
    }
}
```

### 3.2 ツール実行ループ

```go
func runWithToolLoop(ctx context.Context, client *api.Client, mcpMgr *mcp.Manager, req *api.GenerateRequest, formatter output.Formatter) error {
    maxIterations := 10

    for i := 0; i < maxIterations; i++ {
        resp, err := client.Generate(ctx, req)
        if err != nil {
            return err
        }

        // ツール呼び出しがなければ終了
        if len(resp.Candidates) == 0 || resp.Candidates[0].Content.Parts[0].FunctionCall == nil {
            return formatter.WriteResponse(resp)
        }

        // ツール実行
        fc := resp.Candidates[0].Content.Parts[0].FunctionCall
        result, err := mcpMgr.CallTool(ctx, fc.Name, fc.Args)
        if err != nil {
            return err
        }

        // 履歴に追加して再実行
        req.Contents = append(req.Contents,
            api.Content{
                Role:  "model",
                Parts: []api.Part{{FunctionCall: fc}},
            },
            api.Content{
                Role:  "user",
                Parts: []api.Part{{FunctionResp: &api.FunctionResp{
                    Name:     fc.Name,
                    Response: result.(map[string]interface{}),
                }}},
            },
        )
    }

    return fmt.Errorf("max tool iterations reached")
}
```

## 4. エラー処理詳細

```go
// internal/errors.go
package internal

// ExitCode は終了コード
type ExitCode int

const (
    ExitSuccess   ExitCode = 0
    ExitGeneral   ExitCode = 1
    ExitAuth      ExitCode = 2
    ExitAPI       ExitCode = 3
    ExitConfig    ExitCode = 4
    ExitMCP       ExitCode = 5
    ExitInterrupt ExitCode = 130
)

// AppError はアプリケーションエラー
type AppError struct {
    Code       ExitCode
    Type       string
    Message    string
    Suggestion string
    Cause      error
}

func (e *AppError) Error() string {
    return e.Message
}

func (e *AppError) Unwrap() error {
    return e.Cause
}

// ToJSON はJSON形式に変換する
func (e *AppError) ToJSON() map[string]interface{} {
    return map[string]interface{}{
        "error": map[string]interface{}{
            "code":       int(e.Code),
            "type":       e.Type,
            "message":    e.Message,
            "suggestion": e.Suggestion,
        },
    }
}
```

## 5. 依存関係

```go
// go.mod
module github.com/towada/geminimini

go 1.21

require (
    github.com/spf13/cobra v1.8.0
    golang.org/x/oauth2 v0.15.0
)

// 間接依存（最小限に抑える）
require (
    github.com/inconshreveable/mousetrap v1.1.0 // indirect
    github.com/spf13/pflag v1.0.5 // indirect
)
```

## 6. ビルド設定

```makefile
# Makefile

VERSION := $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: build clean test lint

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o geminimini .

build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/geminimini-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/geminimini-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/geminimini-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/geminimini-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/geminimini-windows-amd64.exe .

test:
	go test -v -race -cover ./...

lint:
	go vet ./...
	gofmt -d .

clean:
	rm -rf geminimini dist/
```
