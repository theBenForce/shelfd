-- Paragraphs (Passage Chunks)
CREATE TABLE IF NOT EXISTS paragraphs (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_id TEXT NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    start_paragraph INTEGER NOT NULL,
    end_paragraph INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_paragraphs_book ON paragraphs(book_id);
CREATE INDEX IF NOT EXISTS idx_paragraphs_chapter ON paragraphs(chapter_id, chapter_index);

-- Virtual Vector Table (256 float embeddings)
CREATE VIRTUAL TABLE IF NOT EXISTS vec_paragraphs USING vec0(
    paragraph_id TEXT PRIMARY KEY,
    embedding float[256]
);

-- Cascade vector deletion on paragraph delete
CREATE TRIGGER IF NOT EXISTS trg_delete_paragraph_vec AFTER DELETE ON paragraphs
BEGIN
    DELETE FROM vec_paragraphs WHERE paragraph_id = old.id;
END;

-- Full-Text Search (FTS5) for instant lexical retrieval
CREATE VIRTUAL TABLE IF NOT EXISTS paragraphs_fts USING fts5(
    paragraph_id UNINDEXED,
    book_id UNINDEXED,
    chapter_id UNINDEXED,
    content
);

-- Triggers to maintain FTS5 index in sync with paragraphs
CREATE TRIGGER IF NOT EXISTS trg_paragraphs_fts_insert AFTER INSERT ON paragraphs
BEGIN
    INSERT INTO paragraphs_fts(rowid, paragraph_id, book_id, chapter_id, content)
    VALUES (new.rowid, new.id, new.book_id, new.chapter_id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS trg_paragraphs_fts_delete AFTER DELETE ON paragraphs
BEGIN
    DELETE FROM paragraphs_fts WHERE rowid = old.rowid;
END;

-- Clean up deprecated 1536-dimension chapter vector table
DROP TRIGGER IF EXISTS trg_delete_chapter_vec;
DROP TABLE IF EXISTS vec_chapters;
