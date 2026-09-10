-- Migration 0007: Add fixed layout and viewport spread metadata
ALTER TABLE books ADD COLUMN layout TEXT NOT NULL DEFAULT 'reflowable';
ALTER TABLE books ADD COLUMN rendition_spread TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE books ADD COLUMN rendition_orientation TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE books ADD COLUMN page_progression_direction TEXT NOT NULL DEFAULT 'ltr';

ALTER TABLE chapters ADD COLUMN href TEXT;
ALTER TABLE chapters ADD COLUMN page_width INTEGER;
ALTER TABLE chapters ADD COLUMN page_height INTEGER;
ALTER TABLE chapters ADD COLUMN page_spread TEXT;
