-- Add book_type and duration_seconds to books table
ALTER TABLE books ADD COLUMN book_type TEXT NOT NULL DEFAULT 'ebook';
ALTER TABLE books ADD COLUMN duration_seconds REAL;

-- Create audio_chapters table for chapter offsets and durations
CREATE TABLE IF NOT EXISTS audio_chapters (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    title TEXT NOT NULL,
    start_offset_sec REAL NOT NULL,
    duration_sec REAL NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audio_chapters_book_id ON audio_chapters(book_id, chapter_index);

-- Create book_files table for multi-format associated files
CREATE TABLE IF NOT EXISTS book_files (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    file_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size_bytes INTEGER,
    duration_seconds REAL,
    bitrate_kbps INTEGER,
    page_count INTEGER,
    mime_type TEXT,
    file_modified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_book_files_book_id ON book_files(book_id, file_type);
