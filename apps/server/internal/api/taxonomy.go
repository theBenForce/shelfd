package api

import (
	"fmt"
	"net/http"

	"github.com/shelfd/shelfd/internal/repository"
)

type TaxonomyHandler struct {
	repo repository.StorageEngine
}

func NewTaxonomyHandler(repo repository.StorageEngine) *TaxonomyHandler {
	return &TaxonomyHandler{repo: repo}
}

func (h *TaxonomyHandler) ListAuthors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authors, err := h.repo.ListAuthors(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list authors: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authors": authors,
	})
}

func (h *TaxonomyHandler) ListGenres(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	genres, err := h.repo.ListGenres(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list genres: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"genres": genres,
	})
}

func (h *TaxonomyHandler) ListSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	seriesList, err := h.repo.ListSeries(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list series: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"series": seriesList,
	})
}
