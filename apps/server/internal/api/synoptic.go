package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

type SynopticHandler struct {
	service *ai.SynopticService
	logger  *slog.Logger
}

func NewSynopticHandler(repo repository.StorageEngine, aiClient ai.Client, logger *slog.Logger) *SynopticHandler {
	if logger == nil {
		logger = slog.Default()
	}
	var service *ai.SynopticService
	if repo != nil && aiClient != nil {
		service = ai.NewSynopticService(repo, aiClient)
	}
	return &SynopticHandler{
		service: service,
		logger:  logger,
	}
}

type SynopticQueryRequest struct {
	Query              string   `json:"query"`
	BookIDs            []string `json:"book_ids,omitempty"`
	AuthorIDs          []string `json:"author_ids,omitempty"`
	GenreID            *string  `json:"genre_id,omitempty"`
	MaxPassagesPerBook int      `json:"max_passages_per_book,omitempty"`
	Limit              int      `json:"limit,omitempty"`
}

type SynopticCitation struct {
	BookID         string  `json:"book_id"`
	BookTitle      string  `json:"book_title"`
	AuthorName     string  `json:"author_name,omitempty"`
	ChapterID      string  `json:"chapter_id"`
	ChapterTitle   string  `json:"chapter_title,omitempty"`
	ChapterIndex   int     `json:"chapter_index"`
	StartParagraph int     `json:"start_paragraph"`
	EndParagraph   int     `json:"end_paragraph"`
	Excerpt        string  `json:"excerpt"`
	CosineDistance float32 `json:"cosine_distance,omitempty"`
}

type SynopticQueryResponse struct {
	Summary          string             `json:"summary"`
	ConsensusPoints  []ai.SynopticPoint `json:"consensus_points"`
	DivergencePoints []ai.SynopticPoint `json:"divergence_points"`
	Citations        []SynopticCitation `json:"citations"`
	BooksCovered     int                `json:"books_covered"`
}

func (h *SynopticHandler) Query(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var payload SynopticQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON request body")
		return
	}

	req := ai.SynopticRequest{
		Query:              payload.Query,
		BookIDs:            payload.BookIDs,
		AuthorIDs:          payload.AuthorIDs,
		GenreID:            payload.GenreID,
		MaxPassagesPerBook: payload.MaxPassagesPerBook,
		Limit:              payload.Limit,
	}

	if err := ai.ValidateSynopticRequest(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.service == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "AI service not configured for synoptic synthesis")
		return
	}

	result, err := h.service.Synthesize(r.Context(), req)
	if err != nil {
		if errors.Is(err, ai.ErrInvalidSynopticRequest) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ai.ErrSynopticUnavailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "AI service not configured for synoptic synthesis")
			return
		}
		h.logger.Error("synoptic synthesis failed", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to run synoptic synthesis")
		return
	}

	citations := make([]SynopticCitation, 0, len(result.Citations))
	for _, c := range result.Citations {
		citations = append(citations, SynopticCitation{
			BookID:         c.BookID,
			BookTitle:      c.BookTitle,
			AuthorName:     c.AuthorName,
			ChapterID:      c.ChapterID,
			ChapterTitle:   c.ChapterTitle,
			ChapterIndex:   c.ChapterIndex,
			StartParagraph: c.StartParagraph,
			EndParagraph:   c.EndParagraph,
			Excerpt:        c.Excerpt,
			CosineDistance: c.CosineDistance,
		})
	}

	consensus := result.ConsensusPoints
	if consensus == nil {
		consensus = []ai.SynopticPoint{}
	}
	divergence := result.DivergencePoints
	if divergence == nil {
		divergence = []ai.SynopticPoint{}
	}

	writeJSON(w, http.StatusOK, SynopticQueryResponse{
		Summary:          result.Summary,
		ConsensusPoints:  consensus,
		DivergencePoints: divergence,
		Citations:        citations,
		BooksCovered:     result.BooksCovered,
	})
}
