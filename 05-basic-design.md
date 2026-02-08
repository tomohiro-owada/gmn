# 基本設計

## 1. システムアーキテクチャ

### 1.1 全体構成

```
┌──────────────────────────────────────────────────────────────────────┐
│                            geminimini                                │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                      Presentation Layer                        │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐│ │
│  │  │   CLI    │  │  Input   │  │  Output  │  │     Formatter    ││ │
│  │  │  Parser  │  │  Reader  │  │  Writer  │  │ (text/json/stream)│ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘│ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                 │                                    │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                      Application Layer                         │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐│ │
│  │  │ Generate │  │  Stream  │  │   Tool   │  │     Session      ││ │
│  │  │ Service  │  │ Service  │  │ Executor │  │    Manager       ││ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘│ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                 │                                    │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                      Infrastructure Layer                      │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐│ │
│  │  │   Auth   │  │  Gemini  │  │   MCP    │  │     Config       ││ │
│  │  │ Manager  │  │  Client  │  │  Client  │  │     Loader       ││ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘│ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                 │                                    │
└─────────────────────────────────┼────────────────────────────────────┘
                                  │
          ┌───────────────────────┼───────────────────────┐
          │                       │                       │
          ▼                       ▼                       ▼
   ┌──────────────┐      ┌──────────────┐      ┌──────────────┐
   │   ~/.gemini/ │      │  Gemini API  │      │  MCP Server  │
   │  (認証/設定)  │      │              │      │  (External)  │
   └──────────────┘      └──────────────┘      └──────────────┘
```

### 1.2 レイヤー責務

| レイヤー | 責務 |
|---------|------|
| Presentation | CLI パース、入出力、フォーマット |
| Application | ビジネスロジック、ツール実行オーケストレーション |
| Infrastructure | 外部システムとの通信、永続化 |

## 2. パッケージ構成

```
geminimini/
├── main.go                 # エントリーポイント
├── cmd/
│   └── root.go            # CLI コマンド定義
├── internal/
│   ├── auth/              # 認証管理
│   │   ├── auth.go        # 認証インターフェース
│   │   ├── oauth.go       # OAuth トークン管理
│   │   └── keychain.go    # macOS Keychain アクセス
│   ├── api/               # Gemini API クライアント
│   │   ├── client.go      # API クライアント
│   │   ├── request.go     # リクエスト構造体
│   │   ├── response.go    # レスポンス構造体
│   │   └── stream.go      # ストリーミング処理
│   ├── mcp/               # MCP クライアント
│   │   ├── client.go      # MCP クライアント
│   │   ├── transport/     # トランスポート実装
│   │   │   ├── stdio.go   # Stdio トランスポート
│   │   │   ├── sse.go     # SSE トランスポート
│   │   │   └── http.go    # HTTP トランスポート
│   │   └── protocol.go    # JSON-RPC プロトコル
│   ├── config/            # 設定管理
│   │   ├── config.go      # 設定構造体
│   │   └── loader.go      # 設定読み込み
│   ├── input/             # 入力処理
│   │   ├── reader.go      # 標準入力読み込み
│   │   └── file.go        # ファイル読み込み
│   └── output/            # 出力処理
│       ├── text.go        # テキスト出力
│       ├── json.go        # JSON 出力
│       └── stream.go      # ストリーミング出力
├── go.mod
├── go.sum
└── Makefile
```

## 3. コンポーネント設計

### 3.1 CLI コンポーネント

```go
// cmd/root.go
type CLI struct {
    prompt       string
    model        string
    outputFormat string
    files        []string
    dirs         []string
    timeout      time.Duration
    debug        bool
}
```

**責務**:
- コマンドライン引数のパース
- 各コンポーネントの初期化と連携
- 終了コードの管理

### 3.2 Auth コンポーネント

```go
// internal/auth/auth.go
type Credentials struct {
    AccessToken  string
    RefreshToken string
    TokenType    string
    ExpiresAt    time.Time
}

type AuthManager interface {
    LoadCredentials() (*Credentials, error)
    RefreshToken(creds *Credentials) (*Credentials, error)
    GetHTTPClient() (*http.Client, error)
}
```

**責務**:
- OAuth トークンの読み込み
- トークンのリフレッシュ
- 認証済み HTTP クライアントの提供

### 3.3 API コンポーネント

```go
// internal/api/client.go
type Client struct {
    httpClient *http.Client
    baseURL    string
    config     *config.Config
}

type GenerateRequest struct {
    Model    string
    Contents []Content
    Config   GenerationConfig
}

type GenerateResponse struct {
    Text         string
    FinishReason string
    Usage        UsageMetadata
    ToolCalls    []ToolCall
}
```

**責務**:
- Gemini API との通信
- リクエスト/レスポンスのシリアライズ
- ストリーミングレスポンスの処理

### 3.4 MCP コンポーネント

```go
// internal/mcp/client.go
type MCPClient struct {
    transport Transport
    tools     []Tool
}

type Transport interface {
    Connect() error
    Send(request *JSONRPCRequest) error
    Receive() (*JSONRPCResponse, error)
    Close() error
}

type Tool struct {
    Name        string
    Description string
    InputSchema json.RawMessage
}
```

**責務**:
- MCP サーバーとの接続管理
- ツールの発見と実行
- JSON-RPC プロトコル処理

### 3.5 Config コンポーネント

```go
// internal/config/config.go
type Config struct {
    Security   SecurityConfig
    MCPServers map[string]MCPServerConfig
    General    GeneralConfig
}

type MCPServerConfig struct {
    Command string
    Args    []string
    Env     map[string]string
    CWD     string
    URL     string
    Type    string
    Headers map[string]string
    Timeout int
}
```

**責務**:
- 設定ファイルの読み込み
- デフォルト値の適用
- 環境変数によるオーバーライド

## 4. データフロー

### 4.1 基本リクエストフロー

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│  User   │───▶│   CLI   │───▶│  Auth   │───▶│   API   │
│         │    │         │    │ Manager │    │ Client  │
└─────────┘    └─────────┘    └─────────┘    └─────────┘
                    │                             │
                    │                             ▼
                    │                      ┌─────────────┐
                    │                      │ Gemini API  │
                    │                      └─────────────┘
                    │                             │
                    ▼                             ▼
              ┌─────────┐                  ┌─────────────┐
              │ Output  │◀─────────────────│  Response   │
              │ Writer  │                  │             │
              └─────────┘                  └─────────────┘
```

### 4.2 MCP ツール実行フロー

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│ Gemini  │───▶│   API   │───▶│  Tool   │───▶│   MCP   │
│   API   │    │ Client  │    │Executor │    │ Client  │
└─────────┘    └─────────┘    └─────────┘    └─────────┘
     ▲                                            │
     │                                            ▼
     │                                     ┌─────────────┐
     │                                     │ MCP Server  │
     │                                     └─────────────┘
     │                                            │
     │         ┌─────────┐    ┌─────────┐        │
     └─────────│   API   │◀───│  Tool   │◀───────┘
               │ Client  │    │ Result  │
               └─────────┘    └─────────┘
```

### 4.3 ストリーミングフロー

```
┌─────────────┐      ┌─────────┐      ┌─────────┐      ┌─────────┐
│ Gemini API  │─────▶│ Stream  │─────▶│ Output  │─────▶│ Stdout  │
│   Stream    │chunk │ Reader  │event │ Writer  │      │         │
└─────────────┘      └─────────┘      └─────────┘      └─────────┘
                          │
                          ▼ (if tool_call)
                    ┌─────────┐
                    │   MCP   │
                    │ Client  │
                    └─────────┘
```

## 5. インターフェース設計

### 5.1 外部インターフェース

| インターフェース | プロトコル | 認証 |
|----------------|-----------|------|
| Gemini API | HTTPS REST | OAuth Bearer Token |
| MCP Stdio | Stdio (JSON-RPC) | N/A |
| MCP SSE | HTTPS SSE (JSON-RPC) | Optional Headers |
| MCP HTTP | HTTPS (JSON-RPC) | Optional Headers |

### 5.2 ファイルインターフェース

| ファイル | 形式 | 用途 |
|---------|------|------|
| `~/.gemini/settings.json` | JSON | 設定 |
| `~/.gemini/oauth_creds.json` | JSON | OAuth トークン（レガシー） |
| `~/.gemini/google_accounts.json` | JSON | アカウント情報 |
| macOS Keychain | Binary | OAuth トークン（推奨） |

## 6. エラーハンドリング設計

### 6.1 エラー階層

```
error (Go 標準)
├── AuthError        # 認証関連エラー
│   ├── TokenExpired
│   ├── RefreshFailed
│   └── NoCredentials
├── APIError         # API 関連エラー
│   ├── RateLimited
│   ├── ServerError
│   └── InvalidRequest
├── MCPError         # MCP 関連エラー
│   ├── ConnectionFailed
│   ├── ToolNotFound
│   └── ExecutionFailed
└── ConfigError      # 設定関連エラー
    ├── FileNotFound
    └── ParseError
```

### 6.2 エラー処理方針

| エラー種別 | 対応 |
|-----------|------|
| 一時的エラー (429, 503) | リトライ（指数バックオフ） |
| 認証エラー (401) | トークンリフレッシュ試行 |
| クライアントエラー (4xx) | ユーザーにメッセージ表示 |
| サーバーエラー (5xx) | リトライ後、エラー表示 |
| MCP 接続エラー | サーバー設定確認を促す |

## 7. セキュリティ設計

### 7.1 認証情報の保護

```
┌───────────────────────────────────────────────────────────┐
│                    認証情報フロー                          │
├───────────────────────────────────────────────────────────┤
│                                                           │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐   │
│  │  Keychain   │───▶│    Auth     │───▶│   Memory    │   │
│  │  / File     │    │   Manager   │    │  (Runtime)  │   │
│  └─────────────┘    └─────────────┘    └─────────────┘   │
│                                              │           │
│                                              ▼           │
│                                       ┌─────────────┐   │
│                                       │  HTTP Req   │   │
│                                       │  (Bearer)   │   │
│                                       └─────────────┘   │
│                                                           │
│  ※ ログ出力時はトークンをマスキング                        │
│  ※ 一時ファイルへの書き出し禁止                           │
│  ※ エラーメッセージにトークンを含めない                    │
│                                                           │
└───────────────────────────────────────────────────────────┘
```

### 7.2 入力検証

| 入力 | 検証内容 |
|------|---------|
| ファイルパス | パストラバーサル防止、存在確認 |
| URL | スキーム検証 (https only) |
| MCP コマンド | 設定ファイルからのみ（ユーザー入力不可） |
