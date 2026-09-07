# Storage Decoupling — Audiobookshelf Compatibility

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team
* Date: 2026-09-07

## Context and Problem Statement

The user already operates an Audiobookshelf (ABS) instance serving an existing collection of ebooks and audiobooks. `shelfd` must operate on the same collection without corrupting ABS metadata, causing permission locks, or polluting the library directory with database files. How should storage and file access be decoupled?

## Decision Drivers

* Full coexistence with Audiobookshelf without requiring duplicate book files.
* Non-destructive reading and scanning.
* Automatic discovery of newly uploaded books by Audiobookshelf.
* Clean separation of application state (database, temporary files, caches) from library assets.

## Considered Options

* **Decoupled Mounts (`/library` for shared books, `/data` for internal state)**
* **Single Unified Mount (`/data` containing books and internal files)**
* **Read-Only Library with Separate Upload Staging Directory**

## Decision Outcome

Chosen option: **Decoupled Mounts**, mounting `/library` read-write for books and `/data` for internal application state. Newly uploaded books are written to `/library/<Author>/<Title>/<Title>.epub`.

### Positive Consequences

* Clean separation: `/data` stores `sqlite.db`, tokens, and cover cache; `/library` contains only human-readable book folders.
* Audiobookshelf picks up new books on its next automated scan because `shelfd` writes in ABS's expected directory format.
* Zero pollution of user book directories with hidden index or database files.

### Negative Consequences

* Requires two volume mounts in Docker Compose (`./data:/data` and `/path/to/ebooks:/library:rw`).
* Deleting a book in `shelfd` must safely check and remove book files in `/library` without touching sibling files.

## Pros and Cons of the Options

### Decoupled Mounts

* Good, because Audiobookshelf can share the exact same library folder seamlessly.
* Good, because internal state stays isolated in `/data`.
* Good, because backups of application state are isolated from multi-gigabyte media storage.
* Bad, because requires managing directory permissions (PUID/PGID) across two containers.

### Single Unified Mount

* Good, because only one volume mount needed.
* Bad, because mixes internal SQLite files with human-readable books, violating Audiobookshelf folder expectations.

### Read-Only Library

* Good, because zero risk of modifying existing files.
* Bad, because users cannot upload new EPUBs directly through the `shelfd` interface into their primary library.
