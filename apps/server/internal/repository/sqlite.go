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
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/ulid"
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

	if b.Layout == "" {
		b.Layout = "reflowable"
	}
	if b.RenditionSpread == "" {
		b.RenditionSpread = "auto"
	}
	if b.RenditionOrientation == "" {
		b.RenditionOrientation = "auto"
	}
	if b.PageProgressionDirection == "" {
		b.PageProgressionDirection = "ltr"
	}

	query := `
		INSERT INTO books (id, title, description, language, publisher, identifier, file_path, cover_path, file_size_bytes, file_modified_at, published_date, layout, rendition_spread, rendition_orientation, page_progression_direction, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		b.ID, b.Title, b.Description, b.Language, b.Publisher, b.Identifier,
		b.FilePath, b.CoverPath, b.FileSizeBytes, b.FileModifiedAt, b.PublishedDate,
		b.Layout, b.RenditionSpread, b.RenditionOrientation, b.PageProgressionDirection, b.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating book: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetBookByID(ctx context.Context, id string) (*Book, error) {
	query := `
		SELECT id, title, description, language, publisher, identifier, file_path, cover_path, file_size_bytes, file_modified_at, published_date, layout, rendition_spread, rendition_orientation, page_progression_direction, created_at
		FROM books WHERE id = ?
	`
	b := &Book{}
	var fileModAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.Title, &b.Description, &b.Language, &b.Publisher, &b.Identifier,
		&b.FilePath, &b.CoverPath, &b.FileSizeBytes, &fileModAt, &b.PublishedDate,
		&b.Layout, &b.RenditionSpread, &b.RenditionOrientation, &b.PageProgressionDirection, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying book by id: %w", err)
	}
	if fileModAt.Valid {
		b.FileModifiedAt = &fileModAt.Time
	}
	return b, nil
}

func (r *SQLiteStorageEngine) GetBookByFilePath(ctx context.Context, filePath string) (*Book, error) {
	query := `
		SELECT id, title, description, language, publisher, identifier, file_path, cover_path, file_size_bytes, file_modified_at, published_date, layout, rendition_spread, rendition_orientation, page_progression_direction, created_at
		FROM books WHERE file_path = ?
	`
	b := &Book{}
	var fileModAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, filePath).Scan(
		&b.ID, &b.Title, &b.Description, &b.Language, &b.Publisher, &b.Identifier,
		&b.FilePath, &b.CoverPath, &b.FileSizeBytes, &fileModAt, &b.PublishedDate,
		&b.Layout, &b.RenditionSpread, &b.RenditionOrientation, &b.PageProgressionDirection, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying book by file path: %w", err)
	}
	if fileModAt.Valid {
		b.FileModifiedAt = &fileModAt.Time
	}
	return b, nil
}

func (r *SQLiteStorageEngine) UpdateBook(ctx context.Context, b *Book) error {
	if b.Layout == "" {
		b.Layout = "reflowable"
	}
	if b.RenditionSpread == "" {
		b.RenditionSpread = "auto"
	}
	if b.RenditionOrientation == "" {
		b.RenditionOrientation = "auto"
	}
	if b.PageProgressionDirection == "" {
		b.PageProgressionDirection = "ltr"
	}

	query := `
		UPDATE books
		SET title = ?, description = ?, language = ?, publisher = ?, identifier = ?,
		    file_path = ?, cover_path = ?, file_size_bytes = ?, file_modified_at = ?, published_date = ?,
		    layout = ?, rendition_spread = ?, rendition_orientation = ?, page_progression_direction = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query,
		b.Title, b.Description, b.Language, b.Publisher, b.Identifier,
		b.FilePath, b.CoverPath, b.FileSizeBytes, b.FileModifiedAt, b.PublishedDate,
		b.Layout, b.RenditionSpread, b.RenditionOrientation, b.PageProgressionDirection, b.ID,
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
	var joins string

	if filter.SeriesID != nil {
		joins = " JOIN book_series bs ON bs.book_id = b.id AND bs.series_id = ?"
		args = append(args, *filter.SeriesID)
	}
	if filter.AuthorID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?)")
		args = append(args, *filter.AuthorID)
	}
	if filter.GenreID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?)")
		args = append(args, *filter.GenreID)
	}
	if filter.TopicID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_topics WHERE topic_id = ?)")
		args = append(args, *filter.TopicID)
	}
	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		conditions = append(conditions, "(LOWER(b.title) LIKE ? OR LOWER(b.description) LIKE ?)")
		term := "%" + strings.ToLower(strings.TrimSpace(*filter.Search)) + "%"
		args = append(args, term, term)
	}

	query := `
		SELECT b.id, b.title, b.description, b.language, b.publisher, b.identifier, b.file_path, b.cover_path, b.file_size_bytes, b.file_modified_at, b.published_date, b.layout, b.rendition_spread, b.rendition_orientation, b.page_progression_direction, b.created_at
		FROM books b
	` + joins
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if filter.SeriesID != nil {
		query += " ORDER BY CASE WHEN bs.sequence_number IS NULL THEN 1 ELSE 0 END ASC, bs.sequence_number ASC, LOWER(b.title) ASC"
	} else if filter.SortBy == "created_at" && strings.ToLower(filter.SortOrder) == "desc" {
		query += " ORDER BY b.created_at DESC"
	} else {
		query += " ORDER BY LOWER(b.title) ASC, b.created_at DESC"
	}

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
		var fileModAt sql.NullTime
		err := rows.Scan(
			&b.ID, &b.Title, &b.Description, &b.Language, &b.Publisher, &b.Identifier,
			&b.FilePath, &b.CoverPath, &b.FileSizeBytes, &fileModAt, &b.PublishedDate,
			&b.Layout, &b.RenditionSpread, &b.RenditionOrientation, &b.PageProgressionDirection, &b.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning book row: %w", err)
		}
		if fileModAt.Valid {
			b.FileModifiedAt = &fileModAt.Time
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
	var photoURL sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT id, name, photo_url, created_at FROM authors WHERE id = ?", id).Scan(&a.ID, &a.Name, &photoURL, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying author by id: %w", err)
	}
	if photoURL.Valid {
		a.PhotoURL = &photoURL.String
	}
	return a, nil
}

func (r *SQLiteStorageEngine) GetAuthorByName(ctx context.Context, name string) (*Author, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, ErrNotFound
	}
	a := &Author{}
	var photoURL sql.NullString
	err := r.db.QueryRowContext(ctx, "SELECT id, name, photo_url, created_at FROM authors WHERE name = ? COLLATE NOCASE", cleanName).Scan(&a.ID, &a.Name, &photoURL, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = r.db.QueryRowContext(ctx, "SELECT id, name, photo_url, created_at FROM authors WHERE name LIKE ? ORDER BY LENGTH(name) ASC LIMIT 1", "%"+cleanName+"%").Scan(&a.ID, &a.Name, &photoURL, &a.CreatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying author by name: %w", err)
	}
	if photoURL.Valid {
		a.PhotoURL = &photoURL.String
	}
	return a, nil
}

func (r *SQLiteStorageEngine) ListAuthors(ctx context.Context) ([]*Author, error) {
	query := `
		SELECT a.id, a.name, a.photo_url, a.created_at, COUNT(ba.book_id) AS book_count
		FROM authors a
		LEFT JOIN book_authors ba ON ba.author_id = a.id
		GROUP BY a.id, a.name, a.photo_url, a.created_at
		ORDER BY a.name COLLATE NOCASE ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing authors: %w", err)
	}
	defer rows.Close()

	var authors []*Author
	for rows.Next() {
		a := &Author{}
		var photoURL sql.NullString
		if err := rows.Scan(&a.ID, &a.Name, &photoURL, &a.CreatedAt, &a.BookCount); err != nil {
			return nil, fmt.Errorf("scanning author: %w", err)
		}
		if photoURL.Valid {
			a.PhotoURL = &photoURL.String
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
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, ErrNotFound
	}
	g := &Genre{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM genres WHERE name = ? COLLATE NOCASE", cleanName).Scan(&g.ID, &g.Name, &g.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM genres WHERE name LIKE ? ORDER BY LENGTH(name) ASC LIMIT 1", "%"+cleanName+"%").Scan(&g.ID, &g.Name, &g.CreatedAt)
	}
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

// --- Topics ---

func (r *SQLiteStorageEngine) UpsertTopic(ctx context.Context, name string) (*Topic, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("topic name cannot be empty")
	}

	query := `
		INSERT INTO topics (id, name, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET name = excluded.name
		RETURNING id, name, created_at
	`
	t := &Topic{}
	err := r.db.QueryRowContext(ctx, query, uuid.NewString(), trimmed).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting topic: %w", err)
	}
	return t, nil
}

func (r *SQLiteStorageEngine) GetTopicByID(ctx context.Context, id string) (*Topic, error) {
	t := &Topic{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM topics WHERE id = ?", id).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying topic by id: %w", err)
	}
	return t, nil
}

func (r *SQLiteStorageEngine) GetTopicByName(ctx context.Context, name string) (*Topic, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, ErrNotFound
	}
	t := &Topic{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM topics WHERE name = ? COLLATE NOCASE", cleanName).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = r.db.QueryRowContext(ctx, "SELECT id, name, created_at FROM topics WHERE name LIKE ? ORDER BY LENGTH(name) ASC LIMIT 1", "%"+cleanName+"%").Scan(&t.ID, &t.Name, &t.CreatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying topic by name: %w", err)
	}
	return t, nil
}

func (r *SQLiteStorageEngine) ListTopics(ctx context.Context) ([]*Topic, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, created_at FROM topics ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, fmt.Errorf("listing topics: %w", err)
	}
	defer rows.Close()

	var topics []*Topic
	for rows.Next() {
		t := &Topic{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
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
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, ErrNotFound
	}
	s := &Series{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM series WHERE name = ? COLLATE NOCASE", cleanName).Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = r.db.QueryRowContext(ctx, "SELECT id, name, description, created_at FROM series WHERE name LIKE ? ORDER BY LENGTH(name) ASC LIMIT 1", "%"+cleanName+"%").Scan(&s.ID, &s.Name, &s.Description, &s.CreatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying series by name: %w", err)
	}
	return s, nil
}

func (r *SQLiteStorageEngine) ListSeries(ctx context.Context) ([]*Series, error) {
	query := `
		SELECT s.id, s.name, s.description, s.created_at, COUNT(bs.book_id) AS book_count, MIN(bs.book_id) AS cover_book_id
		FROM series s
		LEFT JOIN book_series bs ON bs.series_id = s.id
		GROUP BY s.id, s.name, s.description, s.created_at
		ORDER BY s.name COLLATE NOCASE ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing series: %w", err)
	}
	defer rows.Close()

	var seriesList []*Series
	for rows.Next() {
		s := &Series{}
		var desc sql.NullString
		var coverBookID sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &desc, &s.CreatedAt, &s.BookCount, &coverBookID); err != nil {
			return nil, fmt.Errorf("scanning series: %w", err)
		}
		if desc.Valid {
			s.Description = &desc.String
		}
		if coverBookID.Valid {
			s.CoverBookID = &coverBookID.String
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

func (r *SQLiteStorageEngine) UnlinkBookGenre(ctx context.Context, bookID, genreID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM book_genres WHERE book_id = ? AND genre_id = ?", bookID, genreID)
	if err != nil {
		return fmt.Errorf("unlinking book genre: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) LinkBookTopic(ctx context.Context, bookID, topicID string) error {
	query := `
		INSERT INTO book_topics (book_id, topic_id, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(book_id, topic_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, bookID, topicID)
	if err != nil {
		return fmt.Errorf("linking book topic: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) UnlinkBookTopic(ctx context.Context, bookID, topicID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM book_topics WHERE book_id = ? AND topic_id = ?", bookID, topicID)
	if err != nil {
		return fmt.Errorf("unlinking book topic: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetBookTopics(ctx context.Context, bookID string) ([]*Topic, error) {
	query := `
		SELECT t.id, t.name, t.created_at
		FROM topics t
		JOIN book_topics bt ON bt.topic_id = t.id
		WHERE bt.book_id = ?
		ORDER BY t.name COLLATE NOCASE ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book topics: %w", err)
	}
	defer rows.Close()

	var topics []*Topic
	for rows.Next() {
		t := &Topic{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning book topic: %w", err)
		}
		topics = append(topics, t)
	}
	if topics == nil {
		topics = []*Topic{}
	}
	return topics, rows.Err()
}

func (r *SQLiteStorageEngine) PruneOrphanedGenres(ctx context.Context) (int, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM genres WHERE id NOT IN (SELECT DISTINCT genre_id FROM book_genres)")
	if err != nil {
		return 0, fmt.Errorf("pruning orphaned genres: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
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
		c.ID = ulid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO chapters (id, book_id, chapter_index, title, summary, content_plain, href, page_width, page_height, page_spread, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.BookID, c.ChapterIndex, c.Title, c.Summary, c.ContentPlain,
		c.Href, c.PageWidth, c.PageHeight, c.PageSpread, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating chapter: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetChaptersByBookID(ctx context.Context, bookID string) ([]*Chapter, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, href, page_width, page_height, page_spread, created_at
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
		var href, pageSpread sql.NullString
		var pageWidth, pageHeight sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain,
			&href, &pageWidth, &pageHeight, &pageSpread, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning chapter row: %w", err)
		}
		if href.Valid {
			c.Href = &href.String
		}
		if pageWidth.Valid {
			w := int(pageWidth.Int64)
			c.PageWidth = &w
		}
		if pageHeight.Valid {
			h := int(pageHeight.Int64)
			c.PageHeight = &h
		}
		if pageSpread.Valid {
			c.PageSpread = &pageSpread.String
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

func (r *SQLiteStorageEngine) GetBookSpine(ctx context.Context, bookID string) ([]*SpineItem, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, href, page_width, page_height, page_spread
		FROM chapters
		WHERE book_id = ?
		ORDER BY chapter_index ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("getting book spine: %w", err)
	}
	defer rows.Close()

	var spine []*SpineItem
	for rows.Next() {
		item := &SpineItem{}
		var href, pageSpread sql.NullString
		var pageWidth, pageHeight sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.BookID, &item.ChapterIndex, &item.Title, &item.Summary,
			&href, &pageWidth, &pageHeight, &pageSpread,
		); err != nil {
			return nil, fmt.Errorf("scanning spine row: %w", err)
		}
		if href.Valid {
			item.Href = &href.String
		}
		if pageWidth.Valid {
			w := int(pageWidth.Int64)
			item.PageWidth = &w
		}
		if pageHeight.Valid {
			h := int(pageHeight.Int64)
			item.PageHeight = &h
		}
		if pageSpread.Valid {
			item.PageSpread = &pageSpread.String
		}
		spine = append(spine, item)
	}
	return spine, rows.Err()
}

func (r *SQLiteStorageEngine) GetChapterByID(ctx context.Context, id string) (*Chapter, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, href, page_width, page_height, page_spread, created_at
		FROM chapters WHERE id = ?
	`
	c := &Chapter{}
	var href, pageSpread sql.NullString
	var pageWidth, pageHeight sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain,
		&href, &pageWidth, &pageHeight, &pageSpread, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying chapter by id: %w", err)
	}
	if href.Valid {
		c.Href = &href.String
	}
	if pageWidth.Valid {
		w := int(pageWidth.Int64)
		c.PageWidth = &w
	}
	if pageHeight.Valid {
		h := int(pageHeight.Int64)
		c.PageHeight = &h
	}
	if pageSpread.Valid {
		c.PageSpread = &pageSpread.String
	}
	return c, nil
}

func (r *SQLiteStorageEngine) GetChapterByBookAndIndex(ctx context.Context, bookID string, chapterIndex int) (*Chapter, error) {
	query := `
		SELECT id, book_id, chapter_index, title, summary, content_plain, href, page_width, page_height, page_spread, created_at
		FROM chapters WHERE book_id = ? AND chapter_index = ?
	`
	c := &Chapter{}
	var href, pageSpread sql.NullString
	var pageWidth, pageHeight sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, bookID, chapterIndex).Scan(
		&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain,
		&href, &pageWidth, &pageHeight, &pageSpread, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying chapter by book and index: %w", err)
	}
	if href.Valid {
		c.Href = &href.String
	}
	if pageWidth.Valid {
		w := int(pageWidth.Int64)
		c.PageWidth = &w
	}
	if pageHeight.Valid {
		h := int(pageHeight.Int64)
		c.PageHeight = &h
	}
	if pageSpread.Valid {
		c.PageSpread = &pageSpread.String
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

func (r *SQLiteStorageEngine) UpdateChapterContent(ctx context.Context, chapterID string, contentPlain string) error {
	query := `UPDATE chapters SET content_plain = ? WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, contentPlain, chapterID)
	if err != nil {
		return fmt.Errorf("updating chapter content: %w", err)
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

func (r *SQLiteStorageEngine) UpdateChapter(ctx context.Context, c *Chapter) error {
	query := `
		UPDATE chapters
		SET chapter_index = ?, title = ?, summary = ?, content_plain = ?, href = ?, page_width = ?, page_height = ?, page_spread = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query,
		c.ChapterIndex, c.Title, c.Summary, c.ContentPlain,
		c.Href, c.PageWidth, c.PageHeight, c.PageSpread, c.ID,
	)
	if err != nil {
		return fmt.Errorf("updating chapter: %w", err)
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
		SELECT id, book_id, chapter_index, title, summary, content_plain, href, page_width, page_height, page_spread, created_at
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
		var href, pageSpread sql.NullString
		var pageWidth, pageHeight sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.BookID, &c.ChapterIndex, &c.Title, &c.Summary, &c.ContentPlain,
			&href, &pageWidth, &pageHeight, &pageSpread, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning unindexed chapter: %w", err)
		}
		if href.Valid {
			c.Href = &href.String
		}
		if pageWidth.Valid {
			w := int(pageWidth.Int64)
			c.PageWidth = &w
		}
		if pageHeight.Valid {
			h := int(pageHeight.Int64)
			c.PageHeight = &h
		}
		if pageSpread.Valid {
			c.PageSpread = &pageSpread.String
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

func (r *SQLiteStorageEngine) InsertChapterVector(ctx context.Context, chapterID string, embedding []float32) error {
	paras, err := r.GetParagraphsByChapterID(ctx, chapterID)
	if err != nil || len(paras) == 0 {
		ch, chErr := r.GetChapterByID(ctx, chapterID)
		if chErr != nil {
			return chErr
		}
		p := &Paragraph{
			BookID:         ch.BookID,
			ChapterID:      ch.ID,
			ChapterIndex:   ch.ChapterIndex,
			StartParagraph: 1,
			EndParagraph:   1,
			Content:        ch.ContentPlain,
		}
		if err := r.CreateParagraphs(ctx, []*Paragraph{p}); err != nil {
			return err
		}
		paras = []*Paragraph{p}
	}
	return r.InsertParagraphVector(ctx, paras[0].ID, embedding)
}

func (r *SQLiteStorageEngine) SearchVectorChapters(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error) {
	return r.SearchVectorParagraphs(ctx, queryEmbedding, filter)
}

func (r *SQLiteStorageEngine) CountBooks(ctx context.Context, filter BookFilter) (int, error) {
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
	if filter.TopicID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_topics WHERE topic_id = ?)")
		args = append(args, *filter.TopicID)
	}
	if filter.SeriesID != nil {
		conditions = append(conditions, "b.id IN (SELECT book_id FROM book_series WHERE series_id = ?)")
		args = append(args, *filter.SeriesID)
	}
	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		conditions = append(conditions, "b.title LIKE ?")
		args = append(args, "%"+strings.TrimSpace(*filter.Search)+"%")
	}

	query := "SELECT COUNT(*) FROM books b"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting books: %w", err)
	}
	return count, nil
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

func (r *SQLiteStorageEngine) UpdateUserPassword(ctx context.Context, userID string, passwordHash string) error {
	query := `
		UPDATE users
		SET password_hash = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("updating user password: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
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

func (r *SQLiteStorageEngine) ListAPITokensByUserID(ctx context.Context, userID string) ([]*APIToken, error) {
	query := `
		SELECT id, user_id, token_hash, name, created_at
		FROM api_tokens
		WHERE user_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*APIToken
	for rows.Next() {
		t := &APIToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning api token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
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

// --- Upload Jobs ---

func (r *SQLiteStorageEngine) CreateUploadJob(ctx context.Context, job *UploadJob) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}
	if job.Status == "" {
		job.Status = "queued"
	}

	query := `
		INSERT INTO upload_jobs (id, filename, staged_path, status, metadata, has_cover, book_id, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.Filename, job.StagedPath, job.Status,
		job.Metadata, job.HasCover, job.BookID, job.ErrorMessage, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating upload job: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetUploadJob(ctx context.Context, id string) (*UploadJob, error) {
	query := `
		SELECT id, filename, staged_path, status, metadata, has_cover, book_id, error_message, created_at, updated_at
		FROM upload_jobs
		WHERE id = ?
	`
	j := &UploadJob{}
	var meta sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&j.ID, &j.Filename, &j.StagedPath, &j.Status,
		&meta, &j.HasCover, &j.BookID, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting upload job: %w", err)
	}
	if meta.Valid {
		j.Metadata = &meta.String
	}
	return j, nil
}

func (r *SQLiteStorageEngine) UpdateUploadJobStatus(ctx context.Context, id string, status string, bookID *string, errMessage *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE upload_jobs
		SET status = ?, book_id = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query, status, bookID, errMessage, now, id)
	if err != nil {
		return fmt.Errorf("updating upload job status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update upload job result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) UpdateUploadJobCommit(ctx context.Context, id string, status string, metadata *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE upload_jobs
		SET status = ?, metadata = COALESCE(?, metadata), updated_at = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query, status, metadata, now, id)
	if err != nil {
		return fmt.Errorf("updating upload job commit: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update upload job commit result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) UpdateUploadJobCover(ctx context.Context, id string, hasCover bool) error {
	now := time.Now().UTC()
	query := `
		UPDATE upload_jobs
		SET has_cover = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query, hasCover, now, id)
	if err != nil {
		return fmt.Errorf("updating upload job cover: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update upload job cover result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) DeleteUploadJob(ctx context.Context, id string) error {
	query := `DELETE FROM upload_jobs WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting upload job: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete upload job result: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) GetPendingUploadJobs(ctx context.Context, limit int) ([]*UploadJob, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT id, filename, staged_path, status, metadata, has_cover, book_id, error_message, created_at, updated_at
		FROM upload_jobs
		WHERE status IN ('queued', 'processing')
		ORDER BY created_at ASC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending upload jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*UploadJob
	for rows.Next() {
		j := &UploadJob{}
		var meta sql.NullString
		if err := rows.Scan(&j.ID, &j.Filename, &j.StagedPath, &j.Status, &meta, &j.HasCover, &j.BookID, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning upload job: %w", err)
		}
		if meta.Valid {
			j.Metadata = &meta.String
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *SQLiteStorageEngine) ListUploadJobs(ctx context.Context, limit int, status ...string) ([]*UploadJob, error) {
	if limit <= 0 {
		limit = 20
	}
	var query string
	var args []any
	if len(status) > 0 && status[0] != "" {
		query = `
			SELECT id, filename, staged_path, status, metadata, has_cover, book_id, error_message, created_at, updated_at
			FROM upload_jobs
			WHERE status = ?
			ORDER BY created_at DESC
			LIMIT ?
		`
		args = []any{status[0], limit}
	} else {
		query = `
			SELECT id, filename, staged_path, status, metadata, has_cover, book_id, error_message, created_at, updated_at
			FROM upload_jobs
			ORDER BY created_at DESC
			LIMIT ?
		`
		args = []any{limit}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying upload jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*UploadJob
	for rows.Next() {
		j := &UploadJob{}
		var meta sql.NullString
		if err := rows.Scan(&j.ID, &j.Filename, &j.StagedPath, &j.Status, &meta, &j.HasCover, &j.BookID, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning upload job: %w", err)
		}
		if meta.Valid {
			j.Metadata = &meta.String
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func (r *SQLiteStorageEngine) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	status := &QueueStatus{}

	var totalParagraphs, pendingParagraphs int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM paragraphs").Scan(&totalParagraphs); err != nil {
		return nil, fmt.Errorf("counting total paragraphs: %w", err)
	}

	if totalParagraphs > 0 {
		query := `
			SELECT COUNT(*)
			FROM paragraphs p
			LEFT JOIN vec_paragraphs v ON p.id = v.paragraph_id
			WHERE v.paragraph_id IS NULL
		`
		if err := r.db.QueryRowContext(ctx, query).Scan(&pendingParagraphs); err != nil {
			return nil, fmt.Errorf("counting pending paragraphs: %w", err)
		}
		status.TotalChapters = totalParagraphs
		status.PendingChapters = pendingParagraphs
		status.IndexedChapters = totalParagraphs - pendingParagraphs
		status.ProgressPercent = (float64(status.IndexedChapters) / float64(totalParagraphs)) * 100.0
	} else {
		// Fallback when no paragraphs have been chunked yet but chapters exist
		var totalChapters int
		if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chapters").Scan(&totalChapters); err != nil {
			return nil, fmt.Errorf("counting total chapters: %w", err)
		}
		status.TotalChapters = totalChapters
		if totalChapters > 0 {
			status.PendingChapters = totalChapters
			status.IndexedChapters = 0
			status.ProgressPercent = 0.0
		} else {
			status.ProgressPercent = 100.0
		}
	}

	// Pending uploads
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM upload_jobs WHERE status IN ('queued', 'processing')").Scan(&status.PendingUploads); err != nil {
		return nil, fmt.Errorf("counting pending uploads: %w", err)
	}

	// Staged uploads awaiting review
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM upload_jobs WHERE status = 'staged'").Scan(&status.StagedUploads); err != nil {
		return nil, fmt.Errorf("counting staged uploads: %w", err)
	}

	status.IsActive = status.PendingChapters > 0 || status.PendingUploads > 0

	return status, nil
}

func (r *SQLiteStorageEngine) CreateBookmark(ctx context.Context, bookmark *Bookmark) error {
	if bookmark.ID == "" {
		bookmark.ID = uuid.NewString()
	}
	if bookmark.CreatedAt.IsZero() {
		bookmark.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO bookmarks (id, book_id, chapter_id, title, progress, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		bookmark.ID,
		bookmark.BookID,
		bookmark.ChapterID,
		bookmark.Title,
		bookmark.Progress,
		bookmark.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting bookmark: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) ListBookmarksByBookID(ctx context.Context, bookID string) ([]*Bookmark, error) {
	query := `
		SELECT id, book_id, chapter_id, title, progress, created_at
		FROM bookmarks
		WHERE book_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("querying bookmarks: %w", err)
	}
	defer rows.Close()

	var bookmarks []*Bookmark
	for rows.Next() {
		b := &Bookmark{}
		if err := rows.Scan(&b.ID, &b.BookID, &b.ChapterID, &b.Title, &b.Progress, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning bookmark: %w", err)
		}
		bookmarks = append(bookmarks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating bookmarks: %w", err)
	}
	if bookmarks == nil {
		bookmarks = []*Bookmark{}
	}
	return bookmarks, nil
}

func (r *SQLiteStorageEngine) DeleteBookmark(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM bookmarks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting bookmark: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteStorageEngine) CreateHighlight(ctx context.Context, highlight *Highlight) error {
	if highlight.ID == "" {
		highlight.ID = uuid.NewString()
	}
	if highlight.CreatedAt.IsZero() {
		highlight.CreatedAt = time.Now()
	}
	if highlight.Color == "" {
		highlight.Color = "yellow"
	}

	query := `
		INSERT INTO highlights (id, book_id, chapter_id, selected_text, note, color, start_offset, end_offset, start_paragraph, end_paragraph, location, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		highlight.ID,
		highlight.BookID,
		highlight.ChapterID,
		highlight.SelectedText,
		highlight.Note,
		highlight.Color,
		highlight.StartOffset,
		highlight.EndOffset,
		highlight.StartParagraph,
		highlight.EndParagraph,
		highlight.Location,
		highlight.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting highlight: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) ListHighlightsByBookID(ctx context.Context, bookID string) ([]*Highlight, error) {
	query := `
		SELECT id, book_id, chapter_id, selected_text, note, color, start_offset, end_offset, start_paragraph, end_paragraph, location, created_at
		FROM highlights
		WHERE book_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("querying highlights: %w", err)
	}
	defer rows.Close()

	var highlights []*Highlight
	for rows.Next() {
		h := &Highlight{}
		if err := rows.Scan(
			&h.ID,
			&h.BookID,
			&h.ChapterID,
			&h.SelectedText,
			&h.Note,
			&h.Color,
			&h.StartOffset,
			&h.EndOffset,
			&h.StartParagraph,
			&h.EndParagraph,
			&h.Location,
			&h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning highlight: %w", err)
		}
		highlights = append(highlights, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating highlights: %w", err)
	}
	if highlights == nil {
		highlights = []*Highlight{}
	}
	return highlights, nil
}

func (r *SQLiteStorageEngine) DeleteHighlight(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM highlights WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting highlight: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Paragraphs & Passage Retrieval ---

func (r *SQLiteStorageEngine) CreateParagraphs(ctx context.Context, paragraphs []*Paragraph) error {
	if len(paragraphs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO paragraphs (id, book_id, chapter_id, chapter_index, start_paragraph, end_paragraph, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing paragraph insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, p := range paragraphs {
		if p.ID == "" {
			p.ID = ulid.New()
		}
		if p.CreatedAt.IsZero() {
			p.CreatedAt = now
		}
		_, err := stmt.ExecContext(ctx,
			p.ID,
			p.BookID,
			p.ChapterID,
			p.ChapterIndex,
			p.StartParagraph,
			p.EndParagraph,
			p.Content,
			p.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting paragraph %s: %w", p.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing paragraphs: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetParagraphsByBookID(ctx context.Context, bookID string) ([]*Paragraph, error) {
	query := `
		SELECT id, book_id, chapter_id, chapter_index, start_paragraph, end_paragraph, content, created_at
		FROM paragraphs
		WHERE book_id = ?
		ORDER BY chapter_index ASC, start_paragraph ASC
	`
	rows, err := r.db.QueryContext(ctx, query, bookID)
	if err != nil {
		return nil, fmt.Errorf("querying paragraphs by book ID: %w", err)
	}
	defer rows.Close()

	var paras []*Paragraph
	for rows.Next() {
		p := &Paragraph{}
		if err := rows.Scan(&p.ID, &p.BookID, &p.ChapterID, &p.ChapterIndex, &p.StartParagraph, &p.EndParagraph, &p.Content, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning paragraph: %w", err)
		}
		paras = append(paras, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating paragraphs: %w", err)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, nil
}

func (r *SQLiteStorageEngine) GetParagraphsByChapterID(ctx context.Context, chapterID string) ([]*Paragraph, error) {
	query := `
		SELECT id, book_id, chapter_id, chapter_index, start_paragraph, end_paragraph, content, created_at
		FROM paragraphs
		WHERE chapter_id = ?
		ORDER BY start_paragraph ASC
	`
	rows, err := r.db.QueryContext(ctx, query, chapterID)
	if err != nil {
		return nil, fmt.Errorf("querying paragraphs by chapter ID: %w", err)
	}
	defer rows.Close()

	var paras []*Paragraph
	for rows.Next() {
		p := &Paragraph{}
		if err := rows.Scan(&p.ID, &p.BookID, &p.ChapterID, &p.ChapterIndex, &p.StartParagraph, &p.EndParagraph, &p.Content, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning paragraph: %w", err)
		}
		paras = append(paras, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating paragraphs: %w", err)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, nil
}

func (r *SQLiteStorageEngine) GetUnindexedParagraphs(ctx context.Context, limit int) ([]*Paragraph, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT p.id, p.book_id, p.chapter_id, p.chapter_index, p.start_paragraph, p.end_paragraph, p.content, p.created_at
		FROM paragraphs p
		LEFT JOIN vec_paragraphs v ON p.id = v.paragraph_id
		WHERE v.paragraph_id IS NULL
		ORDER BY p.created_at ASC, p.chapter_index ASC, p.start_paragraph ASC
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying unindexed paragraphs: %w", err)
	}
	defer rows.Close()

	var paras []*Paragraph
	for rows.Next() {
		p := &Paragraph{}
		if err := rows.Scan(&p.ID, &p.BookID, &p.ChapterID, &p.ChapterIndex, &p.StartParagraph, &p.EndParagraph, &p.Content, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning unindexed paragraph: %w", err)
		}
		paras = append(paras, p)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, rows.Err()
}

func (r *SQLiteStorageEngine) DeleteParagraphsByBookID(ctx context.Context, bookID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM paragraphs WHERE book_id = ?", bookID)
	if err != nil {
		return fmt.Errorf("deleting paragraphs by book ID: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) InsertParagraphVectors(ctx context.Context, items []ParagraphVector) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	delStmt, err := tx.PrepareContext(ctx, "DELETE FROM vec_paragraphs WHERE paragraph_id = ?")
	if err != nil {
		return fmt.Errorf("preparing delete stmt: %w", err)
	}
	defer delStmt.Close()

	insStmt, err := tx.PrepareContext(ctx, "INSERT INTO vec_paragraphs (paragraph_id, embedding) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("preparing insert stmt: %w", err)
	}
	defer insStmt.Close()

	for _, item := range items {
		blob, err := sqlite_vec.SerializeFloat32(item.Embedding)
		if err != nil {
			return fmt.Errorf("serializing embedding: %w", err)
		}

		if _, err := delStmt.ExecContext(ctx, item.ParagraphID); err != nil {
			return fmt.Errorf("deleting existing paragraph vector: %w", err)
		}

		if _, err := insStmt.ExecContext(ctx, item.ParagraphID, blob); err != nil {
			return fmt.Errorf("inserting paragraph vector: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing batch paragraph vectors: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) InsertParagraphVector(ctx context.Context, paragraphID string, embedding []float32) error {
	return r.InsertParagraphVectors(ctx, []ParagraphVector{{ParagraphID: paragraphID, Embedding: embedding}})
}

func (r *SQLiteStorageEngine) SearchVectorParagraphs(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error) {
	blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("serializing query embedding: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	k := limit
	if filter.AuthorID != nil || filter.GenreID != nil || filter.TopicID != nil || filter.SeriesID != nil || filter.BookID != nil {
		if k < 100 {
			k = 100
		}
	}

	bookID := ""
	if filter.BookID != nil {
		bookID = *filter.BookID
	}
	authorID := ""
	if filter.AuthorID != nil {
		authorID = *filter.AuthorID
	}
	genreID := ""
	if filter.GenreID != nil {
		genreID = *filter.GenreID
	}
	topicID := ""
	if filter.TopicID != nil {
		topicID = *filter.TopicID
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
			p.start_paragraph AS start_paragraph,
			p.end_paragraph AS end_paragraph,
			p.content AS content,
			vec_distance_cosine(v.embedding, ?) AS distance
		FROM vec_paragraphs v
		JOIN paragraphs p ON v.paragraph_id = p.id
		JOIN chapters c ON p.chapter_id = c.id
		JOIN books b ON p.book_id = b.id
		WHERE v.embedding MATCH ? AND k = ?
		  AND (? = '' OR b.id = ?)
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_topics WHERE topic_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query,
		blob, blob, k,
		bookID, bookID,
		authorID, authorID,
		genreID, genreID,
		topicID, topicID,
		seriesID, seriesID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("executing vector paragraph search query: %w", err)
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
			&hit.StartParagraph,
			&hit.EndParagraph,
			&hit.Content,
			&hit.Distance,
		); err != nil {
			return nil, fmt.Errorf("scanning search hit: %w", err)
		}
		hit.Summary = hit.Content
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search hits: %w", err)
	}
	if hits == nil {
		hits = []*SearchHit{}
	}
	return hits, nil
}

func (r *SQLiteStorageEngine) SearchFTSParagraphs(ctx context.Context, queryText string, filter SearchFilter) ([]*SearchHit, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}

	bookID := ""
	if filter.BookID != nil {
		bookID = *filter.BookID
	}
	authorID := ""
	if filter.AuthorID != nil {
		authorID = *filter.AuthorID
	}
	genreID := ""
	if filter.GenreID != nil {
		genreID = *filter.GenreID
	}
	topicID := ""
	if filter.TopicID != nil {
		topicID = *filter.TopicID
	}
	seriesID := ""
	if filter.SeriesID != nil {
		seriesID = *filter.SeriesID
	}

	cleanQuery := strings.TrimSpace(queryText)
	if cleanQuery == "" {
		return []*SearchHit{}, nil
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
			p.start_paragraph AS start_paragraph,
			p.end_paragraph AS end_paragraph,
			p.content AS content,
			bm25(paragraphs_fts) AS rank
		FROM paragraphs_fts f
		JOIN paragraphs p ON f.paragraph_id = p.id
		JOIN chapters c ON p.chapter_id = c.id
		JOIN books b ON p.book_id = b.id
		WHERE paragraphs_fts MATCH ?
		  AND (? = '' OR b.id = ?)
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_topics WHERE topic_id = ?))
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
		ORDER BY rank ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query,
		cleanQuery,
		bookID, bookID,
		authorID, authorID,
		genreID, genreID,
		topicID, topicID,
		seriesID, seriesID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("executing FTS paragraph search query: %w", err)
	}
	defer rows.Close()

	var hits []*SearchHit
	for rows.Next() {
		hit := &SearchHit{}
		var rank float64
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
			&hit.StartParagraph,
			&hit.EndParagraph,
			&hit.Content,
			&rank,
		); err != nil {
			return nil, fmt.Errorf("scanning FTS search hit: %w", err)
		}
		hit.Distance = rank
		hit.Summary = hit.Content
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating FTS search hits: %w", err)
	}
	if hits == nil {
		hits = []*SearchHit{}
	}
	return hits, nil
}

func (r *SQLiteStorageEngine) BackfillParagraphs(ctx context.Context) (int, error) {
	query := `
		SELECT id, book_id, chapter_index, content_plain
		FROM chapters
		WHERE id NOT IN (SELECT DISTINCT chapter_id FROM paragraphs)
		ORDER BY book_id ASC, chapter_index ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("querying unmigrated chapters: %w", err)
	}
	defer rows.Close()

	type chInfo struct {
		id           string
		bookID       string
		chapterIndex int
		contentPlain string
	}

	var toChunk []chInfo
	for rows.Next() {
		var c chInfo
		if err := rows.Scan(&c.id, &c.bookID, &c.chapterIndex, &c.contentPlain); err != nil {
			return 0, fmt.Errorf("scanning chapter for backfill: %w", err)
		}
		toChunk = append(toChunk, c)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterating chapters for backfill: %w", err)
	}

	totalBackfilled := 0
	for _, c := range toChunk {
		chunks := epub.ChunkChapterParagraphs(c.contentPlain, 400, 1200)
		if len(chunks) == 0 {
			continue
		}
		var paras []*Paragraph
		for _, chk := range chunks {
			paras = append(paras, &Paragraph{
				BookID:         c.bookID,
				ChapterID:      c.id,
				ChapterIndex:   c.chapterIndex,
				StartParagraph: chk.StartParagraph,
				EndParagraph:   chk.EndParagraph,
				Content:        chk.Content,
			})
		}
		if err := r.CreateParagraphs(ctx, paras); err != nil {
			return totalBackfilled, fmt.Errorf("creating paragraphs for chapter %s: %w", c.id, err)
		}
		totalBackfilled += len(paras)
	}

	return totalBackfilled, nil
}

// --- OAuth 2.0 ---

func (r *SQLiteStorageEngine) CreateOAuthClient(ctx context.Context, client *OAuthClient) error {
	if client.CreatedAt.IsZero() {
		client.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_clients (id, client_secret, client_name, redirect_uris, grant_types, response_types, scope, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, client.ID, client.ClientSecret, client.ClientName, client.RedirectURIs, client.GrantTypes, client.ResponseTypes, client.Scope, client.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating oauth client: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetOAuthClientByID(ctx context.Context, id string) (*OAuthClient, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_secret, client_name, redirect_uris, grant_types, response_types, scope, created_at
		FROM oauth_clients WHERE id = ?
	`, id)
	client := new(OAuthClient)
	err := row.Scan(&client.ID, &client.ClientSecret, &client.ClientName, &client.RedirectURIs, &client.GrantTypes, &client.ResponseTypes, &client.Scope, &client.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting oauth client: %w", err)
	}
	return client, nil
}

func (r *SQLiteStorageEngine) CreateOAuthCode(ctx context.Context, code *OAuthCode) error {
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_codes (code, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, code.Code, code.ClientID, code.UserID, code.RedirectURI, code.CodeChallenge, code.CodeChallengeMethod, code.Scope, code.ExpiresAt, code.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating oauth code: %w", err)
	}
	return nil
}

func (r *SQLiteStorageEngine) GetOAuthCode(ctx context.Context, codeStr string) (*OAuthCode, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT code, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope, expires_at, created_at
		FROM oauth_codes WHERE code = ?
	`, codeStr)
	code := new(OAuthCode)
	err := row.Scan(&code.Code, &code.ClientID, &code.UserID, &code.RedirectURI, &code.CodeChallenge, &code.CodeChallengeMethod, &code.Scope, &code.ExpiresAt, &code.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting oauth code: %w", err)
	}
	return code, nil
}

func (r *SQLiteStorageEngine) DeleteOAuthCode(ctx context.Context, codeStr string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM oauth_codes WHERE code = ?", codeStr)
	if err != nil {
		return fmt.Errorf("deleting oauth code: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}



