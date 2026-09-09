# Shelfd

A lightweight, self-hosted, AI-native ebook server and reader designed to run seamlessly alongside your existing [Audiobookshelf](https://www.audiobookshelf.org/) collection.

```text
Host System:
  /mnt/storage/ebooks/         <-- Shared with Audiobookshelf (read-write for EPUBs)
  /mnt/storage/shelfd/data/    <-- Isolated database (sqlite.db), vector index & covers

Container Mounts:
  - /mnt/storage/ebooks:/library:rw      # Shared ebook collection
  - /mnt/storage/shelfd/data:/data:rw   # Database, embeddings, covers
```

---

## Key Features

* **Zero-Interference Audiobookshelf Coexistence**: Mounts your existing Audiobookshelf library directly (`/library`), scans EPUBs non-destructively, and writes newly uploaded books to `<Author>/<Title>/<Title>.epub` so Audiobookshelf automatically picks them up. Never mutates `.metadata.json`, `desc.txt`, or audio files.
* **Semantic Search via Vectors**: Generates chapter-level summaries using local or cloud AI models (Ollama / OpenAI) and indexes them using embedded `sqlite-vec`.
* **Model Context Protocol (MCP)**: Native HTTP/SSE MCP server allows AI agents (Cursor, Claude Desktop, autonomous agents) to search your library, read chapters, and synthesize knowledge across books.
* **Normalized Taxonomy**: Filterable authors, genres, and series with sequence numbers extracted directly from Dublin Core and EPUB 3 / Calibre collections.
* **Single-Container Homelab Packaging**: Built in Go with embedded SQLite and `sqlite-vec`, packaged into a minimal Alpine container (< 50MB) with PUID/PGID user mapping and graceful signal handling.
* **Modern Cross-Platform Client**: Flutter-based reading app (`Shelf`) for iOS, Android, and macOS/Desktop featuring Warm Editorial Bone, Sepia, and Dark reading themes.

---

## Quick Start with Docker Compose

1. Create a `docker-compose.yml` file:

```yaml
services:
  shelfd:
    image: ghcr.io/thebenforce/shelved:latest
    container_name: shelfd
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      # Match PUID/PGID to your host user so Audiobookshelf and Shelfd share permissions
      - PUID=1000
      - PGID=1000
      - UMASK=002
      - TZ=UTC

      # Optional AI Configuration (defaults to local Ollama on host)
      - SHELFD_AI_PROVIDER=ollama
      - SHELFD_AI_BASE_URL=http://host.docker.internal:11434
      - SHELFD_AI_EMBEDDING_MODEL=nomic-embed-text
      - SHELFD_AI_SUMMARY_MODEL=llama3.2:3b
    volumes:
      # Shelfd state: sqlite database, vector index, and cached covers
      - ./data:/data
      # Shared Audiobookshelf library (read-write for EPUB upload)
      - /path/to/my/ebooks:/library:rw
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

2. Start the daemon:

```bash
docker compose up -d
```

3. Check logs for the initial administrator credentials and MCP API token:

```bash
docker compose logs -f shelfd
```

---

## Audiobookshelf Coexistence & File Permissions

### The Audiobookshelf Sanctity Guarantee
Shelfd is built from the ground up to coexist peacefully with Audiobookshelf:
* **No Sidecar Mutations**: Shelfd will never touch, modify, or delete `.metadata.json`, `desc.txt`, `cover.jpg`, or audio tracks (`.mp3`, `.m4b`, `.flac`).
* **Clean Ingestion Path**: Newly uploaded EPUB files are written to `/library/<Author>/<Title>/<Title>.epub` using sanitized path segments. Audiobookshelf's periodic library scanner detects them automatically without manual imports.
* **No Recursive Chown on Startup**: Unlike many containers that run `chown -R` on all mounts, Shelfd's entrypoint strictly manages permissions for `/data` only. Your shared `/library` files are never touched on startup, preventing I/O freezes and preserving file timestamps on large libraries.

### PUID / PGID Permissions
To ensure both Shelfd and Audiobookshelf can read and write to your shared media folders without file permission locks, configure `PUID` and `PGID`:

| System | Recommended PUID | Recommended PGID | Notes |
| :--- | :--- | :--- | :--- |
| **Standard Linux / Debian** | `1000` | `1000` | Run `id $USER` to check your host UID/GID |
| **Unraid** | `99` | `100` | Standard `nobody:users` ownership on Unraid shares |
| **TrueNAS SCALE** | `568` / `1000` | `568` / `1000` | Match your dataset ACL owner |
| **Synology DSM** | `1026` | `100` | Match your admin or media user UID |

Shelfd applies `UMASK=002` by default, ensuring all newly created files and directories are group-writable (`rw-rw-r--` / `rwxrwxr-x`).

---

## AI Configuration (Semantic Search & Summarization)

Shelfd supports both local self-hosted models (via Ollama) and cloud providers (via OpenAI).

### Option A: Local Ollama (Recommended for Homelabs)
Ensure Ollama is running on your host machine or homelab server with the required models:
```bash
ollama pull nomic-embed-text
ollama pull llama3.2:3b
```

In `docker-compose.yml`:
```yaml
environment:
  - SHELFD_AI_PROVIDER=ollama
  - SHELFD_AI_BASE_URL=http://host.docker.internal:11434
  - SHELFD_AI_EMBEDDING_MODEL=nomic-embed-text
  - SHELFD_AI_EMBEDDING_DIMENSIONS=1536
  - SHELFD_AI_SUMMARY_MODEL=llama3.2:3b
```

### Option B: OpenAI
```yaml
environment:
  - SHELFD_AI_PROVIDER=openai
  - SHELFD_AI_BASE_URL=https://api.openai.com/v1
  - SHELFD_AI_API_KEY=sk-your-openai-api-key
  - SHELFD_AI_EMBEDDING_MODEL=text-embedding-3-small
  - SHELFD_AI_EMBEDDING_DIMENSIONS=1536
  - SHELFD_AI_SUMMARY_MODEL=gpt-4o-mini
```

---

## Model Context Protocol (MCP) Setup

Shelfd embeds a standards-compliant JSON-RPC 2.0 MCP server over HTTP/SSE. AI coding agents and assistants can search your library, inspect metadata, and read chapter content directly.

### Endpoints
* **SSE Handshake**: `GET http://<host>:8080/mcp/sse`
* **JSON-RPC Messages**: `POST http://<host>:8080/mcp/messages?sessionId=<id>`

### Connecting Claude Desktop
Add the following to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "shelfd": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-sse",
        "http://localhost:8080/mcp/sse",
        "--headers",
        "Authorization: Bearer shelfd_your_api_token_here"
      ]
    }
  }
}
```

### Available MCP Tools
* `search_library`: Hybrid semantic vector and keyword search across your catalog.
* `get_book_metadata`: Retrieves detailed metadata, taxonomy, and chapter listings for a book.
* `read_chapter_content`: Paginated chapter reading with configurable windowing to protect context window limits.

---

## Asynchronous Book Upload Queue

To protect homelab disk I/O and prevent HTTP timeouts on slow connections or large EPUB files, book uploads are processed through a persistent background queue.

### 1. Upload EPUB (`POST /api/v1/books/upload`)
Upload an EPUB file via multipart form. The server immediately stages the file, validates the archive header, enqueues the job, and returns HTTP **`202 Accepted`**:

```bash
curl -X POST http://localhost:8080/api/v1/books/upload \
  -H "Authorization: Bearer <token>" \
  -F "file=@dune.epub"
```

Response:
```json
{
  "job_id": "c1f7b02d-4bf5-4b13-bbba-cba063833bcf",
  "status": "queued",
  "filename": "dune.epub",
  "message": "Upload enqueued for processing",
  "created_at": "2026-09-07T13:55:00Z"
}
```

### 2. Poll Job Status (`GET /api/v1/books/upload/jobs/{id}`)
Check background ingestion progress:

```bash
curl http://localhost:8080/api/v1/books/upload/jobs/c1f7b02d-4bf5-4b13-bbba-cba063833bcf \
  -H "Authorization: Bearer <token>"
```

Response upon completion:
```json
{
  "id": "c1f7b02d-4bf5-4b13-bbba-cba063833bcf",
  "filename": "dune.epub",
  "status": "completed",
  "book_id": "8fa24056-b072-466d-886d-fb0df9f086b9",
  "created_at": "2026-09-07T13:55:00Z",
  "updated_at": "2026-09-07T13:55:03Z"
}
```

### 3. List Recent Jobs (`GET /api/v1/books/upload/jobs`)
List recent upload queue tasks:

```bash
curl "http://localhost:8080/api/v1/books/upload/jobs?limit=10" \
  -H "Authorization: Bearer <token>"
```

---

## Client Application (`Shelf`)

The client application is located in `apps/app` and built with Flutter:
* **Editorial Typography**: Customizable font sizes, line heights, margins, and serif/sans typeface selection.
* **Comfort Themes**: Warm Editorial Bone, Warm Sepia, and Pure Dark (OLED).
* **Instant Connect**: Scan the QR code from the Shelfd web interface or enter your server URL and API credentials manually.

To run the client in development:
```bash
pnpm --filter @shelfd/app dev
```

---

## Environment Variables Reference

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PUID` | `1000` | Host user ID for file ownership |
| `PGID` | `1000` | Host group ID for file ownership |
| `UMASK` | `002` | File creation permission mask |
| `TZ` | `UTC` | Container timezone |
| `SHELFD_CONFIG` | `""` | Path to custom `config.yaml` |
| `SHELFD_SERVER_HOST` | `0.0.0.0` | Listen host |
| `SHELFD_SERVER_PORT` | `8080` | Listen port |
| `SHELFD_JWT_SECRET` | `""` | Secret for signing JWTs (auto-generated if empty) |
| `SHELFD_STORAGE_LIBRARY_DIR` | `/library` | Shared Audiobookshelf library directory |
| `SHELFD_STORAGE_DATA_DIR` | `/data` | Shelfd SQLite and cache directory |
| `SHELFD_AI_PROVIDER` | `ollama` | AI provider (`ollama` or `openai`) |
| `SHELFD_AI_BASE_URL` | `http://host.docker.internal:11434` | Endpoint URL for AI provider |
| `SHELFD_AI_API_KEY` | `""` | API key (required for OpenAI) |
| `SHELFD_AI_EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model for semantic search |
| `SHELFD_AI_EMBEDDING_DIMENSIONS` | `1536` | Vector dimensions for sqlite-vec |
| `SHELFD_AI_SUMMARY_MODEL` | `llama3.2:3b` | Summarization model for chapter ingestion |
| `SHELFD_MCP_ENABLED` | `true` | Enable or disable MCP endpoints |
| `SHELFD_MCP_PATH` | `/mcp` | Base path for MCP routes |

---

## Monorepo & Development

The repository is organized as a Turborepo monorepo using `pnpm`:

```bash
# Build all packages (daemon binary and Flutter app)
pnpm build

# Run unit tests across all packages
pnpm test

# Run linters (go vet and flutter analyze)
pnpm lint

# Clean build artifacts
pnpm clean
```

---

## Documentation

* [Product Brief](docs/PRODUCT_BRIEF.md)
* [Technical Design Document](docs/TECHNICAL_DESIGN_DOCUMENT.md)
* [Macro Implementation Plan](docs/MACRO_PLAN.md)
* [Architecture Decision Records (ADRs)](docs/decisions/index.md)
  * [ADR-0010: Single-Container Docker & Homelab Deployment](docs/decisions/0010-single-container-docker-and-homelab-deployment.md)
  * [ADR-0011: Asynchronous Upload Processing Queue](docs/decisions/0011-asynchronous-upload-processing-queue.md)
* [Antigravity Personas & Workflow](.agents/AGENTS.md)
