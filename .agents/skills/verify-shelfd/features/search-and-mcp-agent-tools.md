# Semantic Search & MCP Agent Tools

Shelfd combines `sqlite-vec` vector similarity and SQLite FTS5 keyword indexing to answer natural-language and exact-phrase queries in `/search`, `GET /api/v1/search`, and external AI agent sessions over the embedded Model Context Protocol (`/mcp`) server.

## Sub-features

- `ui-semantic-search` submits queries from `/search`, renders ranked passage/chapter hits with book attribution, and navigates directly to the matching book/chapter.
- `mcp-search-library` executes JSON-RPC 2.0 `tools/call` for `search_library` authenticated with a `shelfd_...` bearer API token, enforcing token-budgeted summaries.
- `mcp-metadata-and-read` retrieves full catalog taxonomy via `get_book_metadata` and paginated chapter text via `read_chapter_content`.

## How to get to it (user POV)

- Choose `Semantic Search` in the sidebar (`/search`), type a query in `Ask your library anything in natural language...`, and press `Enter` (or click the search button).
- Connect an MCP client (Claude Desktop, Cursor, or agent harness) to `http://127.0.0.1:18080/mcp` with `Authorization: Bearer <shelfd_token>` and invoke `search_library`, `get_book_metadata`, and `read_chapter_content`.

## Driving it with control-shelfd

Preconditions:

- Authenticated browser session and healthy stack (`control-shelfd doctor` exits `0`).
- Lookup `<DUNE_ID>` via `.agents/skills/verify-shelfd/scripts/control-shelfd state`.

- **Drive Semantic Search UI.** Navigate to `/search`, submit `"Gom Jabbar"`, and verify `Dune` results render. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/search" --wait-ms 1200`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Ask your library anything" --value "Gom Jabbar" --submit`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/search.aria.txt`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/search.png`. The ARIA snapshot includes `Dune` and matching passage text from `Chapter 1: The Gom Jabbar`.
- **Drive MCP `search_library`, `get_book_metadata`, and `read_chapter_content`.** Execute all three MCP tools over JSON-RPC 2.0 and save proof artifacts. Run:
  - `.agents/skills/verify-shelfd/scripts/control-shelfd mcp search_library '{"query":"Gom Jabbar"}' > /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/mcp-search.json`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd mcp get_book_metadata '{"book_id":"<DUNE_ID>"}' > /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/mcp-metadata.json`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd mcp read_chapter_content '{"book_id":"<DUNE_ID>","chapter_index":1}' > /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/mcp-chapter.json`
  - `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/search-and-mcp-agent-tools/console.json`

## Gotchas

- `/mcp` authenticates with SHA-256 hashed `shelfd_...` API tokens (not user session cookies); unauthenticated requests return `401 Unauthorized`.
- Background vector indexing runs on a 5-second worker interval after initial startup; FTS5 keyword search works immediately, while pure semantic similarity matures within ~5 seconds of launch against the deterministic local Ollama boundary.
