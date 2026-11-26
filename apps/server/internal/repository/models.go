package repository

import (
	"strings"
	"time"

	"github.com/uptrace/bun"
)

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
	Layout         string     `json:"layout" bun:"layout,default:'reflowable'"`
	RenditionSpread string    `json:"rendition_spread" bun:"rendition_spread,default:'auto'"`
	RenditionOrientation string `json:"rendition_orientation" bun:"rendition_orientation,default:'auto'"`
	PageProgressionDirection string `json:"page_progression_direction" bun:"page_progression_direction,default:'ltr'"`
	BookType       string     `json:"book_type" bun:"book_type,default:'ebook'"`
	DurationSeconds *float64  `json:"duration_seconds,omitempty" bun:"duration_seconds"`
	CreatedAt      time.Time  `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Author represents a book creator or contributor.
type Author struct {
	ID        string    `json:"id" bun:"id,pk"`
	Name      string    `json:"name" bun:"name,notnull"`
	PhotoURL  *string   `json:"photo_url,omitempty" bun:"photo_url"`
	BookCount int       `json:"book_count,omitempty" bun:",scanonly"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Genre represents a literary form or broad genre category.
type Genre struct {
	ID        string    `json:"id" bun:"id,pk"`
	Name      string    `json:"name" bun:"name,notnull"`
	BookCount int       `json:"book_count,omitempty" bun:",scanonly"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Topic represents a specific subject, theme, or concept.
type Topic struct {
	ID        string    `json:"id" bun:"id,pk"`
	Name      string    `json:"name" bun:"name,notnull"`
	BookCount int       `json:"book_count,omitempty" bun:",scanonly"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// Series represents an overarching book series.
type Series struct {
	ID          string    `json:"id" bun:"id,pk"`
	Name        string    `json:"name" bun:"name,notnull"`
	Description *string   `json:"description,omitempty" bun:"description"`
	BookCount   int       `json:"book_count,omitempty" bun:",scanonly"`
	CoverBookID *string   `json:"cover_book_id,omitempty" bun:",scanonly"`
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

// BookTopic links a book to a topic.
type BookTopic struct {
	BookID  string `json:"book_id" bun:"book_id,pk"`
	TopicID string `json:"topic_id" bun:"topic_id,pk"`
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
	Href         *string   `json:"href,omitempty" bun:"href"`
	PageWidth    *int      `json:"page_width,omitempty" bun:"page_width"`
	PageHeight   *int      `json:"page_height,omitempty" bun:"page_height"`
	PageSpread   *string   `json:"page_spread,omitempty" bun:"page_spread"`
	CreatedAt    time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// SpineItem represents a lightweight chapter entry in a book's reading order without plaintext content.
type SpineItem struct {
	ID           string  `json:"id" bun:"id,pk"`
	BookID       string  `json:"book_id" bun:"book_id,notnull"`
	ChapterIndex int     `json:"chapter_index" bun:"chapter_index,notnull"`
	Title        *string `json:"title,omitempty" bun:"title"`
	Summary      string  `json:"summary,omitempty" bun:"summary"`
	Href         *string `json:"href,omitempty" bun:"href"`
	PageWidth    *int    `json:"page_width,omitempty" bun:"page_width"`
	PageHeight   *int    `json:"page_height,omitempty" bun:"page_height"`
	PageSpread   *string `json:"page_spread,omitempty" bun:"page_spread"`
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
	AuthorID   *string
	AuthorName *string
	GenreID    *string
	GenreName  *string
	TopicID    *string
	TopicName  *string
	SeriesID   *string
	Search     *string
	Title      *string
	ISBN       *string
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

func CleanISBNTerm(term string) string {
	s := strings.ToLower(strings.TrimSpace(term))
	s = strings.TrimPrefix(s, "urn:isbn:")
	s = strings.TrimPrefix(s, "urn:uuid:")
	s = strings.TrimPrefix(s, "isbn-13:")
	s = strings.TrimPrefix(s, "isbn-10:")
	s = strings.TrimPrefix(s, "isbn:")

	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	IsEmbedded     bool      `json:"is_embedded" bun:"is_embedded,notnull,default:false"`
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
	BookID          *string
	BookIDs         []string
	AuthorID        *string
	AuthorIDs       []string
	GenreID         *string
	TopicID         *string
	SeriesID        *string
	MaxChapterIndex *int
	Limit           int
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func buildSearchFilterClauses(filter SearchFilter) ([]string, []any) {
	var clauses []string
	var args []any

	if filter.BookID != nil && *filter.BookID != "" {
		clauses = append(clauses, "b.id = ?")
		args = append(args, *filter.BookID)
	}
	if bookIDs := nonEmptyStrings(filter.BookIDs); len(bookIDs) > 0 {
		clauses = append(clauses, "b.id IN ("+placeholders(len(bookIDs))+")")
		for _, id := range bookIDs {
			args = append(args, id)
		}
	}
	if filter.AuthorID != nil && *filter.AuthorID != "" {
		clauses = append(clauses, "b.id IN (SELECT book_id FROM book_authors WHERE author_id = ?)")
		args = append(args, *filter.AuthorID)
	}
	if authorIDs := nonEmptyStrings(filter.AuthorIDs); len(authorIDs) > 0 {
		clauses = append(clauses, "b.id IN (SELECT book_id FROM book_authors WHERE author_id IN ("+placeholders(len(authorIDs))+"))")
		for _, id := range authorIDs {
			args = append(args, id)
		}
	}
	if filter.GenreID != nil && *filter.GenreID != "" {
		clauses = append(clauses, "b.id IN (SELECT book_id FROM book_genres WHERE genre_id = ?)")
		args = append(args, *filter.GenreID)
	}
	if filter.TopicID != nil && *filter.TopicID != "" {
		clauses = append(clauses, "b.id IN (SELECT book_id FROM book_topics WHERE topic_id = ?)")
		args = append(args, *filter.TopicID)
	}
	if filter.SeriesID != nil && *filter.SeriesID != "" {
		clauses = append(clauses, "b.id IN (SELECT book_id FROM book_series WHERE series_id = ?)")
		args = append(args, *filter.SeriesID)
	}
	if filter.MaxChapterIndex != nil {
		clauses = append(clauses, "c.chapter_index <= ?")
		args = append(args, *filter.MaxChapterIndex)
	}

	return clauses, args
}

func sqliteKNNLimit(limit int, filter SearchFilter) int {
	bookIDs := nonEmptyStrings(filter.BookIDs)
	authorIDs := nonEmptyStrings(filter.AuthorIDs)
	hasFilter := (filter.BookID != nil && *filter.BookID != "") ||
		len(bookIDs) > 0 ||
		(filter.AuthorID != nil && *filter.AuthorID != "") ||
		len(authorIDs) > 0 ||
		(filter.GenreID != nil && *filter.GenreID != "") ||
		(filter.TopicID != nil && *filter.TopicID != "") ||
		(filter.SeriesID != nil && *filter.SeriesID != "") ||
		filter.MaxChapterIndex != nil

	if !hasFilter {
		return limit
	}

	multiplier := 5
	if len(bookIDs) > 1 || len(authorIDs) > 1 || filter.MaxChapterIndex != nil {
		multiplier = 10
	}
	k := limit * multiplier
	if k < 100 {
		k = 100
	}
	if filter.MaxChapterIndex != nil && k < 200 {
		k = 200
	}
	if k > 2000 {
		k = 2000
	}
	return k
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

// AudioChapter represents a chapter marker with timestamp offsets in an audiobook.
type AudioChapter struct {
	bun.BaseModel  `bun:"table:audio_chapters"`
	ID             string    `json:"id" bun:"id,pk"`
	BookID         string    `json:"book_id" bun:"book_id,notnull"`
	ChapterIndex   int       `json:"chapter_index" bun:"chapter_index,notnull"`
	Title          string    `json:"title" bun:"title,notnull"`
	StartOffsetSec float64   `json:"start_offset_sec" bun:"start_offset_sec,notnull"`
	DurationSec    float64   `json:"duration_sec" bun:"duration_sec,notnull"`
	CreatedAt      time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// BookFile represents an associated physical file (epub, audiobook, pdf, cover, etc.) linked to a book.
type BookFile struct {
	bun.BaseModel   `bun:"table:book_files"`
	ID              string     `json:"id" bun:"id,pk"`
	BookID          string     `json:"book_id" bun:"book_id,notnull"`
	FileType        string     `json:"file_type" bun:"file_type,notnull"`
	FilePath        string     `json:"file_path" bun:"file_path,notnull"`
	FileSizeBytes   *int64     `json:"file_size_bytes,omitempty" bun:"file_size_bytes"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty" bun:"duration_seconds"`
	BitrateKbps     *int       `json:"bitrate_kbps,omitempty" bun:"bitrate_kbps"`
	PageCount       *int       `json:"page_count,omitempty" bun:"page_count"`
	MimeType        *string    `json:"mime_type,omitempty" bun:"mime_type"`
	FileModifiedAt  *time.Time `json:"file_modified_at,omitempty" bun:"file_modified_at"`
	CreatedAt       time.Time  `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}

// AudiobookProgress represents user listening progress on an audiobook.
type AudiobookProgress struct {
	bun.BaseModel   `bun:"table:audiobook_progress"`
	ID              string    `json:"id" bun:"id,pk"`
	BookID          string    `json:"book_id" bun:"book_id,notnull"`
	UserID          string    `json:"user_id" bun:"user_id,notnull,default:''"`
	PositionSeconds float64   `json:"position_seconds" bun:"position_seconds,notnull"`
	Speed           float64   `json:"speed" bun:"speed,notnull,default:1.0"`
	IsCompleted     bool      `json:"is_completed" bun:"is_completed,notnull,default:false"`
	CreatedAt       time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt       time.Time `json:"updated_at" bun:"updated_at,nullzero,default:current_timestamp"`
}

// QueueStatus represents the aggregate state of background processing jobs.
type QueueStatus struct {
	TotalChapters   int     `json:"total_chapters"`
	IndexedChapters int     `json:"indexed_chapters"`
	PendingChapters int     `json:"pending_chapters"`
	PendingUploads  int     `json:"pending_uploads"`
	StagedUploads   int     `json:"staged_uploads"`
	ProgressPercent float64 `json:"progress_percent"`
	IsActive        bool    `json:"is_active"`
	CurrentBook     string  `json:"current_book,omitempty"`
	CurrentChapter  string  `json:"current_chapter,omitempty"`
}

// OAuthClient represents a dynamically registered OAuth 2.0 client (RFC 7591).
type OAuthClient struct {
	bun.BaseModel `bun:"table:oauth_clients"`
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
	bun.BaseModel `bun:"table:oauth_codes"`
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

// PluginState persists enable/disable state and cached manifest metadata for an installed plugin.
type PluginState struct {
	bun.BaseModel      `bun:"table:plugins_state"`
	Name               string    `json:"name" bun:"name,pk"`
	Enabled            bool      `json:"enabled" bun:"enabled,notnull"`
	InstalledVersion   string    `json:"installed_version" bun:"installed_version,notnull"`
	CachedManifestJSON string    `json:"cached_manifest_json" bun:"cached_manifest_json,notnull"`
	CreatedAt          time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt          time.Time `json:"updated_at" bun:"updated_at,nullzero,default:current_timestamp"`
}

// PluginUserConfig stores per-user JSON configuration (with secret fields encrypted at rest) for a plugin.
type PluginUserConfig struct {
	bun.BaseModel `bun:"table:plugin_configs"`
	ID            string    `json:"id" bun:"id,pk"`
	UserID        string    `json:"user_id" bun:"user_id,notnull"`
	PluginName    string    `json:"plugin_name" bun:"plugin_name,notnull"`
	ConfigJSON    string    `json:"config_json" bun:"config_json,notnull"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt     time.Time `json:"updated_at" bun:"updated_at,nullzero,default:current_timestamp"`
}

// BookSource records external plugin provenance so catalog items reflect staged or in-library status.
type BookSource struct {
	bun.BaseModel `bun:"table:book_sources"`
	ID            string    `json:"id" bun:"id,pk"`
	PluginName    string    `json:"plugin_name" bun:"plugin_name,notnull"`
	ExternalID    string    `json:"external_id" bun:"external_id,notnull"`
	UploadJobID   *string   `json:"upload_job_id,omitempty" bun:"upload_job_id"`
	BookID        *string   `json:"book_id,omitempty" bun:"book_id"`
	MetadataJSON  *string   `json:"metadata_json,omitempty" bun:"metadata_json"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt     time.Time `json:"updated_at" bun:"updated_at,nullzero,default:current_timestamp"`
}

type RecapCacheEntry struct {
	bun.BaseModel   `bun:"table:recap_cache"`
	BookID          string    `json:"book_id" bun:"book_id,pk"`
	ChapterIndex    int       `json:"chapter_index" bun:"chapter_index,pk"`
	ParagraphOffset int       `json:"paragraph_offset" bun:"paragraph_offset,pk"`
	Content         string    `json:"content" bun:"content,notnull"`
	CreatedAt       time.Time `json:"created_at" bun:"created_at,nullzero,default:current_timestamp"`
}





