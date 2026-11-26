# Shelfd plugin protocol and SDK reference

## Runtime lifecycle

1. **Discovery**: Shelfd scans `/data/plugins/*/plugin.json` at startup, on `POST /api/v1/plugins/discover`, and after `POST /api/v1/plugins/install` (`.zip` upload).
2. **Handshake (`initialize`)**: When a plugin is enabled, Shelfd spawns `entrypoint.command` with `entrypoint.args` (`cwd` set to the plugin directory, no shell) and sends `initialize` (`protocol_version: "2026-11-01"`, `api_base`, `data_dir`).
3. **Scale-to-zero reaping & job leases**: If `"daemon": false` (default), Shelfd caches the plugin's `tools`, `commands`, `catalog`, `auth`, and `actions` descriptors in `plugins_state`, terminates the child after `idle_timeout_seconds` (default 120s) of inactivity, and respawns it automatically on the next RPC call. While any synchronous RPC or background job (`execution: "job"`) is active, the host holds an active lease on the plugin process so `reapIdleProcesses` never terminates it mid-execution. Background jobs also have a 5-minute activity watchdog that resets on every `progress` notification.
4. **Shutdown**: Shelfd sends `shutdown` (3s deadline), closes `stdin`, sends `SIGTERM`, and sends `SIGKILL` if the process does not exit within 5 seconds.

## `plugin.json` schema

```json
{
  "name": "my-storefront",
  "version": "0.1.0",
  "display_name": "My Storefront",
  "description": "Import purchased EPUB and M4B books or send audiobooks to external devices.",
  "homepage": "https://example.com/shelfd-plugin-my-storefront",
  "entrypoint": {
    "command": "node",
    "args": ["dist/index.mjs"]
  },
  "runtime": {
    "family": "node",
    "min_version": "22.7.0"
  },
  "daemon": false,
  "capabilities": ["catalog", "commands", "tools", "actions", "events"],
  "catalog_modes": ["browse", "search"],
  "supported_formats": ["epub", "m4b"],
  "api_scopes": ["uploads:stage", "books:read"],
  "permissions": {
    "network_domains": ["api.example.com", "cdn.example.com"],
    "library_read": true,
    "library_write": false
  },
  "catalog": {
    "modes": ["browse", "search"],
    "supported_formats": ["epub", "m4b"],
    "search_hint": "Search your purchased books...",
    "requires_auth": true
  },
  "auth": {
    "methods": ["oauth_pkce", "token"],
    "title": "Link My Storefront Account",
    "description": "Sign in with OAuth 2.0 PKCE or paste an API token.",
    "callback_param": "code",
    "fields": [
      {
        "key": "access_token",
        "title": "Personal Access Token",
        "secret": true,
        "required": true
      }
    ]
  },
  "actions": [
    {
      "name": "send_to_device",
      "title": "Send to Device",
      "description": "Upload audiobook chapters to an external player.",
      "icon": "speaker",
      "targets": ["book", "series"],
      "media_types": ["audiobook"],
      "mode": "form",
      "execution": "job"
    }
  ],
  "config_schema": {
    "type": "object",
    "required": ["session_cookie"],
    "properties": {
      "session_cookie": {
        "type": "string",
        "title": "Session Cookie",
        "description": "Paste your session cookie or a browser Copy-as-cURL command.",
        "x-shelfd-secret": true,
        "x-shelfd-curl-paste": true,
        "x-shelfd-cookie-name": "_session_id"
      },
      "preferred_format": {
        "type": "string",
        "title": "Preferred Format",
        "enum": ["epub", "m4b"],
        "default": "epub"
      }
    }
  }
}
```

### Custom `config_schema` extensions

- `"x-shelfd-secret": true` (or `"format": "password"`): Encrypts the field at rest in `plugin_user_configs` using AES-256-GCM (HKDF-SHA256 key derived from `jwt_secret` + plugin name). Redacts the value to `"__SHELFD_REDACTED__"` on `GET /api/v1/plugins/{name}/config`.
- `"x-shelfd-hidden": true`: Omits the field from Shelfd's plugin settings form and `Save Settings` payload while keeping it in `config_schema` so server-side encryption (`x-shelfd-secret`), redaction, and `config/patch` persistence continue to work. Useful for `auth`-managed tokens, timestamps, client credentials, and internal endpoints. When all properties are hidden, Shelfd hides the `Save Settings` button and displays only the Account Linking (`auth`) section.
- `"x-shelfd-curl-paste": true`: Adds a "Paste cURL" button in Shelfd's UI and instructs the Go host to extract `Authorization: Bearer ...` or cookies from pasted `curl` commands before encrypting.
- `"x-shelfd-cookie-name": "<cookie_name>"`: Extracts a specific cookie value (such as `_simpleauth_sess`) from a pasted `Cookie:` header or `-b` flag.

## Declarative Auth & Outbound Actions (`"actions"`)

### `AuthSpec` and `auth/*` handlers

Plugins that link user accounts declare `auth` (`AuthSpec`) and implement `auth` handlers in `definePlugin`:

- `auth/status` (`GET /api/v1/plugins/{name}/auth/status`): Returns `AuthStatusResult` (`connected` / `linked`, `configured`, `account_label`, `method`, `message`).
- `auth/begin` (`POST /api/v1/plugins/{name}/auth/begin`): Accepts `{ method, redirect_uri, state, code_challenge, code_challenge_method }` and returns `AuthBeginResult` (`authorize_url`, `method`, `callback_param`). Shelfd's web client passes `<origin>/settings?plugin=<name>` as `redirect_uri`; external OAuth providers must whitelist that callback URL, and `authorize_url` must request all API scopes needed by the plugin's endpoints (missing scopes surface as upstream HTTP `403` even when the access token is valid and unexpired).
- `auth/complete` (`POST /api/v1/plugins/{name}/auth/complete`): Exchanges OAuth `code` + `code_verifier` or direct `values` / token credentials, persists encrypted tokens via `await ctx.patchConfig(...)`, and returns `AuthStatusResult`.
- `auth/disconnect` (`POST` or `DELETE /api/v1/plugins/{name}/auth/disconnect`): Clears stored tokens via `await ctx.patchConfig(...)` and returns `AuthStatusResult`.

### `ActionDef`, `ActionFormSpec`, and `InputFieldSpec`

Each action in `actions` (`ActionDef`) specifies:
- `targets`: `Array<"book" | "series">`
- `media_types`: `Array<"ebook" | "audiobook">`
- `mode`: `"immediate"` (invokes directly on click) or `"form"` (calls `actions/prepare` first to render a dynamic dialog)
- `execution`: `"sync"` (waits for `ActionResult`) or `"job"` (spawns a background `PluginJob` and returns `status: "job_started"` immediately)

When `mode: "form"`, `actions/prepare` (`POST /api/v1/plugins/{name}/actions/{action}/prepare`) returns an `ActionFormSpec`:
- `title`, `description`, `submit_label`
- `auth_required`, `auth_message`: When true and the user's account is unlinked, Shelfd shows a direct prompt to open the plugin's auth configuration dialog.
- `entries`: Optional list of `ActionEntry` items (`id`, `index`, `title`, `subtitle`, `icon_uri`, `image_url`) representing chapters, files, or series books.
- `fields`: Array of declarative `InputFieldSpec` controls:
  - `kind`: `"text" | "boolean" | "select" | "segmented" | "remote_select" | "asset_picker"`
  - `visible_when`: `{ field: string, equals?: string, in?: string[] }` for conditional field visibility.
  - `options`: Inline `OptionItem[]` (`value`, `label`, `subtitle`, `description`, `image_url`, `icon_uri`, `badge`, `count`, `meta`).
  - `options_source` & `searchable`: Queries `actions/options` (`GET /api/v1/plugins/{name}/actions/{action}/options?source=...&tab=...&q=...`) dynamically.
  - `asset_shape` (`"square_pixel" | "cover"`), `tabs` (`AssetPickerTab[]`), `allow_upload` (enables 5 MB multipart uploads to `POST /api/v1/plugins/{name}/actions/{action}/assets` -> `actions/upload_asset`), and `per_entry_override` (`override_key`, `override_label`) for assigning custom values or icons per chapter/book row in `entries`.

### `ActionResult` outcomes

`actions/invoke` (`POST /api/v1/plugins/{name}/actions/{action}/invoke`) returns an `ActionResult`:
- `status: "completed"`: Immediate completion with optional `message` and `external_url`.
- `status: "client_handoff"`: Instructs the client to download `download_api_path` (e.g. `/api/v1/books/{id}/markdown` with `file_name` and `mime_type`) and open `open_url` in the browser.
- `status: "job_started"`: Returned automatically by the host when `execution: "job"` (or `async: true`), including `job_id` and initial `job` snapshot (`GET /api/v1/plugins/jobs`, `DELETE /api/v1/plugins/jobs/{id}`).

## `@thebenforce/shelfd-plugin-sdk` API

```ts
import {
  definePlugin,
  createPoliteFetcher,
  AuthExpiredError,
  PluginRPCError,
  truncateForMCP,
  type PluginContext,
} from "@thebenforce/shelfd-plugin-sdk";
```

### `PluginContext<TConfig>`

Every `catalog/*`, `auth/*`, `actions/*`, `commands/invoke`, and `tools/call` handler receives `ctx`:

- `ctx.userId`: Active Shelfd user ID.
- `ctx.config`: Decrypted per-user configuration matching `TConfig`.
- `ctx.apiBase`: Loopback Shelfd server URL (for example `http://127.0.0.1:8080`).
- `ctx.invocationToken`: Short-lived (5-minute) JWT restricted to `plugin.json` `api_scopes`.
- `ctx.dataDir`: Per-plugin persistent scratch directory (`/data/plugins/<name>/.data`).
- `await ctx.fetchLoopback(path, init)`: Performs an authenticated loopback HTTP request against Shelfd (`ctx.apiBase`), automatically refreshing `ctx.invocationToken` via `host/refresh_token` and retrying once if the token expired with HTTP `401`.
- `await ctx.openLoopbackStream(path)`: Opens a streaming response (`{ stream: ReadableStream<Uint8Array>, contentLength?: number, contentType?: string }`) from a loopback endpoint such as `/api/v1/books/{id}/files/{fileId}/stream` without buffering the file in memory.
- `await ctx.getBookMarkdown(bookId)`: Fetches compiled book Markdown (metadata, synopsis, highlights, notes, and chapters) and filename from `GET /api/v1/books/{id}/markdown` (requires `"books:read"` scope).
- `await ctx.refreshInvocationToken()`: Explicitly requests a fresh 5-minute scoped loopback JWT from the host via `host/refresh_token`.
- `await ctx.stageBook(input)`: Uploads a book from disk (`filePath`, streamed via `fs.openAsBlob` with minimal RAM) or memory (`data`)—plus optional `cover`, `metadataDefaults`, and `externalId`—via `POST /api/v1/books/upload/stage` into `status = 'staged'` (requires `"uploads:stage"` scope; automatically refreshes expired 5-minute loopback JWTs and re-streams a fresh `FormData` payload on HTTP `401` or early socket close).
- `await readResponseBytesWithProgress(response, onProgress, options)`: SDK helper that streams binary download responses, reporting `fraction` when `Content-Length` is known and periodic `loadedBytes` keepalives when chunked so background staging jobs keep the host watchdog alive.
- `await ctx.patchConfig(partialValues)`: Sends `config/patch` to the Go host to merge and encrypt rotated tokens for `ctx.userId`.
- `ctx.reportProgress({ jobId, batchId, current, total, percent, externalId, title, message })`: Sends a live progress frame to the host (resetting the 5-minute job watchdog and updating Shelfd's Background Tasks UI over SSE; `jobId` defaults automatically to `req.job_id` during background `actions/invoke` and `catalog/stage` runs).
- `ctx.emitEvent(event, data)`: Broadcasts a custom `plugin_event` payload (< 64 KB).
- `ctx.log(level, message)`: Sends structured diagnostic logs to the plugin log ring buffer (`Settings -> Plugins -> Logs`).

## Wire methods summary

| Direction | Method | Purpose |
| :--- | :--- | :--- |
| Host -> Plugin | `initialize` | Startup handshake; returns `name`, `version`, `tools`, `commands`, `catalog`, `auth`, `actions`. |
| Host -> Plugin | `config/update` | Notification when a user saves updated settings (`onConfigUpdate`). |
| Host -> Plugin | `auth/status` | Returns account-linking status for the current user. |
| Host -> Plugin | `auth/begin` | Initiates OAuth 2.0 PKCE or interactive login flow. |
| Host -> Plugin | `auth/complete` | Completes OAuth code exchange or token linking. |
| Host -> Plugin | `auth/disconnect` | Unlinks the user's account credentials. |
| Host -> Plugin | `actions/prepare` | Returns declarative `ActionFormSpec` (`fields`, `entries`) for a target book or series. |
| Host -> Plugin | `actions/options` | Queries dynamic `OptionItem[]` choices for `remote_select` or `asset_picker` fields. |
| Host -> Plugin | `actions/upload_asset` | Uploads a custom asset (up to 5 MB, base64-encoded) and returns an `OptionItem`. |
| Host -> Plugin | `actions/invoke` | Executes an action synchronously or inside a background `PluginJob`. |
| Host -> Plugin | `catalog/browse` | Lists owned items or topic shelves (`items`, `groups`, `next_cursor`, `total_count`). |
| Host -> Plugin | `catalog/search` | Searches remote catalog by `query`, `cursor`, and `filters`. |
| Host -> Plugin | `catalog/stage` | Downloads requested `items` (`external_id`, `title`, `preferred_format`) synchronously or inside a background `PluginJob` (`job_id`) and stages them via `ctx.stageBook()`. |
| Host -> Plugin | `commands/invoke` | Runs a named UI/REST command (`commands[name].execute`). |
| Host -> Plugin | `tools/call` | Runs an MCP tool (`tools[name].execute`). |
| Host -> Plugin | `shutdown` | Graceful teardown request before process exit. |
| Plugin -> Host | `config/patch` | Request (`ctx.patchConfig`) to persist refreshed per-user secrets encrypted. |
| Plugin -> Host | `host/refresh_token` | Request (`ctx.refreshInvocationToken`) to renew the 5-minute scoped loopback JWT during long jobs. |
| Plugin -> Host | `progress` / `event` / `log` | One-way JSON-RPC notifications (< 64 KB per frame). |

## Loopback API scopes

- `"uploads:stage"`: Permits `POST /api/v1/books/upload/stage` and `POST /api/v1/books/upload/jobs/{id}/cover`.
- `"uploads:write"`: Permits `POST /api/v1/books/upload`.
- `"books:read"`: Permits `GET` and `HEAD` on `/api/v1/books`, `/api/v1/books/{id}`, `/api/v1/books/{id}/markdown`, `/api/v1/books/{id}/files/{fileId}/stream`, `/api/v1/audiobooks/{id}/chapters`, `/api/v1/audiobooks/{id}/stream`, `/api/v1/series`, `/api/v1/series/{id}`, `/api/v1/authors`, `/api/v1/genres`, `/api/v1/topics`, and `/api/v1/search`. All mutating methods (`POST`, `PUT`, `DELETE`) on library endpoints are blocked with HTTP `403`.

## Error codes

- `-32001` (`AuthExpiredError`): Credentials missing, expired, or lacking required scopes. Shelfd maps this to HTTP `401` on the plugin endpoint and shows an "Update Credentials" / reconnect prompt. When probing fallback API endpoints, only throw `AuthExpiredError` immediately on HTTP `401`; record HTTP `403` and try remaining fallback endpoints first before throwing `AuthExpiredError`.
- `-32601`: Unsupported method or unknown command/tool/action.
- `-32602`: Invalid parameters.

