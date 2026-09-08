# Book Details Page & In-Book RAG Chat

* Status: accepted
* Deciders: Lead Systems Architect, @core, @designer, @mcp, @reader
* Date: 2026-09-08

## Context and Problem Statement

Previously, clicking on a book in the library (`/library`) navigated directly to the reader view (`/reader/:bookId`), displaying chapter text immediately without giving the user access to book metadata, synopsis, table of contents, personal highlights/bookmarks, or an interactive way to explore the book's content.

Users requested that navigating to a URL such as `http://localhost:8080/#/reader/:bookId` displays a **Book Details** page containing:
1. Book metadata (cover, title, author, series, publisher, published date, language, chapters count, reading progress).
2. Saved personal annotations (highlights and bookmarks).
3. An AI Chat feature using the configured LLM API (Ollama or OpenAI, previously used for chapter summaries) that utilizes Retrieval-Augmented Generation (RAG) to look up and cite relevant book chapters.

In addition, the user requested that Stitch MCP be used to prototype mobile and desktop designs, adhering to Warm Editorial Bone typography and responsive bento layouts.

## Decision Drivers

* **User Experience & Discoverability**: Allow readers to review book synopsis, table of contents, and annotations before jumping into reading.
* **Retrieval-Augmented Generation (RAG)**: Ground AI answers strictly in chapter summaries and vector embeddings scoped to the selected book, providing accurate chapter citations.
* **Responsive Editorial Design**: Adhere to Stitch-designed mobile-first responsive prototypes (Warm Editorial Bone palette, serif titles, sans body, 12-column desktop bento, touch targets >= 48px).
* **Storage Decoupling & Homelab Invariants**: Never mutate Audiobookshelf sidecar files; store bookmarks and highlights strictly in `/data/sqlite.db` with cascading foreign keys to `books(id)`.
* **DRY, KISS, and TDD**: Reuse existing AI configuration, storage engine abstractions, and Riverpod patterns with comprehensive unit and widget tests.

## Decision Outcome

Chosen option: **Dedicated Book Details View with In-Book RAG Chat and Cascading Annotations Schema**.

### 1. Database Schema & Storage (`internal/database`, `internal/repository`)
* Added migration `0003_bookmarks_highlights.sql`:
  - `bookmarks` table (`id`, `book_id`, `chapter_id`, `title`, `progress`, `created_at`).
  - `highlights` table (`id`, `book_id`, `chapter_id`, `selected_text`, `note`, `color`, `created_at`).
  - Both tables enforce `FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE`.
* Added storage methods in `StorageEngine`:
  - `CreateBookmark`, `ListBookmarksByBookID`, `DeleteBookmark`.
  - `CreateHighlight`, `ListHighlightsByBookID`, `DeleteHighlight`.
  - Updated `SearchVectorChapters` with `BookID *string` filtering in `SearchFilter`.

### 2. Multi-Turn LLM Chat & RAG Retrieval (`internal/ai`, `internal/api`)
* Extended universal `ai.Client` with `Chat(ctx, messages []ChatMessage) (string, error)` implemented across both Ollama (`/api/chat`) and OpenAI (`/v1/chat/completions`).
* Added `POST /api/v1/books/{id}/chat` endpoint:
  - Generates query embedding via `ai.GenerateEmbedding(message)`.
  - Performs RAG vector search via `repo.SearchVectorChapters` scoped to `book_id` with limit 3.
  - Falls back to chapter summaries from `repo.GetChaptersByBookID` if vector search is unindexed.
  - Injects retrieved chapter context and recent chat history into a system prompt.
  - Returns `reply` and `citations` with chapter index, title, and summary.
* Added CRUD endpoints for bookmarks and highlights under `/api/v1/books/{id}/bookmarks` and `/api/v1/books/{id}/highlights` with direct DELETE routes `/api/v1/bookmarks/{id}` and `/api/v1/highlights/{id}`.

### 3. Flutter Client Architecture (`apps/app`)
* **Models**: Added `Bookmark`, `Highlight`, `BookCitation`, `BookChatMessage`, `BookChatResponse`. Extended `Book` to deserialize annotations and metadata.
* **Routing**:
  - `/reader/:bookId`: renders `BookDetailView(bookId: bookId)`.
  - `/reader/:bookId/read` and `/reader/:bookId/read/:chapterIdentifier`: renders `ReaderView`.
  - Backward compatibility preserves direct `/reader/:bookId/:chapterIdentifier`.
* **State Management**:
  - `bookDetailProvider(bookId)`: manages book details, adds/removes bookmarks and highlights.
  - `bookChatProvider(bookId)`: manages multi-turn conversation, loading states, and citations.
* **UI Design**:
  - Mobile: Single-column stack with Hero card, segmented tabs, and responsive lists.
  - Desktop: 12-column layout with 360px sticky metadata card on left and segmented tabs on right.
  - Reader integration: added quick bookmark action and back navigation returning to Book Details.

## Consequences

### Positive
* Users can view book details, resume reading, and review annotations before reading.
* In-book AI companion allows natural language questions grounded in the book with direct chapter citations.
* Monorepo linting, builds, and tests pass cleanly across Go and Flutter.
* All Stitch design specifications (Bone theme, typography contrast, responsive bento) are implemented faithfully.

### Negative / Trade-offs
* Navigating to `/reader/:bookId` now requires an additional click to "Resume Reading" to enter full reader mode, though a prominent CTA button is always in view.
