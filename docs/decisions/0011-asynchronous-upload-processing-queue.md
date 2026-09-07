# Asynchronous Upload Processing Queue

* Status: accepted
* Deciders: Lead Systems Architect, Founding Team, @core, @librarian
* Date: 2026-09-07

## Context and Problem Statement

EPUB uploads previously processed synchronously within the HTTP request handler (`POST /api/v1/books/upload`). Synchronous processing blocks the client connection while reading archives, parsing chapters, copying files, and cataloging metadata into SQLite. On resource-constrained homelab hardware (e.g. Raspberry Pi, low-power NAS devices) or during concurrent uploads, synchronous processing risks HTTP request timeouts, thread starvation, and potential catalog corruption if the server restarts mid-upload. Furthermore, clients received no granular insight into background ingestion progress or errors. How should book upload ingestion be architected to ensure crash resilience, homelab disk I/O throttling, and immediate responsive client feedback?

## Decision Drivers

* **Immediate Client Responsiveness**: Fast HTTP acknowledgments (`202 Accepted`) with a trackable job ID so clients and mobile apps never hang on slow connections.
* **Crash Resilience & Recovery**: In-flight or pending jobs survive process restarts and automatically re-queue upon cold start.
* **Homelab I/O Throttling**: Sequential queue processing avoids disk I/O starvation and CPU spikes on low-power storage hardware.
* **Audiobookshelf Library Sanctity**: Newly uploaded books are strictly saved to `/library/<Author>/<Title>/<Title>.epub` using atomic renames without modifying existing Audiobookshelf sidecars.
* **Status Observability**: Standard REST endpoints for polling job progress (`queued`, `processing`, `completed`, `failed`).

## Considered Options

* **SQLite-Backed Persistent Queue with Dedicated Worker & Staged Storage**
* **In-Memory Go Channels (`chan UploadJob`)**
* **External Message Broker (Redis / RabbitMQ / Celery)**

## Decision Outcome

Chosen option: **SQLite-Backed Persistent Queue with Dedicated Worker & Staged Storage**.

1. **Staging & Validation**:
   - Incoming uploads enforce `.epub` extension and 60MB payload limit.
   - Archive headers are quickly validated using `zip.OpenReader`.
   - File is staged to `/data/uploads/<job_id>.epub`.
   - An `upload_jobs` row is inserted in SQLite with `status = 'queued'`.
   - Returns HTTP `202 Accepted` immediately with `job_id`, `status`, `filename`, and timestamp.
2. **Dedicated Background Worker**:
   - `UploadWorker` monitors the queue via instant notification channels (`Trigger()`) and periodic polling.
   - Processes jobs sequentially to protect homelab disk I/O.
   - Transitions job status from `queued` to `processing`.
   - Extracts metadata, stages book into `/library/<Author>/<Title>/<Title>.epub`, records database entries, triggers semantic chapter indexing, and deletes the staged file.
   - Updates job status to `completed` (with `book_id`) or `failed` (with descriptive `error_message`).
3. **Crash Recovery**:
   - On daemon startup, `UploadWorker.Start()` scans for interrupted `processing` jobs from prior runs and resets them to `queued` for automated reprocessing.
4. **Job Observability Endpoints**:
   - `GET /api/v1/books/upload/jobs/{id}` returns live job state and linked `book_id`.
   - `GET /api/v1/books/upload/jobs` returns a list of recent upload jobs.

### Positive Consequences

* **Non-Blocking Ingestion**: Clients receive sub-50ms responses regardless of EPUB file size or background AI processing load.
* **Zero Lost Uploads**: If the server is killed or restarts during ingestion, staged files and job records remain persisted in SQLite and resume upon restart.
* **Zero External Dependencies**: Implemented natively in Go and SQLite without adding Redis or third-party queue brokers.
* **Sanctity of `/library` Preserved**: Files are staged safely in `/data/uploads/` until atomic placement into `/library`.

### Negative Consequences

* Clients must poll `GET /api/v1/books/upload/jobs/{id}` or listen to updates to know when the book is ready for reading.
* Requires storage capacity in `/data/uploads` while jobs are in queue.
