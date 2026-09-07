# Shelfd: MVP Implementation Macro Plan

This plan establishes the sequence of milestones, engineering tasks, verification gates, and responsible personas for implementing the **Shelfd** MVP.

---

## Architecture Milestones Overview

```mermaid
gantt
    title Shelfd MVP Implementation Sequence
    dateFormat  X
    axisFormat %d
    section Backend Core
    M1: Config, DB & Storage Engine   :active, m1, 0, 3
    M2: EPUB Parser & Ingestion       :m2, after m1, 4
    M3: Semantic Ingestion Pipeline   :m3, after m2, 4
    section Protocols & APIs
    M4: Model Context Protocol (MCP)  :m4, after m3, 3
    M5: REST API & User Authentication:m5, after m4, 3
    section Client & Delivery
    M6: Flutter Reader Client (Shelf) :m6, after m5, 5
    M7: Dockerization & Homelab Release:m7, after m6, 2
```

---

## Milestone 1: Core Scaffolding, Configuration & Storage Engine
* **Responsible Personas**: `@core`, `@homelab`, `@security`
* **Objective**: Establish the Go backend architecture, parse configuration, and instantiate the SQLite + `sqlite-vec` storage layer.

### Tasks
1. **Configuration Loader (`internal/config`)**:
   * Implement YAML parsing for `config.yaml` with environment variable substitution.
   * Validate port, directory paths, and AI provider configurations.
2. **Database Engine & Migration Runner (`internal/database`)**:
   * Setup embedded SQLite driver with Cgo enabling `sqlite-vec`.
   * Create schema migrations for:
     * `users`, `api_tokens`
     * `authors`, `genres`, `series`
     * `books`, `book_authors`, `book_genres`, `book_series`
     * `chapters`, `vec_chapters` (vec0 virtual table)
   * Enforce `PRAGMA foreign_keys = ON;` and verify cascade deletion across junction tables.
3. **Storage Engine Repository Layer (`internal/repository`)**:
   * Implement the `StorageEngine` interface for all relational CRUD operations.
   * Implement upsert helpers for authors, genres, and series with sequence numbers.

### Verification Gate
* `go test ./internal/...` passes.
* Migration creates `/data/sqlite.db`. Inserting a book with authors, chapters, and vectors, followed by deleting the book, completely cleans up all linked tables.

---

## Milestone 2: EPUB Parser & Audiobookshelf Ingestion Engine
* **Responsible Personas**: `@librarian`, `@security`, `@core`
* **Objective**: Build a resilient, non-destructive EPUB unpacking and cataloging engine that seamlessly shares the `/library` directory with Audiobookshelf.

### Tasks
1. **Safe EPUB Container Unpacker (`internal/epub`)**:
   * Implement zip extraction with strict Zip Slip path traversal guards (`@security`).
   * Parse `META-INF/container.xml` to find the root `.opf` package file.
2. **Metadata & Series Extraction (`internal/epub/metadata`)**:
   * Extract Dublin Core fields: title, authors (`dc:creator`), genres (`dc:subject`), publisher, synopsis.
   * Extract series metadata with priority cascade:
     * Primary: EPUB 3 `belongs-to-collection` + `group-position`.
     * Fallback: Calibre `calibre:series` + `calibre:series_index`.
3. **Cover & Chapter Text Processing (`internal/epub/content`)**:
   * Extract cover image to `/data/covers/{book_id}.jpg`.
   * Parse spine items in order, strip XML/HTML tags, and extract clean plaintext per chapter while preserving paragraph breaks (`\n\n`).
4. **Audiobookshelf-Compatible Ingestion & Scanner (`internal/scanner`)**:
   * Implement background crawler for `/library`.
   * Implement upload writer targeting strictly:
     ```text
     /library/<Author>/<Title>/<Title>.epub
     ```
   * Ensure zero sidecar mutations (never touch `.metadata.json`, `desc.txt`, or audio files).

### Verification Gate
* Ingest a batch of test EPUBs (EPUB 2 and EPUB 3).
* Verify authors, genres, and series sequence numbers are accurately stored in SQLite.
* Verify existing Audiobookshelf sidecar files in `/library` are untouched.

---

## Milestone 3: Semantic Ingestion Pipeline (`sqlite-vec`)
* **Responsible Personas**: `@mcp`, `@core`
* **Objective**: Connect external AI models to summarize chapters, compute vector embeddings, and enable hybrid semantic search.

### Tasks
1. **Universal AI Client (`internal/ai`)**:
   * Implement an HTTP client supporting both Ollama (`/api/embeddings`, `/api/generate`) and OpenAI-compatible endpoints (`/v1/embeddings`, `/v1/chat/completions`).
   * Add SSRF guards on `ai.base_url`.
2. **Asynchronous Chapter Summarizer Queue (`internal/worker`)**:
   * Implement a non-blocking worker pool to process newly ingested books.
   * Send chapter plaintext to the configured LLM with the standardized 2–4 sentence summary prompt.
   * Save chapter summary and plaintext content into the `chapters` table.
3. **Vector Embedding & Storage (`internal/vector`)**:
   * Generate vector embeddings for each chapter summary.
   * Insert embeddings into the `vec_chapters` virtual table linked to `chapter.id`.
4. **Hybrid Search Query Builder (`internal/repository`)**:
   * Build combined vector distance + metadata SQL queries supporting optional author, genre, and series filters.

### Verification Gate
* Run natural language queries against ingested books (e.g., *"How do the characters escape the station?"*).
* Verify relevant chapter summaries are returned with distance scores under 0.70.

---

## Milestone 4: Model Context Protocol (MCP) Server
* **Responsible Personas**: `@mcp`, `@security`
* **Objective**: Expose the server to external AI agents (Claude Desktop, Cursor) over HTTP/SSE.

### Tasks
1. **HTTP/SSE Transport Engine (`internal/mcp`)**:
   * Implement `GET /mcp/sse` declaring the JSON-RPC message endpoint.
   * Implement `POST /mcp/messages` handling client requests.
   * Add Bearer API token authentication middleware validating against SHA-256 hashed tokens in `api_tokens`.
2. **Exposed Tool Handlers**:
   * `search_library`: Vector search returning book title, author, chapter title, sequence, score, and summary (< 100 tokens/hit).
   * `get_book_metadata`: Retrieves complete book metadata, TOC, and all chapter summaries.
   * `read_chapter_content`: Returns plain text for a specified chapter with `start_paragraph` and `end_paragraph` windowing.
3. **Protocol Auditing & Testing**:
   * Verify schema definitions against the official MCP JSON-RPC 2.0 specification.

### Verification Gate
* Connect the official `@modelcontextprotocol/inspector` to `http://localhost:8080/mcp/sse`.
* Execute all three tools through the inspector and verify responses and error codes.

---

## Milestone 5: REST API & User Authentication
* **Responsible Personas**: `@core`, `@security`
* **Objective**: Build the HTTP REST API powering the Flutter client.

### Tasks
1. **Authentication Endpoints**:
   * `POST /api/v1/auth/login`: Validates credentials (bcrypt/argon2id) and issues short-lived JWTs.
   * `POST /api/v1/auth/tokens`: Generates and manages persistent Bearer API tokens for MCP.
2. **Library Browsing & Filtering Endpoints**:
   * `GET /api/v1/books`: Paginated list of books with query parameters for `author_id`, `genre_id`, `series_id`, and title search.
   * `GET /api/v1/books/{id}`: Detailed book view with chapter TOC.
   * `GET /api/v1/books/{id}/cover`: Streams cached cover images.
   * `GET /api/v1/books/{id}/chapters/{index}`: Fetches chapter content for reading.
3. **Upload & Ingestion Endpoints**:
   * `POST /api/v1/books/upload`: Multipart file upload with atomic write to `/library`.
   * `POST /api/v1/library/scan`: Triggers background library rescan.

### Verification Gate
* End-to-end integration test covering login -> upload -> scan -> fetch -> read.

---

## Milestone 6: Flutter Client (`Shelf`) MVP
* **Responsible Personas**: `@reader`, `@homelab`
* **Objective**: Deliver the cross-platform reader application running on mobile and desktop.

### Tasks
1. **App Shell & State Scaffolding (`apps/app`)**:
   * Set up Riverpod state management and GoRouter navigation.
   * Implement light, dark, and sepia reading themes.
2. **Server Connection & Authentication**:
   * Server URL input and login screen with secure token storage.
3. **Library & Discovery Views**:
   * Grid view displaying cover art, book titles, and authors.
   * Series view grouping books in sequence order.
   * Semantic search screen with NLP query bar and chapter hit cards.
4. **Typography & Reader View**:
   * Clean, distraction-free reading canvas with customizable margins, font sizes, line height, and font family selection (Serif/Sans).
   * Chapter navigation drawer and local offline caching.

### Verification Gate
* Build and launch the application on macOS and mobile simulators (`pnpm --filter @shelfd/app build`).
* Browse library, perform a semantic search, and read a book smoothly.

---

## Milestone 7: Dockerization & Homelab Packaging
* **Responsible Personas**: `@homelab`, `@security`
* **Objective**: Deliver the single-container production deployment package for self-hosters.

### Tasks
1. **Multi-Stage Dockerfile**:
   * Builder stage compiling the Go binary with Cgo and `sqlite-vec`.
   * Minimal runtime stage (Alpine or Debian-slim) keeping the image < 50MB.
2. **Docker Compose & Volume Mapping**:
   * Validate default `docker-compose.yml` with `./data:/data` and `/path/to/ebooks:/library:rw`.
   * Implement PUID/PGID container user mapping to avoid permission issues.
3. **Documentation & Release Artifacts**:
   * Finalize `README.md`, setup guides, and sample `config.yaml`.

### Verification Gate
* Run `docker compose up -d` on a clean host with an existing Audiobookshelf library.
* Verify zero permission errors, full library scan, and working MCP endpoints.
