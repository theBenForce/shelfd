-- Upload jobs queue for asynchronous processing
CREATE TABLE IF NOT EXISTS upload_jobs (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    staged_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued', -- queued, processing, completed, failed
    book_id TEXT REFERENCES books(id) ON DELETE SET NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_upload_jobs_status ON upload_jobs(status, created_at);
