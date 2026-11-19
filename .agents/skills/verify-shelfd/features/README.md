# Shelfd verification map

This directory is the maintained source for verifying the user-facing behavior of Shelfd (`apps/server`) and the Shelf web client (`apps/app`). Read this index before driving the app, then follow the matching feature file as the verification recipe.

## Baseline preconditions

- Launch an isolated stack via `.agents/skills/verify-shelfd/scripts/control-shelfd launch` (or `launch --foreground` in daemonized command sandboxes).
- Default isolated endpoints are `http://127.0.0.1:18080` (API, MCP & Shelf Flutter Web SPA), `http://127.0.0.1:13000` (Ollama AI boundary stub), and `127.0.0.1:19222` (Chromium CDP).
- Set `SHELFD_VERIFY_RUN_ID`, `SHELFD_VERIFY_PORT`, `SHELFD_VERIFY_AI_PORT`, and `SHELFD_VERIFY_CDP_PORT` when running multiple verification sessions concurrently.
- Seeded catalog fixtures include `Dune` (`Frank Herbert`, `Dune Chronicles #1`, reflowable EPUB with `.metadata.json` and `desc.txt` Audiobookshelf sidecars) and `The Tale of Peter Rabbit` (`Beatrix Potter`, pre-paginated EPUB 3 with SMIL media overlays and playable MP3 audio), plus an uningested upload fixture at `$RUNTIME_DIR/fixtures/earthsea.epub`.
- Run `.agents/skills/verify-shelfd/scripts/control-shelfd doctor` before the first drive and after any unexpected failure.
- Never drive an instance that was not started by this verification run.

## Driving conventions

- Start every recipe from an authenticated session on `/books` (after completing the `/connect` login recipe in `connect-and-catalog-shell.md`) unless its preconditions state otherwise.
- Prefer ARIA roles and accessible names (`--role button --name "..."`, `--role textbox --name "..."`) over coordinates or DOM indices.
- Run browser actions through `.agents/skills/verify-shelfd/scripts/control-shelfd browser ...`.
- Run authenticated REST checks through `.agents/skills/verify-shelfd/scripts/control-shelfd api ...`, multipart uploads through `upload`, MCP JSON-RPC calls through `mcp`, and CLI invocations through `cli -- ...`.
- Do not delete `/tmp/shelfd-verify-evidence` during cleanup.

## Proof and skip reporting

- Capture both the user action and the resulting state, not only a final screen.
- UI proof requires an ARIA snapshot (`.aria.txt`), a screenshot (`.png`) showing app identity, and a clean console audit (`console --assert-clean --path ...`).
- Mutation proof requires a second read-only confirmation (API query, reopened UI view, or filesystem/sidecar checksum check).
- Fixed-layout EPUB proof requires `browser iframes --assert-normalized` (`bodyChildren > 0` and `nestedAbsoluteSpans === 0`).
- Report any unreachable path (such as live Yoto OAuth or cloud NotebookLM Enterprise push without external credentials) with the attempted route and unmet prerequisite; never claim a skipped entry point was verified through another path.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph describing the user-visible behavior, followed by four H2 sections in exact order:

1. `Sub-features` — short IDs with one line per behavior.
2. `How to get to it (user POV)` — every user-facing entry point.
3. `Driving it with control-shelfd` — starts with `Preconditions:` and pairs each user action with exact commands and observable results.
4. `Gotchas` — traps that can waste or invalidate a verification run.

## Features

- [Connect, Auth & Catalog Shell](./connect-and-catalog-shell.md) covers `/connect` server pairing, persistent `AppShell` sidebar navigation across Books, Series, Authors, and Genres, Book Detail inspection, and sign-out.
- [EPUB Reader, Themes & Annotations](./epub-reader-and-annotations.md) covers fullscreen reflowable EPUB reading (`/books/:id/read`), chapter transitions, bookmark creation/persistence, and reader return navigation.
- [Audiobook & Read-Along Playback](./audiobook-and-readalong-playback.md) covers `/books/:id/listen`, `MiniPlayerBar` position preservation across minimize/reopen, fixed-layout SMIL `<iframe>` rendering, and live toggling between Audiobook and Read-Along modes.
- [Book Upload, Queue SSE & Library Scan](./book-upload-and-library-scan.md) covers asynchronous EPUB upload (`POST /api/v1/books/upload` & `/uploads`), background ingestion into `/library/<Author>/<Title>/<Title>.epub`, `Rescan Library` / `-scan`, and Audiobookshelf sidecar sanctity.
- [Semantic Search & MCP Agent Tools](./search-and-mcp-agent-tools.md) covers `/search` natural-language/keyword discovery, `GET /api/v1/search`, and external agent tool calls (`search_library`, `get_book_metadata`, `read_chapter_content`) over `/mcp`.
