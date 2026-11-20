# Shelfd Plugin System: Design Plan

Status: **Proposal** (Step 2 of the feature workflow — architectural review pending)
Date: 2026-11-18

## 1. Problem

Shelfd is a single-container Go daemon. Its integrations today (Yoto, NotebookLM,
metadata auto-populate) are **hardcoded modules**: each one gets a config section in
`config.yaml`, handlers wired by hand into `internal/api/router.go`, and (for Yoto)
its own per-user account table in the DB. Adding another integration requires a code
release, a migration, and a rebuild of the whole image.

Meanwhile `audiobook-downloader` (`libby-cli`) already integrates with Shelfd — but
from *outside*: it's a NestJS CLI that logs in over REST, checks library ownership,
and stage-uploads EPUBs. That works, yet the capabilities (OverDrive/Hoopla loans,
downloads, BIF/CFC unscramble) are invisible to:

* the **Flutter client** (no UI for loans/downloads),
* the **MCP server** (agents can search books but cannot acquire them),
* the **event stream** (no live progress to subscribers).

Goal: let third-party programs such as `libby-cli` run **inside** Shelfd so their
capabilities surface through Shelfd's existing surfaces — REST API, MCP tools, SSE
events — with one config, one container identity, and no per-integration Go rewrites.

## 2. Options Considered

| Option | Verdict | Why |
| :--- | :--- | :--- |
| Go native `plugin` package | **Rejected** | Broken on macOS/Windows; requires identical build flags + cgo; incompatible with this project's dev loop and Docker cross-compilation (ADR-0010 cross-platform build). |
| Rewrite `libby-cli` in Go | **Rejected** | Puppeteer stealth automation, `__bif_cfc3` cipher, and XHR chunk capture are a large moving target. Reimplementing in Go gains nothing but maintenance burden. |
| Embedded JS runtime (goja/WASM) | **Rejected** | Cannot run Node/Puppeteer; wasm builds of that stack are impractical. |
| Separate sidecar container | **Deferred escape hatch** | Works, but breaks the "single container by default" homelab identity (ADR-0010) and makes event/MCP integration second-class. Keep as an L1 sandbox level (§10). |
| **Managed subprocess + JSON-RPC host protocol** | **Selected** | Same lifecycle model as LSP servers and Shelfd's own MCP clients; zero new network surface; keeps the TS ecosystem where browser automation lives; `libby-cli` needs only a thin daemon adapter to become a plugin. |

Key property: the protocol is deliberately shaped like **MCP JSON-RPC 2.0**. Shelfd
already speaks this protocol as a *server* (ADR-0006); the plugin host speaks it as a
*client*. One mental model, one wire format on both sides of the process boundary.

## 3. Architecture

```mermaid
graph TD
    subgraph Container[shelfd container]
        Core["Go core: API, DB, workers, MCP server"]
        PH["Plugin Host<br/>(internal/plugins)"]
        P1["libby plugin process<br/>node dist/plugin-server.js"]
        P2["other plugin processes..."]
        Core -->|register + dispatch| PH
        PH <-->|"JSON-RPC 2.0 over stdio (NDJSON)"| P1
        PH <-->|"JSON-RPC 2.0 over stdio"| P2
        P1 -->|Puppeteer| Chromium["Chromium (full image variant)"]
    end
    Flutter["Flutter client"] -->|REST /api/v1/plugins/* + SSE| Core
    Agents["External AI agents"] -->|MCP /mcp/sse (aggregated tools)| Core
```

The plugin host is a new Go package that owns plugin discovery, lifecycle, and the
stdio bridge. Everything else — REST, MCP, events — is **aggregation into existing
surfaces**, not new parallel surfaces:

* Plugin **tools** → aggregated into the single `/mcp/sse` endpoint (§7).
* Plugin **commands** → mounted under `/api/v1/plugins/{name}/...` (§6).
* Plugin **events** → forwarded to the existing `events.Hub` SSE stream (§8).
* Plugin **config/secrets** → DB-backed per user, encrypted at rest (§9), mirroring
  the `YotoAccount` pattern.

## 4. Plugin Manifest (`plugin.json`)

A plugin is a directory under `plugins.dir` (default `/data/plugins`, host-mountable)
containing a manifest:

```json
{
  "name": "libby",
  "version": "1.4.0",
  "display_name": "OverDrive & Hoopla Downloader",
  "description": "Download loans from OverDrive and Hoopla libraries.",
  "entrypoint": {
    "command": "node",
    "args": ["dist/plugin-server.js"],
    "workdir": "."
  },
  "runtime": { "family": "node", "min_version": "v20" },
  "capabilities": {
    "tools": true,
    "commands": true,
    "events": true
  },
  "permissions": {
    "network_egress": ["*.overdrive.com", "*.hoopla.com", "*.contentreserve.com"],
    "write_library": true,
    "read_library": true
  },
  "config_schema": {
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
      "libraries": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "provider": { "enum": ["overdrive", "hoopla"] },
            "branch_id": { "type": "string" },
            "username": { "type": "string" }
          }
        }
      }
    }
  }
}
```

Validation (host-side, at discovery): `name` matches `^[a-z][a-z0-9_-]{1,32}$`;
`version` is semver; entrypoint command resolves on `PATH`; `workdir` stays inside the
plugin directory. Invalid manifests are logged and skipped — a bad plugin MUST NOT
break core startup.

## 5. Host ↔ Plugin Protocol (JSON-RPC 2.0 over stdio)

Transport: newline-delimited JSON over the child's stdin/stdout. stderr is captured by
the host and mirrored to slog under `component=plugin.<name>` (never contains secrets).

### Handshake & lifecycle

| Direction | Method | Notes |
| :--- | :--- | :--- |
| host → plugin | `initialize` `{protocol_version, shelfd_version, data_dir}` | 30s response timeout; plugin replies with `{name, version, capabilities, tools?, commands?}` |
| host → plugin | `initialized` | notification after successful handshake |
| host → plugin | `shutdown` | host sends on stop/disable; plugin must exit within 5s or is SIGKILL'd |
| plugin → host | notifications: `notifications/log`, `notifications/progress` | mirrored to slog / SSE (§8) |

Host responsibilities: spawn via `exec.CommandContext` (explicit argv, **no shell**);
restart-on-crash with exponential backoff (1s → 60s cap, max 5 consecutive fails then
parked as `unhealthy`); surface state (`starting`, `running`, `unhealthy`,
`disabled`) in the REST API.

### Invocation

| Direction | Method | Notes |
| :--- | :--- | :--- |
| host → plugin | `tools/call` `{name, arguments, user_id}` | MCP tool invocation (agents) |
| host → plugin | `commands/invoke` `{command, args, user_id}` | user-triggered action from REST/Flutter |
| host → plugin | `config/update` notification `{user_id, config}` | decrypted per-user config push on login/config change |
| plugin → host | responses + `notifications/event` `{name, data, user_id?}` | forwarded to hub (§8) |

`user_id` is carried on every invocation because Shelfd is multi-user and plugins like
libby hold *per-user* library credentials. The plugin decides what a command means per
user; it never receives another user's config.

### Environment (process-level, non-secret)

The host sets: `SHELFD_API_BASE=http://127.0.0.1:<port>`,
`SHELFD_PLUGIN_TOKEN=<scoped API token>` (see §9), `SHELFD_DATA_DIR=/data/plugins/<name>`
(writable scratch, survives restarts, part of the backup story since it lives under
`/data`). Secret material flows **only** via the `config/update` RPC — env vars leak
through `/proc` and crash dumps.

## 6. REST Surface (new routes, behind existing JWT middleware)

```
GET    /api/v1/plugins                          list plugins: name, version, capabilities, state
GET    /api/v1/plugins/{name}                   manifest summary + runtime state + logs (tail)
PUT    /api/v1/plugins/{name}/config            save per-user config (encrypted at rest); triggers config/update push
GET    /api/v1/plugins/{name}/config            read own config (secrets masked)
POST   /api/v1/plugins/{name}/commands/{command}   invoke a declared command with JSON body args
POST   /api/v1/plugins/{name}/enable | disable     operator toggle (per user)
```

The Flutter client renders these generically: JSON Schema → form fields for config,
declared commands → buttons. No per-plugin Dart code in v1.

## 7. MCP Tool Aggregation

At startup the host collects each running plugin's tool definitions (from the
`initialize` reply, refreshed on `tools/list`) and merges them into the existing MCP
server's tool list under namespaced names: `{plugin}_{tool}` (e.g. `libby_list_loans`).
`tools/call` for a namespaced tool is dispatched to that plugin with the invoking
user's id resolved from the API token (`api_tokens.user_id`, already present — ADR-0009).

Effect: one MCP endpoint serves *both* library retrieval and acquisition, so an agent
can go "find sci-fi I don't have yet" → "check my OverDrive loans" → "download and
upload it" without leaving Shelfd. This is the highest-value integration point of the
whole design and costs ~100 lines because Shelfd already parses/emits MCP JSON-RPC.

## 8. Event Integration

Plugins emit `notifications/event`; the host re-publishes them to the existing
`events.Hub` as type `plugin_event`:

```json
{ "type": "plugin_event", "data": { "plugin": "libby", "event": "download_progress", "data": { "title": "...", "progress": 0.42 } } }
```

Flutter already subscribes to `/api/v1/queue/events`; it renders unknown event types
via a simple generic progress widget keyed on `plugin_event`. Event names are validated
against a charset whitelist; payloads capped at 64KB (drops are logged).

## 9. Config, Secrets & Token Security (@security)

* **Per-user config table** (migration following the `YotoAccount` precedent):

```sql
CREATE TABLE plugin_configs (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plugin_name TEXT NOT NULL,
    key         TEXT NOT NULL,           -- or: single JSON blob per (user, plugin); prefer blob
    value_enc   BYTEA NOT NULL,          -- AES-256-GCM, nonce-prefixed
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, plugin_name)
);
```

Key derivation: `HKDF(SHA-256, ikm=server jwt_secret, salt="shelfd/plugin-enc", info=plugin_name)` —
reuses the already-rotated-at-first-boot JWT secret (`ensureJWTSecret`) and keeps
"copy `/data/sqlite.db` = complete backup" (ADR-0010) intact because the wrapping key
lives in `config.yaml`, which operators already keep alongside.

* **Plugin API token**: at host startup, Shelfd creates one `api_tokens` row per enabled
  plugin (name `plugin:<name>`, SHA-256 hash stored exactly like existing tokens —
  ADR-0009). The plaintext is passed once via env at spawn. Plugins use it against
  `SHELFD_API_BASE` for everything the core API offers — including the same staged
  upload flow (`/books/upload/stage` → jobs → commit) that libby-cli already uses from
  the CLI today.
* **Path containment**: plugins receive only their own `/data/plugins/<name>` dir plus
  the library root they declared permission for. Zip Slip rules from `internal/scanner/sanitize.go`
  apply to anything a plugin uploads via the API (already enforced server-side).
* **Honest limits (v1)**: egress network domains in `permissions` are *advisory* —
  shown in the UI, logged on connect, not enforced. No seccomp/userns sandboxing in v1;
  the trust model is "operator installs this binary into their container" (same trust
  as installing any Docker image). Stated explicitly in the manifest docs.

## 10. Sandboxing Levels (future-proofing the transport choice)

The stdio JSON-RPC contract means the host is the only thing coupling plugin to core:

| Level | Isolation | Transport | Status |
| :--- | :--- | :--- | :--- |
| L0 | Same container, child process | stdio NDJSON | **v1** |
| L1 | Separate container (sidecar service) | same protocol over loopback HTTP + bearer token | planned escape hatch for heavy/less-trusted plugins |
| L2 | WASM inside Go runtime | native calls | only for pure-logic plugins; out of scope for libby-class |

Nothing in v1 locks us out of L1: the host's dispatcher already treats a plugin as
`{Request} → {Response}` with a context deadline; swapping the pipe for an HTTP
client is a local change.

## 11. Reference Plugin: `libby-cli` Walkthrough

What changes in `audiobook-downloader`:

1. **New entrypoint** `src/plugin-server.ts`: a thin NestJS (or plain TS) service that
   - reads/writes NDJSON JSON-RPC on stdio,
   - implements `initialize`, `shutdown`, `tools/list`, `tools/call`,
     `commands/invoke`, and handles `config/update`,
   - reuses existing services directly (`OverdriveService`, `HooplaService`,
     `ShelfdService`) — the NestJS DI graph already keeps them decoupled from
     nest-commander prompts;
   - emits `notifications/event` for loan/download progress.
2. **Headless mode**: in-process command execution replaces interactive Inquirer
   prompts (the CLI stays for standalone use). Per-user config comes from the
   `config/update` push instead of `~/.libby-downloader/config.yaml`.
3. **Manifest** `plugin.json` as in §4; package layout gains a `plugins/libby/`
   folder or the repo root becomes plugin-shippable with `bun run build` output in
   `dist/`.
4. **Zero change** to how it talks to Shelfd for uploads: it keeps calling
   `/api/v1/books/upload/*` with the issued token — exactly its current behavior.

Suggested tool surface (MCP-visible): `list_loans`, `search_catalog` (query provider
catalogs), `download_book` (args: loan id or isbn; returns upload job id + progress
events), and optionally `list_libraries` / `link_library` status.

## 12. Packaging & Image Strategy (@homelab)

* **Default image unchanged.** The <50MB Go-only image keeps ADR-0010's promise; with
  no plugins installed the host does nothing (no `plugins.dir` → no processes).
* **Second build target** `Dockerfile.full`: same core + Node 20 runtime + headless
  Chromium (`chromium --no-sandbox` under PUID, or host-mounted Chrome via
  `PUPPETEER_EXECUTABLE_PATH`). Plugins are added by mounting their directories into
  `/data/plugins` (or `/plugins`) — image rebuild per plugin is not required.
* Compose adds a commented-out alternative service block selecting the full variant.
* Backup guarantee preserved: everything plugin-owned lives under `/data`
  (`/data/plugins/<name>` scratch + `plugin_configs` rows in `sqlite.db`).

Image-size consequence: `full` variant lands in the 300–400MB class (Chromium
dominates). Acceptable because it's opt-in; homelabs running libby already have
Node/Chromium on the host today — the container just consolidates ops.

## 13. Go Package Layout & Wiring

```
internal/plugins/
├── manifest.go      // PluginManifest type, validation, discovery scan
├── transport.go     // NDJSON JSON-RPC client: correlation, timeouts, notifications
├── process.go       // spawn, env assembly, backoff restarts, graceful shutdown
├── host.go          // lifecycle orchestrator: start/stop/restart all plugins, state machine
├── bridge.go        // dispatch REST command invocations / MCP tool calls → plugin RPC
├── config_store.go  // per-user config: HKDF-derived AES-GCM encrypt/decrypt over StorageEngine
└── *_test.go        // fake-plugin subprocess (small Go helper binary or shell script) for e2e-ish unit tests
```

`repository.StorageEngine` gains four small methods (`Get/Set/DeletePluginConfig`,
`ListPluginConfigs`) or a parallel `plugins.PluginConfigStore` — decide in review; the
blob-per-(user,plugin) shape prefers a dedicated tiny store to avoid growing the
already ~100-method engine interface further.

Wiring in `cmd/server/main.go` (after workers, before router): construct host, start,
pass to `api.NewRouter` (new `PluginHandler`) and to the MCP server for tool
aggregation. `defer host.Stop()` mirrors existing worker shutdowns. `router.go` gains
one `mux.Handle` block for `/api/v1/plugins/...`.

## 14. Phased Rollout

| Phase | Scope | Exit criteria |
| :--- | :--- | :--- |
| **P0 — Host core** | manifest + discovery + process lifecycle + transport + state REST endpoints + unit tests with a stub plugin binary | Stub plugin starts, crashes, restarts with backoff, reports state via API; `go test ./internal/plugins/...` green |
| **P1 — Surfaces** | command dispatch, per-user config store + encryption, event bridge to hub | Flutter generic plugin screen can configure a fake plugin and watch its events stream |
| **P2 — MCP aggregation** | tool merge + namespaced dispatch under token user context | MCP inspector shows `stub_...` tools; agent invokes one successfully |
| **P3 — libby reference plugin** | `plugin-server.ts` adapter in audiobook-downloader + `Dockerfile.full` + chromium setup | End-to-end: agent or Flutter triggers loan download → EPUB staged/uploaded → appears in library → progress streamed the whole way |

Each phase ships independently; P0–P2 touch only shelfd, P3 is cross-repo.

## 15. Risks & Open Questions

1. **Chromium in containers on ARM NAS hardware** — memory pressure vs the <50MB-idle
   identity. Mitigation: full image opt-in; plugin can also drive a host-mounted Chrome.
   Need real measurement on target hardware before marketing it.
2. **Plugin long-lived state (cookies/sessions)** — lives in `/data/plugins/<name>`;
   expiry handling belongs to the plugin. Shelfd only guarantees persistence + backup.
3. **Config schema coverage for UI** — JSON Schema → form mapping supports v1 of
   string/number/boolean/array-of-object fields; exotic schemas degrade to "edit raw
   JSON" fallback.
4. **Multi-user concurrency** — two users invoking `libby_download_book` concurrently
   against one OverDrive account: queueing policy is the *plugin's* decision (v1:
   document it; libby serializes per library account). Host enforces nothing beyond
   per-plugin single process.
5. **Scheduling/cron triggers** — deliberately deferred; the MCP + agent model already
   gives "my agent runs downloads nightly", which covers the real use case without a
   new subsystem.
6. **Update flow** — plugins are operator-managed files today (copy dir, restart via
   API). In-image bundled updates (like Calibre-web) later if demand emerges.
