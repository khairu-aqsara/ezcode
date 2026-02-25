# EzCode

[![Go Version](https://img.shields.io/badge/go-1.24.5-00ADD8?logo=go)](https://golang.org/doc/go1.24)
[![Module](https://img.shields.io/badge/module-github.com%2Fkhairu--aqsara%2Fezcode-blue)](https://github.com/khairu-aqsara/ezcode)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A terminal UI for semantically exploring any codebase and its Git history through a locally-running MCP server backed by Qdrant vector search.

---

## Overview

EzCode is a Go CLI application built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) that connects to a [`qdrant-mcp-server`](https://github.com/mhalder/qdrant-mcp-server) instance. It supports two launch modes:

- **Docker mode** (default) — EzCode generates a Docker Compose file, starts the server image as a container, and connects over HTTP on `localhost:3000`.
- **Stdio mode** — EzCode spawns the server's Node.js process directly on your machine using stdin/stdout transport. No Docker or container runtime required.

The application exposes four functional tabs: a **Setup** screen for editing credentials and server mode, an **Index Codebase** tab, an **Index Git** tab, and a **Chat** tab that performs a retrieval-augmented generation (RAG) loop. When you submit a question, EzCode calls `contextual_search` on the MCP server to retrieve semantically relevant code and commit context, then forwards that context to your configured LLM endpoint to generate a grounded answer.

Configuration is stored in `~/.ezcode/config.json`. On first run, EzCode opens directly on the Setup tab; on subsequent runs it attempts to connect to an already-running server (or launches one) before showing the Chat tab.

---

## Features

- **Two server modes** — Docker mode runs the MCP server as a container; stdio mode spawns the Node.js process directly, eliminating the Docker/Podman dependency entirely.
- **First-run setup wizard** — interactive TUI form pre-populated with sensible defaults for all required environment variables and server mode selection; validates that an API key is present before saving.
- **Smart MCP connection** — in Docker mode, attempts to reach an already-running server first and skips container startup if it responds. In stdio mode, spawns the process immediately.
- **Index Codebase** — calls `reindex_changes` for incremental indexing of file-system changes, falling back automatically to a full `index_codebase` call whenever the codebase has not been indexed yet or the incremental snapshot is missing.
- **Index Git History** — calls `index_git_history` to embed commit messages, diffs, and author metadata into a separate Qdrant collection.
- **Index status cards** — live display of collection name, chunk count, and last-indexed timestamp fetched from `get_index_status` and `get_git_index_status`.
- **RAG Chat** — submits your question to `contextual_search` (code + Git results), then calls your LLM endpoint (`/chat/completions`) and renders the markdown-formatted answer inline.
- **MCP Prompts** — `Ctrl+P` opens an interactive picker of all prompts exposed by the server (e.g. `investigate_code_with_history`, `security_audit_search`). Type to filter, arrow keys to navigate, `Enter` to load. Prompts can also be invoked directly with `/prompt <name> [key=value ...]` in the chat input.
- **Glamour markdown rendering** — AI responses are rendered with full terminal markdown styling via [Glamour](https://github.com/charmbracelet/glamour).
- **Automatic project path** — the current working directory at launch is used as the project root. In Docker mode it is mounted read-only into the container; in stdio mode the real path is passed directly to the server.
- **Config persistence** — all settings are saved to `~/.ezcode/config.json` and reloaded on the next run.
- **Cross-platform** — Docker exec wrapper has separate implementations for Unix/macOS and Windows (suppresses console flicker on Windows).
- **Retry with backoff** — all MCP tool calls retry up to three times with exponential backoff (1 s, 2 s, 4 s) before surfacing an error.

---

## Prerequisites

### Always required

| Requirement | Notes |
|---|---|
| **Go 1.24+** | Required to build from source. |
| **Qdrant instance** | A running Qdrant server reachable from the machine running the MCP server. The default URL is `http://localhost:6333`. |
| **API key** | An OpenAI-compatible API key for both embeddings and chat. The default config targets the Gemini endpoint, so a [Google AI Studio](https://aistudio.google.com/app/apikey) key works out of the box. |

### Docker mode (default)

| Requirement | Notes |
|---|---|
| **Docker or Podman** with **Compose v2** | The `docker compose` (v2) subcommand must be available in `PATH`. |
| **`qdrant-mcp-server` image** | The Docker image must be present locally. Pull it with `docker pull mhalder/qdrant-mcp-server` or build it from [mhalder/qdrant-mcp-server](https://github.com/mhalder/qdrant-mcp-server). |

### Stdio mode

| Requirement | Notes |
|---|---|
| **Node.js 22 or 24** | The server is a Node.js application. Run `node --version` to confirm. |
| **`qdrant-mcp-server` built locally** | Clone and build the server once: see [Stdio mode setup](#stdio-mode-setup) below. |

---

## Installation

### Build from source

```bash
git clone https://github.com/khairu-aqsara/ezcode.git
cd ezcode
make build
```

The binary is written to `bin/ezcode`.

### Install to PATH (optional)

```bash
make install
```

This runs `go install` and places the binary in `$GOPATH/bin`.

### Cross-compile

```bash
make build-linux          # Linux amd64  → bin/ezcode-linux-amd64
make build-darwin-arm64   # macOS ARM    → bin/ezcode-darwin-arm64
make build-windows        # Windows amd64 → bin/ezcode-windows-amd64.exe
make build-all            # All platforms at once
```

---

## Stdio mode setup

Stdio mode runs the MCP server as a local Node.js subprocess — no Docker or container runtime needed.

### 1. Clone and build the server

```bash
git clone https://github.com/mhalder/qdrant-mcp-server.git
cd qdrant-mcp-server

# Node 22
npm install

# Node 24 (requires C++20 flag for native module compilation)
CXXFLAGS='-std=c++20' npm install

npm run build
# Output: build/index.js
```

### 2. Start Qdrant

Qdrant must be running and reachable at `http://localhost:6333`. The simplest way:

```bash
docker run -d -p 6333:6333 qdrant/qdrant
```

Or use an existing Qdrant instance — just update `QDRANT_URL` in the Setup tab.

### 3. Configure EzCode

In the EzCode **Setup** tab (or by editing `~/.ezcode/config.json` directly), set:

| Field | Value |
|---|---|
| **Server Mode** | `stdio` |
| **Server Path** | `/absolute/path/to/qdrant-mcp-server/build/index.js` |
| **QDRANT_URL** | `http://localhost:6333` |
| **OPENAI_API_KEY** | Your API key |
| **OPENAI_BASE_URL** | `https://generativelanguage.googleapis.com/v1beta/openai` (Gemini) or `https://api.openai.com/v1` (OpenAI) |

> **Note:** EzCode automatically maps `OPENAI_BASE_URL` to `EMBEDDING_BASE_URL` when launching the stdio process, since that is the env var the server's embedding factory reads. You do not need to set `EMBEDDING_BASE_URL` manually.

> **Note:** EzCode also rewrites `host.docker.internal` to `localhost` in `QDRANT_URL` when running in stdio mode, so existing Docker-mode configs can be switched to stdio without manually editing the URL.

---

## Docker mode setup

Docker mode starts the MCP server as a container managed by `docker compose`.

### 1. Pull the image

```bash
docker pull mhalder/qdrant-mcp-server
# or
podman pull mhalder/qdrant-mcp-server
```

### 2. Start Qdrant

The container connects to your host Qdrant via `host.docker.internal:6333` (Docker Desktop on macOS/Windows) or the IP of the Docker bridge. Adjust `QDRANT_URL` in the Setup tab if your Qdrant is elsewhere.

### 3. Configure EzCode

Leave **Server Mode** as `docker` (the default). Set your `OPENAI_API_KEY` and adjust `QDRANT_URL` if needed. The compose file is generated automatically at `~/.ezcode/docker-compose.yaml` on first run and is never overwritten unless deleted.

---

## Configuration

EzCode stores its configuration at `~/.ezcode/config.json`. The file is created automatically with the defaults listed below on first run.

### Top-level fields

| Key | Default | Description |
|---|---|---|
| `image` | `mhalder/qdrant-mcp-server` | Docker image name. Docker mode only. |
| `server_mode` | `docker` | Launch mode: `docker` or `stdio`. |
| `server_path` | *(empty)* | Absolute path to `build/index.js`. Stdio mode only. |

### Environment variables

These are passed to the MCP server process (as container env in Docker mode, as process env in stdio mode).

| Key | Default | Description |
|---|---|---|
| `OPENAI_API_KEY` | *(empty — required)* | API key for embeddings and LLM. With the Gemini endpoint, use a Google AI Studio key. |
| `QDRANT_API_KEY` | *(empty)* | API key for Qdrant, if your instance requires authentication. |
| `EMBEDDING_MODEL` | `models/gemini-embedding-001` | Embedding model identifier. |
| `LLM_MODEL` | `gemini-2.5-flash` | LLM model for chat answer generation. |
| `TRANSPORT_MODE` | `http` | Stored for Docker mode reference. Overridden to `stdio` automatically in stdio mode. |
| `EMBEDDING_PROVIDER` | `openai` | Embedding backend. Use `openai` for any OpenAI-compatible endpoint (including Gemini). |
| `OPENAI_BASE_URL` | `https://generativelanguage.googleapis.com/v1beta/openai` | Base URL for the embedding and LLM API. EzCode maps this to `EMBEDDING_BASE_URL` for the server in stdio mode. |
| `LOG_LEVEL` | `info` | Server log verbosity (`debug`, `info`, `warn`, `error`). |
| `QDRANT_URL` | `http://localhost:6333` | URL of your Qdrant instance. In Docker mode, use `http://host.docker.internal:6333` to reach a host-side Qdrant. |
| `HTTP_PORT` | `3000` | HTTP port the MCP server listens on. Docker mode only. |
| `EMBEDDING_DIMENSIONS` | `3072` | Vector dimension for the embedding model. Must match your model's output size. `gemini-embedding-001` outputs 3072. |

### Example `~/.ezcode/config.json` — stdio mode

```json
{
  "image": "mhalder/qdrant-mcp-server",
  "server_mode": "stdio",
  "server_path": "/home/user/qdrant-mcp-server/build/index.js",
  "env": {
    "OPENAI_API_KEY": "your-api-key-here",
    "QDRANT_API_KEY": "",
    "EMBEDDING_MODEL": "models/gemini-embedding-001",
    "LLM_MODEL": "gemini-2.5-flash",
    "TRANSPORT_MODE": "http",
    "EMBEDDING_PROVIDER": "openai",
    "OPENAI_BASE_URL": "https://generativelanguage.googleapis.com/v1beta/openai",
    "LOG_LEVEL": "info",
    "QDRANT_URL": "http://localhost:6333",
    "HTTP_PORT": "3000",
    "EMBEDDING_DIMENSIONS": "3072"
  }
}
```

### Example `~/.ezcode/config.json` — Docker mode

```json
{
  "image": "mhalder/qdrant-mcp-server",
  "server_mode": "docker",
  "env": {
    "OPENAI_API_KEY": "your-api-key-here",
    "QDRANT_API_KEY": "",
    "EMBEDDING_MODEL": "models/gemini-embedding-001",
    "LLM_MODEL": "gemini-2.5-flash",
    "TRANSPORT_MODE": "http",
    "EMBEDDING_PROVIDER": "openai",
    "OPENAI_BASE_URL": "https://generativelanguage.googleapis.com/v1beta/openai",
    "LOG_LEVEL": "info",
    "QDRANT_URL": "http://host.docker.internal:6333",
    "HTTP_PORT": "3000",
    "EMBEDDING_DIMENSIONS": "3072"
  }
}
```

> **Note:** EzCode automatically sets `PROJECT_PATH` to the current working directory at launch. This value is not persisted in the config file — it is re-evaluated every time the binary runs.

---

## Usage

### Starting EzCode

Navigate to the project directory you want to index, then run the binary:

```bash
cd /path/to/your/project
./ezcode
```

EzCode runs in alternate-screen mode so your terminal history is not polluted. Press `Ctrl+C` at any time to quit. In Docker mode, if EzCode started the container during this session it will automatically run `docker compose down` before exiting.

### First-run wizard

On the very first launch (when `~/.ezcode/config.json` does not exist), EzCode opens directly on the **Setup** tab. Fill in your API key, choose a server mode, and press `Enter` to save. All fields are pre-populated with the defaults described in the [Configuration](#configuration) section.

On subsequent launches, the saved config is loaded, and the app moves straight to attempting an MCP connection.

### Tab overview

| Tab | Purpose |
|---|---|
| **Setup** | Edit all configuration fields including server mode and server path. Shown automatically on first run. Press `Esc` to return to Chat without saving (after first run). |
| **Index Codebase** | Shows the current index status (collection name, chunk count, last indexed time). Press `Enter` to run smart indexing: incremental if a snapshot exists, full indexing if not yet indexed. |
| **Index Git** | Shows the Git history index status. Press `Enter` to embed commit messages, diffs, and author metadata into a separate Qdrant collection. |
| **Chat** | Type a question and press `Enter`. EzCode retrieves relevant code and Git context via `contextual_search`, then calls your LLM to generate a grounded answer. Press `Ctrl+P` to open the MCP prompts picker. |

---

## TUI Navigation

| Key | Action |
|---|---|
| `Tab` / `→` | Move to the next tab |
| `Shift+Tab` / `←` | Move to the previous tab |
| `Enter` | Submit form (Setup) / trigger indexing (Index tabs) / send message (Chat) |
| `Up` / `Down` | Scroll viewport (Index and Chat tabs); move between form fields (Setup) |
| `Page Up` / `Page Down` | Scroll viewport by page |
| `Esc` | Cancel Setup form and return to Chat (only available after first run) |
| `Ctrl+P` | Open MCP prompts picker (Chat tab, only when prompts are available) |
| `Ctrl+C` | Quit EzCode (triggers graceful Docker shutdown if applicable) |

**Setup form field navigation:**

| Key | Action |
|---|---|
| `Tab` / `Down` | Focus the next input field |
| `Shift+Tab` / `Up` | Focus the previous input field |
| `Enter` | Validate and save the form |

**Prompts picker navigation:**

| Key | Action |
|---|---|
| Type characters | Filter the list by name or description |
| `Up` / `Down` | Move selection |
| `Enter` | Load the selected prompt name into the chat input |
| `Esc` | Close the picker without selecting |

### Using MCP Prompts

The `qdrant-mcp-server` ships with a set of guided workflow prompts (e.g. `investigate_code_with_history`, `trace_feature_evolution`, `security_audit_search`). To use them, copy `prompts.example.json` from the server repository to `prompts.json` in the server's working directory and restart. EzCode will then list them via `Ctrl+P`.

Prompts that require arguments can be invoked from the chat input with:

```
/prompt investigate_code_with_history repo=/path/to/project topic=authentication
```

Arguments are passed as `key=value` pairs separated by spaces. EzCode fetches the expanded prompt text from the server and displays the result directly in the Chat tab.

---

## Project Structure

```
.
├── cmd/
│   └── ezcode/
│       └── main.go             # Entry point: config loading, compose generation, Bubble Tea program setup
│
├── internal/
│   ├── config/
│   │   ├── config.go           # Config struct (ServerMode, ServerPath), NewDefaultConfig, LoadConfig, SaveConfig
│   │   └── config_test.go
│   │
│   ├── docker/
│   │   ├── manager.go          # Manager struct, Commander interface, IsInstalled, ComposeUp, ComposeDown
│   │   ├── manager_unix.go     # OSCommander.Run for Linux/macOS (build tag: !windows)
│   │   ├── manager_windows.go  # OSCommander.Run for Windows (suppresses console window)
│   │   ├── manager_test.go
│   │   ├── compose.go          # ComposeTemplate const, GenerateComposeFile
│   │   └── compose_test.go
│   │
│   └── mcp/
│       ├── manager.go          # MCPClient interface, Manager, GetDefaultClient, GetStdioClient,
│       │                       # adaptEnvForStdio, Initialize, Ping, IndexCodebase, IndexGitHistory,
│       │                       # ContextualSearch, ReindexChanges, GetIndexStatus, GetGitIndexStatus,
│       │                       # ListPrompts, GetPrompt, ParseIndexStatus
│       └── manager_test.go
│
└── ui/
    └── components/
        ├── app.go              # AppModel (top-level Bubble Tea model), WasDockerStarted
        ├── app_test.go
        ├── chat.go             # ChatModel: tab routing, RAG search, index commands, prompts picker,
        │                       # mcpProjectPath (Docker vs stdio path resolution), viewport rendering
        ├── config_form.go      # ConfigForm: 13-field TUI form (11 env vars + server_mode + server_path)
        ├── config_form_test.go
        └── dashboard.go        # DashboardModel: MCP ping, Docker/stdio startup, connection state machine
```

---

## Development

A `Makefile` is provided for all common workflows. Run `make` with no arguments to see every available target:

```bash
make
```

### Common targets

| Target | Description |
|---|---|
| `make build` | Format, vet, and build the binary to `bin/ezcode` |
| `make run` | Build and run the application |
| `make install` | Build and install the binary to `$GOPATH/bin` |
| `make test` | Run all tests |
| `make test-verbose` | Run all tests with verbose output |
| `make test-coverage` | Run tests and produce `coverage.html` |
| `make test-config` | Run only the `internal/config` package tests |
| `make test-docker` | Run only the `internal/docker` package tests |
| `make test-mcp` | Run only the `internal/mcp` package tests |
| `make test-components` | Run only the `ui/components` package tests |
| `make test-run test=TestName` | Run a single test by name across all packages |
| `make fmt` | Format all Go files |
| `make fmt-check` | Check formatting without modifying files |
| `make vet` | Run `go vet` |
| `make lint` | Run `fmt` + `vet` together |
| `make tidy` | Tidy `go.mod` and `go.sum` |
| `make deps` | Download and verify dependencies |
| `make clean` | Remove `bin/` and coverage files |
| `make build-linux` | Cross-compile for Linux (amd64) |
| `make build-darwin-arm64` | Cross-compile for macOS (Apple Silicon) |
| `make build-windows` | Cross-compile for Windows (amd64) |
| `make build-all` | Build for all supported platforms |
| `make dev` | Full development cycle: fmt → vet → test → build |
| `make check` | CI-style check: fmt-check → vet → test |
| `make version` | Print version, commit, and Go runtime info |

---

## How the RAG Pipeline Works

When you submit a question in the Chat tab, EzCode executes the following steps:

1. **Retrieval** — calls `contextual_search` on the MCP server with your query, a code result limit of 5, and a Git result limit of 2. The `correlate: true` flag asks the server to cross-reference code and commit results.
2. **Context assembly** — the server returns text content blocks containing matching code snippets, file paths, commit messages, and diffs. EzCode concatenates all text blocks into a single context string.
3. **Generation** — EzCode POSTs a `chat/completions` request to `OPENAI_BASE_URL` with a system prompt instructing the LLM to answer strictly from the provided context (no outside knowledge), and to include actual code snippets from that context in its answer. If the context does not contain a relevant answer, the model is instructed to say so rather than guess.
4. **Rendering** — the LLM response is rendered as terminal markdown via Glamour and displayed in the chat viewport.

---

## Environment Variable Notes

### `OPENAI_BASE_URL` vs `EMBEDDING_BASE_URL`

EzCode stores the API base URL as `OPENAI_BASE_URL` in config. The `qdrant-mcp-server` process reads `EMBEDDING_BASE_URL` for the embedding provider. EzCode handles this translation automatically:

- **Stdio mode** — `adaptEnvForStdio()` sets both `EMBEDDING_BASE_URL` and `OPENAI_BASE_URL` from your config value before spawning the process.
- **Docker mode** — the generated compose file sets both `EMBEDDING_BASE_URL=${OPENAI_BASE_URL}` and `OPENAI_BASE_URL=${OPENAI_BASE_URL}`.

You only need to set `OPENAI_BASE_URL` in EzCode's config.

### `QDRANT_URL` and `host.docker.internal`

`host.docker.internal` is a Docker-specific DNS alias that resolves to the host machine from inside a container. It does not resolve on the host itself.

- **Docker mode** — use `http://host.docker.internal:6333` to reach a Qdrant instance running on your host machine.
- **Stdio mode** — use `http://localhost:6333`. EzCode automatically rewrites `host.docker.internal` to `localhost` when launching the stdio process, so switching an existing Docker-mode config to stdio mode works without manually editing the URL.

---

## License

MIT — see [LICENSE](LICENSE) for details.
