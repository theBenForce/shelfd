-- Add file modification timestamp and index on file_path for incremental scanning
ALTER TABLE books ADD COLUMN IF NOT EXISTS file_modified_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_books_file_path ON books(file_path);
