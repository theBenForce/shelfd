CREATE TABLE IF NOT EXISTS recap_cache (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    paragraph_offset INTEGER NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (book_id, chapter_index, paragraph_offset)
);
