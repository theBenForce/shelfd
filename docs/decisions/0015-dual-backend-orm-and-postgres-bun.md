# Dual Database Backend (SQLite & PostgreSQL) via Bun ORM with pgvector

* Status: accepted
* Deciders: Lead Systems Architect, @core, @librarian, @mcp, @reader, @homelab
* Date: 2026-09-08

## Context and Problem Statement

Shelfd originally used embedded SQLite with `sqlite-vec` (ADR-0002) as its sole storage and vector retrieval engine. For standard personal libraries (< 100 books), this provided a zero-friction, single-binary homelab deployment.

However, as personal libraries scale to 1,000+ books (predominantly nonfiction and technical texts comprising 400,000 to 500,000+ paragraphs and 15+ GB of raw EPUB data), embedded SQLite reaches practical operational limits:
1. **Vector Index Scalability**: `sqlite-vec` performs flat SIMD scans over brute-force embeddings. At 500,000 vectors, brute-force scans degrade retrieval latency and consume significant host memory during RAG queries.
2. **Database File Size & Concurrency**: With 500,000 embedded text chunks, relational metadata, and FTS5 indexes, a single SQLite database file approaches 2–3 GB, increasing file contention between the ingestion scanner and concurrent client reads.
3. **Homelab Flexibility**: Advanced homelab operators already run dedicated, high-availability PostgreSQL clusters and prefer leveraging existing infrastructure, backups, and `pgvector` HNSW indexing.

How can Shelfd support 1,000+ book libraries with sub-second vector search on PostgreSQL while maintaining its single-container, zero-dependency SQLite architecture for smaller setups?

## Decision Drivers

* **High-Scale Headroom**: Support libraries of 1,000+ books (500,000+ paragraphs) with sub-10ms approximate nearest neighbor (ANN) vector search via HNSW indexes and native tsvector lexical search.
* **Preserve Zero-Friction SQLite Default**: The default homelab deployment must remain a single Docker container or standalone binary requiring zero external services.
* **Clean Architectural Abstraction (DRY & KISS)**: Avoid maintaining two disconnected raw SQL repository layers. Unify relational CRUD under a lightweight, idiomatic Go data mapper.
* **Low Daemon Footprint**: Ensure the ORM layer does not blow past Shelfd's < 50MB idle RAM ceiling or introduce heavy runtime reflection overhead.

## Considered Options

* **Bun ORM with Dialect-Aware Storage Engine (Chosen)**: Use `uptrace/bun` to unify model mappings and relational queries across SQLite and PostgreSQL, with dialect-switched vector and FTS search routines.
* **Dual Raw SQL Driver Packages**: Maintain two separate repository implementations (`sqlite.go` and `postgres.go`) duplicating all book, author, spine, bookmark, and highlight queries.
* **GORM**: Heavyweight Go ORM with widespread driver support, but substantial memory overhead, complex hook lifecycles, and opaque query generation.
* **PostgreSQL Only**: Drop SQLite completely and mandate PostgreSQL with `pgvector` for all users.

## Decision Outcome

Chosen option: **Bun ORM with Dialect-Aware Storage Engine**.

1. **Lightweight ORM (`uptrace/bun`)**:
   * Bun is a SQL-first Golang client for PostgreSQL, SQLite, and MySQL that wraps `database/sql` with zero-allocation query building and clean struct mapping.
   * Preserves full visibility and control over generated SQL while eliminating repetitive CRUD boilerplate across both dialects.
2. **Unified `BunStorageEngine`**:
   * Implements the existing domain [`StorageEngine`](file:///Users/bforce/repos/shelved/apps/server/internal/repository/storage.go) interface.
   * Common relational operations (Books, Authors, Genres, Series, Spines, Chapters, Bookmarks, Highlights, Users, Upload Jobs) run identically on both SQLite and PostgreSQL.
   * Vector KNN retrieval uses dialect-specific branches:
     * **PostgreSQL**: Cosine distance operator (`<=>`) powered by an HNSW vector index (`vector_cosine_ops`) for sub-10ms lookups across 500k+ rows.
     * **SQLite**: Flat `vec_distance_cosine` scans on `vec_paragraphs` using SIMD instructions.
   * Lexical search uses dialect-specific branches:
     * **PostgreSQL**: Native `tsvector` with `plainto_tsquery('english', ?)` and GIN indexing.
     * **SQLite**: `paragraphs_fts MATCH ?` with BM25 ranking.
3. **Dialect-Specific Migrations**:
   * SQLite migrations continue to run from `apps/server/internal/database/migrations/`.
   * PostgreSQL migrations run from `apps/server/internal/database/migrations_pg/`, automatically configuring `CREATE EXTENSION IF NOT EXISTS vector` and HNSW/GIN indexes.
4. **Configuration & Docker Compose**:
   * Enabled via `SHELFD_DATABASE_POSTGRES_DSN` or `database.postgres_dsn` in `config.yaml`.
   * When unset, Shelfd defaults seamlessly to `/data/sqlite.db`.
   * Added `postgres` service (`pgvector/pgvector:pg16`) to `docker-compose.yml` for multi-container deployments.

## Consequences

### Positive Consequences

* **1,000+ Book Scale**: Postgres + `pgvector` HNSW index handles 500k+ paragraphs with instant ANN retrieval, eliminating the SQLite flat-scan ceiling.
* **Single Repository Implementation**: >90% of database logic is shared via Bun, cutting maintenance surface area and preventing cross-dialect drift.
* **Backward Compatible**: Existing single-container homelab users require no configuration changes and continue using SQLite seamlessly.
* **Zero Performance Hit for Small Installs**: Bun's lean architecture avoids GORM's reflection and memory bloat, keeping idle RAM well under 50MB.

### Negative Consequences

* **Two Migration Directories**: Migration DDL must be authored for both SQLite and PostgreSQL when schema changes occur.
* **PostgreSQL Container Dependency**: Scaling beyond SQLite requires managing a secondary container or connecting to an external database.
