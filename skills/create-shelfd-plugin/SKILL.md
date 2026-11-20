---
name: create-shelfd-plugin
description: Scaffold, implement, verify, and package out-of-process TypeScript plugins for Shelfd using @shelfd/plugin-sdk. Use when creating a new Shelfd importer, storefront catalog connector (such as Humble Bundle, Packt Pub, Kobo, or OPDS), outbound book/series action plugin (such as Yoto or NotebookLM), MCP tool plugin, or custom command plugin in this repo or an external plugin repository.
---

# Create a Shelfd plugin

Shelfd runs plugins as isolated child processes over NDJSON JSON-RPC 2.0 on `stdio`. Plugins written in TypeScript use `@shelfd/plugin-sdk` and compile into a single zero-dependency `dist/index.mjs` file packaged alongside `plugin.json` in a `.zip` archive.

## Workflow

1. **Choose the capability set** before writing code:
   - `"catalog"`: Exposes a browsable or searchable book catalog in Shelfd's generic Storefront Catalog UI (`catalog/browse`, `catalog/search`, `catalog/stage`). Use `"browse"` when listing a user's owned library or bundles, `"search"` when querying a large public or remote index, or both.
   - `"actions"`: Exposes outbound book or series actions (`actions/prepare`, `actions/options`, `actions/upload_asset`, `actions/invoke`) and optional interactive account linking (`auth/status`, `auth/begin`, `auth/complete`, `auth/disconnect`). Supports immediate actions (`mode: "immediate"`), dynamic declarative form dialogs (`mode: "form"` with `InputFieldSpec` controls), synchronous execution (`execution: "sync"`), and background job transfers (`execution: "job"`).
   - `"commands"`: Exposes one-shot actions in the plugin configuration dialog (`POST /api/v1/plugins/{name}/commands/{cmd}`), such as `verify_session`.
   - `"tools"`: Exposes MCP tools to AI agents (`{plugin_name}_{tool_name}`). Keep every tool response compact (`truncateForMCP(text, 4000)`, under 100 tokens per hit summary).
   - `"events"`: Emits live progress or domain notifications forwarded to `/api/v1/queue/events`.

2. **Copy the starter files** from [assets/template](assets/template/) into a directory whose folder name matches `plugin.json` `"name"` (`^[a-z][a-z0-9_-]{1,32}$`).

3. **Design `plugin.json`, `config_schema`, `auth`, and `actions`**:
   - Set `"daemon": false` for storefront importers, outbound action plugins, and request/response tools so Shelfd reaps idle Node processes after `idle_timeout_seconds` (default 120s) and respawns on demand. Active background jobs (`execution: "job"`) automatically hold a host lease until completion so non-daemon plugins are never reaped mid-transfer. Set `"daemon": true` only for plugins that run internal timers, webhooks, or persistent sockets.
   - Request only the minimum `"api_scopes"` needed (`"uploads:stage"` for importers; `"books:read"` for action plugins reading books, Markdown exports, series metadata, or audio streams).
   - List every external hostname in `"permissions.network_domains"` so operators see them in Settings before enabling the plugin.
   - Mark credentials in `config_schema.properties` with `"x-shelfd-secret": true` so Shelfd encrypts them at rest with AES-256-GCM and masks them with `"__SHELFD_REDACTED__"` in API responses.
   - Mark internal or `auth`-managed config fields (such as `access_token`, `refresh_token`, `token_expires_at`, `client_id`, `redirect_uri`, or internal base URLs) with `"x-shelfd-hidden": true` so they stay in `config_schema` for server-side encryption/redaction and DB persistence without cluttering the plugin settings dialog.
   - Add `"x-shelfd-curl-paste": true` (and optional `"x-shelfd-cookie-name": "<cookie>"`) on session cookie or Bearer token fields so users can paste a browser DevTools "Copy as cURL" command directly into Settings.
   - When a plugin supports OAuth 2.0 PKCE or interactive token linking, declare `"auth"` (`methods`: `["oauth_pkce", "token"]`, `title`, `description`, `callback_param`, `fields`) and implement `auth: { status, begin, complete, disconnect }`. Shelfd passes `<origin>/settings?plugin=<name>` as `redirect_uri`; request every API scope required by your endpoints in `auth/begin` (not just `openid profile offline_access`) and assert the `scope` parameter in tests.
   - When a plugin exposes book or series actions, declare `"actions"` in `plugin.json` (`name`, `title`, `description`, `icon`, `targets`: `["book", "series"]`, `media_types`: `["ebook", "audiobook"]`, `mode`: `"immediate" | "form"`, `execution`: `"sync" | "job"`).

4. **Implement handlers with `@shelfd/plugin-sdk`**:
   - Read [references/protocol-and-sdk.md](references/protocol-and-sdk.md) for the `definePlugin` contract, `PluginContext` methods, declarative `ActionFormSpec` / `InputFieldSpec` controls, and JSON-RPC wire rules.
   - For authenticated storefronts (cookie sessions, refresh tokens, OPDS), read [references/authenticated-storefront-recipes.md](references/authenticated-storefront-recipes.md).
   - Never write directly to `/library` or Shelfd's SQLite/PostgreSQL database. Always stage downloaded books through `await ctx.stageBook({ data, filename, externalId, cover, metadataDefaults })` and read library content through `ctx.fetchLoopback(...)`, `ctx.openLoopbackStream(...)`, or `ctx.getBookMarkdown(...)`.
   - For large audio or media transfers, use `await ctx.openLoopbackStream(path)` to stream a web `ReadableStream<Uint8Array>` directly into outbound `fetch` requests without buffering whole files in memory.
   - Long-running background jobs (`execution: "job"`) should report progress via `ctx.reportProgress({ jobId: req.job_id, current, total, percent, title, message })` (which resets the host's 5-minute activity watchdog and broadcasts SSE `job_progress` events). `ctx.fetchLoopback` and `ctx.getBookMarkdown` automatically renew expired 5-minute invocation JWTs via `ctx.refreshInvocationToken()` (`host/refresh_token`) on HTTP 401.
   - Never write debug text to `process.stdout`. `definePlugin()` redirects `console.log` to `stderr`, and any raw non-JSON bytes on `stdout` corrupt the JSON-RPC stream.
   - Wrap outbound HTTP calls with `createPoliteFetcher({ minIntervalMs: 350 })` to respect rate limits and `Retry-After` headers.
   - Throw `new AuthExpiredError("...")` whenever an external service returns HTTP 401 or a terminal 403 so Shelfd prompts the user to update credentials (`-32001`). When probing multiple fallback endpoints, only abort immediately on HTTP 401; let HTTP 403 try remaining candidate endpoints first in case one endpoint requires a narrower or different OAuth scope.
   - When an API rotates tokens during a request or `auth/complete` finishes, call `await ctx.patchConfig({ access_token, refresh_token })` so Shelfd encrypts and saves the new tokens for the active user.

5. **Build, verify, test, and pack**:
   - Run `npx shelfd-plugin build .` to bundle `src/index.ts` into `dist/index.mjs`.
   - Run `npx shelfd-plugin verify .` to spawn the built plugin in a mock JSON-RPC host harness and verify manifest validity, clean `stdout` framing, `initialize` handshake, `auth/status` and `actions/prepare` or `actions/invoke` contracts, and `shutdown` exit. Do not ship a plugin until `verify` passes all checks.
   - Write unit tests with `node --test` using a mocked `fetchImpl` passed to `createPoliteFetcher` so tests never hit live external servers.
   - Run `npx shelfd-plugin pack .` to produce `<name>-<version>.zip` for upload in Shelfd (`Settings -> Plugins & Storefronts -> Install .zip`).

## Hard invariants

- **Directory and manifest name match**: `plugin.json` `"name"` must equal the plugin directory name when installed under `/data/plugins/<name>/`.
- **Zero secrets in environment variables**: Shelfd strips `SHELFD_JWT_SECRET`, database DSNs, and AI keys from child environments. Read user credentials exclusively from `ctx.config` and pass `ctx.invocationToken` via `ctx.fetchLoopback()` / `ctx.stageBook()`.
- **Provenance External IDs**: Always pass a stable, unique `external_id` in `CatalogItem` and `externalId` in `ctx.stageBook(...)`. Shelfd records `(plugin_name, external_id)` in `book_sources` so already-staged and already-imported books automatically show `Staged` or `In Library` badges in the UI.
- **Metadata fallbacks**: Commercial PDFs and M4B files often lack clean embedded OPF metadata. Always pass `metadataDefaults` (`title`, `authors`, `publisher`, `description`, `genres`) to `ctx.stageBook(...)` so Shelfd fills missing fields while keeping embedded EPUB metadata authoritative when present.

