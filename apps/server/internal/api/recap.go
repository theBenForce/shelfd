package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

type BookRecapRequest struct {
	ChapterIndex    int  `json:"chapter_index"`
	ParagraphOffset int  `json:"paragraph_offset"`
	ForceRefresh    bool `json:"force_refresh"`
}

type RecapHandler struct {
	service *ai.RecapService
	logger  *slog.Logger
}

func NewRecapHandler(repo repository.StorageEngine, aiClient ai.Client, logger *slog.Logger) *RecapHandler {
	if logger == nil {
		logger = slog.Default()
	}
	var chatClient ai.ChatClient
	if aiClient != nil {
		chatClient = aiClient
	}
	return &RecapHandler{
		service: ai.NewRecapService(repo, chatClient, logger),
		logger:  logger,
	}
}

func (h *RecapHandler) GenerateRecap(w http.ResponseWriter, r *http.Request) {
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

	var req BookRecapRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request payload: %v", err))
			return
		}
	}

	if req.ChapterIndex < 0 {
		writeJSONError(w, http.StatusBadRequest, "chapter_index must be non-negative")
		return
	}
	if req.ParagraphOffset < 0 {
		writeJSONError(w, http.StatusBadRequest, "paragraph_offset must be non-negative")
		return
	}

	result, err := h.service.Generate(r.Context(), bookID, req.ChapterIndex, req.ParagraphOffset, req.ForceRefresh)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrInvalidRecapPosition):
			writeJSONError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "Book not found")
		case errors.Is(err, ai.ErrRecapAIClientUnavailable):
			writeJSONError(w, http.StatusServiceUnavailable, "AI service is not configured")
		default:
			h.logger.Error("recap generation failed", "book_id", bookID, "chapter_index", req.ChapterIndex, "error", err)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate recap: %v", err))
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}
