-- Normalized Topics
CREATE TABLE IF NOT EXISTS topics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_topics_name_lower ON topics (LOWER(name));

-- Junction: Books <-> Topics
CREATE TABLE IF NOT EXISTS book_topics (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (book_id, topic_id)
);
CREATE INDEX IF NOT EXISTS idx_book_topics_topic ON book_topics(topic_id);
