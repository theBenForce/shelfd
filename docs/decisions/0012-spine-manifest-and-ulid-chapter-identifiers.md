# Spine Manifest & ULID Chapter Identifiers

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team, @core, @reader, @mcp
* Date: 2026-09-08

## Context and Problem Statement

When opening a book in the Shelfd reader client, the application requested `GET /api/v1/books/{bookId}/chapters/0`, resulting in an HTTP 404 Not Found response. Investigation revealed a mismatch: the server's EPUB parser indexed chapters starting at 1 (`chapter_index >= 1`), whereas the client reader assumed 0-indexed chapters.

Beyond the immediate off-by-one discrepancy, relying on sequential integer indices (`/chapters/{index}`) for reading progress, table of contents, and deep links presented several systemic issues:
1. **Index Ambiguity**: Client-server discrepancies regarding 0-indexed vs. 1-indexed collections cause fragile contract assumptions and broken routes.
2. **Heavy Payload Overhead**: Fetching book details or chapter listings previously included full plain text content (`content_plain`) for every chapter, consuming megabytes of network bandwidth and memory for large books.
3. **Fragile Navigation**: Hardcoding integer loops forces readers to guess total chapter counts and lacks real chapter titles, word counts, and navigation metadata.
4. **AI & MCP Context Drift**: External AI agents reading chapters via Model Context Protocol (MCP) lacked canonical, immutable chapter IDs for quoting, referencing, and semantic citation.

How should chapters be identified, navigated, and represented in Shelfd to ensure stable addressing, fast initial loading, and clean client-server contracts?

## Decision Drivers

* **Canonical Addressing**: Stable, unique chapter identifiers that survive re-indexing and provide immutable deep links.
* **Lightweight Spine Manifest**: Fast metadata exchange allowing the client to render Table of Contents and calculate progress without downloading full book text.
* **Lexicographical Monotonicity**: Identifiers should maintain natural reading order and creation timestamp sorting.
* **Zero Database Migration Friction**: Must leverage existing SQLite schema (`chapters.id TEXT PRIMARY KEY`) without requiring destructive schema migrations.
* **Strict Backward Compatibility**: Existing clients and bookmark links requesting integer indices (including index 0) must resolve gracefully without 404 errors.
* **AI & MCP Protocol Integration**: Expose canonical chapter identifiers in MCP tools (`get_book_metadata`, `read_chapter_content`) with LOD token control.

## Considered Options

* **EPUB Spine Manifest with Monotonic ULID Chapter Identifiers and Dual Resolution** (Chosen)
* **Strict Sequential Zero-Indexed Integers (`0..N-1`)**
* **UUIDv4 Identifiers Without Spine Metadata**

## Decision Outcome

Chosen option: **EPUB Spine Manifest with Monotonic ULID Chapter Identifiers and Dual Resolution**.

1. **Monotonic ULID Generator**:
   - Implemented a dependency-free, thread-safe ULID generator (`internal/ulid`) producing Crockford Base32 26-character strings.
   - Combines a 48-bit millisecond timestamp with 80 bits of cryptographically secure entropy, guaranteeing monotonic ordering within the same millisecond.
2. **Lightweight Spine Manifest**:
   - Introduced `SpineItem` struct containing `id`, `chapter_index`, `title`, `word_count`, and `byte_size`.
   - Added `GetBookSpine(bookID)` storage query that omits heavy `content_plain` strings, reducing book detail payloads from megabytes to sub-5KB JSON.
   - `BookDetailResponse` includes `spine: []SpineItem` for instant Table of Contents rendering.
3. **Dual-Resolution Chapter Endpoints**:
   - `GET /api/v1/books/{bookId}/chapters/{chapter}` accepts either:
     - A 26-character ULID or 36-character UUID string (looked up directly via `GetChapter`).
     - A positive 1-indexed sequential integer (looked up via `GetChapterByBookAndIndex`).
     - Integer `0`: gracefully resolves to `spine[0]` to prevent 404 regressions for legacy clients.
   - Added direct chapter endpoint `GET /api/v1/chapters/{id}` for direct canonical access.
4. **MCP Protocol Alignment**:
   - Updated MCP tools: `get_book_metadata` returns chapter ULIDs and indices; `read_chapter_content` accepts optional `chapter_id` in addition to `chapter_index`.
5. **Client Architecture (`apps/app`)**:
   - Flutter reader fetches book detail with spine manifest to build dynamic Table of Contents with actual chapter titles.
   - Reader routes support `/reader/:bookId` (defaults to first chapter in spine) and `/reader/:bookId/:chapterIdentifier`.
   - Prev/Next buttons respect spine bounds (`canPrev`, `canNext`) with safety guards.

### Positive Consequences

* **Eliminates Off-by-One Failures**: Resolves the root cause of the 404 bug while permanently removing index-base ambiguity.
* **Instant Reader Cold Starts**: Book detail response omits full chapter plain text, speeding up initial reader open times.
* **Canonical Deep Linking**: Users and external AI agents can link directly to specific chapters using `/api/v1/chapters/{id}`.
* **Zero DDL Migration**: Existing SQLite databases are fully compatible without running schema alterations.
* **Backward Compatible**: Existing MCP callers and external integrations using integer chapter indices continue working seamlessly.

### Negative Consequences

* Chapter resolution handler must perform lightweight format inspection (integer vs. ULID/UUID) on `{chapter}` parameter.
* Client must query the book spine or detail to obtain canonical IDs for table of contents rendering.
