# Structured Server Logging with `log/slog`

* Status: accepted
* Deciders: Lead Systems Architect, @core, @security
* Date: 2026-09-09

## Context and Problem Statement

The `shelfd` server previously used standard library `log.Printf` and `log.Fatalf` in `cmd/server/main.go`, while background workers (`internal/worker`), the MCP server (`internal/mcp`), and library scanner used `log/slog`. Furthermore, incoming HTTP requests to `/api/v1/`, `/mcp/`, and `/oauth/` had no structured logging middleware, meaning request paths, HTTP response status codes, latencies, and client IP addresses were invisible in standard server and container logs.

What logging library and architecture should Shelfd adopt to provide clear operational observability while adhering to the low-memory (<50MB RAM) and zero-dependency footprint requirements of a homelab daemon?

## Decision Drivers

* **Minimal Memory & Dependency Footprint**: Zero binary bloat or transitive supply chain dependencies, keeping container images lightweight and memory consumption < 50MB.
* **Architectural Consistency**: Unify daemon startup, background workers, MCP servers, and HTTP middleware under a single standard logging interface.
* **Operational Observability**: Log incoming HTTP requests with method, path, response status, duration (ms), bytes written, remote IP, and authenticated user ID.
* **Configurable Levels & Formats**: Support standard log levels (`debug`, `info`, `warn`, `error`) and formats (`text` for human console reading, `json` for Docker/Loki collectors) via `config.yaml` and environment variables (`SHELFD_LOG_LEVEL`, `SHELFD_LOG_FORMAT`).
* **Noise Reduction**: Avoid log spam from continuous healthcheck probes (`/health`).

## Considered Options

* **Go Standard Library `log/slog` (Chosen)**: Built into Go 1.21+, zero external dependencies, low allocation overhead, native JSON and Text handlers, dynamic log levels via `slog.LevelVar` / `slog.HandlerOptions`.
* **`charmbracelet/log`**: High visual polish and colorized console badges, but introduces ~10 transitive dependencies into `go.mod`. Can still be plugged in as a drop-in `slog.Handler` later if desired.
* **`rs/zerolog`**: High performance zero-allocation JSON logger, but introduces a non-standard API incompatible with existing `*slog.Logger` instances in the codebase.
* **`uber-go/zap`**: Enterprise-grade structured logger with complex API surface (`SugaredLogger` vs `Logger`), disproportionate for a lightweight single-daemon application.

## Decision Outcome

Chosen option: **Go Standard Library `log/slog`**.

### 1. Configuration (`internal/config`)
* Added `LoggingConfig` with `Level` (`debug`, `info`, `warn`, `error`) and `Format` (`text`, `json`).
* Added environment variable overrides: `SHELFD_LOG_LEVEL` and `SHELFD_LOG_FORMAT`.
* Defaults: `Level: "info"`, `Format: "text"`.

### 2. HTTP Request Logging Middleware (`internal/api/middleware.go`)
* Implemented `RequestLoggerMiddleware(logger *slog.Logger)`:
  * Wraps `http.ResponseWriter` with `statusResponseWriter` to capture HTTP status code and bytes written.
  * Implements `http.Flusher` and `Unwrap()` to preserve compatibility with SSE streaming (`/mcp/sse`, `/api/v1/queue/stream`) and `http.ResponseController`.
  * Logs at `slog.LevelDebug` for `/health` (preventing healthcheck poll spam) and `slog.LevelInfo` for normal traffic (elevating to `warn` for 4xx and `error` for 5xx).
  * Captures client IP (`X-Forwarded-For`, `X-Real-IP`, or `RemoteAddr`) and authenticated user context.

### 3. Server Standardization (`cmd/server/main.go`)
* Configured `slog.SetDefault(logger)` based on active configuration format and level.
* Replaced all legacy `log.Printf` and `log.Fatalf` invocations with structured `slog.Info`, `slog.Warn`, and `slog.Error`.
* Wrapped the root HTTP server handler with `RequestLoggerMiddleware`.
* Wired `logger` into `RouterConfig`, `NewLibraryHandler`, `NewBookHandler`, `NewAuthHandler`, and workers.

### 4. Security & Audit Logging
* Added security audit warnings in `AuthHandler.Login` for failed authentication attempts with username and remote IP.
* Added structured lifecycle logging for book uploads, chapter ingestion, and MCP tool execution.

## Pros and Cons of the Options

### Go Standard Library `log/slog`
* Good, because zero external dependencies in `go.mod`.
* Good, because standard across Go 1.21+ and natively compatible with existing workers.
* Good, because both human-friendly text and machine-readable JSON formats are supported out-of-the-box.
* Good, because third-party handlers (e.g. `charmbracelet/log`) can be swapped in without modifying any application code.
* Bad, because default text handler does not include ANSI color styling out-of-the-box.
