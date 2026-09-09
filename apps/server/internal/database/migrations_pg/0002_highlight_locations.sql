-- Add location metadata and multi-paragraph span columns to highlights
ALTER TABLE highlights ADD COLUMN IF NOT EXISTS start_offset INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN IF NOT EXISTS end_offset INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN IF NOT EXISTS start_paragraph INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN IF NOT EXISTS end_paragraph INTEGER DEFAULT 0;
ALTER TABLE highlights ADD COLUMN IF NOT EXISTS location TEXT;
