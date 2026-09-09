-- Add file modification timestamp and index on file_path for incremental scanning
ALTER TABLE books ADD COLUMN file_modified_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_books_file_path ON books(file_path);
