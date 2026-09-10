package api

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/ulid"
	"github.com/shelfd/shelfd/internal/worker"
)

type BookHandler struct {
	repo         repository.StorageEngine
	ingester     *scanner.Ingester
	worker       *worker.Worker
	uploadWorker *worker.UploadWorker
	aiClient     ai.Client
	dataDir      string
	libraryDir   string
	hub          *events.Hub
	logger       *slog.Logger
}

func NewBookHandler(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	worker *worker.Worker,
	uploadWorker *worker.UploadWorker,
	aiClient ai.Client,
	dataDir string,
	libraryDir string,
	hub *events.Hub,
	logger *slog.Logger,
) *BookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BookHandler{
		repo:         repo,
		ingester:     ingester,
		worker:       worker,
		uploadWorker: uploadWorker,
		aiClient:     aiClient,
		dataDir:      dataDir,
		libraryDir:   libraryDir,
		hub:          hub,
		logger:       logger,
	}
}

type BookListItem struct {
	ID             string                         `json:"id"`
	Title          string                         `json:"title"`
	Description    *string                        `json:"description,omitempty"`
	Language       *string                        `json:"language,omitempty"`
	Publisher      *string                        `json:"publisher,omitempty"`
	Identifier     *string                        `json:"identifier,omitempty"`
	FilePath       string                         `json:"file_path"`
	CoverPath      *string                        `json:"cover_path,omitempty"`
	FileSizeBytes  *int64                         `json:"file_size_bytes,omitempty"`
	FileModifiedAt *time.Time                     `json:"file_modified_at,omitempty"`
	PublishedDate            *string                        `json:"published_date,omitempty"`
	Layout                   string                         `json:"layout"`
	RenditionSpread          string                         `json:"rendition_spread"`
	RenditionOrientation     string                         `json:"rendition_orientation"`
	PageProgressionDirection string                         `json:"page_progression_direction"`
	Authors                  []*repository.Author           `json:"authors"`
	Genres                   []*repository.Genre            `json:"genres"`
	Series                   []*repository.BookSeriesDetail `json:"series"`
}

type BookDetailResponse struct {
	BookListItem
	Spine      []*repository.SpineItem   `json:"spine"`
	Chapters   []*repository.Chapter     `json:"chapters"`
	Bookmarks  []*repository.Bookmark    `json:"bookmarks"`
	Highlights []*repository.Highlight   `json:"highlights"`
}

// BuildBookListItem loads relational metadata for a book and constructs a BookListItem DTO.
func BuildBookListItem(ctx context.Context, repo repository.StorageEngine, b *repository.Book) BookListItem {
	authors, _ := repo.GetBookAuthors(ctx, b.ID)
	genres, _ := repo.GetBookGenres(ctx, b.ID)
	seriesList, _ := repo.GetBookSeries(ctx, b.ID)

	return BookListItem{
		ID:                       b.ID,
		Title:                    b.Title,
		Description:              b.Description,
		Language:                 b.Language,
		Publisher:                b.Publisher,
		Identifier:               b.Identifier,
		FilePath:                 b.FilePath,
		CoverPath:                b.CoverPath,
		FileSizeBytes:            b.FileSizeBytes,
		FileModifiedAt:           b.FileModifiedAt,
		PublishedDate:            b.PublishedDate,
		Layout:                   b.Layout,
		RenditionSpread:          b.RenditionSpread,
		RenditionOrientation:     b.RenditionOrientation,
		PageProgressionDirection: b.PageProgressionDirection,
		Authors:                  authors,
		Genres:                   genres,
		Series:                   seriesList,
	}
}

type BookCitation struct {
	ChapterID      string  `json:"chapter_id"`
	ChapterIndex   int     `json:"chapter_index"`
	ChapterTitle   *string `json:"chapter_title,omitempty"`
	StartParagraph int     `json:"start_paragraph,omitempty"`
	EndParagraph   int     `json:"end_paragraph,omitempty"`
	Excerpt        string  `json:"excerpt,omitempty"`
	Summary        string  `json:"summary"`
}

type BookChatRequest struct {
	Message string           `json:"message"`
	Query   string           `json:"query,omitempty"`
	History []ai.ChatMessage `json:"history,omitempty"`
}

type BookChatResponse struct {
	Reply     string         `json:"reply"`
	Citations []BookCitation `json:"citations"`
}

func (h *BookHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	q := r.URL.Query()
	filter := repository.BookFilter{}

	if authorID := strings.TrimSpace(q.Get("author_id")); authorID != "" {
		filter.AuthorID = &authorID
	}
	if genreID := strings.TrimSpace(q.Get("genre_id")); genreID != "" {
		filter.GenreID = &genreID
	}
	if seriesID := strings.TrimSpace(q.Get("series_id")); seriesID != "" {
		filter.SeriesID = &seriesID
	}
	if search := strings.TrimSpace(q.Get("search")); search != "" {
		filter.Search = &search
	}
	if sortBy := strings.TrimSpace(q.Get("sort_by")); sortBy != "" {
		filter.SortBy = sortBy
	}
	if sortOrder := strings.TrimSpace(q.Get("sort_order")); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	limit := 50
	if rawLimit := q.Get("limit"); rawLimit != "" {
		if val, err := strconv.Atoi(rawLimit); err == nil && val > 0 {
			limit = val
		}
	} else if rawPerPage := q.Get("per_page"); rawPerPage != "" {
		if val, err := strconv.Atoi(rawPerPage); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 1000 {
		limit = 1000
	}
	filter.Limit = limit

	offset := 0
	if rawOffset := q.Get("offset"); rawOffset != "" {
		if val, err := strconv.Atoi(rawOffset); err == nil && val >= 0 {
			offset = val
		}
	} else if rawPage := q.Get("page"); rawPage != "" {
		if val, err := strconv.Atoi(rawPage); err == nil && val > 0 {
			offset = (val - 1) * limit
		}
	}
	filter.Offset = offset

	books, err := h.repo.ListBooks(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list books: %v", err))
		return
	}

	total, err := h.repo.CountBooks(r.Context(), filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to count books: %v", err))
		return
	}

	var items []BookListItem
	for _, b := range books {
		items = append(items, BuildBookListItem(r.Context(), h.repo, b))
	}

	h.logger.Debug("Listed books", "count", len(items), "total", total, "limit", limit, "offset", offset)

	writeJSON(w, http.StatusOK, map[string]any{
		"books":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *BookHandler) GetBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	authors, _ := h.repo.GetBookAuthors(r.Context(), book.ID)
	genres, _ := h.repo.GetBookGenres(r.Context(), book.ID)
	seriesList, _ := h.repo.GetBookSeries(r.Context(), book.ID)
	spine, _ := h.repo.GetBookSpine(r.Context(), book.ID)
	chapters, _ := h.repo.GetChaptersByBookID(r.Context(), book.ID)
	// Clear ContentPlain on chapters in BookDetailResponse to keep payload lightweight
	for _, c := range chapters {
		c.ContentPlain = ""
	}
	bookmarks, _ := h.repo.ListBookmarksByBookID(r.Context(), book.ID)
	highlights, _ := h.repo.ListHighlightsByBookID(r.Context(), book.ID)

	writeJSON(w, http.StatusOK, BookDetailResponse{
		BookListItem: BookListItem{
			ID:                       book.ID,
			Title:                    book.Title,
			Description:              book.Description,
			Language:                 book.Language,
			Publisher:                book.Publisher,
			Identifier:               book.Identifier,
			FilePath:                 book.FilePath,
			CoverPath:                book.CoverPath,
			FileSizeBytes:            book.FileSizeBytes,
			FileModifiedAt:           book.FileModifiedAt,
			PublishedDate:            book.PublishedDate,
			Layout:                   book.Layout,
			RenditionSpread:          book.RenditionSpread,
			RenditionOrientation:     book.RenditionOrientation,
			PageProgressionDirection: book.PageProgressionDirection,
			Authors:                  authors,
			Genres:                   genres,
			Series:                   seriesList,
		},
		Spine:      spine,
		Chapters:   chapters,
		Bookmarks:  bookmarks,
		Highlights: highlights,
	})
}

func (h *BookHandler) GetBookCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			bookID = parts[len(parts)-2]
		}
	}

	var coverFile string

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if err == nil && book != nil && book.CoverPath != nil && *book.CoverPath != "" {
		// 1. Check relative to libraryDir
		libPath := filepath.Join(h.libraryDir, *book.CoverPath)
		if fi, err := os.Stat(libPath); err == nil && !fi.IsDir() {
			coverFile = libPath
		}
		// 2. Check relative to dataDir
		if coverFile == "" {
			dataPath := filepath.Join(h.dataDir, *book.CoverPath)
			if fi, err := os.Stat(dataPath); err == nil && !fi.IsDir() {
				coverFile = dataPath
			}
		}
		// 3. Absolute path
		if coverFile == "" {
			if fi, err := os.Stat(*book.CoverPath); err == nil && !fi.IsDir() {
				coverFile = *book.CoverPath
			}
		}
	}

	// 4. Fallback check covers/{bookID}.* in dataDir
	if coverFile == "" {
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
			df := filepath.Join(h.dataDir, "covers", bookID+ext)
			if fi, err := os.Stat(df); err == nil && !fi.IsDir() {
				coverFile = df
				break
			}
		}
	}

	if coverFile == "" {
		writeJSONError(w, http.StatusNotFound, "Cover image not found")
		return
	}

	contentType := "image/jpeg"
	switch strings.ToLower(filepath.Ext(coverFile)) {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, coverFile)
}

func (h *BookHandler) GetChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	identifier := r.PathValue("index")
	if identifier == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		identifier = parts[len(parts)-1]
	}

	var chapter *repository.Chapter
	var err error

	if index, parseErr := strconv.Atoi(identifier); parseErr == nil {
		if index == 0 {
			// Fallback: Resolve legacy index 0 gracefully to the first chapter in the book's spine
			spine, spineErr := h.repo.GetBookSpine(r.Context(), bookID)
			if spineErr == nil && len(spine) > 0 {
				chapter, err = h.repo.GetChapterByID(r.Context(), spine[0].ID)
			} else {
				err = repository.ErrNotFound
			}
		} else {
			chapter, err = h.repo.GetChapterByBookAndIndex(r.Context(), bookID, index)
		}
	} else if ulid.IsValid(identifier) || isUUID(identifier) {
		// Lookup by chapter ID (ULID or UUID)
		chapter, err = h.repo.GetChapterByID(r.Context(), identifier)
		if err == nil && chapter != nil && chapter.BookID != bookID {
			writeJSONError(w, http.StatusNotFound, "Chapter not found")
			return
		}
	} else {
		writeJSONError(w, http.StatusBadRequest, "Invalid chapter identifier")
		return
	}

	if errors.Is(err, repository.ErrNotFound) || chapter == nil {
		writeJSONError(w, http.StatusNotFound, "Chapter not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chapter: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, chapter)
}

func (h *BookHandler) GetChapterDirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	chapterID := r.PathValue("id")
	if chapterID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		chapterID = parts[len(parts)-1]
	}

	chapter, err := h.repo.GetChapterByID(r.Context(), chapterID)
	if errors.Is(err, repository.ErrNotFound) || chapter == nil {
		writeJSONError(w, http.StatusNotFound, "Chapter not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chapter: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, chapter)
}

func (h *BookHandler) resolveBookFilePath(b *repository.Book) string {
	if b == nil || b.FilePath == "" {
		return ""
	}
	// 1. Check relative to libraryDir
	libPath := filepath.Join(h.libraryDir, b.FilePath)
	if fi, err := os.Stat(libPath); err == nil && !fi.IsDir() {
		return libPath
	}
	// 2. Check relative to dataDir
	dataPath := filepath.Join(h.dataDir, b.FilePath)
	if fi, err := os.Stat(dataPath); err == nil && !fi.IsDir() {
		return dataPath
	}
	// 3. Absolute path
	if fi, err := os.Stat(b.FilePath); err == nil && !fi.IsDir() {
		return b.FilePath
	}
	return ""
}

func (h *BookHandler) GetBookAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	assetPath := r.PathValue("path")
	if bookID == "" || assetPath == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "books" && i+1 < len(parts) {
				bookID = parts[i+1]
			}
			if p == "assets" && i+1 < len(parts) {
				assetPath = strings.Join(parts[i+1:], "/")
				break
			}
		}
	}

	if bookID == "" || assetPath == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID and asset path required")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	epubPath := h.resolveBookFilePath(book)
	if epubPath == "" {
		writeJSONError(w, http.StatusNotFound, "Book file not found on disk")
		return
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open epub: %v", err))
		return
	}
	defer reader.Close()

	assetData, mimeType, err := reader.ExtractAsset(assetPath)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("Asset not found: %v", err))
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(assetData)
}

func (h *BookHandler) GetChapterHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	identifier := r.PathValue("index")
	if bookID == "" || identifier == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "books" && i+1 < len(parts) {
				bookID = parts[i+1]
			}
			if p == "chapters" && i+1 < len(parts) {
				identifier = parts[i+1]
			}
		}
	}

	if bookID == "" || identifier == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID and chapter index required")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	var chapter *repository.Chapter
	if index, parseErr := strconv.Atoi(identifier); parseErr == nil {
		if index == 0 {
			spine, spineErr := h.repo.GetBookSpine(r.Context(), bookID)
			if spineErr == nil && len(spine) > 0 {
				chapter, err = h.repo.GetChapterByID(r.Context(), spine[0].ID)
			} else {
				err = repository.ErrNotFound
			}
		} else {
			chapter, err = h.repo.GetChapterByBookAndIndex(r.Context(), bookID, index)
		}
	} else if ulid.IsValid(identifier) || isUUID(identifier) {
		chapter, err = h.repo.GetChapterByID(r.Context(), identifier)
		if err == nil && chapter != nil && chapter.BookID != bookID {
			writeJSONError(w, http.StatusNotFound, "Chapter not found")
			return
		}
	} else {
		writeJSONError(w, http.StatusBadRequest, "Invalid chapter identifier")
		return
	}

	if errors.Is(err, repository.ErrNotFound) || chapter == nil {
		writeJSONError(w, http.StatusNotFound, "Chapter not found")
		return
	}

	epubPath := h.resolveBookFilePath(book)
	if epubPath == "" {
		writeJSONError(w, http.StatusNotFound, "Book file not found on disk")
		return
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open epub: %v", err))
		return
	}
	defer reader.Close()

	var rawHTML []byte
	var docHref string
	if chapter.Href != nil && *chapter.Href != "" {
		docHref = *chapter.Href
		rawHTML, err = reader.ExtractRawDocument(docHref)
	}

	if len(rawHTML) == 0 {
		parsedChapters, pErr := reader.ExtractChapters()
		if pErr == nil {
			for _, pc := range parsedChapters {
				if pc.Index == chapter.ChapterIndex {
					docHref = pc.Href
					rawHTML, err = reader.ExtractRawDocument(docHref)
					break
				}
			}
		}
	}

	if err != nil || len(rawHTML) == 0 {
		writeJSONError(w, http.StatusNotFound, "Chapter document not found in epub")
		return
	}

	rewrittenHTML := epub.RewritePageHTML(string(rawHTML), bookID, docHref, chapter.PageWidth, chapter.PageHeight)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rewrittenHTML))
}

func (h *BookHandler) UploadBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if r.URL.Query().Get("stage") == "true" {
		h.StageUploadBook(w, r)
		return
	}

	// Limit upload size to 60MB
	if err := r.ParseMultipartForm(60 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in multipart upload")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".epub") {
		writeJSONError(w, http.StatusBadRequest, "Only .epub files are accepted")
		return
	}

	// 1. Stage upload to dataDir/uploads/<job_id>.epub
	jobID := uuid.NewString()
	uploadsDir := filepath.Join(h.dataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create uploads directory")
		return
	}

	stagedPath := filepath.Join(uploadsDir, jobID+".epub")
	stagedFile, err := os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create staged upload file")
		return
	}

	if _, err := io.Copy(stagedFile, file); err != nil {
		stagedFile.Close()
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusInternalServerError, "Failed to write staged upload file")
		return
	}
	stagedFile.Close()

	// 2. Validate zip archive header
	zipReader, err := zip.OpenReader(stagedPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid EPUB archive: %v", err))
		return
	}
	zipReader.Close()

	// 3. Create upload job in database
	job := &repository.UploadJob{
		ID:         jobID,
		Filename:   header.Filename,
		StagedPath: stagedPath,
		Status:     "queued",
	}
	if err := h.repo.CreateUploadJob(r.Context(), job); err != nil {
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to enqueue upload job: %v", err))
		return
	}

	// 4. Trigger upload background worker
	if h.uploadWorker != nil {
		h.uploadWorker.Trigger()
	}

	h.logger.Info("Enqueued book upload job", "job_id", job.ID, "filename", job.Filename, "size_bytes", header.Size)

	// 5. Return 202 Accepted
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     job.ID,
		"status":     job.Status,
		"filename":   job.Filename,
		"message":    "Upload enqueued for processing",
		"created_at": job.CreatedAt,
	})
}

// StagedMetadataDTO represents parsed EPUB metadata for user inspection before saving.
type StagedMetadataDTO struct {
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	Series         *string  `json:"series,omitempty"`
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Publisher      *string  `json:"publisher,omitempty"`
	Language       *string  `json:"language,omitempty"`
	Genres         []string `json:"genres,omitempty"`
}

// CommitUploadRequest represents the user-confirmed or edited metadata to commit to /library.
type CommitUploadRequest struct {
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Authors        []string `json:"authors,omitempty"`
	Series         *string  `json:"series,omitempty"`
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Publisher      *string  `json:"publisher,omitempty"`
	Language       *string  `json:"language,omitempty"`
	Genres         []string `json:"genres,omitempty"`
}

// StageUploadBook stages an EPUB file and extracts its metadata and cover preview without committing to /library.
func (h *BookHandler) StageUploadBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if err := r.ParseMultipartForm(60 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in multipart upload")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".epub") {
		writeJSONError(w, http.StatusBadRequest, "Only .epub files are accepted")
		return
	}

	jobID := uuid.NewString()
	uploadsDir := filepath.Join(h.dataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create uploads directory")
		return
	}

	stagedPath := filepath.Join(uploadsDir, jobID+".epub")
	stagedFile, err := os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create staged upload file")
		return
	}

	if _, err := io.Copy(stagedFile, file); err != nil {
		stagedFile.Close()
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusInternalServerError, "Failed to write staged upload file")
		return
	}
	stagedFile.Close()

	// Parse EPUB metadata and check for cover
	epubReader, err := epub.Open(stagedPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid EPUB archive: %v", err))
		return
	}
	parsed, err := epubReader.ParseBook()
	if err != nil {
		epubReader.Close()
		_ = os.Remove(stagedPath)
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse EPUB metadata: %v", err))
		return
	}

	coverBytes, _, coverErr := epubReader.ExtractCoverImage()
	hasCover := false
	if coverErr == nil && len(coverBytes) > 0 {
		hasCover = true
		coverPath := filepath.Join(uploadsDir, jobID+".cover")
		_ = os.WriteFile(coverPath, coverBytes, 0644)
	}
	epubReader.Close()

	authors := make([]string, 0, len(parsed.Authors))
	for _, a := range parsed.Authors {
		if strings.TrimSpace(a.Name) != "" {
			authors = append(authors, strings.TrimSpace(a.Name))
		}
	}
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}

	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	var series *string
	var seqNum *float64
	if parsed.Series != nil && strings.TrimSpace(parsed.Series.Name) != "" {
		s := strings.TrimSpace(parsed.Series.Name)
		series = &s
		seqNum = parsed.Series.SequenceNumber
	}

	var desc *string
	if strings.TrimSpace(parsed.Description) != "" {
		d := strings.TrimSpace(parsed.Description)
		desc = &d
	}

	var pub *string
	if strings.TrimSpace(parsed.Publisher) != "" {
		p := strings.TrimSpace(parsed.Publisher)
		pub = &p
	}

	var lang *string
	if strings.TrimSpace(parsed.Language) != "" {
		l := strings.TrimSpace(parsed.Language)
		lang = &l
	}

	dto := StagedMetadataDTO{
		Title:          title,
		Authors:        authors,
		Series:         series,
		SequenceNumber: seqNum,
		Description:    desc,
		Publisher:      pub,
		Language:       lang,
		Genres:         parsed.Genres,
	}

	metaBytes, _ := json.Marshal(dto)
	metaStr := string(metaBytes)

	var warnings []string
	if len(authors) == 1 && authors[0] == "Unknown" {
		warnings = append(warnings, "No author found in EPUB metadata")
	}
	if title == "Untitled" {
		warnings = append(warnings, "No title found in EPUB metadata")
	}

	job := &repository.UploadJob{
		ID:         jobID,
		Filename:   header.Filename,
		StagedPath: stagedPath,
		Status:     "staged",
		Metadata:   &metaStr,
		HasCover:   hasCover,
	}
	if err := h.repo.CreateUploadJob(r.Context(), job); err != nil {
		_ = os.Remove(stagedPath)
		_ = os.Remove(filepath.Join(uploadsDir, jobID+".cover"))
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to record staged upload: %v", err))
		return
	}

	h.logger.Info("Staged book upload for review", "job_id", job.ID, "filename", job.Filename, "title", title)

	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":     job.ID,
		"status":     job.Status,
		"filename":   job.Filename,
		"metadata":   dto,
		"has_cover":  hasCover,
		"warnings":   warnings,
		"created_at": job.CreatedAt,
	})
}

// GetUploadJobCover returns the extracted cover image for a staged upload job.
func (h *BookHandler) GetUploadJobCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		jobID = extractIDFromPath(r.URL.Path, "jobs")
	}
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	coverPath := filepath.Join(h.dataDir, "uploads", jobID+".cover")
	if data, err := os.ReadFile(coverPath); err == nil && len(data) > 0 {
		w.Header().Set("Content-Type", http.DetectContentType(data))
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// Fallback: extract directly from staged EPUB file
	job, err := h.repo.GetUploadJob(r.Context(), jobID)
	if err == nil && job != nil && job.StagedPath != "" {
		if reader, err := epub.Open(job.StagedPath); err == nil {
			defer reader.Close()
			if data, _, err := reader.ExtractCoverImage(); err == nil && len(data) > 0 {
				_ = os.WriteFile(coverPath, data, 0644)
				w.Header().Set("Content-Type", http.DetectContentType(data))
				w.Header().Set("Cache-Control", "public, max-age=3600")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
				return
			}
		}
	}

	writeJSONError(w, http.StatusNotFound, "Cover image not found for staged upload")
}

// CommitUploadJob applies any edited metadata to the EPUB on disk and finalizes ingestion into /library.
func (h *BookHandler) CommitUploadJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		jobID = extractIDFromPath(r.URL.Path, "jobs")
	}
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	job, err := h.repo.GetUploadJob(r.Context(), jobID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Upload job not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get upload job: %v", err))
		return
	}

	if job.Status != "staged" {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Upload job is not in 'staged' status (current status: %s)", job.Status))
		return
	}

	var req CommitUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request body: %v", err))
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeJSONError(w, http.StatusBadRequest, "Book title is required")
		return
	}

	authors := req.Authors
	if len(authors) == 0 && strings.TrimSpace(req.Author) != "" {
		authors = []string{strings.TrimSpace(req.Author)}
	}
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}
	primaryAuthor := authors[0]

	// Update the EPUB file package metadata on disk
	update := epub.MetadataUpdate{
		Title:          title,
		Authors:        authors,
		Series:         req.Series,
		SequenceNumber: req.SequenceNumber,
		Description:    req.Description,
		Publisher:      req.Publisher,
		Language:       req.Language,
		Genres:         req.Genres,
	}
	if err := epub.UpdateMetadata(job.StagedPath, update); err != nil {
		h.logger.Error("Failed to update EPUB metadata", "job_id", job.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update EPUB metadata: %v", err))
		return
	}

	// Ingest the updated book into /library/<Author>/<Title>/<Title>.epub
	stagedFile, err := os.Open(job.StagedPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open staged EPUB: %v", err))
		return
	}
	defer stagedFile.Close()

	book, err := h.ingester.SaveUpload(r.Context(), primaryAuthor, title, stagedFile)
	stagedFile.Close()
	if err != nil {
		h.logger.Error("Failed to ingest book into library", "job_id", job.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to ingest book into library: %v", err))
		return
	}

	// Clean up staged file and cached cover
	_ = os.Remove(job.StagedPath)
	_ = os.Remove(filepath.Join(h.dataDir, "uploads", job.ID+".cover"))

	_ = h.repo.UpdateUploadJobStatus(r.Context(), job.ID, "completed", &book.ID, nil)

	if h.worker != nil {
		h.worker.Trigger()
	}

	if h.hub != nil && book != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookAdded,
			Data: BuildBookListItem(r.Context(), h.repo, book),
		})
	}

	h.logger.Info("Committed book upload to library", "job_id", job.ID, "book_id", book.ID, "title", book.Title, "author", primaryAuthor)

	writeJSON(w, http.StatusCreated, book)
}

// DeleteUploadJob discards a staged upload job and removes temporary files.
func (h *BookHandler) DeleteUploadJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		jobID = extractIDFromPath(r.URL.Path, "jobs")
	}
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	job, err := h.repo.GetUploadJob(r.Context(), jobID)
	if err == nil && job != nil {
		_ = os.Remove(job.StagedPath)
		_ = os.Remove(filepath.Join(h.dataDir, "uploads", job.ID+".cover"))
		_ = h.repo.DeleteUploadJob(r.Context(), jobID)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BookHandler) GetUploadJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		jobID = extractIDFromPath(r.URL.Path, "jobs")
	}
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}

	job, err := h.repo.GetUploadJob(r.Context(), jobID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Upload job not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get upload job: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *BookHandler) ListUploadJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limit := 20
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if val, err := strconv.Atoi(rawLimit); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 100 {
		limit = 100
	}

	jobs, err := h.repo.ListUploadJobs(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list upload jobs: %v", err))
		return
	}

	if jobs == nil {
		jobs = []*repository.UploadJob{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func extractIDFromPath(path, segment string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == segment && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func (h *BookHandler) ReparseBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	if err := h.ingester.ReparseBookChapters(r.Context(), bookID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Book not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reparse book chapters: %v", err))
		return
	}

	if h.worker != nil {
		h.worker.Trigger()
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Book chapters reparsed successfully",
	})
}

func (h *BookHandler) ChatBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	book, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	if h.aiClient == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "AI service is not configured")
		return
	}

	var req BookChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %v", err))
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = strings.TrimSpace(req.Query)
	}
	if message == "" {
		writeJSONError(w, http.StatusBadRequest, "Message cannot be empty")
		return
	}

	var citations []BookCitation
	var contextBuilder strings.Builder

	// RAG: 1. Generate query embedding and search paragraphs scoped to bookID
	queryEmbedding, err := h.aiClient.GenerateEmbedding(r.Context(), message)
	if err == nil && len(queryEmbedding) > 0 {
		hits, err := h.repo.SearchVectorParagraphs(r.Context(), queryEmbedding, repository.SearchFilter{
			BookID: &bookID,
			Limit:  5,
		})
		if err == nil && len(hits) > 0 {
			for _, hit := range hits {
				chTitle := fmt.Sprintf("Chapter %d", hit.ChapterIndex)
				if hit.ChapterTitle != nil && *hit.ChapterTitle != "" {
					chTitle = fmt.Sprintf("Chapter %d: %s", hit.ChapterIndex, *hit.ChapterTitle)
				}
				contextBuilder.WriteString(fmt.Sprintf("[%s (Paragraphs %d-%d)]\n%s\n\n", chTitle, hit.StartParagraph, hit.EndParagraph, hit.Content))
				citations = append(citations, BookCitation{
					ChapterID:      hit.ChapterID,
					ChapterIndex:   hit.ChapterIndex,
					ChapterTitle:   hit.ChapterTitle,
					StartParagraph: hit.StartParagraph,
					EndParagraph:   hit.EndParagraph,
					Excerpt:        hit.Content,
					Summary:        hit.Content,
				})
			}
		}
	}

	// 2. Fallback to FTS5 search across paragraphs if vector search yielded no hits
	if len(citations) == 0 {
		ftsHits, err := h.repo.SearchFTSParagraphs(r.Context(), message, repository.SearchFilter{
			BookID: &bookID,
			Limit:  5,
		})
		if err == nil && len(ftsHits) > 0 {
			for _, hit := range ftsHits {
				chTitle := fmt.Sprintf("Chapter %d", hit.ChapterIndex)
				if hit.ChapterTitle != nil && *hit.ChapterTitle != "" {
					chTitle = fmt.Sprintf("Chapter %d: %s", hit.ChapterIndex, *hit.ChapterTitle)
				}
				contextBuilder.WriteString(fmt.Sprintf("[%s (Paragraphs %d-%d)]\n%s\n\n", chTitle, hit.StartParagraph, hit.EndParagraph, hit.Content))
				citations = append(citations, BookCitation{
					ChapterID:      hit.ChapterID,
					ChapterIndex:   hit.ChapterIndex,
					ChapterTitle:   hit.ChapterTitle,
					StartParagraph: hit.StartParagraph,
					EndParagraph:   hit.EndParagraph,
					Excerpt:        hit.Content,
					Summary:        hit.Content,
				})
			}
		}
	}

	// 3. Fallback to chapter summaries if still empty (e.g. legacy un-chunked book)
	if len(citations) == 0 {
		chapters, _ := h.repo.GetChaptersByBookID(r.Context(), bookID)
		for i, c := range chapters {
			if i >= 3 {
				break
			}
			if c.Summary != "" {
				title := fmt.Sprintf("Chapter %d", c.ChapterIndex)
				if c.Title != nil && *c.Title != "" {
					title = fmt.Sprintf("Chapter %d: %s", c.ChapterIndex, *c.Title)
				}
				contextBuilder.WriteString(fmt.Sprintf("[%s]\n%s\n\n", title, c.Summary))
				citations = append(citations, BookCitation{
					ChapterID:    c.ID,
					ChapterIndex: c.ChapterIndex,
					ChapterTitle: c.Title,
					Excerpt:      c.Summary,
					Summary:      c.Summary,
				})
			}
		}
	}

	systemPrompt := fmt.Sprintf(
		"You are an insightful, knowledgeable literary companion assisting a reader with the book \"%s\". "+
			"Answer the reader's question thoughtfully and accurately based on the book context below. "+
			"Ground your answer in the provided chapter text and summaries, and mention relevant chapters where applicable.\n\n"+
			"Retrieved Book Context:\n%s",
		book.Title, contextBuilder.String(),
	)

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	// Append recent conversation history (last 6 items)
	if len(req.History) > 0 {
		history := req.History
		if len(history) > 6 {
			history = history[len(history)-6:]
		}
		for _, m := range history {
			if m.Role == "user" || m.Role == "assistant" {
				messages = append(messages, m)
			}
		}
	}
	messages = append(messages, ai.ChatMessage{Role: "user", Content: message})

	reply, err := h.aiClient.Chat(r.Context(), messages)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("AI chat failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, BookChatResponse{
		Reply:     reply,
		Citations: citations,
	})
}

func (h *BookHandler) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	bookmarks, err := h.repo.ListBookmarksByBookID(r.Context(), bookID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list bookmarks: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, bookmarks)
}

func (h *BookHandler) CreateBookmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	var bm repository.Bookmark
	if err := json.NewDecoder(r.Body).Decode(&bm); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %v", err))
		return
	}

	bm.BookID = bookID
	if strings.TrimSpace(bm.Title) == "" {
		bm.Title = "Bookmark"
	}

	if err := h.repo.CreateBookmark(r.Context(), &bm); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create bookmark: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, bm)
}

func (h *BookHandler) DeleteBookmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookmarkID := r.PathValue("bookmarkId")
	if bookmarkID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		bookmarkID = parts[len(parts)-1]
	}

	if err := h.repo.DeleteBookmark(r.Context(), bookmarkID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Bookmark not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete bookmark: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *BookHandler) ListHighlights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	highlights, err := h.repo.ListHighlightsByBookID(r.Context(), bookID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list highlights: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, highlights)
}

func (h *BookHandler) CreateHighlight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	if bookID == "" {
		bookID = extractIDFromPath(r.URL.Path, "books")
	}
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID required")
		return
	}

	var hl repository.Highlight
	if err := json.NewDecoder(r.Body).Decode(&hl); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %v", err))
		return
	}

	hl.BookID = bookID
	if strings.TrimSpace(hl.SelectedText) == "" {
		writeJSONError(w, http.StatusBadRequest, "Selected text cannot be empty")
		return
	}

	hl.Color = strings.ToLower(strings.TrimSpace(hl.Color))
	switch hl.Color {
	case "yellow", "blue", "pink", "orange":
		// valid kindle color
	default:
		hl.Color = "yellow"
	}

	if err := h.repo.CreateHighlight(r.Context(), &hl); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create highlight: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, hl)
}

func (h *BookHandler) DeleteHighlight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	highlightID := r.PathValue("highlightId")
	if highlightID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		highlightID = parts[len(parts)-1]
	}

	if err := h.repo.DeleteHighlight(r.Context(), highlightID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Highlight not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete highlight: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *BookHandler) SearchLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, []*repository.SearchHit{})
		return
	}

	limit := 10
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if val, err := strconv.Atoi(rawLimit); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 50 {
		limit = 50
	}

	if h.aiClient == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "AI service not configured for semantic search")
		return
	}

	queryEmbedding, err := h.aiClient.GenerateEmbedding(r.Context(), query)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate embedding: %v", err))
		return
	}

	filter := repository.SearchFilter{
		Limit: limit,
	}
	hits, err := h.repo.SearchVectorParagraphs(r.Context(), queryEmbedding, filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Vector search failed: %v", err))
		return
	}

	if hits == nil {
		hits = []*repository.SearchHit{}
	}

	writeJSON(w, http.StatusOK, hits)
}


