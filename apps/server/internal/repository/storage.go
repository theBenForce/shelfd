package repository

import "context"

// StorageEngine defines the storage abstraction for Shelfd.
// Handlers and services must interact solely through this interface
// so external backends (e.g. PostgreSQL + pgvector) can be swapped in without modifying domain logic.
type StorageEngine interface {
	// Books
	CreateBook(ctx context.Context, book *Book) error
	GetBookByID(ctx context.Context, id string) (*Book, error)
	GetBookByFilePath(ctx context.Context, filePath string) (*Book, error)
	UpdateBook(ctx context.Context, book *Book) error
	DeleteBook(ctx context.Context, id string) error
	ListBooks(ctx context.Context, filter BookFilter) ([]*Book, error)
	CountBooks(ctx context.Context, filter BookFilter) (int, error)

	// Authors
	UpsertAuthor(ctx context.Context, name string) (*Author, error)
	GetAuthorByID(ctx context.Context, id string) (*Author, error)
	GetAuthorByName(ctx context.Context, name string) (*Author, error)
	ListAuthors(ctx context.Context) ([]*Author, error)

	// Genres
	UpsertGenre(ctx context.Context, name string) (*Genre, error)
	GetGenreByID(ctx context.Context, id string) (*Genre, error)
	GetGenreByName(ctx context.Context, name string) (*Genre, error)
	ListGenres(ctx context.Context) ([]*Genre, error)

	// Series
	UpsertSeries(ctx context.Context, name string, description *string) (*Series, error)
	GetSeriesByID(ctx context.Context, id string) (*Series, error)
	GetSeriesByName(ctx context.Context, name string) (*Series, error)
	ListSeries(ctx context.Context) ([]*Series, error)

	// Junction links
	LinkBookAuthor(ctx context.Context, bookID, authorID, role string) error
	LinkBookGenre(ctx context.Context, bookID, genreID string) error
	LinkBookSeries(ctx context.Context, bookID, seriesID string, sequenceNumber *float64) error
	GetBookAuthors(ctx context.Context, bookID string) ([]*Author, error)
	GetBookGenres(ctx context.Context, bookID string) ([]*Genre, error)
	GetBookSeries(ctx context.Context, bookID string) ([]*BookSeriesDetail, error)

	// Chapters
	CreateChapter(ctx context.Context, chapter *Chapter) error
	GetChaptersByBookID(ctx context.Context, bookID string) ([]*Chapter, error)
	GetBookSpine(ctx context.Context, bookID string) ([]*SpineItem, error)
	GetChapterByID(ctx context.Context, id string) (*Chapter, error)
	GetChapterByBookAndIndex(ctx context.Context, bookID string, chapterIndex int) (*Chapter, error)
	UpdateChapterSummary(ctx context.Context, chapterID string, summary string) error
	UpdateChapterContent(ctx context.Context, chapterID string, contentPlain string) error
	GetUnindexedChapters(ctx context.Context, limit int) ([]*Chapter, error)

	// Paragraphs
	CreateParagraphs(ctx context.Context, paragraphs []*Paragraph) error
	GetParagraphsByBookID(ctx context.Context, bookID string) ([]*Paragraph, error)
	GetParagraphsByChapterID(ctx context.Context, chapterID string) ([]*Paragraph, error)
	GetUnindexedParagraphs(ctx context.Context, limit int) ([]*Paragraph, error)
	DeleteParagraphsByBookID(ctx context.Context, bookID string) error
	BackfillParagraphs(ctx context.Context) (int, error)

	// Vectors & Full-Text Search
	InsertChapterVector(ctx context.Context, chapterID string, embedding []float32) error
	SearchVectorChapters(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error)
	InsertParagraphVector(ctx context.Context, paragraphID string, embedding []float32) error
	InsertParagraphVectors(ctx context.Context, items []ParagraphVector) error
	SearchVectorParagraphs(ctx context.Context, queryEmbedding []float32, filter SearchFilter) ([]*SearchHit, error)
	SearchFTSParagraphs(ctx context.Context, query string, filter SearchFilter) ([]*SearchHit, error)

	// Users & Tokens
	CreateUser(ctx context.Context, user *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	CreateAPIToken(ctx context.Context, token *APIToken) error
	GetAPITokenByHash(ctx context.Context, tokenHash string) (*APIToken, error)
	ListAPITokensByUserID(ctx context.Context, userID string) ([]*APIToken, error)
	DeleteAPIToken(ctx context.Context, id string) error

	// Upload Jobs
	CreateUploadJob(ctx context.Context, job *UploadJob) error
	GetUploadJob(ctx context.Context, id string) (*UploadJob, error)
	UpdateUploadJobStatus(ctx context.Context, id string, status string, bookID *string, errMessage *string) error
	GetPendingUploadJobs(ctx context.Context, limit int) ([]*UploadJob, error)
	ListUploadJobs(ctx context.Context, limit int) ([]*UploadJob, error)

	// Queue & Background Jobs Status
	GetQueueStatus(ctx context.Context) (*QueueStatus, error)

	// Bookmarks
	CreateBookmark(ctx context.Context, bookmark *Bookmark) error
	ListBookmarksByBookID(ctx context.Context, bookID string) ([]*Bookmark, error)
	DeleteBookmark(ctx context.Context, id string) error

	// Highlights
	CreateHighlight(ctx context.Context, highlight *Highlight) error
	ListHighlightsByBookID(ctx context.Context, bookID string) ([]*Highlight, error)
	DeleteHighlight(ctx context.Context, id string) error

	// Lifecycle
	Close() error
}
