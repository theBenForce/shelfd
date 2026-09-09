-- Add photo_url to authors table
ALTER TABLE authors ADD COLUMN IF NOT EXISTS photo_url TEXT;
