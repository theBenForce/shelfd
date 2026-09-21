-- Track paragraph vector indexing status directly to avoid expensive anti-joins
ALTER TABLE paragraphs ADD COLUMN IF NOT EXISTS is_embedded BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill existing vectorized paragraphs
UPDATE paragraphs SET is_embedded = TRUE WHERE id IN (SELECT paragraph_id FROM vec_paragraphs);

-- Partial index for unindexed lookups and pending counts
CREATE INDEX IF NOT EXISTS idx_paragraphs_unindexed ON paragraphs(created_at ASC, chapter_index ASC, start_paragraph ASC) WHERE is_embedded = FALSE;
