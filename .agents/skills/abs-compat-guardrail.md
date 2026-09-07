# Audiobookshelf Compatibility Guardrail Manual

This technical manual instructs the AI on maintaining non-destructive coexistence with existing Audiobookshelf instances.

## Invariants
1. **Separation of Mounts**:
   * `/library`: Human-readable book storage shared with Audiobookshelf.
   * `/data`: Private application state (`sqlite.db`, covers cache, temporary staging).
2. **Path Convention on Upload**:
   * New uploads must strictly be written to:
     ```text
     /library/<Author>/<Title>/<Title>.epub
     ```
   * Sanitize directory and file names (strip illegal characters `/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`).
3. **Non-Destructive Operations**:
   * Never delete, move, or overwrite Audiobookshelf sidecar files:
     - `.metadata.json`
     - `desc.txt`
     - `reader-progress.json`
     - Audio files (`*.m4b`, `*.mp3`, `*.flac`)
   * Never overwrite existing cover files in `/library`.
4. **Atomic Ingest**:
   * Write newly uploaded books to temporary files (e.g. `book.epub.tmp`) before moving to final destination so Audiobookshelf scanners do not read partial files.
