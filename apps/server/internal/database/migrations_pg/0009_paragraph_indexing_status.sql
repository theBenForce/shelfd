-- Track paragraph vector indexing status directly to avoid expensive anti-joins
ALTER TABLE paragraphs ADD COLUMN IF NOT EXISTS is_embedded BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill existing vectorized paragraphs
UPDATE paragraphs SET is_embedded = TRUE FROM vec_paragraphs WHERE paragraphs.id = vec_paragraphs.paragraph_id AND NOT paragraphs.is_embedded;

-- Partial index for unindexed lookups and pending counts
CREATE INDEX IF NOT EXISTS idx_paragraphs_unindexed ON paragraphs(created_at ASC, chapter_index ASC, start_paragraph ASC) WHERE is_embedded = FALSE;
