-- Track paragraph vector indexing status directly to avoid expensive anti-joins
ALTER TABLE paragraphs ADD COLUMN is_embedded BOOLEAN NOT NULL DEFAULT 0;

-- Backfill existing vectorized paragraphs
UPDATE paragraphs SET is_embedded = 1 WHERE id IN (SELECT paragraph_id FROM vec_paragraphs);

-- Partial index for unindexed lookups and pending counts
CREATE INDEX IF NOT EXISTS idx_paragraphs_unindexed ON paragraphs(created_at, chapter_index, start_paragraph) WHERE is_embedded = 0;
