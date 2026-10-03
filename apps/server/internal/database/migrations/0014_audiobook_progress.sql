-- Create audiobook_progress table for tracking user listening position and speed
CREATE TABLE IF NOT EXISTS audiobook_progress (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL DEFAULT '',
    position_seconds REAL NOT NULL DEFAULT 0.0,
    speed REAL NOT NULL DEFAULT 1.0,
    is_completed BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(book_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_audiobook_progress_book_user ON audiobook_progress(book_id, user_id);
