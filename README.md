<p align="center">
  <img src="gmn.png" alt="gmn logo" width="150">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/github/v/release/tomohiro-owada/gmn?style=for-the-badge" alt="Release">
  <img src="https://img.shields.io/github/license/tomohiro-owada/gmn?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/github/actions/workflow/status/tomohiro-owada/gmn/ci.yml?style=for-the-badge&label=CI" alt="CI">
</p>

<p align="center">
  <strong>A lightweight, non-interactive Gemini CLI written in Go</strong><br>
  <em>A love letter to <a href="https://github.com/google-gemini/gemini-cli">Google's Gemini CLI</a></em>
</p>

<p align="center">
  <a href="#-why-gmn">Why gmn?</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-mcp-support">MCP</a> •
  <a href="#-benchmarks">Benchmarks</a>
</p>

---

## ⚡ Why gmn?

The official Gemini CLI is an **amazing tool** with excellent MCP support and seamless Google authentication. However, for scripting and automation, its Node.js runtime adds startup overhead.

**gmn** reimplements the core functionality in Go, achieving **37x faster startup** while maintaining full compatibility with the official CLI's authentication.

```
$ time gmn "hi" > /dev/null
0.02s user 0.01s system

$ time gemini -p "hi" > /dev/null
0.94s user 0.20s system
```

## 📦 Installation

### ⚠️ Prerequisites (Required)

**gmn does not have its own authentication.** You must authenticate once using the official Gemini CLI first:

```bash
npm install -g @google/gemini-cli
gemini  # Choose "Login with Google"
```

gmn reuses these credentials automatically from `~/.gemini/`. Your free tier quota or Workspace Code Assist quota applies.

### Homebrew
```bash
brew install tomohiro-owada/tap/gmn
```

### Go
```bash
go install github.com/tomohiro-owada/gmn@latest
```

### Binary
Download from [Releases](https://github.com/tomohiro-owada/gmn/releases)

## 🚀 Quick Start

```bash
# Simple prompt
gmn "Explain quantum computing"

# With file context
gmn "Review this code" -f main.go

# Pipe input
cat error.log | gmn "What's wrong?"

# JSON output
gmn "List 3 colors" -o json

# Use different model
gmn "Write a poem" -m gemini-2.5-pro
```

## 📋 Usage

```
gmn [prompt] [flags]
gmn mcp <command>

Flags:
  -p, --prompt string          Prompt (alternative to positional arg)
  -m, --model string           Model (default "gemini-2.5-flash")
  -f, --file strings           Files to include
  -o, --output-format string   text, json, stream-json (default "text")
  -t, --timeout duration       Timeout (default 5m)
      --debug                  Debug output
  -v, --version                Version

MCP Commands:
  gmn mcp list                 List MCP servers and tools
  gmn mcp call <server> <tool> Call an MCP tool
```

## 🔌 MCP Support

gmn supports [Model Context Protocol](https://modelcontextprotocol.io/) servers.

Configure in `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/mcp-server"
    }
  }
}
```

`gmn` also understands newer Gemini CLI config fields such as
`requiredMcpServers`. Required servers are merged into the effective MCP server
set and override local servers with the same name, matching the upstream
configuration model.

```bash
# List available tools
gmn mcp list

# Call a tool
gmn mcp call my-server tool-name arg=value
```

Current limitation: `gmn mcp` executes stdio-based MCP servers. HTTP/SSE MCP
server definitions are parsed and preserved for config compatibility, but direct
HTTP/SSE transport execution is not implemented yet.

## 📊 Benchmarks

| Metric | gmn | Official CLI | Improvement |
|--------|-----|--------------|-------------|
| Startup | **23ms** | 847ms | **37x** |
| Binary | 5.6MB | ~200MB | **35x** |
| Runtime | None | Node.js | - |

## 🏗️ Build

```bash
git clone https://github.com/tomohiro-owada/gmn.git
cd gmn
make build          # Current platform
make cross-compile  # All platforms
```

## 🚫 What's NOT Included

- Interactive/TUI mode → use official CLI
- OAuth flow → authenticate with official CLI first
- API Key / Vertex AI auth

## 📄 License

Apache License 2.0 — See [LICENSE](LICENSE)

This project is a derivative work based on [Gemini CLI](https://github.com/google-gemini/gemini-cli) by Google LLC.

## 🙏 Acknowledgments

- [Google Gemini CLI](https://github.com/google-gemini/gemini-cli) — The incredible original
- [Google Gemini API](https://ai.google.dev/) — The underlying API
