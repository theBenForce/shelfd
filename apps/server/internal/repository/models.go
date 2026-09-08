package repository

import "time"

// Book represents an ebook in the catalog.
type Book struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   *string   `json:"description,omitempty"`
	Language      *string   `json:"language,omitempty"`
	Publisher     *string   `json:"publisher,omitempty"`
	Identifier    *string   `json:"identifier,omitempty"`
	FilePath      string    `json:"file_path"`
	CoverPath     *string   `json:"cover_path,omitempty"`
	FileSizeBytes *int64    `json:"file_size_bytes,omitempty"`
	PublishedDate *string   `json:"published_date,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Author represents a book creator or contributor.
type Author struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Genre represents a subject, category, or tag.
type Genre struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Series represents an overarching book series.
type Series struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// BookAuthor links a book to an author with an attributed role.
type BookAuthor struct {
	BookID   string `json:"book_id"`
	AuthorID string `json:"author_id"`
	Role     string `json:"role"`
}

// BookGenre links a book to a genre.
type BookGenre struct {
	BookID  string `json:"book_id"`
	GenreID string `json:"genre_id"`
}

// BookSeries links a book to a series with an optional sequence number (e.g. 1.0, 2.5).
type BookSeries struct {
	BookID         string   `json:"book_id"`
	SeriesID       string   `json:"series_id"`
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
}

// BookSeriesDetail represents series metadata bundled with a book's sequence number.
type BookSeriesDetail struct {
	Series
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
}

// Chapter represents a parsed section/spine item from an EPUB with summary and text content.
type Chapter struct {
	ID           string    `json:"id"`
	BookID       string    `json:"book_id"`
	ChapterIndex int       `json:"chapter_index"`
	Title        *string   `json:"title,omitempty"`
	Summary      string    `json:"summary"`
	ContentPlain string    `json:"content_plain"`
	CreatedAt    time.Time `json:"created_at"`
}

// SpineItem represents a lightweight chapter entry in a book's reading order without plaintext content.
type SpineItem struct {
	ID           string  `json:"id"`
	BookID       string  `json:"book_id"`
	ChapterIndex int     `json:"chapter_index"`
	Title        *string `json:"title,omitempty"`
	Summary      string  `json:"summary,omitempty"`
}

// User represents a user account for authentication.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// APIToken represents a bearer token for MCP agents and API access.
type APIToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// BookFilter specifies search and filtering criteria for books.
type BookFilter struct {
	AuthorID *string
	GenreID  *string
	SeriesID *string
	Search   *string
	Limit    int
	Offset   int
}

// SearchHit represents a semantic vector search result across chapter summaries.
type SearchHit struct {
	BookID       string   `json:"book_id"`
	BookTitle    string   `json:"book_title"`
	CoverPath    *string  `json:"cover_path,omitempty"`
	AuthorName   *string  `json:"author_name,omitempty"`
	SeriesName   *string  `json:"series_name,omitempty"`
	SeriesIndex  *float64 `json:"series_index,omitempty"`
	ChapterID    string   `json:"chapter_id"`
	ChapterIndex int      `json:"chapter_index"`
	ChapterTitle *string  `json:"chapter_title,omitempty"`
	Summary      string   `json:"summary"`
	Distance     float64  `json:"distance"`
}

// SearchFilter specifies filters applied in conjunction with vector search.
type SearchFilter struct {
	AuthorID *string
	GenreID  *string
	SeriesID *string
	Limit    int
}

// UploadJob represents an asynchronous book ingestion task.
type UploadJob struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	StagedPath   string    `json:"-"`
	Status       string    `json:"status"` // queued, processing, completed, failed
	BookID       *string   `json:"book_id,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// QueueStatus represents the aggregate state of background processing jobs.
type QueueStatus struct {
	TotalChapters   int     `json:"total_chapters"`
	IndexedChapters int     `json:"indexed_chapters"`
	PendingChapters int     `json:"pending_chapters"`
	PendingUploads  int     `json:"pending_uploads"`
	ProgressPercent float64 `json:"progress_percent"`
	IsActive        bool    `json:"is_active"`
	CurrentBook     string  `json:"current_book,omitempty"`
	CurrentChapter  string  `json:"current_chapter,omitempty"`
}

