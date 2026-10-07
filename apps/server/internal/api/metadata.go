package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/metadata"
	"github.com/shelfd/shelfd/internal/repository"
)

// MetadataHandler handles search and auto-population of book metadata from external providers.
type MetadataHandler struct {
	repo       repository.StorageEngine
	service    *metadata.Service
	dataDir    string
	libraryDir string
	hub        *events.Hub
	logger     *slog.Logger
}

// NewMetadataHandler constructs a new MetadataHandler.
func NewMetadataHandler(
	repo repository.StorageEngine,
	dataDir string,
	libraryDir string,
	hub *events.Hub,
	logger *slog.Logger,
) *MetadataHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &MetadataHandler{
		repo:       repo,
		service:    metadata.NewService(nil),
		dataDir:    dataDir,
		libraryDir: libraryDir,
		hub:        hub,
		logger:     logger,
	}
}

// SetService allows overriding the metadata service (useful for testing).
func (h *MetadataHandler) SetService(svc *metadata.Service) {
	h.service = svc
}

// Search queries external metadata providers (Open Library, Google Books, Apple Books).
func (h *MetadataHandler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	q := r.URL.Query().Get("q")
	title := r.URL.Query().Get("title")
	author := r.URL.Query().Get("author")
	isbn := r.URL.Query().Get("isbn")
	provider := r.URL.Query().Get("provider")

	if q == "" && title == "" && author == "" && isbn == "" {
		writeJSONError(w, http.StatusBadRequest, "At least one search query parameter (q, title, author, isbn) is required")
		return
	}

	results, err := h.service.Search(r.Context(), metadata.SearchQuery{
		Query:    q,
		Title:    title,
		Author:   author,
		ISBN:     isbn,
		Provider: provider,
	})
	if err != nil {
		h.logger.Error("Failed to search metadata providers", "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to search metadata: %v", err))
		return
	}

	if results == nil {
		results = []metadata.SearchResult{}
	}

	writeJSON(w, http.StatusOK, results)
}

// FetchCoverRequest represents a request to download cover art from a URL.
type FetchCoverRequest struct {
	CoverURL string `json:"cover_url"`
}

// FetchBookCover downloads remote cover art and attaches it to an existing book.
func (h *MetadataHandler) FetchBookCover(w http.ResponseWriter, r *http.Request) {
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

	var req FetchCoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if strings.TrimSpace(req.CoverURL) == "" {
		writeJSONError(w, http.StatusBadRequest, "cover_url is required")
		return
	}

	coverData, contentType, err := h.service.FetchCoverImage(r.Context(), req.CoverURL)
	if err != nil {
		h.logger.Error("Failed to fetch cover image from URL", "cover_url", req.CoverURL, "error", err)
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to fetch cover from URL: %v", err))
		return
	}

	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}

	// 1. Audiobookshelf convention: Save cover file adjacent to EPUB in book directory
	epubPath := h.resolveBookFilePath(book)
	var relCoverPath string
	if epubPath != "" {
		bookDir := filepath.Dir(epubPath)
		coverFilename := "cover" + ext
		fullCoverPath := filepath.Join(bookDir, coverFilename)
		if err := os.WriteFile(fullCoverPath, coverData, 0644); err == nil {
			if strings.HasPrefix(fullCoverPath, h.libraryDir) {
				relCoverPath, _ = filepath.Rel(h.libraryDir, fullCoverPath)
			}
		}

		// Update internal EPUB embedded cover
		if err := epub.UpdateCover(epubPath, coverData, contentType); err != nil {
			h.logger.Warn("Failed to update EPUB embedded cover", "book_id", book.ID, "error", err)
		}
	}

	// 2. Fallback save in private dataDir/covers/<bookID>.<ext>
	if relCoverPath == "" {
		coversDir := filepath.Join(h.dataDir, "covers")
		_ = os.MkdirAll(coversDir, 0755)
		fullCoverPath := filepath.Join(coversDir, bookID+ext)
		if err := os.WriteFile(fullCoverPath, coverData, 0644); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to save cover image")
			return
		}
		relCoverPath = filepath.Join("covers", bookID+ext)
	}

	book.CoverPath = &relCoverPath
	if err := h.repo.UpdateBook(r.Context(), book); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update book: %v", err))
		return
	}

	if h.hub != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookUpdated,
			Data: BuildBookListItem(r.Context(), h.repo, book),
		})
	}

	h.logger.Info("Fetched and updated book cover", "book_id", book.ID, "cover_path", relCoverPath)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "success",
		"cover_path": relCoverPath,
	})
}

// FetchJobCover downloads remote cover art and attaches it to a staged upload job.
func (h *MetadataHandler) FetchJobCover(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) || job == nil {
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

	var req FetchCoverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if strings.TrimSpace(req.CoverURL) == "" {
		writeJSONError(w, http.StatusBadRequest, "cover_url is required")
		return
	}

	coverData, contentType, err := h.service.FetchCoverImage(r.Context(), req.CoverURL)
	if err != nil {
		h.logger.Error("Failed to fetch job cover from URL", "cover_url", req.CoverURL, "error", err)
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to fetch cover from URL: %v", err))
		return
	}

	uploadsDir := filepath.Join(h.dataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to access uploads directory")
		return
	}

	coverPath := filepath.Join(uploadsDir, job.ID+".cover")
	if err := os.WriteFile(coverPath, coverData, 0644); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save cover image")
		return
	}

	// Update staged EPUB if it exists
	if job.StagedPath != "" {
		if err := epub.UpdateCover(job.StagedPath, coverData, contentType); err != nil {
			h.logger.Warn("Failed to update cover inside staged EPUB", "job_id", job.ID, "error", err)
		}
	}

	// Update database record
	if err := h.repo.UpdateUploadJobCover(r.Context(), job.ID, true); err != nil {
		h.logger.Warn("Failed to update upload job has_cover status", "job_id", job.ID, "error", err)
	}

	if h.hub != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventQueueStatus,
			Data: map[string]string{"job_id": job.ID, "status": job.Status},
		})
	}

	h.logger.Info("Fetched and updated staged job cover", "job_id", job.ID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "success",
	})
}

func (h *MetadataHandler) resolveBookFilePath(b *repository.Book) string {
	if b == nil || b.FilePath == "" {
		return ""
	}
	libPath := filepath.Join(h.libraryDir, b.FilePath)
	if fi, err := os.Stat(libPath); err == nil && !fi.IsDir() {
		return libPath
	}
	dataPath := filepath.Join(h.dataDir, b.FilePath)
	if fi, err := os.Stat(dataPath); err == nil && !fi.IsDir() {
		return dataPath
	}
	if fi, err := os.Stat(b.FilePath); err == nil && !fi.IsDir() {
		return b.FilePath
	}
	return ""
}
