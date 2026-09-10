package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/pgvector/pgvector-go"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/ulid"
)

// BunStorageEngine implements StorageEngine using Bun ORM, supporting both SQLite and PostgreSQL.
type BunStorageEngine struct {
	db *bun.DB
}

var _ StorageEngine = (*BunStorageEngine)(nil)

// NewBunStorageEngine creates a new BunStorageEngine instance.
func NewBunStorageEngine(db *bun.DB) *BunStorageEngine {
	return &BunStorageEngine{db: db}
}

// DB returns the underlying Bun DB handle.
func (r *BunStorageEngine) DB() *bun.DB {
	return r.db
}

// Close closes the database connection.
func (r *BunStorageEngine) Close() error {
	return r.db.Close()
}

func (r *BunStorageEngine) isPG() bool {
	return r.db.Dialect().Name() == dialect.PG
}

// --- Books ---

func (r *BunStorageEngine) CreateBook(ctx context.Context, book *Book) error {
	if book.ID == "" {
		book.ID = ulid.New()
	}
	if book.CreatedAt.IsZero() {
		book.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(book).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating book: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetBookByID(ctx context.Context, id string) (*Book, error) {
	book := new(Book)
	err := r.db.NewSelect().Model(book).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting book by ID: %w", err)
	}
	return book, nil
}

func (r *BunStorageEngine) GetBookByFilePath(ctx context.Context, filePath string) (*Book, error) {
	book := new(Book)
	err := r.db.NewSelect().Model(book).Where("file_path = ?", filePath).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting book by file path: %w", err)
	}
	return book, nil
}

func (r *BunStorageEngine) UpdateBook(ctx context.Context, book *Book) error {
	res, err := r.db.NewUpdate().Model(book).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating book: %w", err)
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

func (r *BunStorageEngine) DeleteBook(ctx context.Context, id string) error {
	res, err := r.db.NewDelete().Model((*Book)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting book: %w", err)
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

func (r *BunStorageEngine) applyBookFilter(q *bun.SelectQuery, filter BookFilter) *bun.SelectQuery {
	if filter.AuthorID != nil {
		q = q.Where("id IN (SELECT book_id FROM book_authors WHERE author_id = ?)", *filter.AuthorID)
	}
	if filter.GenreID != nil {
		q = q.Where("id IN (SELECT book_id FROM book_genres WHERE genre_id = ?)", *filter.GenreID)
	}
	if filter.SeriesID != nil {
		q = q.Where("id IN (SELECT book_id FROM book_series WHERE series_id = ?)", *filter.SeriesID)
	}
	if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
		term := "%" + strings.ToLower(strings.TrimSpace(*filter.Search)) + "%"
		q = q.Where("(LOWER(title) LIKE ? OR LOWER(description) LIKE ?)", term, term)
	}
	return q
}

func (r *BunStorageEngine) ListBooks(ctx context.Context, filter BookFilter) ([]*Book, error) {
	var books []*Book
	q := r.db.NewSelect().Model(&books)
	q = r.applyBookFilter(q, filter)

	if filter.SeriesID != nil {
		q = q.Join("JOIN book_series AS bs ON bs.book_id = book.id AND bs.series_id = ?", *filter.SeriesID)
		q = q.OrderExpr("CASE WHEN bs.sequence_number IS NULL THEN 1 ELSE 0 END ASC, bs.sequence_number ASC, LOWER(book.title) ASC")
	} else if filter.SortBy == "created_at" && strings.ToLower(filter.SortOrder) == "desc" {
		q = q.Order("created_at DESC")
	} else {
		q = q.OrderExpr("LOWER(book.title) ASC, book.created_at DESC")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	q = q.Limit(limit)

	if filter.Offset > 0 {
		q = q.Offset(filter.Offset)
	}

	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("listing books: %w", err)
	}
	if books == nil {
		books = []*Book{}
	}
	return books, nil
}

func (r *BunStorageEngine) CountBooks(ctx context.Context, filter BookFilter) (int, error) {
	q := r.db.NewSelect().Model((*Book)(nil))
	q = r.applyBookFilter(q, filter)
	count, err := q.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting books: %w", err)
	}
	return count, nil
}

// --- Authors ---

func (r *BunStorageEngine) UpsertAuthor(ctx context.Context, name string) (*Author, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("author name cannot be empty")
	}

	author := new(Author)
	err := r.db.NewSelect().Model(author).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
	if err == nil {
		return author, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up author: %w", err)
	}

	author = &Author{
		ID:        ulid.New(),
		Name:      cleanName,
		CreatedAt: time.Now().UTC(),
	}

	_, err = r.db.NewInsert().Model(author).Exec(ctx)
	if err != nil {
		err2 := r.db.NewSelect().Model(author).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
		if err2 == nil {
			return author, nil
		}
		return nil, fmt.Errorf("inserting author: %w", err)
	}
	return author, nil
}

func (r *BunStorageEngine) GetAuthorByID(ctx context.Context, id string) (*Author, error) {
	author := new(Author)
	err := r.db.NewSelect().Model(author).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting author by ID: %w", err)
	}
	return author, nil
}

func (r *BunStorageEngine) GetAuthorByName(ctx context.Context, name string) (*Author, error) {
	author := new(Author)
	err := r.db.NewSelect().Model(author).Where("LOWER(name) = LOWER(?)", strings.TrimSpace(name)).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting author by name: %w", err)
	}
	return author, nil
}

func (r *BunStorageEngine) ListAuthors(ctx context.Context) ([]*Author, error) {
	var authors []*Author
	err := r.db.NewSelect().
		TableExpr("authors AS a").
		ColumnExpr("a.id, a.name, a.photo_url, a.created_at").
		ColumnExpr("COUNT(ba.book_id) AS book_count").
		Join("LEFT JOIN book_authors AS ba ON ba.author_id = a.id").
		GroupExpr("a.id, a.name, a.photo_url, a.created_at").
		OrderExpr("LOWER(a.name) ASC").
		Scan(ctx, &authors)
	if err != nil {
		return nil, fmt.Errorf("listing authors: %w", err)
	}
	if authors == nil {
		authors = []*Author{}
	}
	return authors, nil
}

// --- Genres ---

func (r *BunStorageEngine) UpsertGenre(ctx context.Context, name string) (*Genre, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("genre name cannot be empty")
	}

	genre := new(Genre)
	err := r.db.NewSelect().Model(genre).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
	if err == nil {
		return genre, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up genre: %w", err)
	}

	genre = &Genre{
		ID:        ulid.New(),
		Name:      cleanName,
		CreatedAt: time.Now().UTC(),
	}

	_, err = r.db.NewInsert().Model(genre).Exec(ctx)
	if err != nil {
		err2 := r.db.NewSelect().Model(genre).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
		if err2 == nil {
			return genre, nil
		}
		return nil, fmt.Errorf("inserting genre: %w", err)
	}
	return genre, nil
}

func (r *BunStorageEngine) GetGenreByID(ctx context.Context, id string) (*Genre, error) {
	genre := new(Genre)
	err := r.db.NewSelect().Model(genre).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting genre by ID: %w", err)
	}
	return genre, nil
}

func (r *BunStorageEngine) GetGenreByName(ctx context.Context, name string) (*Genre, error) {
	genre := new(Genre)
	err := r.db.NewSelect().Model(genre).Where("LOWER(name) = LOWER(?)", strings.TrimSpace(name)).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting genre by name: %w", err)
	}
	return genre, nil
}

func (r *BunStorageEngine) ListGenres(ctx context.Context) ([]*Genre, error) {
	var genres []*Genre
	err := r.db.NewSelect().Model(&genres).Order("name ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing genres: %w", err)
	}
	if genres == nil {
		genres = []*Genre{}
	}
	return genres, nil
}

// --- Series ---

func (r *BunStorageEngine) UpsertSeries(ctx context.Context, name string, description *string) (*Series, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return nil, fmt.Errorf("series name cannot be empty")
	}

	series := new(Series)
	err := r.db.NewSelect().Model(series).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
	if err == nil {
		if description != nil && *description != "" {
			series.Description = description
			_, _ = r.db.NewUpdate().Model(series).Column("description").WherePK().Exec(ctx)
		}
		return series, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("looking up series: %w", err)
	}

	series = &Series{
		ID:          ulid.New(),
		Name:        cleanName,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	_, err = r.db.NewInsert().Model(series).Exec(ctx)
	if err != nil {
		err2 := r.db.NewSelect().Model(series).Where("LOWER(name) = LOWER(?)", cleanName).Limit(1).Scan(ctx)
		if err2 == nil {
			return series, nil
		}
		return nil, fmt.Errorf("inserting series: %w", err)
	}
	return series, nil
}

func (r *BunStorageEngine) GetSeriesByID(ctx context.Context, id string) (*Series, error) {
	series := new(Series)
	err := r.db.NewSelect().Model(series).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting series by ID: %w", err)
	}
	return series, nil
}

func (r *BunStorageEngine) GetSeriesByName(ctx context.Context, name string) (*Series, error) {
	series := new(Series)
	err := r.db.NewSelect().Model(series).Where("LOWER(name) = LOWER(?)", strings.TrimSpace(name)).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting series by name: %w", err)
	}
	return series, nil
}

func (r *BunStorageEngine) ListSeries(ctx context.Context) ([]*Series, error) {
	var seriesList []*Series
	err := r.db.NewSelect().
		TableExpr("series AS s").
		ColumnExpr("s.id, s.name, s.description, s.created_at").
		ColumnExpr("COUNT(bs.book_id) AS book_count").
		ColumnExpr("MIN(bs.book_id) AS cover_book_id").
		Join("LEFT JOIN book_series AS bs ON bs.series_id = s.id").
		GroupExpr("s.id, s.name, s.description, s.created_at").
		OrderExpr("LOWER(s.name) ASC").
		Scan(ctx, &seriesList)
	if err != nil {
		return nil, fmt.Errorf("listing series: %w", err)
	}
	if seriesList == nil {
		seriesList = []*Series{}
	}
	return seriesList, nil
}

// --- Junction Links ---

func (r *BunStorageEngine) LinkBookAuthor(ctx context.Context, bookID, authorID, role string) error {
	if role == "" {
		role = "author"
	}
	ba := &BookAuthor{BookID: bookID, AuthorID: authorID, Role: role}
	_, err := r.db.NewInsert().Model(ba).
		On("CONFLICT (book_id, author_id) DO UPDATE").
		Set("role = EXCLUDED.role").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("linking book author: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) LinkBookGenre(ctx context.Context, bookID, genreID string) error {
	bg := &BookGenre{BookID: bookID, GenreID: genreID}
	_, err := r.db.NewInsert().Model(bg).
		On("CONFLICT (book_id, genre_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("linking book genre: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) LinkBookSeries(ctx context.Context, bookID, seriesID string, sequenceNumber *float64) error {
	bs := &BookSeries{BookID: bookID, SeriesID: seriesID, SequenceNumber: sequenceNumber}
	_, err := r.db.NewInsert().Model(bs).
		On("CONFLICT (book_id, series_id) DO UPDATE").
		Set("sequence_number = EXCLUDED.sequence_number").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("linking book series: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetBookAuthors(ctx context.Context, bookID string) ([]*Author, error) {
	var authors []*Author
	err := r.db.NewSelect().
		TableExpr("authors AS a").
		ColumnExpr("a.id, a.name, a.created_at").
		Join("JOIN book_authors AS ba ON a.id = ba.author_id").
		Where("ba.book_id = ?", bookID).
		Order("a.name ASC").
		Scan(ctx, &authors)
	if err != nil {
		return nil, fmt.Errorf("getting book authors: %w", err)
	}
	if authors == nil {
		authors = []*Author{}
	}
	return authors, nil
}

func (r *BunStorageEngine) GetBookGenres(ctx context.Context, bookID string) ([]*Genre, error) {
	var genres []*Genre
	err := r.db.NewSelect().
		TableExpr("genres AS g").
		ColumnExpr("g.id, g.name, g.created_at").
		Join("JOIN book_genres AS bg ON g.id = bg.genre_id").
		Where("bg.book_id = ?", bookID).
		Order("g.name ASC").
		Scan(ctx, &genres)
	if err != nil {
		return nil, fmt.Errorf("getting book genres: %w", err)
	}
	if genres == nil {
		genres = []*Genre{}
	}
	return genres, nil
}

func (r *BunStorageEngine) GetBookSeries(ctx context.Context, bookID string) ([]*BookSeriesDetail, error) {
	type rawDetail struct {
		ID             string    `bun:"id"`
		Name           string    `bun:"name"`
		Description    *string   `bun:"description"`
		CreatedAt      time.Time `bun:"created_at"`
		SequenceNumber *float64  `bun:"sequence_number"`
	}

	var rawList []rawDetail
	err := r.db.NewSelect().
		TableExpr("series AS s").
		ColumnExpr("s.id, s.name, s.description, s.created_at, bs.sequence_number").
		Join("JOIN book_series AS bs ON s.id = bs.series_id").
		Where("bs.book_id = ?", bookID).
		Order("s.name ASC").
		Scan(ctx, &rawList)
	if err != nil {
		return nil, fmt.Errorf("getting book series: %w", err)
	}

	details := make([]*BookSeriesDetail, len(rawList))
	for i, row := range rawList {
		details[i] = &BookSeriesDetail{
			Series: Series{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				CreatedAt:   row.CreatedAt,
			},
			SequenceNumber: row.SequenceNumber,
		}
	}
	return details, nil
}

// --- Chapters ---

func (r *BunStorageEngine) CreateChapter(ctx context.Context, chapter *Chapter) error {
	if chapter.ID == "" {
		chapter.ID = ulid.New()
	}
	if chapter.CreatedAt.IsZero() {
		chapter.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(chapter).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating chapter: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetChaptersByBookID(ctx context.Context, bookID string) ([]*Chapter, error) {
	var chapters []*Chapter
	err := r.db.NewSelect().Model(&chapters).
		Where("book_id = ?", bookID).
		Order("chapter_index ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting chapters by book ID: %w", err)
	}
	if chapters == nil {
		chapters = []*Chapter{}
	}
	return chapters, nil
}

func (r *BunStorageEngine) GetBookSpine(ctx context.Context, bookID string) ([]*SpineItem, error) {
	var spine []*SpineItem
	err := r.db.NewSelect().
		TableExpr("chapters AS c").
		ColumnExpr("c.id, c.book_id, c.chapter_index, c.title, c.summary, c.href, c.page_width, c.page_height, c.page_spread").
		Where("c.book_id = ?", bookID).
		Order("c.chapter_index ASC").
		Scan(ctx, &spine)
	if err != nil {
		return nil, fmt.Errorf("getting book spine: %w", err)
	}
	if spine == nil {
		spine = []*SpineItem{}
	}
	return spine, nil
}

func (r *BunStorageEngine) GetChapterByID(ctx context.Context, id string) (*Chapter, error) {
	ch := new(Chapter)
	err := r.db.NewSelect().Model(ch).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting chapter by ID: %w", err)
	}
	return ch, nil
}

func (r *BunStorageEngine) GetChapterByBookAndIndex(ctx context.Context, bookID string, chapterIndex int) (*Chapter, error) {
	ch := new(Chapter)
	err := r.db.NewSelect().Model(ch).
		Where("book_id = ? AND chapter_index = ?", bookID, chapterIndex).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting chapter by book and index: %w", err)
	}
	return ch, nil
}

func (r *BunStorageEngine) UpdateChapterSummary(ctx context.Context, chapterID string, summary string) error {
	res, err := r.db.NewUpdate().
		Model((*Chapter)(nil)).
		Set("summary = ?", summary).
		Where("id = ?", chapterID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating chapter summary: %w", err)
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

func (r *BunStorageEngine) UpdateChapterContent(ctx context.Context, chapterID string, contentPlain string) error {
	res, err := r.db.NewUpdate().
		Model((*Chapter)(nil)).
		Set("content_plain = ?", contentPlain).
		Where("id = ?", chapterID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating chapter content: %w", err)
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

func (r *BunStorageEngine) UpdateChapter(ctx context.Context, chapter *Chapter) error {
	res, err := r.db.NewUpdate().Model(chapter).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating chapter: %w", err)
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

func (r *BunStorageEngine) GetUnindexedChapters(ctx context.Context, limit int) ([]*Chapter, error) {
	if limit <= 0 {
		limit = 10
	}
	var chapters []*Chapter
	err := r.db.NewSelect().Model(&chapters).
		Where("summary = '' OR summary = 'No summary available.'").
		Order("created_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting unindexed chapters: %w", err)
	}
	if chapters == nil {
		chapters = []*Chapter{}
	}
	return chapters, nil
}

// --- Paragraphs ---

func (r *BunStorageEngine) CreateParagraphs(ctx context.Context, paragraphs []*Paragraph) error {
	if len(paragraphs) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, p := range paragraphs {
		if p.ID == "" {
			p.ID = ulid.New()
		}
		if p.CreatedAt.IsZero() {
			p.CreatedAt = now
		}
	}

	// Chunk inserts into batches of 500 to stay within SQL parameter limits
	batchSize := 500
	for i := 0; i < len(paragraphs); i += batchSize {
		end := i + batchSize
		if end > len(paragraphs) {
			end = len(paragraphs)
		}
		batch := paragraphs[i:end]
		_, err := r.db.NewInsert().Model(&batch).Exec(ctx)
		if err != nil {
			return fmt.Errorf("inserting paragraph batch %d-%d: %w", i, end, err)
		}
	}
	return nil
}

func (r *BunStorageEngine) GetParagraphsByBookID(ctx context.Context, bookID string) ([]*Paragraph, error) {
	var paras []*Paragraph
	err := r.db.NewSelect().Model(&paras).
		Where("book_id = ?", bookID).
		Order("chapter_index ASC", "start_paragraph ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting paragraphs by book ID: %w", err)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, nil
}

func (r *BunStorageEngine) GetParagraphsByChapterID(ctx context.Context, chapterID string) ([]*Paragraph, error) {
	var paras []*Paragraph
	err := r.db.NewSelect().Model(&paras).
		Where("chapter_id = ?", chapterID).
		Order("start_paragraph ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting paragraphs by chapter ID: %w", err)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, nil
}

func (r *BunStorageEngine) GetUnindexedParagraphs(ctx context.Context, limit int) ([]*Paragraph, error) {
	if limit <= 0 {
		limit = 50
	}
	var paras []*Paragraph
	err := r.db.NewSelect().
		TableExpr("paragraphs AS p").
		ColumnExpr("p.id, p.book_id, p.chapter_id, p.chapter_index, p.start_paragraph, p.end_paragraph, p.content, p.created_at").
		Join("LEFT JOIN vec_paragraphs AS v ON p.id = v.paragraph_id").
		Where("v.paragraph_id IS NULL").
		OrderExpr("p.created_at ASC, p.chapter_index ASC, p.start_paragraph ASC").
		Limit(limit).
		Scan(ctx, &paras)
	if err != nil {
		return nil, fmt.Errorf("getting unindexed paragraphs: %w", err)
	}
	if paras == nil {
		paras = []*Paragraph{}
	}
	return paras, nil
}

func (r *BunStorageEngine) DeleteParagraphsByBookID(ctx context.Context, bookID string) error {
	_, err := r.db.NewDelete().Model((*Paragraph)(nil)).Where("book_id = ?", bookID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting paragraphs by book ID: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) BackfillParagraphs(ctx context.Context) (int, error) {
	type chInfo struct {
		ID           string `bun:"id"`
		BookID       string `bun:"book_id"`
		ChapterIndex int    `bun:"chapter_index"`
		ContentPlain string `bun:"content_plain"`
	}

	var unmigrated []chInfo
	err := r.db.NewSelect().
		TableExpr("chapters AS c").
		ColumnExpr("c.id, c.book_id, c.chapter_index, c.content_plain").
		Where("c.id NOT IN (SELECT DISTINCT chapter_id FROM paragraphs)").
		Order("c.book_id ASC", "c.chapter_index ASC").
		Scan(ctx, &unmigrated)
	if err != nil {
		return 0, fmt.Errorf("querying unmigrated chapters: %w", err)
	}

	totalParas := 0
	var paras []*Paragraph
	for _, ch := range unmigrated {
		chunks := epub.ChunkChapterParagraphs(ch.ContentPlain, 400, 1200)
		for _, chk := range chunks {
			paras = append(paras, &Paragraph{
				BookID:         ch.BookID,
				ChapterID:      ch.ID,
				ChapterIndex:   ch.ChapterIndex,
				StartParagraph: chk.StartParagraph,
				EndParagraph:   chk.EndParagraph,
				Content:        chk.Content,
			})
		}
		if len(paras) >= 500 {
			if err := r.CreateParagraphs(ctx, paras); err != nil {
				return totalParas, fmt.Errorf("saving backfilled paragraphs: %w", err)
			}
			totalParas += len(paras)
			paras = nil
		}
	}

	if len(paras) > 0 {
		if err := r.CreateParagraphs(ctx, paras); err != nil {
			return totalParas, fmt.Errorf("saving backfilled paragraphs: %w", err)
		}
		totalParas += len(paras)
	}

	return totalParas, nil
}

// --- Vectors & Full-Text Search ---

func (r *BunStorageEngine) InsertChapterVector(ctx context.Context, chapterID string, embedding []float32) error {
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

func (r *BunStorageEngine) SearchVectorChapters(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error) {
	return r.SearchVectorParagraphs(ctx, queryEmbedding, filter)
}

func (r *BunStorageEngine) InsertParagraphVectors(ctx context.Context, items []ParagraphVector) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if r.isPG() {
		query := `
			INSERT INTO vec_paragraphs (paragraph_id, embedding)
			VALUES (?, ?)
			ON CONFLICT (paragraph_id) DO UPDATE SET embedding = EXCLUDED.embedding
		`
		for _, item := range items {
			vec := pgvector.NewVector(item.Embedding)
			if _, err := tx.ExecContext(ctx, query, item.ParagraphID, vec); err != nil {
				return fmt.Errorf("inserting postgres paragraph vector %s: %w", item.ParagraphID, err)
			}
		}
		return tx.Commit()
	}

	// SQLite
	delQuery := "DELETE FROM vec_paragraphs WHERE paragraph_id = ?"
	insQuery := "INSERT INTO vec_paragraphs (paragraph_id, embedding) VALUES (?, ?)"
	for _, item := range items {
		blob, err := sqlite_vec.SerializeFloat32(item.Embedding)
		if err != nil {
			return fmt.Errorf("serializing embedding for %s: %w", item.ParagraphID, err)
		}
		if _, err := tx.ExecContext(ctx, delQuery, item.ParagraphID); err != nil {
			return fmt.Errorf("deleting sqlite paragraph vector %s: %w", item.ParagraphID, err)
		}
		if _, err := tx.ExecContext(ctx, insQuery, item.ParagraphID, blob); err != nil {
			return fmt.Errorf("inserting sqlite paragraph vector %s: %w", item.ParagraphID, err)
		}
	}

	return tx.Commit()
}

func (r *BunStorageEngine) InsertParagraphVector(ctx context.Context, paragraphID string, embedding []float32) error {
	return r.InsertParagraphVectors(ctx, []ParagraphVector{{ParagraphID: paragraphID, Embedding: embedding}})
}

func (r *BunStorageEngine) SearchVectorParagraphs(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error) {
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
	seriesID := ""
	if filter.SeriesID != nil {
		seriesID = *filter.SeriesID
	}

	if r.isPG() {
		vec := pgvector.NewVector(queryEmbedding)
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
				(v.embedding <=> ?) AS distance
			FROM vec_paragraphs v
			JOIN paragraphs p ON v.paragraph_id = p.id
			JOIN chapters c ON p.chapter_id = c.id
			JOIN books b ON p.book_id = b.id
			WHERE (? = '' OR b.id = ?)
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?))
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?))
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
			ORDER BY distance ASC
			LIMIT ?
		`
		rows, err := r.db.QueryContext(ctx, query,
			vec,
			bookID, bookID,
			authorID, authorID,
			genreID, genreID,
			seriesID, seriesID,
			limit,
		)
		if err != nil {
			return nil, fmt.Errorf("executing pgvector paragraph search: %w", err)
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
				return nil, fmt.Errorf("scanning pgvector hit: %w", err)
			}
			hit.Summary = hit.Content
			hits = append(hits, hit)
		}
		if hits == nil {
			hits = []*SearchHit{}
		}
		return hits, rows.Err()
	}

	// SQLite flat scan
	blob, err := sqlite_vec.SerializeFloat32(queryEmbedding)
	if err != nil {
		return nil, fmt.Errorf("serializing query embedding: %w", err)
	}

	k := limit
	if filter.AuthorID != nil || filter.GenreID != nil || filter.SeriesID != nil || filter.BookID != nil {
		if k < 100 {
			k = 100
		}
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
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
		ORDER BY distance ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query,
		blob, blob, k,
		bookID, bookID,
		authorID, authorID,
		genreID, genreID,
		seriesID, seriesID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("executing sqlite vector search query: %w", err)
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
			return nil, fmt.Errorf("scanning sqlite vector hit: %w", err)
		}
		hit.Summary = hit.Content
		hits = append(hits, hit)
	}
	if hits == nil {
		hits = []*SearchHit{}
	}
	return hits, rows.Err()
}

func (r *BunStorageEngine) SearchFTSParagraphs(ctx context.Context, queryText string, filter SearchFilter) ([]*SearchHit, error) {
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
	seriesID := ""
	if filter.SeriesID != nil {
		seriesID = *filter.SeriesID
	}

	cleanQuery := strings.TrimSpace(queryText)
	if cleanQuery == "" {
		return []*SearchHit{}, nil
	}

	if r.isPG() {
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
				ts_rank_cd(to_tsvector('english', p.content), plainto_tsquery('english', ?)) AS rank
			FROM paragraphs p
			JOIN chapters c ON p.chapter_id = c.id
			JOIN books b ON p.book_id = b.id
			WHERE to_tsvector('english', p.content) @@ plainto_tsquery('english', ?)
			  AND (? = '' OR b.id = ?)
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?))
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?))
			  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
			ORDER BY rank DESC
			LIMIT ?
		`
		rows, err := r.db.QueryContext(ctx, query,
			cleanQuery, cleanQuery,
			bookID, bookID,
			authorID, authorID,
			genreID, genreID,
			seriesID, seriesID,
			limit,
		)
		if err != nil {
			return nil, fmt.Errorf("executing postgres FTS search query: %w", err)
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
				return nil, fmt.Errorf("scanning postgres FTS hit: %w", err)
			}
			hit.Distance = rank
			hit.Summary = hit.Content
			hits = append(hits, hit)
		}
		if hits == nil {
			hits = []*SearchHit{}
		}
		return hits, rows.Err()
	}

	// SQLite FTS5
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
		  AND (? = '' OR b.id IN (SELECT book_id FROM book_series WHERE series_id = ?))
		ORDER BY rank ASC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query,
		cleanQuery,
		bookID, bookID,
		authorID, authorID,
		genreID, genreID,
		seriesID, seriesID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("executing sqlite FTS search query: %w", err)
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
			return nil, fmt.Errorf("scanning sqlite FTS hit: %w", err)
		}
		hit.Distance = rank
		hit.Summary = hit.Content
		hits = append(hits, hit)
	}
	if hits == nil {
		hits = []*SearchHit{}
	}
	return hits, rows.Err()
}

// --- Users & Tokens ---

func (r *BunStorageEngine) CreateUser(ctx context.Context, user *User) error {
	if user.ID == "" {
		user.ID = ulid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := new(User)
	err := r.db.NewSelect().Model(user).Where("LOWER(username) = LOWER(?)", strings.TrimSpace(username)).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting user by username: %w", err)
	}
	return user, nil
}

func (r *BunStorageEngine) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := new(User)
	err := r.db.NewSelect().Model(user).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting user by ID: %w", err)
	}
	return user, nil
}

func (r *BunStorageEngine) UpdateUserPassword(ctx context.Context, userID string, passwordHash string) error {
	res, err := r.db.NewUpdate().
		Model((*User)(nil)).
		Set("password_hash = ?", passwordHash).
		Where("id = ?", userID).
		Exec(ctx)
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

func (r *BunStorageEngine) CreateAPIToken(ctx context.Context, token *APIToken) error {
	if token.ID == "" {
		token.ID = ulid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(token).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating API token: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetAPITokenByHash(ctx context.Context, tokenHash string) (*APIToken, error) {
	token := new(APIToken)
	err := r.db.NewSelect().Model(token).Where("token_hash = ?", tokenHash).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting API token by hash: %w", err)
	}
	return token, nil
}

func (r *BunStorageEngine) ListAPITokensByUserID(ctx context.Context, userID string) ([]*APIToken, error) {
	var tokens []*APIToken
	err := r.db.NewSelect().Model(&tokens).Where("user_id = ?", userID).Order("created_at DESC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing API tokens: %w", err)
	}
	if tokens == nil {
		tokens = []*APIToken{}
	}
	return tokens, nil
}

func (r *BunStorageEngine) DeleteAPIToken(ctx context.Context, id string) error {
	res, err := r.db.NewDelete().Model((*APIToken)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting API token: %w", err)
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

// --- Upload Jobs ---

func (r *BunStorageEngine) CreateUploadJob(ctx context.Context, job *UploadJob) error {
	if job.ID == "" {
		job.ID = ulid.New()
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
	_, err := r.db.NewInsert().Model(job).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating upload job: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetUploadJob(ctx context.Context, id string) (*UploadJob, error) {
	job := new(UploadJob)
	err := r.db.NewSelect().Model(job).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting upload job: %w", err)
	}
	return job, nil
}

func (r *BunStorageEngine) UpdateUploadJobStatus(ctx context.Context, id string, status string, bookID *string, errMessage *string) error {
	now := time.Now().UTC()
	q := r.db.NewUpdate().
		Model((*UploadJob)(nil)).
		Set("status = ?", status).
		Set("updated_at = ?", now).
		Where("id = ?", id)

	if bookID != nil {
		q = q.Set("book_id = ?", *bookID)
	}
	if errMessage != nil {
		q = q.Set("error_message = ?", *errMessage)
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating upload job status: %w", err)
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

func (r *BunStorageEngine) UpdateUploadJobCommit(ctx context.Context, id string, status string, metadata *string) error {
	now := time.Now().UTC()
	q := r.db.NewUpdate().
		Model((*UploadJob)(nil)).
		Set("status = ?", status).
		Set("updated_at = ?", now).
		Where("id = ?", id)

	if metadata != nil {
		q = q.Set("metadata = ?", *metadata)
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating upload job commit: %w", err)
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

func (r *BunStorageEngine) DeleteUploadJob(ctx context.Context, id string) error {
	res, err := r.db.NewDelete().
		Model((*UploadJob)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting upload job: %w", err)
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

func (r *BunStorageEngine) GetPendingUploadJobs(ctx context.Context, limit int) ([]*UploadJob, error) {
	if limit <= 0 {
		limit = 10
	}
	var jobs []*UploadJob
	err := r.db.NewSelect().Model(&jobs).
		Where("status = ?", "queued").
		Order("created_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting pending upload jobs: %w", err)
	}
	if jobs == nil {
		jobs = []*UploadJob{}
	}
	return jobs, nil
}

func (r *BunStorageEngine) ListUploadJobs(ctx context.Context, limit int) ([]*UploadJob, error) {
	if limit <= 0 {
		limit = 50
	}
	var jobs []*UploadJob
	err := r.db.NewSelect().Model(&jobs).
		Order("created_at DESC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing upload jobs: %w", err)
	}
	if jobs == nil {
		jobs = []*UploadJob{}
	}
	return jobs, nil
}

// --- Queue Status ---

func (r *BunStorageEngine) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	totalParagraphs, err := r.db.NewSelect().Table("paragraphs").Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting total paragraphs: %w", err)
	}

	var pendingParagraphs, indexedParagraphs, totalCount int
	var progressPercent float64

	if totalParagraphs > 0 {
		pending, err := r.db.NewSelect().
			TableExpr("paragraphs AS p").
			Join("LEFT JOIN vec_paragraphs AS v ON p.id = v.paragraph_id").
			Where("v.paragraph_id IS NULL").
			Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("counting pending paragraphs: %w", err)
		}
		pendingParagraphs = pending
		indexedParagraphs = totalParagraphs - pendingParagraphs
		totalCount = totalParagraphs
		progressPercent = (float64(indexedParagraphs) / float64(totalParagraphs)) * 100.0
	} else {
		totalChapters, err := r.db.NewSelect().Table("chapters").Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("counting total chapters: %w", err)
		}
		totalCount = totalChapters
		if totalChapters > 0 {
			pendingParagraphs = totalChapters
			indexedParagraphs = 0
			progressPercent = 0.0
		} else {
			progressPercent = 100.0
		}
	}

	pendingUploads, err := r.db.NewSelect().Table("upload_jobs").
		Where("status = ? OR status = ?", "queued", "processing").
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting pending uploads: %w", err)
	}

	return &QueueStatus{
		TotalChapters:   totalCount,
		IndexedChapters: indexedParagraphs,
		PendingChapters: pendingParagraphs,
		PendingUploads:  pendingUploads,
		ProgressPercent: progressPercent,
		IsActive:        pendingParagraphs > 0 || pendingUploads > 0,
	}, nil
}

// --- Bookmarks ---

func (r *BunStorageEngine) CreateBookmark(ctx context.Context, bookmark *Bookmark) error {
	if bookmark.ID == "" {
		bookmark.ID = ulid.New()
	}
	if bookmark.CreatedAt.IsZero() {
		bookmark.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(bookmark).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating bookmark: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) ListBookmarksByBookID(ctx context.Context, bookID string) ([]*Bookmark, error) {
	var bookmarks []*Bookmark
	err := r.db.NewSelect().Model(&bookmarks).
		Where("book_id = ?", bookID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing bookmarks: %w", err)
	}
	if bookmarks == nil {
		bookmarks = []*Bookmark{}
	}
	return bookmarks, nil
}

func (r *BunStorageEngine) DeleteBookmark(ctx context.Context, id string) error {
	res, err := r.db.NewDelete().Model((*Bookmark)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting bookmark: %w", err)
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

// --- Highlights ---

func (r *BunStorageEngine) CreateHighlight(ctx context.Context, highlight *Highlight) error {
	if highlight.ID == "" {
		highlight.ID = ulid.New()
	}
	if highlight.CreatedAt.IsZero() {
		highlight.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(highlight).Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating highlight: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) ListHighlightsByBookID(ctx context.Context, bookID string) ([]*Highlight, error) {
	var highlights []*Highlight
	err := r.db.NewSelect().Model(&highlights).
		Where("book_id = ?", bookID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing highlights: %w", err)
	}
	if highlights == nil {
		highlights = []*Highlight{}
	}
	return highlights, nil
}

func (r *BunStorageEngine) DeleteHighlight(ctx context.Context, id string) error {
	res, err := r.db.NewDelete().Model((*Highlight)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting highlight: %w", err)
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

// --- OAuth 2.0 ---

func (r *BunStorageEngine) CreateOAuthClient(ctx context.Context, client *OAuthClient) error {
	if client.CreatedAt.IsZero() {
		client.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(client).ModelTableExpr("oauth_clients").Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating oauth client: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetOAuthClientByID(ctx context.Context, id string) (*OAuthClient, error) {
	client := new(OAuthClient)
	err := r.db.NewSelect().Model(client).ModelTableExpr("oauth_clients").Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting oauth client: %w", err)
	}
	return client, nil
}

func (r *BunStorageEngine) CreateOAuthCode(ctx context.Context, code *OAuthCode) error {
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.NewInsert().Model(code).ModelTableExpr("oauth_codes").Exec(ctx)
	if err != nil {
		return fmt.Errorf("creating oauth code: %w", err)
	}
	return nil
}

func (r *BunStorageEngine) GetOAuthCode(ctx context.Context, codeStr string) (*OAuthCode, error) {
	code := new(OAuthCode)
	err := r.db.NewSelect().Model(code).ModelTableExpr("oauth_codes").Where("code = ?", codeStr).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting oauth code: %w", err)
	}
	return code, nil
}

func (r *BunStorageEngine) DeleteOAuthCode(ctx context.Context, codeStr string) error {
	res, err := r.db.NewDelete().Model((*OAuthCode)(nil)).ModelTableExpr("oauth_codes").Where("code = ?", codeStr).Exec(ctx)
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

