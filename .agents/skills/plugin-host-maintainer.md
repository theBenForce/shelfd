# Plugin Host & Storefront Catalog Maintainer Manual

Use this guide when modifying Shelfd's Go plugin host (`apps/server/internal/plugins`), plugin REST endpoints (`apps/server/internal/api/plugins.go`), `@thebenforce/shelfd-plugin-sdk` (`packages/plugin-sdk`), or the Flutter catalog browser (`apps/app/lib/ui/features/plugins/`).

## Core invariants

1. **No direct `/library` or database writes from plugins**:
   - Plugins run out of process over NDJSON JSON-RPC 2.0 on `stdio` (`exec.CommandContext`, never `/bin/sh -c`).
   - Imported books must enter Shelfd exclusively via `POST /api/v1/books/upload/stage` (`status = 'staged'`) so Audiobookshelf path rules (`/library/<Author>/<Title>/<Title>.<ext>`) and duplicate checks always run on commit.

2. **Zero secrets in child `os.Environ`**:
   - `buildCleanPluginEnv` strips `SHELFD_JWT_SECRET`, database DSNs, and AI API keys before spawning any plugin process.
   - User attribution and loopback API permissions travel per request inside `InvocationAuth` using short-lived (5-minute) JWTs minted by `GeneratePluginInvocationJWT` and enforced by `PluginScopeAllowsRequest`.

3. **Encrypted user credentials & sentinel preservation**:
   - Fields marked `"x-shelfd-secret": true` or `"format": "password"` in `config_schema` are encrypted at rest in `plugin_user_configs` with AES-256-GCM (HKDF-SHA256 key derived from `jwt_secret` + plugin name).
   - Fields marked `"x-shelfd-hidden": true` stay in `config_schema` for server-side encryption/redaction and `config/patch` persistence while being omitted from `PluginConfigDialog`.
   - `GET /api/v1/plugins/{name}/config` masks stored secrets with `"__SHELFD_REDACTED__"`.
   - `PUT /api/v1/plugins/{name}/config` preserves existing ciphertext when receiving `"__SHELFD_REDACTED__"`, extracts cookies or Bearer tokens when `"x-shelfd-curl-paste": true`, and encrypts new values.
   - `config/patch` (`plugin -> host`) merges and encrypts rotated tokens for the active user.

4. **Scale-to-zero idle reaping**:
   - Non-daemon plugins (`"daemon": false`) cache `tools`, `commands`, and `catalog` descriptors in `plugins_state` during `initialize`, reap their child process after `idle_timeout_seconds`, and respawn on demand so idle daemon memory stays under 50 MB.

5. **Dual storage engine & provenance parity**:
   - Any query change to `plugins_state`, `plugin_user_configs`, or `book_sources` must be implemented identically in both `SQLiteStorageEngine` (`sqlite.go`) and `BunStorageEngine` (`bun.go`).
   - Deleting an uncommitted staged upload job must delete its uncommitted `book_sources` row (`DeleteUncommittedBookSourceByUploadJob`), and committing an upload job must link `book_id` (`LinkBookSourceByUploadJob`).

6. **Safe `.zip` install & in-place update lifecycle**:
   - `Host.InstallOrUpdateZip` must stage and validate archives via `StagePluginZip` (including `ExpectedName` checks for `POST /api/v1/plugins/{name}/update`) **before** stopping the running child process and swapping directories via `ActivateStagedPlugin`.
   - In-place updates must preserve `.data/` scratch storage, preserve the plugin's `enabled` state (skipping process spawn when disabled), and write `.shelfd-user-managed` so `docker/entrypoint.sh` does not overwrite user-updated plugins on container restart.

7. **Authoring new plugins**:
   - Follow [skills/create-shelfd-plugin/SKILL.md](../../skills/create-shelfd-plugin/SKILL.md) (mirrored in `packages/plugin-sdk/skills/create-shelfd-plugin/SKILL.md`).
   - Keep commercial storefront scrapers in external repositories; only public-domain reference plugins (`plugins/project-gutenberg`) belong in this repository.
