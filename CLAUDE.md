# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project purpose

A Go (Gin) mock server that simulates OpenAI-style streaming chat-completion responses. It exists to provide a deterministic, configurable local backend for frontend testing — not to call any real LLM. The static HTML pages in `static/` are the manual test clients.

## Commands

```bash
# Run the server (binds to [::]:8080 — all interfaces, IPv4 + IPv6)
go run main.go

# Build a binary (output binary name follows module path)
go build -o my-gin-project .

# Tidy / download deps
go mod tidy
go mod download
```

No tests exist (`go test ./...` will be a no-op). No linter is configured — rely on `go vet ./...` and `gofmt -d .` if needed. A VS Code launch config (`.vscode/launch.json`) is present for the Go debugger; the produced binary `my-gin-project` at the repo root is a build artifact.

## Architecture

Everything lives in `main.go` (~390 lines, `package main`). There is no router split, no middleware layer, no config — every handler is an inline closure on the single `gin.Engine`. Keep that in mind before adding abstractions.

**Two HTTP endpoints, both in `main.go`:**

- `POST /v1/chat/completions` — OpenAI-compatible. Accepts `RequestBody{Model, Messages, Stream}`. Supports both streaming (SSE, `text/event-stream`) and non-streaming JSON. Response chunks are shaped like `ResponseChunk` with an extra trailing chunk where `finish_reason = "stop"` and a literal `data: [DONE]` line.
- `POST /v1/custom/chat` — Custom protocol. Accepts `CustomRequestBody{Model, PromptId, Prompt[], IsStream}`. Streaming variant emits `CustomResponseChunk` per token-bundle and a literal `data: [FINISH]` terminator. Non-streaming returns a single JSON object with a `uuid` from `hashicorp/go-uuid`.

**Chunking model (the central behavior to preserve):** both endpoints accept a `token_length` query parameter (default `3`). The server concatenates all `role: "system"` message contents into one string, then slices that string into `rune`-based chunks of `token_length` characters each. Those chunks are emitted in order as the streaming response body. `user` messages are parsed but currently unused in the response — they exist for API-shape compatibility with real OpenAI clients.

**Static UI:** `r.Static("/static", "./static")` serves the test pages. `GET /` returns `static/main.html`, which is a navigation page linking to the two test UIs (`simulate_chat_ai.html`, `simulate_customize_chat.html`). When modifying the UI, edit the files under `static/` directly — there is no template engine or asset pipeline.

**Streaming mechanics:** each chunk is written as `data: <json>\n\n` and flushed via `c.Writer.(http.Flusher).Flush()`. The custom endpoint adds a 100ms `time.Sleep` per chunk; the OpenAI-style endpoint does not. Both rely on a global `responseChunks` / `customResponseChunks` slice being rebuilt per request from the system content.

## Codebase conventions to respect

- Comments and some identifiers are in Chinese (Simplified). Match the existing style when editing — do not translate comments or rename identifiers to English.
- `responseChunks` and `customResponseChunks` are package-level `var` slices (mutable shared state). They are reassigned on every request, so they are effectively per-request despite the declaration. Don't introduce locks unless you change that.
- The "trailing empty chunk with stop/finish signal" is load-bearing for clients that detect end-of-stream by `finish_reason` or `is_stop` rather than `[DONE]`/`[FINISH]`. Keep emitting it.
