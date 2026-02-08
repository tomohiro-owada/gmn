# 仕様書

## 1. システム概要

geminimini は Google Gemini API を利用した非対話型 CLI ツールである。
公式 Gemini CLI の認証情報・設定を流用し、軽量・高速な実行環境を提供する。

## 2. システム構成

```
┌─────────────────────────────────────────────────────────────┐
│                        geminimini                           │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐│
│  │   CLI   │  │  Auth   │  │   API   │  │   MCP Client    ││
│  │ Command │──│ Manager │──│ Client  │──│ (Stdio/SSE/HTTP)││
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘│
│       │            │            │               │          │
│       ▼            ▼            ▼               ▼          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────────┐│
│  │ Config  │  │~/.gemini│  │ Gemini  │  │   MCP Server    ││
│  │ Loader  │  │/oauth_* │  │   API   │  │   (External)    ││
│  └─────────┘  └─────────┘  └─────────┘  └─────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## 3. コマンドライン仕様

### 3.1 基本構文

```
geminimini [flags] [-p <prompt>]
```

### 3.2 フラグ一覧

| フラグ | 短縮 | 型 | デフォルト | 説明 |
|--------|------|-----|-----------|------|
| `--prompt` | `-p` | string | (必須) | 送信するプロンプト |
| `--model` | `-m` | string | `gemini-2.5-flash` | 使用するモデル |
| `--output-format` | `-o` | string | `text` | 出力形式: text, json, stream-json |
| `--include-files` | `-f` | []string | [] | コンテキストに含めるファイル |
| `--include-dirs` | `-d` | []string | [] | コンテキストに含めるディレクトリ |
| `--timeout` | `-t` | duration | `5m` | API タイムアウト |
| `--debug` | | bool | false | デバッグ出力 |
| `--version` | `-v` | bool | false | バージョン表示 |
| `--help` | `-h` | bool | false | ヘルプ表示 |

### 3.3 出力形式

#### text（デフォルト）
ストリーミングでプレーンテキストをリアルタイム出力。人間が見る用途に最適。

```
$ geminimini -p "Hello"
Hello! How can I help you today?  # ← リアルタイムで文字が流れる
```

#### json
非ストリーミング。生成完了後にまとめて構造化 JSON を出力。スクリプトでのパース用途に最適。

```json
{
  "model": "gemini-2.5-flash",
  "response": "Hello! How can I help you today?",
  "usage": {
    "promptTokenCount": 5,
    "candidatesTokenCount": 12,
    "totalTokenCount": 17
  },
  "finishReason": "STOP"
}
```

#### stream-json
ストリーミング形式で NDJSON (Newline Delimited JSON) を出力。リアルタイム処理が必要なスクリプト用途。

```json
{"type":"start","model":"gemini-2.5-flash"}
{"type":"content","text":"Hello"}
{"type":"content","text":"! How can"}
{"type":"content","text":" I help you today?"}
{"type":"tool_call","name":"search","args":{"query":"example"}}
{"type":"tool_result","name":"search","result":{...}}
{"type":"done","usage":{"promptTokenCount":5,"candidatesTokenCount":12,"totalTokenCount":17}}
```

**ストリーミングイベント種別**

| type | 説明 |
|------|------|
| `start` | 生成開始、モデル名を含む |
| `content` | テキストチャンク |
| `tool_call` | MCPツール呼び出し要求 |
| `tool_result` | MCPツール実行結果 |
| `done` | 生成完了、使用量を含む |
| `error` | エラー発生 |

### 3.4 標準入力

標準入力が存在する場合、プロンプトの前に追加される。

```bash
cat code.go | geminimini -p "このコードをレビューして"
```

上記は以下と等価:
```
geminimini -p "<code.goの内容>\n\nこのコードをレビューして"
```

## 4. 認証仕様

### 4.1 認証フロー

```
┌──────────────────────────────────────────────────────────────┐
│                      認証フロー                               │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  1. ~/.gemini/settings.json から認証タイプを確認             │
│     └─ security.auth.selectedType == "oauth-personal"       │
│                                                              │
│  2. OAuth トークンを読み込み                                  │
│     ├─ macOS: Keychain (gemini-cli-oauth) を優先            │
│     ├─ Linux/Windows: ~/.gemini/oauth_creds.json のみ       │
│     └─ Fallback: ~/.gemini/oauth_creds.json                 │
│                                                              │
│  3. トークン有効期限を確認                                    │
│     ├─ 有効: そのまま使用                                    │
│     └─ 期限切れ: refresh_token でリフレッシュ               │
│                                                              │
│  4. API リクエストに Authorization ヘッダーを付与            │
│     └─ Authorization: Bearer <access_token>                 │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**プラットフォーム別トークン保存先**

| OS | 優先 | フォールバック |
|----|------|--------------|
| macOS | Keychain (`gemini-cli-oauth`) | `~/.gemini/oauth_creds.json` |
| Linux | - | `~/.gemini/oauth_creds.json` |
| Windows | - | `~/.gemini/oauth_creds.json` |

### 4.2 認証ファイル構造

#### ~/.gemini/settings.json
```json
{
  "security": {
    "auth": {
      "selectedType": "oauth-personal"
    }
  }
}
```

#### ~/.gemini/oauth_creds.json（レガシー形式）
```json
{
  "access_token": "ya29.xxx...",
  "refresh_token": "1//xxx...",
  "token_type": "Bearer",
  "expiry_date": 1234567890000
}
```

#### ~/.gemini/google_accounts.json
```json
{
  "active": "user@gmail.com",
  "old": []
}
```

### 4.3 トークンリフレッシュ

トークン期限切れ時、以下のエンドポイントでリフレッシュを試行:

- Endpoint: `https://oauth2.googleapis.com/token`
- Method: POST
- Body:
  ```
  grant_type=refresh_token
  refresh_token=<refresh_token>
  client_id=<client_id>
  client_secret=<client_secret>
  ```

リフレッシュ失敗時は認証エラーを返し、公式 CLI での再認証を促す。

## 5. API 仕様

### 5.1 エンドポイント

- Base URL: `https://generativelanguage.googleapis.com/v1beta`
- Generate Content: `POST /models/{model}:generateContent`
- Stream Generate: `POST /models/{model}:streamGenerateContent`

### 5.2 リクエスト形式

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "プロンプト内容"}
      ]
    }
  ],
  "generationConfig": {
    "temperature": 1.0,
    "topP": 0.95,
    "topK": 40,
    "maxOutputTokens": 8192
  }
}
```

### 5.3 レスポンス形式

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {"text": "レスポンス内容"}
        ],
        "role": "model"
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 10,
    "candidatesTokenCount": 50,
    "totalTokenCount": 60
  }
}
```

## 6. MCP 仕様

### 6.1 サポートするトランスポート

| トランスポート | 設定方法 | 用途 |
|--------------|---------|------|
| Stdio | `command` + `args` | ローカルプロセス |
| SSE | `url` + `type: "sse"` | リモート (Server-Sent Events) |
| HTTP | `url` + `type: "http"` | リモート (Streamable HTTP) |

### 6.2 MCP 設定形式

```json
{
  "mcpServers": {
    "serverName": {
      "command": "executable",
      "args": ["arg1", "arg2"],
      "env": {"KEY": "value"},
      "cwd": "/path/to/dir",
      "timeout": 600000
    }
  }
}
```

### 6.3 MCP プロトコル

JSON-RPC 2.0 over 各トランスポート

**ツール機能**
- `tools/list` - ツール一覧取得
- `tools/call` - ツール実行

**プロンプト機能**
- `prompts/list` - プロンプトテンプレート一覧取得
- `prompts/get` - プロンプト取得（引数展開）

**リソース機能**
- `resources/list` - リソース一覧取得
- `resources/read` - リソース読み取り

**接続管理**
- `initialize` - サーバー初期化・キャパビリティ交換
- `shutdown` - 接続終了

## 7. エラー仕様

### 7.1 終了コード

| コード | 意味 |
|-------|------|
| 0 | 正常終了 |
| 1 | 一般エラー |
| 2 | 認証エラー |
| 3 | API エラー |
| 4 | 設定エラー |
| 5 | MCP エラー |
| 130 | ユーザー中断 (Ctrl+C) |

### 7.2 エラー出力形式

#### text モード
```
Error: authentication failed: token expired
Please re-authenticate using: gemini
```

#### json モード
```json
{
  "error": {
    "code": 2,
    "type": "AuthError",
    "message": "authentication failed: token expired",
    "suggestion": "Please re-authenticate using: gemini"
  }
}
```
