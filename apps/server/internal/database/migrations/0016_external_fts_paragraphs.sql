-- Migration 0016: Convert paragraphs_fts to external content table to eliminate duplicate text storage

-- Drop legacy triggers
DROP TRIGGER IF EXISTS trg_paragraphs_fts_insert;
DROP TRIGGER IF EXISTS trg_paragraphs_fts_delete;

-- Drop legacy internal content FTS5 table and its shadow tables
DROP TABLE IF EXISTS paragraphs_fts;

-- Recreate paragraphs_fts as an external content table referencing paragraphs(content)
CREATE VIRTUAL TABLE IF NOT EXISTS paragraphs_fts USING fts5(
    content,
    content='paragraphs',
    content_rowid='rowid'
);

-- Rebuild the FTS5 index from existing paragraphs table rows
INSERT INTO paragraphs_fts(paragraphs_fts) VALUES('rebuild');

-- Create triggers to keep external FTS5 index in sync with paragraphs table
CREATE TRIGGER IF NOT EXISTS trg_paragraphs_fts_insert AFTER INSERT ON paragraphs
BEGIN
    INSERT INTO paragraphs_fts(rowid, content)
    VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS trg_paragraphs_fts_delete AFTER DELETE ON paragraphs
BEGIN
    INSERT INTO paragraphs_fts(paragraphs_fts, rowid, content)
    VALUES ('delete', old.rowid, old.content);
END;

CREATE TRIGGER IF NOT EXISTS trg_paragraphs_fts_update AFTER UPDATE OF content ON paragraphs
BEGIN
    INSERT INTO paragraphs_fts(paragraphs_fts, rowid, content)
    VALUES ('delete', old.rowid, old.content);
    INSERT INTO paragraphs_fts(rowid, content)
    VALUES (new.rowid, new.content);
END;
