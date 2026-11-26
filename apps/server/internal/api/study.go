package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/study"
)

type StudyHandler struct {
	repo     repository.StorageEngine
	aiClient ai.ChatClient
	logger   *slog.Logger
}

func NewStudyHandler(repo repository.StorageEngine, aiClient ai.ChatClient, logger *slog.Logger) *StudyHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &StudyHandler{
		repo:     repo,
		aiClient: aiClient,
		logger:   logger,
	}
}

type studyDeckRequest struct {
	Format         string `json:"format"`
	ChapterIndices []int  `json:"chapter_indices,omitempty"`
	ChapterIndex   *int   `json:"chapter_index,omitempty"`
	MaxCards       int    `json:"max_cards,omitempty"`
}

func (h *StudyHandler) ExportStudyDeck(w http.ResponseWriter, r *http.Request) {
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

	var req studyDeckRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request body: %v", err))
			return
		}
	}

	format, err := study.ParseStudyDeckFormat(req.Format)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
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

	chapterIndices, err := h.resolveChapterIndices(r, bookID, req)
	if err != nil {
		if errors.Is(err, study.ErrInvalidRange) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to resolve chapters: %v", err))
		return
	}

	extractor := study.NewExtractor(h.repo, h.aiClient, h.logger)
	maxPerChapter := req.MaxCards
	if maxPerChapter <= 0 {
		maxPerChapter = study.DefaultMaxItems
	}

	var allCards []study.StudyCard
	var allConcepts []study.StudyConcept

	for _, chIdx := range chapterIndices {
		extractReq := study.ExtractRequest{
			BookID:       book.ID,
			ChapterIndex: chIdx,
			MaxItems:     maxPerChapter,
		}

		cards, err := extractor.GenerateCards(r.Context(), extractReq)
		if err != nil {
			h.writeExtractError(w, err)
			return
		}
		allCards = append(allCards, cards...)

		if format == study.FormatMarkdown {
			concepts, err := extractor.ExtractConcepts(r.Context(), extractReq)
			if err != nil {
				h.writeExtractError(w, err)
				return
			}
			allConcepts = append(allConcepts, concepts...)
		}
	}

	baseName := sanitizeDownloadFilename(book.Title)
	if baseName == "" {
		baseName = "study-deck"
	}

	var (
		payload     []byte
		contentType string
		filename    string
	)

	switch format {
	case study.FormatAPKG:
		payload, err = study.BuildAnkiAPKG(book.Title, allCards)
		contentType = "application/octet-stream"
		filename = baseName + "-study-deck.apkg"
	case study.FormatCSV:
		payload, err = study.BuildAnkiCSV(allCards)
		contentType = "text/csv; charset=utf-8"
		filename = baseName + "-study-deck.csv"
	case study.FormatMarkdown:
		payload, err = study.BuildMarkdownVaultZip(book.Title, allConcepts, allCards)
		contentType = "application/zip"
		filename = baseName + "-study-vault.zip"
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to build study deck: %v", err))
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-transform")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *StudyHandler) resolveChapterIndices(r *http.Request, bookID string, req studyDeckRequest) ([]int, error) {
	if len(req.ChapterIndices) > 0 {
		seen := make(map[int]struct{}, len(req.ChapterIndices))
		indices := make([]int, 0, len(req.ChapterIndices))
		for _, idx := range req.ChapterIndices {
			if idx < 0 {
				return nil, fmt.Errorf("%w: chapter_index %d must be >= 0", study.ErrInvalidRange, idx)
			}
			if _, ok := seen[idx]; !ok {
				seen[idx] = struct{}{}
				indices = append(indices, idx)
			}
		}
		sort.Ints(indices)
		return indices, nil
	}

	if req.ChapterIndex != nil {
		if *req.ChapterIndex < 0 {
			return nil, fmt.Errorf("%w: chapter_index %d must be >= 0", study.ErrInvalidRange, *req.ChapterIndex)
		}
		return []int{*req.ChapterIndex}, nil
	}

	chapters, err := h.repo.GetChaptersByBookID(r.Context(), bookID)
	if err != nil {
		return nil, err
	}
	indices := make([]int, 0, len(chapters))
	for _, ch := range chapters {
		indices = append(indices, ch.ChapterIndex)
	}
	return indices, nil
}

func (h *StudyHandler) writeExtractError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, study.ErrBookNotFound), errors.Is(err, study.ErrChapterNotFound):
		writeJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, study.ErrInvalidRange):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Study extraction failed: %v", err))
	}
}
