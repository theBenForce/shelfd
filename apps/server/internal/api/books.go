package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

type BookHandler struct {
	repo       repository.StorageEngine
	ingester   *scanner.Ingester
	worker     *worker.Worker
	dataDir    string
	libraryDir string
}

func NewBookHandler(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	worker *worker.Worker,
	dataDir string,
	libraryDir string,
) *BookHandler {
	return &BookHandler{
		repo:       repo,
		ingester:   ingester,
		worker:     worker,
		dataDir:    dataDir,
		libraryDir: libraryDir,
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
	PublishedDate  *string                        `json:"published_date,omitempty"`
	Authors        []*repository.Author           `json:"authors"`
	Genres         []*repository.Genre            `json:"genres"`
	Series         []*repository.BookSeriesDetail `json:"series"`
}

type BookDetailResponse struct {
	BookListItem
	Chapters []*repository.Chapter `json:"chapters"`
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

	limit := 20
	if rawLimit := q.Get("limit"); rawLimit != "" {
		if val, err := strconv.Atoi(rawLimit); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 100 {
		limit = 100
	}
	filter.Limit = limit

	offset := 0
	if rawOffset := q.Get("offset"); rawOffset != "" {
		if val, err := strconv.Atoi(rawOffset); err == nil && val >= 0 {
			offset = val
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
		authors, _ := h.repo.GetBookAuthors(r.Context(), b.ID)
		genres, _ := h.repo.GetBookGenres(r.Context(), b.ID)
		seriesList, _ := h.repo.GetBookSeries(r.Context(), b.ID)

		items = append(items, BookListItem{
			ID:            b.ID,
			Title:         b.Title,
			Description:   b.Description,
			Language:      b.Language,
			Publisher:     b.Publisher,
			Identifier:    b.Identifier,
			FilePath:      b.FilePath,
			CoverPath:     b.CoverPath,
			FileSizeBytes: b.FileSizeBytes,
			PublishedDate: b.PublishedDate,
			Authors:       authors,
			Genres:        genres,
			Series:        seriesList,
		})
	}

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
	chapters, _ := h.repo.GetChaptersByBookID(r.Context(), book.ID)

	writeJSON(w, http.StatusOK, BookDetailResponse{
		BookListItem: BookListItem{
			ID:            book.ID,
			Title:         book.Title,
			Description:   book.Description,
			Language:      book.Language,
			Publisher:     book.Publisher,
			Identifier:    book.Identifier,
			FilePath:      book.FilePath,
			CoverPath:     book.CoverPath,
			FileSizeBytes: book.FileSizeBytes,
			PublishedDate: book.PublishedDate,
			Authors:       authors,
			Genres:        genres,
			Series:        seriesList,
		},
		Chapters: chapters,
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

	coverFile := filepath.Join(h.dataDir, "covers", bookID+".jpg")
	if _, err := os.Stat(coverFile); os.IsNotExist(err) {
		// Fallback check cover_path from book record
		book, err := h.repo.GetBookByID(r.Context(), bookID)
		if err == nil && book != nil && book.CoverPath != nil {
			if _, err := os.Stat(*book.CoverPath); err == nil {
				coverFile = *book.CoverPath
			}
		}
	}

	if _, err := os.Stat(coverFile); os.IsNotExist(err) {
		writeJSONError(w, http.StatusNotFound, "Cover image not found")
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
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
	indexStr := r.PathValue("index")
	if indexStr == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		indexStr = parts[len(parts)-1]
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid chapter index")
		return
	}

	chapter, err := h.repo.GetChapterByBookAndIndex(r.Context(), bookID, index)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Chapter not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chapter: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, chapter)
}

func (h *BookHandler) UploadBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
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

	// 1. Stage upload to a temporary file in dataDir/tmp
	tmpDir := filepath.Join(h.dataDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755)
	tmpFile, err := os.CreateTemp(tmpDir, "upload-*.epub")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create temp file")
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		writeJSONError(w, http.StatusInternalServerError, "Failed to write uploaded file")
		return
	}
	tmpFile.Close()

	// 2. Open EPUB to read Author and Title metadata
	epubReader, err := epub.Open(tmpPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid EPUB archive: %v", err))
		return
	}
	parsed, err := epubReader.ParseBook()
	epubReader.Close()
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse EPUB metadata: %v", err))
		return
	}

	author := "Unknown"
	if len(parsed.Authors) > 0 && strings.TrimSpace(parsed.Authors[0].Name) != "" {
		author = parsed.Authors[0].Name
	}
	title := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	if strings.TrimSpace(parsed.Title) != "" {
		title = parsed.Title
	}

	// 3. Save directly into Audiobookshelf library structure and ingest
	stagedFile, err := os.Open(tmpPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read staged upload")
		return
	}
	defer stagedFile.Close()

	book, err := h.ingester.SaveUpload(r.Context(), author, title, stagedFile)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to ingest book: %v", err))
		return
	}

	if h.worker != nil {
		h.worker.Trigger()
	}

	writeJSON(w, http.StatusCreated, book)
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
