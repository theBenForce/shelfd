# External Content Table for SQLite FTS5 Paragraph Index

* Status: accepted
* Deciders: Lead Systems Architect, @core
* Date: 2026-10-12

## Context and problem statement

Shelfd indexes text passages in SQLite using an FTS5 virtual table named `paragraphs_fts` (ADR-0014). The original migration created `paragraphs_fts` without the `content` parameter, causing SQLite to create internal shadow tables (`paragraphs_fts_content`).

As a result, SQLite duplicated every paragraph's text: once in the relational `paragraphs` table and once in the FTS5 shadow table. In addition, the query joined `paragraphs_fts` and `paragraphs` on a string identifier (`f.paragraph_id = p.id`).

How should full-text search be configured to eliminate shadow table text duplication while retaining fast lexical search?

## Decision drivers

* Eliminate duplicate text storage inside `/data/sqlite.db`.
* Preserve instant sub-millisecond lexical search with BM25 ranking.
* Maintain automated index synchronization during insert, update, and delete operations.
* Keep backwards-compatible database migration paths.

## Considered options

* **External Content FTS5 Table referencing `paragraphs` via `rowid` (Chosen)**
* **Contentless FTS5 Table (`content=''`)**
* **Retain Internal Content FTS5 Table**

## Decision outcome

Chosen option: **External Content FTS5 Table referencing `paragraphs` via `rowid`**.

1. **Migration 0016**:
   * Drops legacy triggers and the internal `paragraphs_fts` virtual table.
   * Recreates `paragraphs_fts` with `content='paragraphs', content_rowid='rowid'`.
   * Rebuilds the inverted index using `INSERT INTO paragraphs_fts(paragraphs_fts) VALUES('rebuild')`.
   * Configures `AFTER INSERT`, `AFTER DELETE`, and `AFTER UPDATE` triggers using FTS5 external content sync commands.
2. **Join Optimization**:
   * Updates `SearchFTSParagraphs` in `sqlite.go` and `bun.go` to join on `f.rowid = p.rowid`. Integer rowid seeks replace unindexed string identifier comparisons.

## Consequences

### Positive consequences

* Eliminates the `paragraphs_fts_content` shadow table, cutting SQLite text storage in half.
* Speeds up FTS table joins by using SQLite B-tree 64-bit integer rowid lookups.
* Keeps inverted index synchronized across all insert, update, and delete actions.

### Negative consequences

* Requires `old.content` to be supplied to the FTS5 table on deletion, handled via database triggers.
