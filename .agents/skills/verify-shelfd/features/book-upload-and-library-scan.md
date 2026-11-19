# Book Upload, Queue SSE & Library Scan

Shelfd ingests new EPUBs asynchronously through a persistent upload queue (`POST /api/v1/books/upload`, `202 Accepted`), broadcasts live queue updates over Server-Sent Events (`/api/v1/queue/events`), writes canonical files to `/library/<Author>/<Title>/<Title>.epub`, and rescans shared Audiobookshelf directories without mutating sidecar files.

## Sub-features

- `epub-upload-queue` accepts multipart EPUB uploads, processes them in the background worker, and exposes job completion in `/uploads` and `GET /api/v1/books/upload/jobs/:id`.
- `abs-path-and-sidecars` writes newly ingested EPUBs to `/library/<Author>/<Title>/<Title>.epub` while leaving `.metadata.json` and `desc.txt` byte-identical.
- `library-rescan` triggers non-destructive directory scans from the UI (`Rescan Library`), API (`POST /api/v1/library/scan`), and CLI (`shelfd -scan`).

## How to get to it (user POV)

- Choose `Upload EPUB` in the `/books` header or `Uploads` in the sidebar (`/uploads`) to upload and monitor queued jobs.
- Choose `Rescan Library` in the sidebar or `/books` header to scan `/library` for newly added files.
- Run `shelfd -scan` for one-shot CLI cataloging.

## Driving it with control-shelfd

Preconditions:

- Authenticated browser session on `/books`.
- `$RUNTIME_DIR/fixtures/earthsea.epub` (`A Wizard of Earthsea` by `Ursula K. Le Guin`) has not yet been uploaded.

- **Enqueue multipart EPUB upload.** Upload `earthsea.epub` and capture the `202 Accepted` job ID. Run `.agents/skills/verify-shelfd/scripts/control-shelfd upload` and record `job_id`.
- **Confirm background ingestion & `/uploads` UI.** Poll `GET /api/v1/books/upload/jobs/<job_id>` until `"status":"completed"`, then inspect `/uploads` and `/books` in the browser. Run `.agents/skills/verify-shelfd/scripts/control-shelfd api GET /api/v1/books/upload/jobs/<job_id>`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/uploads" --wait-ms 1500`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser snapshot --path /tmp/shelfd-verify-evidence/book-upload-and-library-scan/uploads.aria.txt`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser screenshot --path /tmp/shelfd-verify-evidence/book-upload-and-library-scan/uploads.png`.
- **Verify filesystem path & sidecar sanctity.** Confirm `$RUNTIME_DIR/library/Ursula K Le Guin/A Wizard Of Earthsea/A Wizard Of Earthsea.epub` exists on disk, trigger `Rescan Library` in the UI, and run `doctor` to verify `.metadata.json` and `desc.txt` checksums. Run `.agents/skills/verify-shelfd/scripts/control-shelfd browser goto "/books" --wait-ms 1500`, `.agents/skills/verify-shelfd/scripts/control-shelfd browser click --role button --name "Rescan Library" --wait-ms 1500`, `.agents/skills/verify-shelfd/scripts/control-shelfd doctor`, and `.agents/skills/verify-shelfd/scripts/control-shelfd browser console --assert-clean --path /tmp/shelfd-verify-evidence/book-upload-and-library-scan/console.json`. `/books` now reports `3 Books` including `1 Book Earthsea Cycle Series`.

## Gotchas

- Upload ingestion runs on a 3-second worker poll (`UploadWorker`); allow up to 4–5 seconds after `202 Accepted` before asserting `status == "completed"`.
- `Shelfd`'s upload path sanitizer strips periods from author initials and title-cases directory/file segments (`Ursula K. Le Guin` / `A Wizard of Earthsea` -> `Ursula K Le Guin/A Wizard Of Earthsea/A Wizard Of Earthsea.epub`).
- Cross-origin SSE (`/api/v1/queue/events`) fails in browsers if `Access-Control-Allow-Headers` omits `Cache-Control`; `control-shelfd doctor` checks this header explicitly.
