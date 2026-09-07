package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/google/uuid"
)

var (
	// ErrNotFound indicates the requested entity was not found.
	ErrNotFound = errors.New("record not found")
)

// SQLiteStorageEngine implements StorageEngine backed by SQLite + sqlite-vec.
type SQLiteStorageEngine struct {
	db *sql.DB
}

// NewSQLiteStorageEngine creates a new repository backed by an active SQLite database.
func NewSQLiteStorageEngine(db *sql.DB) *SQLiteStorageEngine {
	return &SQLiteStorageEngine{db: db}
}

// Close closes the underlying database connection.
func (r *SQLiteStorageEngine) Close() error {
	return r.db.Close()
}

// --- Books ---

func (r *SQLiteStorageEngine) CreateBook(ctx context.Context, b *Book) error {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO books (id, title, description, language, publisher, identifier, file_path, cover_path, file_size_bytes, published_date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		b.ID, b.Title, b.Description, b.Language, b.Publisher, b.Identifier,
		b.FilePath, b.CoverPath, b.FileSizeBytes, b.PublishedDate, b.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating book: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetBookByID(ctx context.Context, id string) (*Book, error) {
	query := `
		SELECT id, title, description, language, publisher, identifier, file_path, cover_path, file_size_bytes, published_date, created_at
		FROM books WHERE id = ?
	`
	b := &Book{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.Title, &b.Description, &b.Language, &b.Publisher, &b.Identifier,
		&b.FilePath, &b.CoverPath, &b.FileSizeBytes, &b.PublishedDate, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying book by id: %w", err)
	}
	return b, nil
}

func (r *SQLiteStorageEngine) UpdateBook(ctx context.Context, b *Book) error {
	query := `
		UPDATE books
		SET title = ?, description = ?, language = ?, publisher = ?, identifier = ?,
		    file_path = ?, cover_path = ?, file_size_bytes = ?, published_date = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query,
		b.Title, b.Description, b.Language, b.Publisher, b.Identifier,
		b.FilePath, b.CoverPath, b.FileSizeBytes, b.PublishedDate, b.ID,
	)
	if err != nil {
		return fmt.Errorf("updating book: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) DeleteBook(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM books WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting book: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) ListBooks(ctx context.Context, filter BookFilter) ([]*Book, error) {
	var conditions []string
	var args []interface{}

	if filter.AuthorID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?)")
		args = append(args, *filter.AuthorID)
	}
	if filter.GenreID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?)")
		args = append(args, *filter.GenreID)
	}
	if filter.SeriesID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_series WHERE series_id = ?)")
		args = append(args, *filter.SeriesID)
	}
	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		conditions = append(conditions, "b.title LIKE ?")
		args = append(args, "%"+strings.TrimSpace(*filter.Search)+"%")
	}

	query := `
		SELECT b.id, b.title, b.description, b.language, b.publisher, b.identifier, b.file_path, b.cover_path, b.file_size_bytes, b.published_date, b.created_at
		FROM books b
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY b.created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ?"
	args = append(args, limit)

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing books: %w", err)
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		b := &Book{}
		err := rows.Scan(
			&b.ID, &b.Title, &b.Description, &b.Language, &b.Publisher, &b.Identifier,
			&b.FilePath, &b.CoverPath, &b.FileSizeBytes, &b.PublishedDate, &b.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning book row: %w", err)
		}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating book rows: %w", err)
	}
	return books, nil
}

// --- Authors ---

func (r *SQLiteStorageEngine) UpsertAuthor(ctx context.Context, name string) (*Author, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("author name cannot be empty")
	}

	query := `
		INSERT INTO authors (id, name, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET name = excluded.name
		RETURNING id, name, created_at
	`
	a := &Author{}
	err := r.db.QueryRowContext(ctx, query, uuid.NewString(), trimmed).Scan(&a.ID, &a.Name, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting author: %w", err)
	}
	return a, nil
}

func (r *SQLiteStorageEngine) GetAuthorByID(ctx context.Context, id string) (*Author, error) {
	a := &Author{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM authors WHERE id = ?", id).Scan(&a.ID, &a.Name, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying author by id: %w", err)
	}
	return a, nil
}

func (r *SQLiteStorageEngine) GetAuthorByName(ctx context.Context, name string) (*Author, error) {
	a := &Author{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM authors WHERE name = ? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&a.ID, &a.Name, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying author by name: %w", err)
	}
	return a, nil
}

func (r *SQLiteStorageEngine) ListAuthors(ctx context.Context) ([]*Author, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, created_at FROM authors ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, fmt.Errorf("listing authors: %w", err)
	}
	defer rows.Close()

	var authors []*Author
	for rows.Next() {
		a := &Author{}
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning author: %w", err)
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}

// --- Genres ---

func (r *SQLiteStorageEngine) UpsertGenre(ctx context.Context, name string) (*Genre, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("genre name cannot be empty")
	}

	query := `
		INSERT INTO genres (id, name, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET name = excluded.name
		RETURNING id, name, created_at
	`
	g := &Genre{}
	err := r.db.QueryRowContext(ctx, query, uuid.NewString(), trimmed).Scan(&g.ID, &g.Name, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting genre: %w", err)
	}
	return g, nil
}

func (r *SQLiteStorageEngine) GetGenreByID(ctx context.Context, id string) (*Genre, error) {
	g := &Genre{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM genres WHERE id = ?", id).Scan(&g.ID, &g.Name, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying genre by id: %w", err)
	}
	return g, nil
}

func (r *SQLiteStorageEngine) GetGenreByName(ctx context.Context, name string) (*Genre, error) {
	g := &Genre{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM genres WHERE name = ? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&g.ID, &g.Name, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying genre by name: %w", err)
	}
	return g, nil
}

func (r *SQLiteStorageEngine) ListGenres(ctx context.Context) ([]*Genre, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, created_at FROM genres ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, fmt.Errorf("listing genres: %w", err)
	}
	defer rows.Close()

	var genres []*Genre
	for rows.Next() {
		g := &Genre{}
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning genre: %w", err)
		}
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

// --- Series ---

func (r *SQLiteStorageEngine) UpsertSeries(ctx context.Context, name string, description *string) (*Series, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("series name cannot be empty")
	}

	query := `
		INSERT INTO series (id, name, description, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET description = COALESCE(excluded.description, series.description)
		RETURNING id, name, description, created_at
	`
	s := &Series{}
	err := r.db.QueryRowContext(ctx, query, uuid.NewString(), trimmed, description).Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting series: %w", err)
	}
	return s, nil
}

func (r *SQLiteStorageEngine) GetSeriesByID(ctx context.Context, id string) (*Series, error) {
	s := &Series{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM series WHERE id = ?", id).Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying series by id: %w", err)
	}
	return s, nil
}

func (r *SQLiteStorageEngine) GetSeriesByName(ctx context.Context, name string) (*Series, error) {
	s := &Series{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM series WHERE name = ? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying series by name: %w", err)
	}
	return s, nil
}

func (r *SQLiteStorageEngine) ListSeries(ctx context.Context) ([]*Series, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, description, created_at FROM series ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, fmt.Errorf("listing series: %w", err)
	}
	defer rows.Close()

	var seriesList []*Series
	for rows.Next() {
		s := &Series{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning series: %w", err)
		}
		seriesList = append(seriesList, s)
	}
	return seriesList, rows.Err()
}

// --- Junction Links ---

func (r *SQLiteStorageEngine) LinkBookAuthor(ctx context.Context, bookID, authorID, role string) error {
	if role == "" {
		role = "author"
	}
	query := `
		INSERT INTO book_authors (book_id, author_id, role)
		VALUES (?, ?, ?)
		ON CONFLICT(book_id, author_id) DO UPDATE SET role = excluded.role
	`
	_, err := r.db.ExecContext(ctx, query, bookID, authorID, role)
	if err != nil {
		return fmt.Errorf("linking book to author: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) LinkBookGenre(ctx context.Context, bookID, genreID string) error {
	query := `
		INSERT INTO book_genres (book_id, genre_id)
		VALUES (?, ?)
		ON CONFLICT(book_id, genre_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, bookID, genreID)
	if err != nil {
		return fmt.Errorf("linking book to genre: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) LinkBookSeries(ctx context.Context, bookID, seriesID string, sequenceNumber *float64) error {
	query := `
		INSERT INTO book_series (book_id, series_id, sequence_number)
		VALUES (?, ?, ?)
		ON CONFLICT(book_id, series_id) DO UPDATE SET sequence_number = excluded.sequence_number
	`
	_, err := r.db.ExecContext(ctx, query, bookID, seriesID, sequenceNumber)
	if err != nil {
		return fmt.Errorf("linking book to series: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetBookAuthors(ctx context.Context, bookID string) ([]*Author, error) {
	query := `
		SELECT a.id, a.name, a.created_at
		FROM authors a
		JOIN book_authors ba ON a.id = ba.author_id
		WHERE ba.book_id = ?
		ORDER BY a.name COLLATE NOCASE ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book authors: %w", err)
	}
	defer rows.Close()

	var authors []*Author
	for rows.Next() {
		a := &Author{}
		if err := rows.Scan(&a.ID, &a.Name, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning book author: %w", err)
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}

func (r *SQLiteStorageEngine) GetBookGenres(ctx context.Context, bookID string) ([]*Genre, error) {
	query := `
		SELECT g.id, g.name, g.created_at
		FROM genres g
		JOIN book_genres bg ON g.id = bg.genre_id
		WHERE bg.book_id = ?
		ORDER BY g.name COLLATE NOCASE ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book genres: %w", err)
	}
	defer rows.Close()

	var genres []*Genre
	for rows.Next() {
		g := &Genre{}
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning book genre: %w", err)
		}
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

func (r *SQLiteStorageEngine) GetBookSeries(ctx context.Context, bookID string) ([]*BookSeriesDetail, error) {
	query := `
		SELECT s.id, s.name, s.description, s.created_at, bs.sequence_number
		FROM series s
		JOIN book_series bs ON s.id = bs.series_id
		WHERE bs.book_id = ?
		ORDER BY bs.sequence_number ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book series: %w", err)
	}
	defer rows.Close()

	var seriesList []*BookSeriesDetail
	for rows.Next() {
		s := &BookSeriesDetail{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt, &s.SequenceNumber); err != nil {
			return nil, fmt.Errorf("scanning book series: %w", err)
		}
		seriesList = append(seriesList, s)
	}
	return seriesList, rows.Err()
}

// --- Chapters & Vectors ---

func (r *SQLiteStorageEngine) CreateChapter(ctx context.Context, c *Chapter) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO chapters (id, book_id, chapter_index, title, summary, content_plain, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.BookID, c.ChapterIndex, c.Title, c.Summary, c.ContentPlain, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating chapter: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetChaptersByBookID(ctx context.Context, bookID string) ([]*Chapter, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, created_at
		FROM chapters
		WHERE book_id = ?
		ORDER BY chapter_index ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting chapters by book id: %w", err)
	}
	defer rows.Close()

	var chapters []*Chapter
	for rows.Next() {
		c := &Chapter{}
		if err := rows.Scan(&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning chapter row: %w", err)
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

func (r *SQLiteStorageEngine) GetChapterByID(ctx context.Context, id string) (*Chapter, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, created_at
		FROM chapters WHERE id = ?
	`
	c := &Chapter{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying chapter by id: %w", err)
	}
	return c, nil
}

func (r *SQLiteStorageEngine) UpdateChapterSummary(ctx context.Context, chapterID string, summary string) error {
	query := `UPDATE chapters SET summary = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, summary, chapterID)
	if err != nil {
		return fmt.Errorf("updating chapter summary: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) GetUnindexedChapters(ctx context.Context, limit int) ([]*Chapter, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, created_at
		FROM chapters
		WHERE summary = ''
		ORDER BY created_at ASC, chapter_index ASC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("getting unindexed chapters: %w", err)
	}
	defer rows.Close()

	var chapters []*Chapter
	for rows.Next() {
		c := &Chapter{}
		if err := rows.Scan(&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning unindexed chapter: %w", err)
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

func (r *SQLiteStorageEngine) InsertChapterVector(ctx context.Context, chapterID string, embedding []float32) error {
	blob, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		return fmt.Errorf("serializing embedding: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO vec_chapters (chapter_id, embedding)
		VALUES (?, ?)
	`
	_, err = r.db.ExecContext(ctx, query, chapterID, blob)
	if err != nil {
		return fmt.Errorf("inserting chapter vector: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) SearchVectorChapters(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error) {
	blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("serializing query embedding: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	k := limit
	if filter.AuthorID != nil || filter.GenreID != nil || filter.SeriesID != nil {
		if k < 100 {
			k = 100
		}
	}

	authorID := ""
	if filter.AuthorID != nil {
		authorID = *filter.AuthorID
	}
	genreID := ""
	if filter.GenreID != nil {
		genreID = *filter.GenreID
	}
	seriesID := ""
	if filter.SeriesID != nil {
		seriesID = *filter.SeriesID
	}

	query := `
		SELECT
			b.id AS book_id,
			b.title AS book_title,
			b.cover_path AS cover_path,
			(SELECT a.name FROM authors a JOIN book_authors ba ON a.id = ba.author_id WHERE ba.book_id = b.id LIMIT 1) AS author_name,
			(SELECT s.name FROM series s JOIN book_series bs ON s.id = bs.series_id WHERE bs.book_id = b.id LIMIT 1) AS series_name,
			(SELECT bs.sequence_number FROM book_series bs WHERE bs.book_id = b.id LIMIT 1) AS series_index,
			c.id AS chapter_id,
			c.chapter_index AS chapter_index,
			c.title AS chapter_title,
			c.summary AS chapter_summary,
			vec_distance_cosine(v.embedding, ?) AS distance
		FROM vec_chapters v
		JOIN chapters c ON v.chapter_id = c.id
		JOIN books b ON c.book_id = b.id
		WHERE v.embedding MATCH ? AND k = ?
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query,
		blob, blob, k,
		authorID, authorID,
		genreID, genreID,
		seriesID, seriesID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("executing vector search query: %w", err)
	}
	defer rows.Close()

	var hits []*SearchHit
	for rows.Next() {
		hit := &SearchHit{}
		if err := rows.Scan(
			&hit.BookID,
			&hit.BookTitle,
			&hit.CoverPath,
			&hit.AuthorName,
			&hit.SeriesName,
			&hit.SeriesIndex,
			&hit.ChapterID,
			&hit.ChapterIndex,
			&hit.ChapterTitle,
			&hit.Summary,
			&hit.Distance,
		); err != nil {
			return nil, fmt.Errorf("scanning search hit: %w", err)
		}
		hits = append(hits, hit)
	}

	return hits, rows.Err()
}

// --- Users & Tokens ---

func (r *SQLiteStorageEngine) CreateUser(ctx context.Context, u *User) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, u.ID, u.Username, u.PasswordHash, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, created_at
		FROM users WHERE username = ?
	`
	u := &User{}
	err := r.db.QueryRowContext(ctx, query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user by username: %w", err)
	}
	return u, nil
}

func (r *SQLiteStorageEngine) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, username, password_hash, created_at
		FROM users WHERE id = ?
	`
	u := &User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user by id: %w", err)
	}
	return u, nil
}

func (r *SQLiteStorageEngine) CreateAPIToken(ctx context.Context, t *APIToken) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO api_tokens (id, user_id, token_hash, name, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, t.ID, t.UserID, t.TokenHash, t.Name, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating api token: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetAPITokenByHash(ctx context.Context, tokenHash string) (*APIToken, error) {
	query := `
		SELECT id, user_id, token_hash, name, created_at
		FROM api_tokens WHERE token_hash = ?
	`
	t := &APIToken{}
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Name, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying api token by hash: %w", err)
	}
	return t, nil
}

func (r *SQLiteStorageEngine) DeleteAPIToken(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM api_tokens WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting api token: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete token result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
