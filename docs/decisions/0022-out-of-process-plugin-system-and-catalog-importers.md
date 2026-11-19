# Out-of-Process Plugin System and Standardized Storefront Catalog Importers

* Status: accepted
* Deciders: Lead Systems Architect, @core, @security, @librarian, @reader
* Date: 2026-11-18

## Context and problem statement

Shelfd users purchase DRM-free ebooks and audiobooks from external storefronts (such as Humble Bundle and Packt Pub) and download public-domain books from catalogs such as Project Gutenberg. Compiling scrapers and auth flows for every storefront directly into the Go daemon bloats the core binary, couples Shelfd releases to third-party website changes, and forces every user to ship connectors they do not use.

How should Shelfd support runtime-installable plugins for authenticated book storefronts, MCP agent tools, and custom commands while keeping idle RAM below 50 MB, protecting user credentials, and preserving `/library` Audiobookshelf compatibility?

## Decision drivers

* Allow operators to install plugins at runtime (`.zip` upload or `/data/plugins/<name>/`) without recompiling Shelfd.
* Render any storefront importer in Flutter without per-store Dart UI code.
* Preserve `< 50 MB` idle daemon RAM when non-daemon plugins are idle.
* Prevent plugins from reading `SHELFD_JWT_SECRET`, database credentials, or mutating `/library` directly.
* Support cookie-based and refresh-token storefront authentication where CAPTCHAs block headless login.
* Keep commercial storefront scrapers in external repositories while shipping `@shelfd/plugin-sdk` and a public-domain `project-gutenberg` reference plugin in-repo.

## Considered options

* **Out-of-process child plugins over NDJSON JSON-RPC 2.0 on `stdio` with scale-to-zero reaping and a standardized `"catalog"` capability (Chosen)**
* **Compiled-in Go provider interfaces**
* **Embedded JavaScript/WASM runtime inside the Go binary (Goja / Wazero)**
* **Separate microservice containers over HTTP**

## Decision outcome

Chosen option: **Out-of-process child plugins over NDJSON JSON-RPC 2.0 on `stdio` with scale-to-zero reaping and a standardized `"catalog"` capability**.

1. **Process Supervision & Scale-to-Zero Lifecycle (`apps/server/internal/plugins`)**:
   * Shelfd discovers `/data/plugins/*/plugin.json` and spawns child processes via `exec.CommandContext` without shell interpolation.
   * On enable or boot, Shelfd performs an `initialize` handshake to cache `tools`, `commands`, and `catalog` descriptors in `plugins_state`.
   * Non-daemon plugins (`"daemon": false`) terminate after `idle_timeout_seconds` (default `120s`) of inactivity and respawn transparently on the next RPC call.
2. **Scoped Loopback Attribution & Zero Env Secrets**:
   * Child environments exclude `SHELFD_JWT_SECRET`, database DSNs, and AI provider keys.
   * Every `catalog/*`, `commands/invoke`, and `tools/call` RPC carries an `InvocationAuth` block containing the active `user_id`, decrypted user config, loopback `api_base`, and a 5-minute JWT restricted to `plugin.json` `api_scopes` (`uploads:stage`, `uploads:write`, `books:read`).
3. **Encrypted Per-User Config & Smart cURL Paste**:
   * Fields marked `"x-shelfd-secret": true` or `"format": "password"` in `config_schema` are encrypted at rest in `plugin_user_configs` using AES-256-GCM with an HKDF-SHA256 key derived from `jwt_secret` and the plugin name.
   * `GET /api/v1/plugins/{name}/config` masks set secrets with `"__SHELFD_REDACTED__"`, `PUT` preserves unchanged ciphertexts and extracts cookies or Bearer tokens from pasted DevTools cURL commands (`"x-shelfd-curl-paste": true`), and `config/patch` (`plugin -> host`) persists rotated refresh tokens.
4. **Standardized `"catalog"` Capability & Staged Review Provenance**:
   * Storefront plugins implement `catalog/browse`, `catalog/search`, and `catalog/stage`.
   * Plugins stage downloaded files through loopback `POST /api/v1/books/upload/stage` (`status = 'staged'`) with `metadata_defaults`, `source_plugin`, and `source_external_id`.
   * `book_sources` tracks `(plugin_name, external_id)` across staged upload jobs and committed books so Flutter's generic `CatalogBrowserView` displays `Available`, `Staged`, and `In Library` states automatically.
5. **TypeScript SDK & Opt-In Container Packaging**:
   * `packages/plugin-sdk` (`@shelfd/plugin-sdk`) provides `definePlugin`, `createPoliteFetcher`, `AuthExpiredError` (`-32001`), and the `shelfd-plugin` CLI (`build`, `verify`, `pack`) producing zero-dependency single-file `dist/index.mjs` bundles.
   * The default `Dockerfile` stage remains a pure-Go minimal image, while an opt-in `runtime-plugins` stage adds Node 22 and seeds `plugins/project-gutenberg`.

## Consequences

### Positive consequences

* One generic Flutter `CatalogBrowserView` and JSON Schema `PluginConfigDialog` support every future storefront plugin without client code changes.
* Crash loops, memory spikes, or hung HTTP requests inside third-party plugins cannot crash the Go daemon, corrupt SQLite, or write invalid paths into `/library`.
* Idle Node processes scale to zero after 120 seconds, preserving homelab RAM budgets.
* Users can authenticate CAPTCHA-gated storefronts by pasting a browser cURL command, with credentials encrypted at rest per user.

### Negative consequences

* Running TypeScript plugins requires `node` (>= 22.7) in the host environment or use of the opt-in `runtime-plugins` container target.
* Cold-spawning an idle plugin adds roughly 60-150 ms to the first request after the idle timeout expires.
