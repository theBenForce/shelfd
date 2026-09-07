# Audiobookshelf Coexistence Rules

## Invariants

1. **Decoupled Mounts:**
   - `/library` is the shared, human-readable book collection (mounted from host, shared with Audiobookshelf).
   - `/data` is the private internal state of `shelfd` (`sqlite.db`, token store, generated cover cache, temporary uploads).
   - Never write internal database files, indices, or logs into `/library`.

2. **Directory Structure on Upload:**
   - When uploading a new EPUB, write it strictly following the Audiobookshelf directory pattern:
     ```text
     /library/<Author>/<Book Title>/<Book Title>.epub
     ```
   - Sanitize author and title directory names for standard filesystem safety (remove illegal path characters `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`).

3. **Non-Destructive Reads and Scans:**
   - During library crawling/scanning, read only `.epub` files.
   - Never modify, overwrite, move, or delete Audiobookshelf metadata files:
     - `.metadata.json`
     - `desc.txt`
     - `reader-progress.json`
     - Audio files (`.m4b`, `.mp3`, `.flac`)
   - Never delete existing cover art placed by Audiobookshelf in the book directory.

4. **Permissions:**
   - Respect standard PUID/PGID container configurations so files written by `shelfd` can be read and scanned by Audiobookshelf without Linux file permission errors.
