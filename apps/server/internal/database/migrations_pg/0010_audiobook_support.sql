-- Add book_type and duration_seconds to books table
ALTER TABLE books ADD COLUMN IF NOT EXISTS book_type VARCHAR(32) NOT NULL DEFAULT 'ebook';
ALTER TABLE books ADD COLUMN IF NOT EXISTS duration_seconds DOUBLE PRECISION;

-- Create audio_chapters table for chapter offsets and durations
CREATE TABLE IF NOT EXISTS audio_chapters (
    id VARCHAR(36) PRIMARY KEY,
    book_id VARCHAR(36) NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    title VARCHAR(512) NOT NULL,
    start_offset_sec DOUBLE PRECISION NOT NULL,
    duration_sec DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audio_chapters_book_id ON audio_chapters(book_id, chapter_index ASC);

-- Create book_files table for multi-format associated files
CREATE TABLE IF NOT EXISTS book_files (
    id VARCHAR(36) PRIMARY KEY,
    book_id VARCHAR(36) NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    file_type VARCHAR(32) NOT NULL,
    file_path VARCHAR(1024) NOT NULL,
    file_size_bytes BIGINT,
    duration_seconds DOUBLE PRECISION,
    bitrate_kbps INTEGER,
    page_count INTEGER,
    mime_type VARCHAR(128),
    file_modified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_book_files_book_id ON book_files(book_id, file_type);
