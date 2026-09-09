-- Add metadata and has_cover to upload_jobs for pre-ingestion editing and review
ALTER TABLE upload_jobs ADD COLUMN metadata TEXT;
ALTER TABLE upload_jobs ADD COLUMN has_cover BOOLEAN DEFAULT 0;
