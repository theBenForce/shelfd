---
name: verify-shelfd
description: "End-to-end verification playbook and live control harness for Shelfd (Go homelab ebook/audiobook daemon, MCP server, and Shelf Flutter Web client). Use when verifying UI flows, EPUB/fixed-layout readers, audiobook & read-along playback, async book uploads, library scans, or MCP agent tools."
---

# Verify `Shelfd`

Drive `Shelfd` (`apps/server`) and `Shelf` (`apps/app` Flutter Web UI) end to end against an isolated SQLite database, disposable Audiobookshelf-shaped `/library` directory, native Shelfd `web_dir` SPA + `HttpOnly` cookie session, deterministic local Ollama boundary, and headless Chromium CDP session.

Default isolated endpoints (`SHELFD_VERIFY_RUN_ID=default`):
- **API, MCP & Shelf Flutter Web SPA**: `http://127.0.0.1:18080` (`SHELFD_VERIFY_PORT` / `SHELFD_VERIFY_WEB_URL`)
- **Ollama AI Boundary Stub**: `http://127.0.0.1:13000` (`SHELFD_VERIFY_AI_PORT`)
- **Headless Chromium CDP**: `127.0.0.1:19222` (`SHELFD_VERIFY_CDP_PORT`)
- **Runtime Scratch State**: `/tmp/shelfd-verify-runtime-default` (`SHELFD_RUNTIME_DIR`)
- **Durable Proof Artifacts**: `/tmp/shelfd-verify-evidence` (`SHELFD_EVIDENCE_DIR`)

**Isolation & Side-by-Side Runs.** Two verification runs can execute concurrently only when given distinct `SHELFD_VERIFY_RUN_ID`, `SHELFD_VERIFY_PORT`, `SHELFD_VERIFY_AI_PORT`, and `SHELFD_VERIFY_CDP_PORT` values. `control-shelfd launch` refuses to clobber any port owned by another process and never reads the repository's `.env` or touches a user's `/library` or `/data` directories.

## Launch

Build the Go server (`-tags sqlite_fts5`), ensure `apps/app/build/web` reflects current Dart sources, seed disposable fixtures (`Dune` reflowable EPUB with `.metadata.json` / `desc.txt` Audiobookshelf sidecars, `The Tale of Peter Rabbit` pre-paginated SMIL read-along EPUB + playable MP3, and `fixtures/earthsea.epub`), start the API, Web SPA, and headless Chromium processes, and wait until `/health` and `/connect` are ready.

In a standard persistent shell:

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd launch
```

In command sandboxes that reap child processes when a non-daemon step exits (such as Antigravity `run_command`), start the supervisor in foreground mode with `IsDaemon: true`:

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd launch --foreground
```

Tear down with `.agents/skills/verify-shelfd/scripts/control-shelfd cleanup`.

## Doctor

Run this read-only check before the first drive and whenever anything looks unexpected. It verifies required binaries, `/tmp/*` runtime isolation, `/health` status, cross-origin CORS `Cache-Control` allowance on `/api/v1/queue/events`, admin JWT validity on `/api/v1/auth/me`, MCP bearer auth on `HEAD /mcp`, SHA-256 integrity of Audiobookshelf `.metadata.json` and `desc.txt` sidecars, and an active Chromium CDP page target.

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd doctor
```

Inspect dynamic IDs, credentials, and fixture paths at any point with:

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd state
```

## Drive

Always prefer stable ARIA roles and accessible names over coordinates. `browser-cdp.mjs` automatically activates Flutter Web's `<flt-semantics-placeholder>` so the full semantic tree (`textbox`, `button`, `StaticText`, etc.) is live.

```bash
# Authenticate through the real /connect UI
.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Server URL" --value "http://127.0.0.1:18080"
.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Username" --value "admin"
.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Password" --value "shelfd-verify-pass-123"
.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Connect & Sync Library" --wait-ms 2000

# Navigate, inspect ARIA tree, click semantic controls, and check fixed-layout EPUB iframes
.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/library.aria.txt
.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "The Tale of Peter Rabbit"
.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books"
.agents/skills/verify-shelfd/scripts/control-shelfd browser iframes --assert-normalized

# Drive authenticated REST, multipart EPUB upload, MCP JSON-RPC 2.0 tools, and isolated CLI scan
.agents/skills/verify-shelfd/scripts/control-shelfd api GET /api/v1/books
.agents/skills/verify-shelfd/scripts/control-shelfd upload
.agents/skills/verify-shelfd/scripts/control-shelfd mcp search_library '{"query":"Gom Jabbar"}'
.agents/skills/verify-shelfd/scripts/control-shelfd cli -- -scan
```

See [`features/README.md`](./features/README.md) for the maintained feature map and per-feature verification recipes.

## Evidence

Write all proof artifacts under `/tmp/shelfd-verify-evidence/<feature-slug>/` (or `$SHELFD_EVIDENCE_DIR/<feature-slug>/`):

- **Real user path**: Drive login, navigation, reader controls, and playback toggles through the rendered UI — never bypass UI flows by mutating internal providers or database tables directly.
- **Action + resulting state**: Capture both an ARIA snapshot (`*.aria.txt`) and a viewport screenshot (`*.png`), plus `browser console --assert-clean --path .../console.json` to prove zero uncaught JS or CORS errors.
- **Side-effect verification**: Confirm mutations from a second read-only view or storage check (e.g., `api GET /api/v1/books/:id/bookmarks`, `api GET /api/v1/audiobooks/:id/progress`, checking `/library/<Author>/<Title>/<Title>.epub` on disk, and `doctor` confirming Audiobookshelf sidecars remain byte-identical).
- **External boundary discipline**: Only the external Ollama HTTP provider (`SHELFD_AI_BASE_URL`) is stubbed at its network boundary for deterministic offline 256-dim embeddings; Go API handlers, SQLite/`sqlite-vec`, FTS5, EPUB/SMIL parsers, audio range streaming, and Flutter Web run unmocked.

Validate collected proof files with:

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd evidence
```

## Cleanup

Stop only the PIDs recorded for this verification run (via command-verified PID files and `stop.signal`) and delete `/tmp/shelfd-verify-runtime-default` while keeping `/tmp/shelfd-verify-evidence` intact. Never kill `shelfd`, `node`, or `chromium` by process name.

```bash
.agents/skills/verify-shelfd/scripts/control-shelfd cleanup
```

Always re-run `control-shelfd evidence` after `cleanup` to confirm proof artifacts survived teardown.

## Helpers

- [`scripts/control-shelfd`](./scripts/control-shelfd) — Executable lifecycle & multi-surface verification harness (`launch [--foreground]`, `doctor`, `state`, `browser`, `api`, `upload`, `mcp`, `cli`, `evidence`, `cleanup`).
- [`scripts/browser-cdp.mjs`](./scripts/browser-cdp.mjs) — Zero-dependency Node.js Chromium CDP client with automatic Flutter Web semantics activation (`goto`, `snapshot`, `click`, `fill`, `press`, `screenshot`, `console`, `iframes`, `eval`).
