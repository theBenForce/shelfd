# Shelfd: Product Brief

## 1. Executive Summary
**Shelfd** is a lightweight, self-hosted, AI-native ebook server and reader. Built to coexist alongside Audiobookshelf on existing home media libraries, Shelfd is designed from the ground up for text-first reading, semantic search, and autonomous AI agent interaction via the Model Context Protocol (MCP).

## 2. Problem Statement
* **Audiobookshelf** is best-in-class for audiobooks, but treating ebooks as an afterthought results in a suboptimal reading and navigation experience.
* **Calibre-web / Kavita** are heavy, rigid, or rely on legacy database models without modern vector or agentic interfaces.
* Modern readers lack the ability to query their book collections semantically (e.g., *"Which of my sci-fi books discuss Dyson spheres?"* or *"Find arguments regarding currency inflation in chapter summaries"*).
* AI agents currently cannot easily read or cross-reference private ebook libraries via standard protocols.

## 3. Target Audience
* **Self-Hosters & Home-Labbers**: Users with existing book libraries managed via Audiobookshelf or Calibre who want a modern, low-resource daemon.
* **Families**: Private, self-hosted access with clean user authentication.
* **AI & Agent Developers**: Users running tools like Claude Desktop, Cursor, or autonomous agents who want their agent to consult their book collection via MCP.

## 4. Core MVP Scope (Phase 1)
* **Single Container by Default**: Go daemon with embedded SQLite and `sqlite-vec` for relational + vector storage.
* **Audiobookshelf Coexistence**: Mounts `/library` directly from the host, reading existing EPUBs and writing new uploads to `<Author>/<Title>/<Title>.epub` without touching ABS metadata.
* **EPUB Ingestion**: Extracts Dublin Core metadata, cover images, chapter structures, and series indexes.
* **Normalized Relational Library**: Filterable authors, genres, and series with sequence numbers.
* **Semantic Vector Search**: Chapter-level summaries generated via external LLMs (Ollama or OpenAI-compatible) and embedded via `sqlite-vec`.
* **Model Context Protocol (MCP) Server**: HTTP/SSE transport exposing `search_library`, `get_book_metadata`, and `read_chapter_content`.
* **Flutter Client (Shelf)**: Clean cross-platform UI for browsing, uploading, and reading.
* **Turborepo Monorepo**: All lifecycle scripts managed via pnpm and Turbo.

## 5. Non-Goals (Deferred to Post-MVP)
* **Audiobooks**: Handled by Audiobookshelf. Shelfd focuses exclusively on ebooks.
* **Full-text Sliding-Window Chunking**: Deferred to Phase 2 (evaluating LanceDB vs sqlite-vec at scale).
* **Cross-device Reading State Sync & Annotations**: Deferred to Phase 2.
* **Multi-tenant Social Features & Public Registration**: Private family self-hosting only.
