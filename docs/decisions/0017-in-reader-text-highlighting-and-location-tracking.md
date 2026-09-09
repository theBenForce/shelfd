# In-Reader Text Highlighting & Location Tracking

* Status: accepted
* Deciders: Lead Systems Architect, @core, @reader, @librarian
* Date: 2026-09-09

## Context and Problem Statement

Previously, Shelfd provided a manual "Add Note / Highlight" button in the book overview app bar actions dialog, requiring users to manually copy/paste quoted text and manually associate it with a book. This broke normal e-reading flow, where users select text directly on the reading canvas and choose a highlight color or attach a note.

Furthermore:
1. Highlights lacked exact character offset and paragraph location tracking, preventing highlights from reliably rendering in-place across devices or after re-opening a book.
2. Cross-device reading required highlights saved on one device to appear immediately when opening the book on another device.
3. Users frequently highlight across paragraph boundaries (e.g., from the end of one paragraph to the start of the next); this must be preserved as a single unified highlight rather than fragmented records.
4. Highlights were stored with arbitrary color names rather than a standard, aesthetically calibrated e-reader palette.

## Decision Drivers

* **Direct Manipulation Reading Ergonomics**: Highlight creation must occur naturally when selecting text on the reading canvas, matching Amazon Kindle behavior.
* **Amazon Kindle Color Standard**: Standardize on Amazon Kindle's 4 distinct highlight colors: Yellow (`#FDE047`), Blue (`#93C5FD`), Pink (`#F472B6`), and Orange (`#FB923C`), with light, sepia, and dark OLED contrast resolution.
* **Cross-Device Sync & Offline Resilience**: Highlights created on one device must sync to the server and appear on other devices upon opening the book, while being cached locally in SQLite for offline access.
* **Multi-Paragraph Unification**: Highlighting across multiple paragraphs must be persisted as a single highlight entity with continuous span rendering across blocks.
* **Exact Location Tracking**: Persist `start_offset`, `end_offset`, `start_paragraph`, `end_paragraph`, and human-readable `location` (`p.N:offset`) in SQLite and PostgreSQL.

## Considered Options

* **In-Reader Native `SelectionArea` with Floating Kindle Toolbar (Chosen)**
* **Retain Manual Book Overview Highlight Dialog**
* **Split Multi-Paragraph Highlights into Separate Records**

## Decision Outcome

Chosen option: **In-Reader Native `SelectionArea` with Floating Kindle Toolbar**.

1. **Database Schema & Migrations**:
   * Added `0005_highlight_locations.sql` (SQLite) and `0002_highlight_locations.sql` (PostgreSQL/Bun) adding `start_offset`, `end_offset`, `start_paragraph`, `end_paragraph`, and `location` columns to `highlights`.
   * Updated repository models and SQL scan/insert queries in `internal/repository/sqlite.go`.
2. **Backend API Normalization**:
   * Updated `POST /api/v1/books/{id}/highlights` to validate and normalize colors against the Kindle palette (`yellow`, `blue`, `pink`, `orange`), defaulting to `yellow`.
   * Added parsing and persistence for all location fields.
3. **Flutter State & Cross-Device Sync**:
   * `ReaderView` triggers `ref.read(bookDetailProvider(widget.bookId).notifier).loadBook()` on initialization to sync latest server highlights on opening the book on any device.
   * `BookDetailNotifier` automatically caches highlights to local storage (`StorageService.cacheHighlights`) for offline access.
   * Highlights matching `_currentChapter.id` are reactive via Riverpod.
4. **Kindle Floating Toolbar & Selection**:
   * Wrapped reader canvas in `SelectionArea` with custom `contextMenuBuilder`.
   * `KindleSelectionToolbar` displays 4 Kindle color circles, a note button, and copy action.
   * Tapping a color circle immediately computes offsets via `findHighlightRange` and saves the highlight.
   * Tapping "Add Note" opens `_AddNoteSheet` with quoted excerpt, color picker, and note input.
   * Tapping existing highlights in text opens `_ExistingHighlightSheet` with quote, note, location, and deletion action.
5. **Multi-Paragraph Highlighting & Span Rendering**:
   * `findHighlightRange` computes unified offsets and bounding paragraph indices whether selection is within one paragraph or crosses multiple paragraphs.
   * `buildBlockSpans` in `reader_markdown.dart` computes intersection segments `[segStart, segEnd]` for each block overlapping `[startOffset, endOffset]`, attaching gesture recognizers for interaction.

## Consequences

### Positive Consequences

* **Kindle-Grade Reading UX**: Seamless selection-to-highlight workflow directly on the reading canvas.
* **Unified Highlights**: Multi-paragraph selections are saved as a single record and rendered cleanly across consecutive blocks.
* **Cross-Device Consistency**: Highlights sync automatically across devices and persist offline.
* **Sanitized Book Overview**: Removed the confusing manual "Add Highlight" dialog from the book detail page, reserving it solely for bookmarks and browsing past highlights.

### Negative Consequences

* Added database columns requiring schema migrations on existing installations.
