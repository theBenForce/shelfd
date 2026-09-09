-- Add metadata and has_cover to upload_jobs for pre-ingestion editing and review
ALTER TABLE upload_jobs ADD COLUMN IF NOT EXISTS metadata TEXT;
ALTER TABLE upload_jobs ADD COLUMN IF NOT EXISTS has_cover BOOLEAN DEFAULT FALSE;
