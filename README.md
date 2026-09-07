# Shelfd

A lightweight, self-hosted, AI-native ebook server and reader designed to run seamlessly alongside your existing [Audiobookshelf](https://www.audiobookshelf.org/) collection.

```text
Host System:
  /mnt/storage/ebooks/         <-- Shared with Audiobookshelf
  /mnt/storage/shelfd/data/    <-- Isolated database (sqlite.db) & covers

Container Mounts:
  - /mnt/storage/ebooks:/library:rw      # Shared library storage
  - /mnt/storage/shelfd/data:/data:rw   # Internal state, index, covers
  - ./config.yaml:/config.yaml:ro
```

## Key Features
* **Zero-Interference Coexistence**: Mounts your existing Audiobookshelf library directly (`/library`), scans EPUBs non-destructively, and writes new uploads to `<Author>/<Title>/<Title>.epub` so Audiobookshelf automatically picks them up.
* **Semantic Search via Vectors**: Generates chapter-level summaries using local or cloud AI models (Ollama / OpenAI) and indexes them using `sqlite-vec`.
* **Model Context Protocol (MCP)**: Native HTTP/SSE MCP server allows AI agents (Cursor, Claude Desktop, autonomous agents) to search your library, read chapters, and synthesize across books.
* **Normalized Metadata**: Filterable authors, genres, and series with sequence numbers extracted directly from Dublin Core and EPUB 3 / Calibre collections.
* **Single-Container Deployment**: Built in Go with embedded SQLite and `sqlite-vec`, packaging everything into a single lightweight Docker container.
* **Modern Cross-Platform Client**: Flutter-based reading app (`Shelf`) for iOS, Android, and macOS/Desktop.

## Quick Start with Docker Compose

```yaml
services:
  shelfd:
    image: ghcr.io/your-org/shelfd:latest
    container_name: shelfd
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
      - /path/to/my/ebooks:/library:rw
      - ./config.yaml:/config.yaml:ro
    environment:
      - CONFIG_PATH=/config.yaml
```

## Monorepo & Development

The repository is organized as a Turborepo monorepo using `pnpm`:

```bash
# Build all packages (caches the Go daemon binary)
pnpm build

# Run local development servers
pnpm dev

# Run unit tests
pnpm test

# Run linters
pnpm lint
```

## Documentation
* [Product Brief](docs/PRODUCT_BRIEF.md)
* [Technical Design Document](docs/TECHNICAL_DESIGN_DOCUMENT.md)
* [Macro Implementation Plan](docs/MACRO_PLAN.md)
* [Architecture Decision Records (ADRs)](docs/decisions/index.md)
* [Antigravity Personas & Workflow](.agents/agents.md)
