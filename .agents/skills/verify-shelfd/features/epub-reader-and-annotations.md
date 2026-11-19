# EPUB Reader, Themes & Annotations

The reflowable EPUB reader (`/books/:bookId/read`) renders clean editorial typography outside `AppShell` for distraction-free reading, steps across spine chapters, saves reading bookmarks, and syncs annotations back to the server.

## Sub-features

- `reader-open` opens `/books/:bookId/read` fullscreen (without `AppShell` chrome) and renders sanitized chapter HTML/paragraphs.
- `chapter-nav` moves between EPUB spine chapters (`Chapter 1: The Gom Jabbar` -> `Chapter 2: Arrakis & Melange`).
- `bookmark-persist` creates a named bookmark and persists it to `GET /api/v1/books/:bookId/bookmarks` and the Book Detail `Highlights & Bookmarks` tab.
- `reader-exit` returns from fullscreen reading back to `/books/:bookId` where `AppShell` navigation is restored.

## How to get to it (user POV)

- From `/books`, open `1 Book Dune Chronicles Series` -> `Dune`, then choose `Start Reading` (or `Resume Reading`).
- Choose a specific chapter row in the Book Detail `Table of Contents` list.
- Navigate directly to `/books/<duneBookId>/read`.

## Driving it with control-shelfd

Preconditions:

- Authenticated browser session (complete `connect-and-catalog-shell.md` login first).
- Lookup `<DUNE_ID>` via `.agents/skills/verify-shelfd/scripts/control-shelfd state`.

- **Open Dune Book Detail.** Navigate to `/books/<DUNE_ID>` and start reading. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books/<DUNE_ID>"` and `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Start Reading" --wait-ms 1500`. The browser navigates to `/books/<DUNE_ID>/read` and renders `Chapter 1: The Gom Jabbar` containing `Bene Gesserit` and `Fear is the mind-killer` with no sidebar `Rescan Library` button.
- **Advance to next chapter.** Move to Chapter 2. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books/<DUNE_ID>/read/2" --wait-ms 1500` (or click the next-chapter control in the reader bar). The ARIA snapshot shows `Chapter 2: Arrakis & Melange` and `sandworms`.
- **Add a bookmark on Book Detail and confirm persistence.** Return to `/books/<DUNE_ID>`, click `Add Bookmark`, save it, and verify both UI and REST state. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books/<DUNE_ID>"`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Add Bookmark" --wait-ms 600`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser fill --role textbox --name "Bookmark Title" --value "Gom Jabbar Passage"`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Save Bookmark" --wait-ms 800`, and `.agents/skills/verify-shelfd/scripts/control-shelfd api GET /api/v1/books/<DUNE_ID>/bookmarks`. Both the `Highlights & Bookmarks` tab and the API JSON include `"Gom Jabbar Passage"`.
- **Capture proof.** Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/epub-reader-and-annotations/reader.aria.txt`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/epub-reader-and-annotations/reader.png`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/epub-reader-and-annotations/console.json`.

## Gotchas

- Reader routes (`/books/:id/read` and `/books/:id/read/:chapterIdentifier`) intentionally exclude `AppShell`; assert fullscreen absence of sidebar nav on `/read` and restoration on `/books/:id`.
- Once reading progress is `> 0`, the Book Detail CTA label changes from `Start Reading` to `Resume Reading` (or `Read EPUB` when companion audio is attached).
