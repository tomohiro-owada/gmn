<p align="center">
  <img src="g.png" alt="g logo" width="150">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/github/v/release/k-sub1995/g?style=for-the-badge" alt="Release">
  <img src="https://img.shields.io/github/license/k-sub1995/g?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/github/actions/workflow/status/k-sub1995/g/ci.yml?style=for-the-badge&label=CI" alt="CI">
</p>

<p align="center">
  <strong>A lightweight Gemini CLI and TUI written in Go</strong><br>
  <em>A love letter to <a href="https://github.com/google-gemini/gemini-cli">Google's Gemini CLI</a></em>
</p>

<p align="center">
  <a href="#-why-g">Why g?</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-mcp-support">MCP</a> •
  <a href="#-benchmarks">Benchmarks</a>
</p>

---

## ⚡ Why g?

The official Gemini CLI is an **amazing tool** with excellent MCP support and seamless Google authentication. However, for scripting and automation, its Node.js runtime adds startup overhead.

**g** reimplements the core functionality in Go, achieving **37x faster startup** while maintaining full compatibility with the official CLI's authentication.

```
$ time g "hi" > /dev/null
0.02s user 0.01s system

$ time gemini -p "hi" > /dev/null
0.94s user 0.20s system
```

## 📦 Installation

### ⚠️ Prerequisites (Required)

**g does not have its own authentication.** You must authenticate once using the official Gemini CLI first:

```bash
npm install -g @google/gemini-cli
gemini  # Choose "Login with Google"
```

g reuses these credentials automatically from `~/.gemini/`. Your free tier quota or Workspace Code Assist quota applies.

### Homebrew

```bash
brew install k-sub1995/tap/g
```

### Go

```bash
go install github.com/k-sub1995/g@latest
```

### Binary

Download from [Releases](https://github.com/k-sub1995/g/releases)

## 🚀 Quick Start

### CLI Mode

Pass a prompt directly:

```bash
g "What is the capital of Japan?"
```

### TUI Mode

Launch interactive TUI (Text User Interface):

```bash
g
```

The TUI provides:

- Interactive prompt editing with arrow key navigation
- Real-time streaming responses
- Command history

## 📋 Usage

```bash
g [prompt] [flags]
g mcp <command>
g version

Flags:
  -p, --prompt string          Prompt (alternative to positional arg)
  -m, --model string           Model (default "gemini-2.5-flash")
  -f, --file strings           Files to include
  -o, --output-format string   text, json, stream-json (default "text")
  -t, --timeout duration       Timeout (default 5m)
      --debug                  Debug output
  -v, --version                Version

MCP Commands:
  g mcp list                 List MCP servers and tools
  g mcp call <server> <tool> Call an MCP tool

Version Command:
  g version                  Print the version number of g
```

## 🔌 MCP Support

g supports [Model Context Protocol](https://modelcontextprotocol.io/) servers.

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

```bash
# List available tools
g mcp list

# Call a tool
g mcp call my-server tool-name arg=value
```

## 📊 Benchmarks

| Metric  | g        | Official CLI | Improvement |
| ------- | -------- | ------------ | ----------- |
| Startup | **23ms** | 847ms        | **37x**     |
| Binary  | 5.6MB    | ~200MB       | **35x**     |
| Runtime | None     | Node.js      | -           |

## 🏗️ Build

```bash
git clone https://github.com/k-sub1995/g.git
cd g
make build          # Current platform
make cross-compile  # All platforms
```

## 🚫 What's NOT Included

- OAuth flow → authenticate with official CLI first
- API Key / Vertex AI auth

## 📄 License

Apache License 2.0 — See [LICENSE](LICENSE)

This project is a derivative work based on [Gemini CLI](https://github.com/google-gemini/gemini-cli) by Google LLC.

## 🙏 Acknowledgments

- [Google Gemini CLI](https://github.com/google-gemini/gemini-cli) — The incredible original
- [Google Gemini API](https://ai.google.dev/) — The underlying API
