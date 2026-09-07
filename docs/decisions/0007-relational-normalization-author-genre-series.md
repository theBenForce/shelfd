# Relational Normalization — Author, Genre, and Series Entities

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

Ebook metadata includes authors, genres, and series. Storing these as unindexed comma-separated strings inside the `books` table makes filtering slow, prevents relational browsing, and complicates search queries that combine semantic search with strict metadata filters (e.g. *"books by Ursula K. Le Guin in the Earthsea series"*). How should metadata be structured in the relational schema?

## Decision Drivers

* Fast indexed filtering across authors, genres, and series.
* Support for books with multiple authors (co-authors, editors, translators).
* Support for books with multiple genres/subjects.
* Explicit support for series reading order (sequence numbers, e.g. Book 1, Book 2.5).

## Considered Options

* **Normalized Entities with Many-to-Many Junction Tables**
* **Flat Denormalized String Columns in `books`**
* **JSON Columns with SQLite JSON1 Functions**

## Decision Outcome

Chosen option: **Normalized Entities with Many-to-Many Junction Tables**, creating dedicated tables (`authors`, `genres`, `series`) linked to `books` via `book_authors`, `book_genres`, and `book_series` with indexed foreign keys.

### Positive Consequences

* Fast, clean SQL filtering in both the REST API and hybrid `sqlite-vec` queries.
* Books can belong to multiple genres and have multiple attributed contributors.
* The `book_series` junction table stores a `sequence_number REAL`, enabling fractional indexes (e.g. Book 1.5, 2.0) and sorting by reading order.
* Renaming an author or genre updates the entire library instantaneously without table-wide row scans.

### Negative Consequences

* Slightly more complex ingestion logic: the EPUB parser must deduplicate and upsert authors and genres before linking them.
* Junction table queries require SQL `JOIN` statements.

## Pros and Cons of the Options

### Normalized Entities (M2M)

* Good, because maintains relational integrity and supports clean indices.
* Good, because accommodates multi-author, multi-genre, and series ordering seamlessly.
* Good, because standard SQL joins integrate directly into hybrid vector search queries.
* Bad, because ingestion requires multi-step insert/upsert transactions.

### Flat Denormalized Columns

* Good, because simplest table structure (`books` table only).
* Bad, because filtering requires slow `LIKE '%author%'` queries that miss edge cases.
* Bad, because cannot properly index or sort series by sequence number.

### JSON Columns

* Good, because avoids multiple tables while retaining semi-structured data.
* Bad, because queries relying on `json_each()` are slower and harder to index in SQLite than native B-tree indexed junction tables.
