# Plugins, Storefronts & Outbound Actions

Shelfd's out-of-process plugin host manages built-in and user-installed plugins (`project-gutenberg`, `yoto`, `notebooklm`) over stdio JSON-RPC 2.0 and scoped loopback HTTP, rendering catalog storefronts in `/plugins/catalog`, encrypted per-user settings and OAuth 2.0 PKCE / token linking in `PluginConfigDialog` (`/settings`), declarative book and series action forms (`PluginActionDialog`, `PluginAssetPickerDialog`, `PluginJobsBanner`), and generic book Markdown export (`GET /api/v1/books/{id}/markdown`).

## Sub-features

- `plugin-discovery-and-settings` lists installed plugins via `GET /api/v1/plugins`, toggles enable/disable state in `/settings`, and persists encrypted per-user secrets (`enc:v1:...`) and OAuth/token state via `/api/v1/plugins/{name}/{config,auth/*}`.
- `plugin-zip-install-and-update` installs or updates `.zip` plugin bundles via `POST /api/v1/plugins/install` (`201 Created` on fresh install, `200 OK` on in-place update) and targeted `POST /api/v1/plugins/{name}/update` (validating that `plugin.json`'s `name` matches `{name}`), stopping any running process before swapping files, preserving `.data/` and enabled/disabled state, writing `.shelfd-user-managed`, and returning `PluginInstallResult` (`updated`, `previous_version`) for version-change feedback in `/settings`.
- `plugin-catalog-storefront` browses and searches external catalog sources such as `project-gutenberg` in `/plugins/catalog` (`GET /api/v1/plugins/{name}/catalog`) and stages selected titles into the upload review queue (`POST /api/v1/plugins/{name}/catalog/stage`).
- `book-markdown-export` compiles book metadata, synopsis, reader highlights/notes, and chapter text at `GET /api/v1/books/{id}/markdown` (accessible via `Download Markdown (.md)` on Book Detail and scoped `books:read` plugin loopback).
- `outbound-action-notebooklm` invokes `POST /api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke` on ebooks, returning `client_handoff` (`download_api_path: /api/v1/books/{id}/markdown`, `open_url: https://notebooklm.google.com/`) in zero-config mode or pushing Markdown to a configured webhook or Discovery Engine endpoint with SSRF protection.
- `outbound-action-yoto` prepares and invokes `yoto`'s `send_to_yoto` form action on audiobooks and series (`/api/v1/plugins/yoto/actions/send_to_yoto/{prepare,options,assets,invoke}`), rendering conditional playlist selection, 16×16 pixel icon pickers and custom uploads, per-entry overrides, and live `job_progress` updates in `PluginJobsBanner` (`/api/v1/plugins/jobs`).

## How to get to it (user POV)

- Open `Settings` (`/settings`) to view installed plugin cards (`Project Gutenberg`, `Yoto Player`, `Google NotebookLM`), toggle plugins on or off, install a new plugin via the header `Install .zip` button, update an existing plugin via its per-plugin `Update .zip` button (seeing version-change banners such as `Updated <Name> v1.0.0 → v1.1.0.`), or click `Configure` (`PluginConfigDialog`) to edit settings and link accounts via OAuth 2.0 PKCE or token paste.
- Choose `Free Books` in the `AppShell` sidebar (`/plugins/catalog`) to search Project Gutenberg and stage public-domain books into the upload queue.
- On any ebook's Book Detail page (`/books/:bookId`), open the `Download` menu and choose `Download Markdown (.md)` or `Send to NotebookLM`.
- On any audiobook's Book Detail page (`/books/:bookId`) or audiobook Series Detail page (`/series/:seriesId`), open the action menu (or click the series action button) and choose `Send to Yoto` to configure target MYO card, cover upload, and 16×16 chapter icons in `PluginActionDialog`, then monitor transfer progress in `PluginJobsBanner`.

## Driving it with control-shelfd

Preconditions:

- Authenticated browser session on `/books` and healthy isolated stack (`control-shelfd doctor` exits `0`).
- Built-in plugins (`project-gutenberg`, `yoto`, `notebooklm`) are built (`pnpm build`) and seeded in `$SHELFD_RUNTIME_DIR/data/plugins`.
- Lookup `<DUNE_ID>` and `<PETER_ID>` via `.agents/skills/verify-shelfd/scripts/control-shelfd state`.

- **Verify installed plugins and legacy route absence.** List active plugins and confirm legacy hardcoded endpoints return `404` or `405`. Run:
  - `.agents/skills/verify-shelfd/scripts/control-shelfd api GET /api/v1/plugins > /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/plugins-list.json`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd api GET "/api/v1/books/<DUNE_ID>/markdown" > /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/dune.md`
  `plugins-list.json` lists `project-gutenberg`, `yoto`, and `notebooklm` with `enabled: true`, and `dune.md` starts with `# Dune` and `## Book Metadata`.
- **Exercise `.zip` plugin install and targeted update.** Package a plugin bundle (e.g., via `node packages/plugin-sdk/cli/shelfd-plugin.mjs pack plugins/project-gutenberg`) and upload multipart `file` to `POST /api/v1/plugins/install` and `POST /api/v1/plugins/{name}/update` with the bearer token from `control-shelfd state`:
  - Uploading a new plugin to `POST /api/v1/plugins/install` returns `201 Created` with `"updated": false` and writes `$SHELFD_RUNTIME_DIR/data/plugins/<name>/.shelfd-user-managed`.
  - Uploading a newer version to `POST /api/v1/plugins/<name>/update` returns `200 OK` with `"updated": true` and `"previous_version"` set to the prior semver, while preserving `.data/` and enabled/disabled state.
  - Uploading an archive whose `plugin.json` `name` differs from `{name}` to `POST /api/v1/plugins/<name>/update` returns `400 Bad Request` without modifying the installed plugin.
- **Exercise `notebooklm` outbound action (zero-config client handoff).** Invoke `send_to_notebooklm` on `Dune`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd api POST /api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke '{"target":{"type":"book","id":"<DUNE_ID>","title":"Dune"}}' > /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/notebooklm-handoff.json`. The response returns `"status": "client_handoff"`, `"download_api_path": "/api/v1/books/<DUNE_ID>/markdown"`, and `"open_url": "https://notebooklm.google.com/"`.
- **Exercise `yoto` declarative form preparation.** Call `actions/prepare` on `The Tale of Peter Rabbit`. Run `.agents/skills/verify-shelfd/scripts/control-shelfd api POST /api/v1/plugins/yoto/actions/send_to_yoto/prepare '{"target":{"type":"book","id":"<PETER_ID>","title":"The Tale of Peter Rabbit"}}' > /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/yoto-prepare.json`. The response returns an `ActionFormSpec` containing `fields` (`mode`, `card_id`, `new_title`, `upload_cover`, `auto_cover_icon`, `default_icon`) and resolved audio chapter `entries`.
- **Inspect Settings plugin cards and Book Detail dynamic action menus in browser.** Navigate to `/settings` and `/books/<DUNE_ID>`, then capture ARIA, screenshot, and clean console proof (confirming both `Install .zip` and per-plugin `Update .zip` controls render). Run:
  - `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/settings" --wait-ms 1200`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/settings.aria.txt`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/settings.png`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/plugins-and-outbound-actions/console.json`

## Gotchas

- Outbound action menu items on Book Detail and Series Detail are filtered by both `targets` (`book` vs `series`) and `media_types` (`ebook` vs `audiobook`): `Send to Yoto` appears only on books/series with audio files (such as `The Tale of Peter Rabbit`), while `Send to NotebookLM` appears on ebooks (such as `Dune`). Disabling a plugin in `/settings` immediately hides its actions from all menus.
- Built-in plugins execute as Node.js child processes from `$SHELFD_RUNTIME_DIR/data/plugins/<name>/dist/index.mjs`; always run `pnpm build` before launching an isolated stack if plugin TypeScript sources changed.
- Installing or updating a plugin via `.zip` writes `.shelfd-user-managed` inside `/data/plugins/<name>/` so `docker/entrypoint.sh` preserves user-installed versions across container restarts; updating a disabled plugin preserves `enabled: false` and does not spawn a child process.
- `plugins/notebooklm` enforces dial-time SSRF validation blocking loopback, RFC1918, link-local, and cloud metadata IPs (`169.254.169.254`, `fd00:ec2::254`) on `webhook_url` and `endpoint` unless `allow_local_endpoints` is explicitly set in local test fixtures.
- Non-daemon plugins scale to zero after `idle_timeout_seconds`, but active background jobs (`execution: "job"`) hold a host lease and refresh their 5-minute scoped loopback JWT via `"host/refresh_token"` so multi-chapter audio transfers are never reaped mid-upload.
