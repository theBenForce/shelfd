package repository

import "time"

// Book represents an ebook in the catalog.
type Book struct {
	ID            string    `json:"id" bun:"id,pk"`
	Title         string    `json:"title" bun:"title,notnull"`
	Description   *string   `json:"description,omitempty" bun:"description"`
	Language      *string   `json:"language,omitempty" bun:"language"`
	Publisher     *string   `json:"publisher,omitempty" bun:"publisher"`
	Identifier    *string   `json:"identifier,omitempty" bun:"identifier"`
	FilePath      string    `json:"file_path" bun:"file_path,notnull"`
	CoverPath     *string   `json:"cover_path,omitempty" bun:"cover_path"`
	FileSizeBytes  *int64     `json:"file_size_bytes,omitempty" bun:"file_size_bytes"`
	FileModifiedAt *time.Time `json:"file_modified_at,omitempty" bun:"file_modified_at"`
	PublishedDate  *string    `json:"published_date,omitempty" bun:"published_date"`
	CreatedAt      time.Time  `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Author represents a book creator or contributor.
type Author struct {
	ID        string    `json:"id" bun:"id,pk"`
	Name      string    `json:"name" bun:"name,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Genre represents a subject, category, or tag.
type Genre struct {
	ID        string    `json:"id" bun:"id,pk"`
	Name      string    `json:"name" bun:"name,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Series represents an overarching book series.
type Series struct {
	ID          string    `json:"id" bun:"id,pk"`
	Name        string    `json:"name" bun:"name,notnull"`
	Description *string   `json:"description,omitempty" bun:"description"`
	CreatedAt   time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// BookAuthor links a book to an author with an attributed role.
type BookAuthor struct {
	BookID   string `json:"book_id" bun:"book_id,pk"`
	AuthorID string `json:"author_id" bun:"author_id,pk"`
	Role     string `json:"role" bun:"role"`
}

// BookGenre links a book to a genre.
type BookGenre struct {
	BookID  string `json:"book_id" bun:"book_id,pk"`
	GenreID string `json:"genre_id" bun:"genre_id,pk"`
}

// BookSeries links a book to a series with an optional sequence number (e.g. 1.0, 2.5).
type BookSeries struct {
	BookID         string   `json:"book_id" bun:"book_id,pk"`
	SeriesID       string   `json:"series_id" bun:"series_id,pk"`
	SequenceNumber *float64 `json:"sequence_number,omitempty" bun:"sequence_number"`
}

// BookSeriesDetail represents series metadata bundled with a book's sequence number.
type BookSeriesDetail struct {
	Series
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
}

// Chapter represents a parsed section/spine item from an EPUB with summary and text content.
type Chapter struct {
	ID           string    `json:"id" bun:"id,pk"`
	BookID       string    `json:"book_id" bun:"book_id,notnull"`
	ChapterIndex int       `json:"chapter_index" bun:"chapter_index,notnull"`
	Title        *string   `json:"title,omitempty" bun:"title"`
	Summary      string    `json:"summary" bun:"summary,notnull"`
	ContentPlain string    `json:"content_plain" bun:"content_plain,notnull"`
	CreatedAt    time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// SpineItem represents a lightweight chapter entry in a book's reading order without plaintext content.
type SpineItem struct {
	ID           string  `json:"id" bun:"id,pk"`
	BookID       string  `json:"book_id" bun:"book_id,notnull"`
	ChapterIndex int     `json:"chapter_index" bun:"chapter_index,notnull"`
	Title        *string `json:"title,omitempty" bun:"title"`
	Summary      string  `json:"summary,omitempty" bun:"summary"`
}

// User represents a user account for authentication.
type User struct {
	ID           string    `json:"id" bun:"id,pk"`
	Username     string    `json:"username" bun:"username,notnull"`
	PasswordHash string    `json:"-" bun:"password_hash,notnull"`
	CreatedAt    time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// APIToken represents a bearer token for MCP agents and API access.
type APIToken struct {
	ID        string    `json:"id" bun:"id,pk"`
	UserID    string    `json:"user_id" bun:"user_id,notnull"`
	TokenHash string    `json:"token_hash" bun:"token_hash,notnull"`
	Name      string    `json:"name" bun:"name,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
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

// Paragraph represents a consolidated text passage within a chapter.
type Paragraph struct {
	ID             string    `json:"id" bun:"id,pk"`
	BookID         string    `json:"book_id" bun:"book_id,notnull"`
	ChapterID      string    `json:"chapter_id" bun:"chapter_id,notnull"`
	ChapterIndex   int       `json:"chapter_index" bun:"chapter_index,notnull"`
	StartParagraph int       `json:"start_paragraph" bun:"start_paragraph,notnull"`
	EndParagraph   int       `json:"end_paragraph" bun:"end_paragraph,notnull"`
	Content        string    `json:"content" bun:"content,notnull"`
	CreatedAt      time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// ParagraphVector maps a paragraph identifier to its computed embedding vector.
type ParagraphVector struct {
	ParagraphID string
	Embedding   []float32
}

// SearchHit represents a vector or full-text search result across paragraphs.
type SearchHit struct {
	BookID         string   `json:"book_id"`
	BookTitle      string   `json:"book_title"`
	CoverPath      *string  `json:"cover_path,omitempty"`
	AuthorName     *string  `json:"author_name,omitempty"`
	SeriesName     *string  `json:"series_name,omitempty"`
	SeriesIndex    *float64 `json:"series_index,omitempty"`
	ChapterID      string   `json:"chapter_id"`
	ChapterIndex   int      `json:"chapter_index"`
	ChapterTitle   *string  `json:"chapter_title,omitempty"`
	StartParagraph int      `json:"start_paragraph,omitempty"`
	EndParagraph   int      `json:"end_paragraph,omitempty"`
	Content        string   `json:"content,omitempty"`
	Summary        string   `json:"summary"` // Maintained for backward compatibility
	Distance       float64  `json:"distance"`
}

// SearchFilter specifies filters applied in conjunction with vector search.
type SearchFilter struct {
	BookID   *string
	AuthorID *string
	GenreID  *string
	SeriesID *string
	Limit    int
}

// Bookmark represents a saved reading location or user bookmark.
type Bookmark struct {
	ID        string    `json:"id" bun:"id,pk"`
	BookID    string    `json:"book_id" bun:"book_id,notnull"`
	ChapterID *string   `json:"chapter_id,omitempty" bun:"chapter_id"`
	Title     string    `json:"title" bun:"title,notnull"`
	Progress  float64   `json:"progress" bun:"progress,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Highlight represents a user-highlighted text passage with optional notes.
type Highlight struct {
	ID             string    `json:"id" bun:"id,pk"`
	BookID         string    `json:"book_id" bun:"book_id,notnull"`
	ChapterID      *string   `json:"chapter_id,omitempty" bun:"chapter_id"`
	SelectedText   string    `json:"selected_text" bun:"selected_text,notnull"`
	Note           *string   `json:"note,omitempty" bun:"note"`
	Color          string    `json:"color" bun:"color,notnull"`
	StartOffset    *int      `json:"start_offset,omitempty" bun:"start_offset"`
	EndOffset      *int      `json:"end_offset,omitempty" bun:"end_offset"`
	StartParagraph *int      `json:"start_paragraph,omitempty" bun:"start_paragraph"`
	EndParagraph   *int      `json:"end_paragraph,omitempty" bun:"end_paragraph"`
	Location       *string   `json:"location,omitempty" bun:"location"`
	CreatedAt      time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// UploadJob represents an asynchronous book ingestion task.
type UploadJob struct {
	ID           string    `json:"id" bun:"id,pk"`
	Filename     string    `json:"filename" bun:"filename,notnull"`
	StagedPath   string    `json:"-" bun:"staged_path,notnull"`
	Status       string    `json:"status" bun:"status,notnull"` // staged, queued, processing, completed, failed
	Metadata     *string   `json:"metadata,omitempty" bun:"metadata"`
	HasCover     bool      `json:"has_cover" bun:"has_cover"`
	BookID       *string   `json:"book_id,omitempty" bun:"book_id"`
	ErrorMessage *string   `json:"error_message,omitempty" bun:"error_message"`
	CreatedAt    time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt    time.Time `json:"updated_at" bun:"updated_at,nullzero,default:current_timestamp"`
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

// OAuthClient represents a dynamically registered OAuth 2.0 client (RFC 7591).
type OAuthClient struct {
	ID            string    `json:"client_id" bun:"id,pk"`
	ClientSecret  *string   `json:"client_secret,omitempty" bun:"client_secret"`
	ClientName    string    `json:"client_name" bun:"client_name,notnull"`
	RedirectURIs  string    `json:"redirect_uris" bun:"redirect_uris,notnull"`
	GrantTypes    string    `json:"grant_types" bun:"grant_types,notnull"`
	ResponseTypes string    `json:"response_types" bun:"response_types,notnull"`
	Scope         *string   `json:"scope,omitempty" bun:"scope"`
	CreatedAt     time.Time `json:"client_id_issued_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// OAuthCode represents an authorization code awaiting token exchange (RFC 6749 / RFC 7636).
type OAuthCode struct {
	Code                string    `json:"code" bun:"code,pk"`
	ClientID            string    `json:"client_id" bun:"client_id,notnull"`
	UserID              string    `json:"user_id" bun:"user_id,notnull"`
	RedirectURI         string    `json:"redirect_uri" bun:"redirect_uri,notnull"`
	CodeChallenge       *string   `json:"code_challenge,omitempty" bun:"code_challenge"`
	CodeChallengeMethod *string   `json:"code_challenge_method,omitempty" bun:"code_challenge_method"`
	Scope               *string   `json:"scope,omitempty" bun:"scope"`
	ExpiresAt           time.Time `json:"expires_at" bun:"expires_at,notnull"`
	CreatedAt           time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}


