# Incremental Library Scanning and Change Detection

* Status: accepted
* Deciders: Lead Systems Architect, @core, @librarian
* Date: 2026-09-09

## Context and Problem Statement

When users trigger a library re-scan (`POST /api/v1/library/scan` or `shelfd --scan`), the scanner crawls `/library` to detect books. Previously, the ingestion pipeline unconditionally called `ReparseBookChapters` for every discovered EPUB that already existed in the catalog.

This unconditional re-parsing had severe operational consequences:
1. `ReparseBookChapters` deleted all paragraphs for the book (`DeleteParagraphsByBookID`).
2. SQLite database triggers (`trg_delete_paragraph_vec`) cascade-deleted all vector embeddings from `vec_paragraphs` whenever paragraphs were removed.
3. The ingester re-chunked the chapter text and inserted new paragraphs with new unique identifiers.
4. Because the newly inserted paragraphs lacked vector entries, the background indexing worker (`worker.Worker`) was triggered, re-generating embeddings for every paragraph of every book in the library via the AI embedding model on every scan.

For libraries of any significant size, this caused excessive disk I/O, consumed unnecessary API tokens/rate limits with external embedding providers (or CPU/GPU on Ollama), and generated massive redundant write activity.

How can Shelfd incrementally scan the library to import only new or modified books without invalidating or re-creating vector embeddings for unchanged books?

## Decision Drivers

* **Sanctity of Existing Embeddings**: Vector embeddings for unchanged books must remain permanent and untouched across re-scans.
* **Efficient Change Detection**: Determining whether a file on disk has changed must be near-instantaneous (metadata stat check), without requiring full EPUB extraction or hashing.
* **Audiobookshelf Coexistence**: Preserve sidecar file sanctity and avoid file locks or mutations in shared directories.
* **Backward Compatibility**: Seamlessly handle existing libraries where books were cataloged before file modification tracking was added.
* **Dual Database Support**: Maintain schema parity across SQLite (`migrations/`) and PostgreSQL (`migrations_pg/`) with Bun ORM.

## Considered Options

* **Incremental Scanning with Timestamp & Size Heuristics (Chosen)**: Store `file_size_bytes` and `file_modified_at` in the `books` table. On scan, compare disk `os.Stat` against stored values at second-level resolution. Skip unchanged files completely; only import new files or update modified files.
* **Content Hashing (SHA-256) on Every Scan**: Read and hash every EPUB file during library crawls. (Rejected: Heavy disk I/O bottlenecks for multi-gigabyte libraries).
* **Unconditional Reparsing with Vector Preservation**: Reparse chapters and compare paragraph text before deciding to delete embeddings. (Rejected: Requires unzipping and parsing every EPUB on every scan, which is slow and wastes CPU).

## Decision Outcome

Chosen option: **Incremental Scanning with Timestamp & Size Heuristics**.

### 1. Database Schema & Indexing
Added migrations:
* SQLite: `internal/database/migrations/0007_book_file_modified_at.sql`
* PostgreSQL: `internal/database/migrations_pg/0004_book_file_modified_at.sql`

Changes:
* Added `file_modified_at` column (`DATETIME` in SQLite, `TIMESTAMP WITH TIME ZONE` in PostgreSQL) to `books`.
* Added index `idx_books_file_path` on `books(file_path)` for $O(1)$ file lookup during scans.
* Updated `repository.Book` model with `FileModifiedAt *time.Time`.
* Updated `SQLiteStorageEngine` and `BunStorageEngine` CRUD operations.

### 2. Scanner Discovery Optimization
* Enhanced `scanner.DiscoveredFile` with `ModTime time.Time` and `SizeBytes int64`, populated directly from `fs.DirEntry.Info()` during `filepath.WalkDir` without additional syscalls.

### 3. Change Detection & Sync Lifecycle (`scanner.Ingester`)
* Defined `SyncStatus` enum: `SyncStatusUnchanged`, `SyncStatusNew`, `SyncStatusModified`.
* Implemented `SyncFile(ctx, fullPath, relativePath)`:
  * If file is not in database: imports as `SyncStatusNew`, recording initial size and modification time.
  * If file exists in database:
    * Compares `FileSizeBytes` and second-truncated `FileModifiedAt.Unix() == fi.ModTime().Unix()`.
    * If unchanged: returns existing book as `SyncStatusUnchanged` immediately. No EPUB opening, no chapter parsing, no paragraph deletion, no vector invalidation.
    * If legacy record (`FileModifiedAt == nil`) but size matches: backfills `FileModifiedAt` without reparsing or invalidating vectors.
    * If modified: updates book metadata, cover, and reparses chapters/paragraphs as `SyncStatusModified`.
* Retained `IngestFile` signature wrapping `SyncFile` for full backward compatibility across all existing callers.
* Consolidated chapter/paragraph parsing into `reparseChaptersFromReader`.

### 4. Selective Indexing Trigger
* `LibraryHandler.Scan` and `cmd/server/main.go` track counts of new, modified, and unchanged books.
* The background worker `worker.Trigger()` is only invoked if `new > 0 || modified > 0 || backfilled > 0`.
* Because unchanged books retain their paragraph records and `vec_paragraphs` entries, the worker's unindexed paragraph query (`LEFT JOIN vec_paragraphs WHERE paragraph_id IS NULL`) returns only paragraphs belonging to newly added or modified books.

## Consequences

### Positive Consequences

* **Zero Unnecessary Embedding Generations**: Re-scanning an unchanged library performs zero vector deletes and calls zero AI embedding APIs.
* **Instant Re-scans**: Checking thousands of files takes milliseconds via indexed path lookups and filesystem stat checks without unzipping EPUB archives.
* **Safe In-Place Updates**: When a book is legitimately edited or replaced on disk, its metadata and paragraphs are automatically updated while all other books remain intact.
* **Automatic Legacy Migration**: Existing books imported prior to this change have their modification timestamps seamlessly backfilled without disrupting vector indexes.
