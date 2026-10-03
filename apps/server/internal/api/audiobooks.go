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

	"github.com/shelfd/shelfd/internal/repository"
)

type AudiobookHandler struct {
	repo       repository.StorageEngine
	dataDir    string
	libraryDir string
	logger     *slog.Logger
}

func NewAudiobookHandler(
	repo repository.StorageEngine,
	dataDir string,
	libraryDir string,
	logger *slog.Logger,
) *AudiobookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AudiobookHandler{
		repo:       repo,
		dataDir:    dataDir,
		libraryDir: libraryDir,
		logger:     logger,
	}
}

func getAudioMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".m4b", ".m4a":
		return "audio/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".opus":
		return "audio/opus"
	case ".epub":
		return "application/epub+zip"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func (h *AudiobookHandler) resolveFilePath(relOrAbsPath string) (string, error) {
	if filepath.IsAbs(relOrAbsPath) {
		if _, err := os.Stat(relOrAbsPath); err == nil {
			return relOrAbsPath, nil
		}
	}
	if h.libraryDir != "" {
		p := filepath.Join(h.libraryDir, relOrAbsPath)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if h.dataDir != "" {
		p := filepath.Join(h.dataDir, relOrAbsPath)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if _, err := os.Stat(relOrAbsPath); err == nil {
		return relOrAbsPath, nil
	}
	return "", fmt.Errorf("file not found: %s", relOrAbsPath)
}

// StreamAudio handles audio streaming (GET and HEAD) with full HTTP byte range support.
func (h *AudiobookHandler) StreamAudio(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID")
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

	// Try to find the audiobook file from book_files first, or fall back to book.FilePath
	audioPath := book.FilePath
	files, _ := h.repo.GetBookFilesByBookID(r.Context(), book.ID)
	for _, f := range files {
		if f.FileType == "audiobook" || f.FileType == "audio" {
			audioPath = f.FilePath
			break
		}
	}

	resolvedPath, err := h.resolveFilePath(audioPath)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Audio file not found on disk")
		return
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open audio file: %v", err))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stat audio file: %v", err))
		return
	}

	mimeType := getAudioMimeType(resolvedPath)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

// StreamBookFile handles streaming/downloading any associated file for a book.
func (h *AudiobookHandler) StreamBookFile(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if bookID == "" || fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID or file ID")
		return
	}

	bf, err := h.repo.GetBookFileByID(r.Context(), fileID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book file not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book file: %v", err))
		return
	}

	if bf.BookID != bookID {
		writeJSONError(w, http.StatusNotFound, "Book file does not belong to specified book")
		return
	}

	resolvedPath, err := h.resolveFilePath(bf.FilePath)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "File not found on disk")
		return
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open file: %v", err))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stat file: %v", err))
		return
	}

	mimeType := "application/octet-stream"
	if bf.MimeType != nil && *bf.MimeType != "" {
		mimeType = *bf.MimeType
	} else {
		mimeType = getAudioMimeType(resolvedPath)
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

// GetChapters returns the parsed audio chapter markers for an audiobook.
func (h *AudiobookHandler) GetChapters(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID")
		return
	}

	_, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	chapters, err := h.repo.GetAudioChaptersByBookID(r.Context(), bookID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get audio chapters: %v", err))
		return
	}
	if chapters == nil {
		chapters = []*repository.AudioChapter{}
	}

	writeJSON(w, http.StatusOK, chapters)
}

type UpdateAudiobookProgressRequest struct {
	PositionSeconds float64 `json:"position_seconds"`
	Speed           float64 `json:"speed"`
	IsCompleted     bool    `json:"is_completed"`
}

// GetProgress returns the user's saved listening progress for an audiobook.
func (h *AudiobookHandler) GetProgress(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID")
		return
	}

	_, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	userID := ""
	if u := CurrentUser(r.Context()); u != nil {
		userID = u.ID
	}

	progress, err := h.repo.GetAudiobookProgress(r.Context(), bookID, userID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSON(w, http.StatusOK, repository.AudiobookProgress{
			BookID:          bookID,
			UserID:          userID,
			PositionSeconds: 0.0,
			Speed:           1.0,
			IsCompleted:     false,
		})
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get audiobook progress: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

// UpdateProgress updates the listening progress for an audiobook.
func (h *AudiobookHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID")
		return
	}

	_, err := h.repo.GetBookByID(r.Context(), bookID)
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	var req UpdateAudiobookProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	speed := req.Speed
	if speed <= 0 {
		speed = 1.0
	}

	userID := ""
	if u := CurrentUser(r.Context()); u != nil {
		userID = u.ID
	}

	progress := &repository.AudiobookProgress{
		BookID:          bookID,
		UserID:          userID,
		PositionSeconds: req.PositionSeconds,
		Speed:           speed,
		IsCompleted:     req.IsCompleted,
	}

	if err := h.repo.UpsertAudiobookProgress(r.Context(), progress); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save progress: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

// DeleteProgress resets/deletes listening progress for an audiobook.
func (h *AudiobookHandler) DeleteProgress(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	if bookID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing book ID")
		return
	}

	userID := ""
	if u := CurrentUser(r.Context()); u != nil {
		userID = u.ID
	}

	if err := h.repo.DeleteAudiobookProgress(r.Context(), bookID, userID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete progress: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
