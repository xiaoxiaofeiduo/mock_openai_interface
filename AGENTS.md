# Repository Guidelines

## Project Structure & Module Organization

This is a small Go 1.20 Gin mock server for deterministic local chat-response testing. The application entry point and all handlers live in `main.go` under `package main`; there is currently no router, middleware, or config package split.

- `main.go`: Gin setup, request/response structs, OpenAI-style and custom chat endpoints.
- `static/`: browser-based manual test clients and assets. `GET /` serves `static/main.html`; `/static/*` serves the remaining files.
- `go.mod` / `go.sum`: module metadata and dependencies.
- `my-gin-project`: built binary artifact; avoid editing it manually.

## Build, Test, and Development Commands

Use standard Go tooling from the repository root:

```bash
go run main.go          # Run the server on [::]:8080
go build -o my-gin-project .  # Build the local binary
go test ./...           # Run tests; currently there are no test files
go vet ./...            # Static checks
gofmt -w main.go        # Format Go source before committing
go mod tidy             # Clean dependency metadata
```

Manual verification is done through `http://localhost:8080/` and the HTML clients under `static/`.

## Coding Style & Naming Conventions

Follow idiomatic Go formatting with tabs via `gofmt`. Keep exported and JSON-facing struct names descriptive, and keep JSON tags in snake case where API compatibility requires it, such as `prompt_id` or `finish_reason`. Existing comments are partly Simplified Chinese; preserve that style when editing nearby code instead of translating unrelated comments.

When changing streaming behavior, preserve the SSE format: each chunk is written as `data: <json>\n\n` and flushed immediately. The trailing stop chunk and `[DONE]` or `[FINISH]` marker are client-visible behavior.

## Testing Guidelines

No automated tests are present yet. Add `*_test.go` files beside the code they exercise, using Go’s standard `testing` package and `net/http/httptest` for Gin handlers. Prefer focused tests for request validation, `token_length` chunking, non-streaming JSON responses, and SSE terminators.

## Commit & Pull Request Guidelines

This directory does not expose git history, so no project-specific commit convention can be inferred. Use concise imperative commit messages, for example `Add custom chat streaming test`. Pull requests should describe the behavior change, list manual or automated verification commands, and include screenshots only when `static/` UI changes are visible.

## Security & Configuration Tips

The server binds to `[::]:8080`, which may expose it beyond localhost depending on the environment. Do not add real API keys or production LLM calls; this project is a mock backend for local testing.
