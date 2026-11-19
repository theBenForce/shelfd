# Shelfd plugin protocol and SDK reference

## Runtime lifecycle

1. **Discovery**: Shelfd scans `/data/plugins/*/plugin.json` at startup, on `POST /api/v1/plugins/discover`, and after `POST /api/v1/plugins/install` (`.zip` upload).
2. **Handshake (`initialize`)**: When a plugin is enabled, Shelfd spawns `entrypoint.command` with `entrypoint.args` (`cwd` set to the plugin directory, no shell) and sends `initialize` (`protocol_version: "2026-11-01"`, `api_base`, `data_dir`).
3. **Scale-to-zero reaping**: If `"daemon": false` (default), Shelfd caches the plugin's `tools`, `commands`, and `catalog` descriptor in `plugins_state`, terminates the child after `idle_timeout_seconds` (default 120s) of inactivity, and respawns it automatically on the next RPC call.
4. **Shutdown**: Shelfd sends `shutdown` (3s deadline), closes `stdin`, sends `SIGTERM`, and sends `SIGKILL` if the process does not exit within 5 seconds.

## `plugin.json` schema

```json
{
  "name": "my-storefront",
  "version": "0.1.0",
  "display_name": "My Storefront",
  "description": "Import purchased EPUB and M4B books from My Storefront.",
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
  "capabilities": ["catalog", "commands", "tools"],
  "catalog_modes": ["browse", "search"],
  "supported_formats": ["epub", "m4b"],
  "api_scopes": ["uploads:stage"],
  "permissions": {
    "network_domains": ["api.example.com", "cdn.example.com"],
    "library_read": false,
    "library_write": false
  },
  "catalog": {
    "modes": ["browse", "search"],
    "supported_formats": ["epub", "m4b"],
    "search_hint": "Search your purchased books...",
    "requires_auth": true
  },
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
- `"x-shelfd-curl-paste": true`: Adds a "Paste cURL" button in Shelfd's UI and instructs the Go host to extract `Authorization: Bearer ...` or cookies from pasted `curl` commands before encrypting.
- `"x-shelfd-cookie-name": "<cookie_name>"`: Extracts a specific cookie value (such as `_simpleauth_sess`) from a pasted `Cookie:` header or `-b` flag.

## `@shelfd/plugin-sdk` API

```ts
import {
  definePlugin,
  createPoliteFetcher,
  AuthExpiredError,
  PluginRPCError,
  truncateForMCP,
  type PluginContext,
} from "@shelfd/plugin-sdk";
```

### `PluginContext<TConfig>`

Every `catalog/*`, `commands/invoke`, and `tools/call` handler receives `(req, ctx)`:

- `ctx.userId`: Active Shelfd user ID.
- `ctx.config`: Decrypted per-user configuration matching `TConfig`.
- `ctx.apiBase`: Loopback Shelfd server URL (for example `http://127.0.0.1:8080`).
- `ctx.invocationToken`: Short-lived (5-minute) JWT restricted to `plugin.json` `api_scopes`.
- `ctx.dataDir`: Per-plugin persistent scratch directory (`/data/plugins/<name>/data`).
- `await ctx.stageBook(input)`: Uploads book bytes (and optional cover bytes + `metadataDefaults` + `externalId`) via `POST /api/v1/books/upload/stage` into `status = 'staged'`.
- `await ctx.patchConfig(partialValues)`: Sends `config/patch` to the Go host to merge and encrypt rotated tokens for `ctx.userId`.
- `ctx.reportProgress({ batchId, current, total, externalId, title, message })`: Sends a live progress frame to connected Flutter clients over SSE.
- `ctx.emitEvent(event, data)`: Broadcasts a custom `plugin_event` payload (< 64 KB).
- `ctx.log(level, message)`: Sends structured diagnostic logs to the plugin log ring buffer (`Settings -> Plugins -> Logs`).

## Wire methods summary

| Direction | Method | Purpose |
| :--- | :--- | :--- |
| Host -> Plugin | `initialize` | Startup handshake; returns `name`, `version`, `tools`, `commands`, `catalog`. |
| Host -> Plugin | `config/update` | Notification when a user saves updated settings (`onConfigUpdate`). |
| Host -> Plugin | `catalog/browse` | Lists owned items or topic shelves (`items`, `groups`, `next_cursor`, `total_count`). |
| Host -> Plugin | `catalog/search` | Searches remote catalog by `query`, `cursor`, and `filters`. |
| Host -> Plugin | `catalog/stage` | Downloads requested `items` (`external_id`, `preferred_format`) and stages them via `ctx.stageBook()`. |
| Host -> Plugin | `commands/invoke` | Runs a named UI/REST command (`commands[name].execute`). |
| Host -> Plugin | `tools/call` | Runs an MCP tool (`tools[name].execute`). |
| Host -> Plugin | `shutdown` | Graceful teardown request before process exit. |
| Plugin -> Host | `config/patch` | Request (`ctx.patchConfig`) to persist refreshed per-user secrets encrypted. |
| Plugin -> Host | `progress` / `event` / `log` | One-way JSON-RPC notifications (< 64 KB per frame). |

## Error codes

- `-32001` (`AuthExpiredError`): Storefront credentials missing or expired. Shelfd maps this to HTTP `401` on the plugin endpoint and shows an "Update Credentials" prompt in the Catalog Browser.
- `-32601`: Unsupported method or unknown command/tool.
- `-32602`: Invalid parameters.
