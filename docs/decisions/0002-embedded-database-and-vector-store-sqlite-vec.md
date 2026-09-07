# Embedded Database & Vector Store — SQLite with sqlite-vec

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

`shelfd` requires persistent relational storage for user accounts, book metadata, authors, genres, and series, alongside vector embeddings for semantic chapter search. The target library size is a few hundred to a few thousand books. Should we use an embedded single-file database like SQLite with `sqlite-vec`, an embedded columnar vector store like LanceDB, or require an external PostgreSQL + `pgvector` container?

## Decision Drivers

* Zero-friction single-container deployment by default.
* Atomic transactions (e.g. deleting a book automatically cleans up its vectors via foreign key cascades).
* Single-file backup and restore ergonomics for self-hosters (`/data/sqlite.db`).
* Sub-millisecond query performance on vector collections under 100,000 vectors.
* Extensibility to connect to existing external PostgreSQL instances if configured.

## Considered Options

* **SQLite + `sqlite-vec` (Embedded Virtual Table `vec0`)**
* **SQLite (Relational) + LanceDB (Vectors)**
* **PostgreSQL + `pgvector` (Standalone / Multi-Container)**

## Decision Outcome

Chosen option: **SQLite + `sqlite-vec`**, because it encapsulates relational metadata and vector embeddings inside a single database file with full ACID transaction support, eliminating multi-database desynchronization.

### Positive Consequences

* Single file state: `/data/sqlite.db` contains all tables, indexes, and vector virtual tables.
* Atomic cascades: `DELETE FROM books WHERE id = ?` cleans up authors, chapters, and vectors in a single SQL operation.
* Low latency: SIMD-accelerated brute-force vector scans take < 1ms for the target library size (~6,000 vectors).
* The storage layer is placed behind a `StorageEngine` interface, allowing external PostgreSQL to be plugged in via configuration.

### Negative Consequences

* `sqlite-vec` brute-force scanning scales up to ~500k vectors before requiring partitioning or external indexing.
* Requires Cgo during compilation.

## Pros and Cons of the Options

### SQLite + sqlite-vec

* Good, because unified relational and vector storage in one file.
* Good, because atomic transactions and standard foreign keys work across metadata and vectors.
* Good, because backups require copying only `/data/sqlite.db`.
* Bad, because not suited for massive multi-million vector datasets without IVF-PQ disk indexing.

### SQLite + LanceDB

* Good, because LanceDB scales to millions of vectors via disk-based IVF-PQ indexing.
* Bad, because dual-state problem: metadata in SQLite and vectors in Lance files can desynchronize on crashes or failed deletes.
* Bad, because LanceDB lacks a stable, official Go SDK, requiring experimental FFI bindings.

### PostgreSQL + pgvector

* Good, because powerful relational engine and battle-tested vector extensions.
* Bad, because requires a multi-container Docker Compose setup by default, breaking the single-container deployment goal.
