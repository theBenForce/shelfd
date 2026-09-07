-- Core users
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- API tokens for MCP and external clients
CREATE TABLE IF NOT EXISTS api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Normalized Authors
CREATE TABLE IF NOT EXISTS authors (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Normalized Genres / Subjects
CREATE TABLE IF NOT EXISTS genres (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Normalized Series
CREATE TABLE IF NOT EXISTS series (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Book metadata
CREATE TABLE IF NOT EXISTS books (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    language TEXT,
    publisher TEXT,
    identifier TEXT,
    file_path TEXT NOT NULL,
    cover_path TEXT,
    file_size_bytes INTEGER,
    published_date TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Junction: Books <-> Authors
CREATE TABLE IF NOT EXISTS book_authors (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    author_id TEXT NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
    role TEXT DEFAULT 'author',
    PRIMARY KEY (book_id, author_id)
);
CREATE INDEX IF NOT EXISTS idx_book_authors_author ON book_authors(author_id);

-- Junction: Books <-> Genres
CREATE TABLE IF NOT EXISTS book_genres (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    genre_id TEXT NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, genre_id)
);
CREATE INDEX IF NOT EXISTS idx_book_genres_genre ON book_genres(genre_id);

-- Junction: Books <-> Series
CREATE TABLE IF NOT EXISTS book_series (
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    sequence_number REAL,
    PRIMARY KEY (book_id, series_id)
);
CREATE INDEX IF NOT EXISTS idx_book_series_seq ON book_series(series_id, sequence_number);

-- Chapters (Text & Summaries)
CREATE TABLE IF NOT EXISTS chapters (
    id TEXT PRIMARY KEY,
    book_id TEXT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chapter_index INTEGER NOT NULL,
    title TEXT,
    summary TEXT NOT NULL,
    content_plain TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_chapters_book ON chapters(book_id, chapter_index);

-- Virtual Vector Table (1536 float embeddings)
CREATE VIRTUAL TABLE IF NOT EXISTS vec_chapters USING vec0(
    chapter_id TEXT PRIMARY KEY,
    embedding float[1536]
);

-- Trigger to delete vector entry when chapter is deleted (enabling cascading delete from book -> chapters -> vec_chapters)
CREATE TRIGGER IF NOT EXISTS trg_delete_chapter_vec AFTER DELETE ON chapters
BEGIN
    DELETE FROM vec_chapters WHERE chapter_id = OLD.id;
END;
