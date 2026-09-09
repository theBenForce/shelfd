-- Add location metadata and multi-paragraph span columns to highlights
ALTER TABLE highlights ADD COLUMN start_offset INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN end_offset INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN start_paragraph INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN end_paragraph INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN location TEXT;
