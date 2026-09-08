package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
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
	dataDir      string
	libraryDir   string
}

func NewBookHandler(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	worker *worker.Worker,
	uploadWorker *worker.UploadWorker,
	dataDir string,
	libraryDir string,
) *BookHandler {
	return &BookHandler{
		repo:         repo,
		ingester:     ingester,
		worker:       worker,
		uploadWorker: uploadWorker,
		dataDir:      dataDir,
		libraryDir:   libraryDir,
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
	Spine    []*repository.SpineItem `json:"spine"`
	Chapters []*repository.Chapter   `json:"chapters"`
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
	spine, _ := h.repo.GetBookSpine(r.Context(), book.ID)
	chapters, _ := h.repo.GetChaptersByBookID(r.Context(), book.ID)
	// Clear ContentPlain on chapters in BookDetailResponse to keep payload lightweight
	for _, c := range chapters {
		c.ContentPlain = ""
	}

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
		Spine:    spine,
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

	// 5. Return 202 Accepted
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     job.ID,
		"status":     job.Status,
		"filename":   job.Filename,
		"message":    "Upload enqueued for processing",
		"created_at": job.CreatedAt,
	})
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

