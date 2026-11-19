---
name: create-shelfd-plugin
description: Scaffold, implement, verify, and package out-of-process TypeScript plugins for Shelfd using @shelfd/plugin-sdk. Use when creating a new Shelfd importer, storefront catalog connector (such as Humble Bundle, Packt Pub, Kobo, or OPDS), MCP tool plugin, or custom command plugin in this repo or an external plugin repository.
---

# Create a Shelfd plugin

Shelfd runs plugins as isolated child processes over NDJSON JSON-RPC 2.0 on `stdio`. Plugins written in TypeScript use `@shelfd/plugin-sdk` and compile into a single zero-dependency `dist/index.mjs` file packaged alongside `plugin.json` in a `.zip` archive.

## Workflow

1. **Choose the capability set** before writing code:
   - `"catalog"`: Exposes a browsable or searchable book catalog in Shelfd's generic Storefront Catalog UI (`catalog/browse`, `catalog/search`, `catalog/stage`). Use `"browse"` when listing a user's owned library or bundles, `"search"` when querying a large public or remote index, or both.
   - `"commands"`: Exposes one-shot actions in the plugin configuration dialog (`POST /api/v1/plugins/{name}/commands/{cmd}`), such as `verify_session`.
   - `"tools"`: Exposes MCP tools to AI agents (`{plugin_name}_{tool_name}`). Keep every tool response compact (`truncateForMCP(text, 4000)`, under 100 tokens per hit summary).
   - `"events"`: Emits live progress or domain notifications forwarded to `/api/v1/queue/events`.

2. **Copy the starter files** from [assets/template](assets/template/) into a directory whose folder name matches `plugin.json` `"name"` (`^[a-z][a-z0-9_-]{1,32}$`).

3. **Design `plugin.json` and `config_schema`**:
   - Set `"daemon": false` for storefront importers and request/response tools so Shelfd reaps idle Node processes after `idle_timeout_seconds` (default 120s) and respawns on demand. Set `"daemon": true` only for plugins that run internal timers, webhooks, or persistent sockets.
   - Request only the minimum `"api_scopes"` needed (`"uploads:stage"` for importers; `"books:read"` if reading library state).
   - List every external hostname in `"permissions.network_domains"` so operators see them in Settings before enabling the plugin.
   - Mark credentials in `config_schema.properties` with `"x-shelfd-secret": true` so Shelfd encrypts them at rest with AES-256-GCM and masks them with `"__SHELFD_REDACTED__"` in API responses.
   - Add `"x-shelfd-curl-paste": true` (and optional `"x-shelfd-cookie-name": "<cookie>"`) on session cookie or Bearer token fields so users can paste a browser DevTools "Copy as cURL" command directly into Settings.

4. **Implement handlers with `@shelfd/plugin-sdk`**:
   - Read [references/protocol-and-sdk.md](references/protocol-and-sdk.md) for the `definePlugin` contract, `PluginContext` methods, and JSON-RPC wire rules.
   - For authenticated storefronts (cookie sessions, refresh tokens, OPDS), read [references/authenticated-storefront-recipes.md](references/authenticated-storefront-recipes.md).
   - Never write directly to `/library` or Shelfd's SQLite/PostgreSQL database. Always stage downloaded books through `await ctx.stageBook({ data, filename, externalId, cover, metadataDefaults })`.
   - Never write debug text to `process.stdout`. `definePlugin()` redirects `console.log` to `stderr`, and any raw non-JSON bytes on `stdout` corrupt the JSON-RPC stream.
   - Wrap outbound HTTP calls with `createPoliteFetcher({ minIntervalMs: 350 })` to respect storefront rate limits and `Retry-After` headers.
   - Throw `new AuthExpiredError("...")` whenever a storefront returns HTTP 401/403 or redirects to a login page so Shelfd prompts the user to update credentials (`-32001`).
   - When an API rotates tokens during a request, call `await ctx.patchConfig({ access_token, refresh_token })` so Shelfd encrypts and saves the new tokens for the active user.

5. **Build, verify, test, and pack**:
   - Run `npx shelfd-plugin build .` to bundle `src/index.ts` into `dist/index.mjs`.
   - Run `npx shelfd-plugin verify .` to spawn the built plugin in a mock JSON-RPC host harness and verify manifest validity, clean `stdout` framing, `initialize` handshake, and `shutdown` exit. Do not ship a plugin until `verify` passes all checks.
   - Write unit tests with `node --test` using a mocked `fetchImpl` passed to `createPoliteFetcher` so tests never hit live external servers.
   - Run `npx shelfd-plugin pack .` to produce `<name>-<version>.zip` for upload in Shelfd (`Settings -> Plugins & Storefronts -> Install .zip`).

## Hard invariants

- **Directory and manifest name match**: `plugin.json` `"name"` must equal the plugin directory name when installed under `/data/plugins/<name>/`.
- **Zero secrets in environment variables**: Shelfd strips `SHELFD_JWT_SECRET`, database DSNs, and AI keys from child environments. Read user credentials exclusively from `ctx.config` and pass `ctx.invocationToken` via `ctx.stageBook()`.
- **Provenance External IDs**: Always pass a stable, unique `external_id` in `CatalogItem` and `externalId` in `ctx.stageBook(...)`. Shelfd records `(plugin_name, external_id)` in `book_sources` so already-staged and already-imported books automatically show `Staged` or `In Library` badges in the UI.
- **Metadata fallbacks**: Commercial PDFs and M4B files often lack clean embedded OPF metadata. Always pass `metadataDefaults` (`title`, `authors`, `publisher`, `description`, `genres`) to `ctx.stageBook(...)` so Shelfd fills missing fields while keeping embedded EPUB metadata authoritative when present.
