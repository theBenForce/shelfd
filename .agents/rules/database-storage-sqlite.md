# Database & Vector Storage Standards

## Invariants

1. **Default Single-File Database:**
   - Default engine is SQLite with `sqlite-vec` virtual tables (`vec0`).
   - SQLite database path is strictly `/data/sqlite.db`.
   - All migrations, relational tables, and vectors reside inside this single file.

2. **Foreign Key Integrity:**
   - Foreign keys must always be enabled (`PRAGMA foreign_keys = ON;`).
   - Deleting a book (`DELETE FROM books WHERE id = ?`) must cascade delete its authors associations, genres, series, chapters, and chapter vector entries automatically.

3. **Pluggable Architecture:**
   - All database queries must go through the Go `StorageEngine` interface.
   - Do not leak SQLite-specific SQL into HTTP handlers or service logic, allowing PostgreSQL + `pgvector` to be substituted via `config.yaml`.
