package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shelfd/shelfd/internal/repository"
)

type TaxonomyHandler struct {
	repo       repository.StorageEngine
	libraryDir string
	dataDir    string
	logger     *slog.Logger
}

func NewTaxonomyHandler(repo repository.StorageEngine, libraryDir, dataDir string, logger *slog.Logger) *TaxonomyHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &TaxonomyHandler{
		repo:       repo,
		libraryDir: libraryDir,
		dataDir:    dataDir,
		logger:     logger,
	}
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

	// Populate relative photo_url for authors
	for _, a := range authors {
		if a.PhotoURL == nil || *a.PhotoURL == "" {
			photoURL := fmt.Sprintf("/api/v1/authors/%s/photo", a.ID)
			a.PhotoURL = &photoURL
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authors": authors,
	})
}

func (h *TaxonomyHandler) GetAuthorPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authorID := r.PathValue("id")
	if authorID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			authorID = parts[len(parts)-2]
		}
	}

	author, err := h.repo.GetAuthorByID(r.Context(), authorID)
	if err != nil || author == nil {
		writeJSONError(w, http.StatusNotFound, "Author not found")
		return
	}

	authorDir := h.resolveAuthorDir(r, authorID, author.Name)

	// 1. Check local directory for author photo
	if photoPath := findLocalAuthorPhoto(authorDir); photoPath != "" {
		serveImageFile(w, r, photoPath)
		return
	}

	// 2. Fetch from OpenLibrary and persist into the author's directory
	if err := os.MkdirAll(authorDir, 0755); err == nil {
		targetPhoto := filepath.Join(authorDir, "author.jpg")
		if fetchAndSaveAuthorPhoto(author.Name, targetPhoto) {
			serveImageFile(w, r, targetPhoto)
			return
		}
	}

	writeJSONError(w, http.StatusNotFound, "Author photo not found")
}

func (h *TaxonomyHandler) UploadAuthorPhoto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authorID := r.PathValue("id")
	if authorID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing author ID")
		return
	}

	author, err := h.repo.GetAuthorByID(r.Context(), authorID)
	if err != nil || author == nil {
		writeJSONError(w, http.StatusNotFound, "Author not found")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid form: max 10MB photo")
		return
	}

	file, _, err := r.FormFile("photo")
	if err != nil {
		file, _, err = r.FormFile("file")
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "No photo file provided")
		return
	}
	defer file.Close()

	authorDir := h.resolveAuthorDir(r, authorID, author.Name)
	if err := os.MkdirAll(authorDir, 0755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create author directory")
		return
	}

	targetPhoto := filepath.Join(authorDir, "author.jpg")
	outFile, err := os.Create(targetPhoto)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to save author photo")
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to write author photo")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "Author photo saved",
		"photo_url": fmt.Sprintf("/api/v1/authors/%s/photo", authorID),
	})
}

func (h *TaxonomyHandler) resolveAuthorDir(r *http.Request, authorID, authorName string) string {
	books, _ := h.repo.ListBooks(r.Context(), repository.BookFilter{AuthorID: &authorID, Limit: 1})
	if len(books) > 0 && books[0].FilePath != "" {
		// Book path is e.g. /library/Author/Title/Title.epub or relative Author/Title/Title.epub
		bookDir := filepath.Dir(books[0].FilePath)
		parentDir := filepath.Dir(bookDir)
		if parentDir != "." && parentDir != "/" {
			if !filepath.IsAbs(parentDir) && h.libraryDir != "" {
				return filepath.Join(h.libraryDir, parentDir)
			}
			return parentDir
		}
	}
	if h.libraryDir != "" {
		return filepath.Join(h.libraryDir, authorName)
	}
	return filepath.Join(h.dataDir, "authors", authorID)
}

func findLocalAuthorPhoto(dir string) string {
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		for _, name := range []string{"author", "folder"} {
			candidate := filepath.Join(dir, name+ext)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func fetchAndSaveAuthorPhoto(authorName string, destPath string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	searchURL := "https://openlibrary.org/search/authors.json?q=" + url.QueryEscape(authorName)
	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Shelfd/0.1.0 (digital-library)")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	defer resp.Body.Close()

	var searchResult struct {
		Docs []struct {
			Key    string `json:"key"`
			Photos []int  `json:"photos"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil || len(searchResult.Docs) == 0 {
		return false
	}

	var photoURL string
	doc := searchResult.Docs[0]
	if len(doc.Photos) > 0 && doc.Photos[0] > 0 {
		photoURL = fmt.Sprintf("https://covers.openlibrary.org/a/id/%d-L.jpg", doc.Photos[0])
	} else if doc.Key != "" {
		photoURL = fmt.Sprintf("https://covers.openlibrary.org/a/olid/%s-L.jpg", doc.Key)
	}
	if photoURL == "" {
		return false
	}

	imgResp, err := client.Get(photoURL)
	if err != nil || imgResp.StatusCode != http.StatusOK {
		if imgResp != nil {
			imgResp.Body.Close()
		}
		return false
	}
	defer imgResp.Body.Close()

	contentType := imgResp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		return false
	}

	data, err := io.ReadAll(io.LimitReader(imgResp.Body, 10*1024*1024))
	if err != nil || len(data) < 1000 {
		return false
	}

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return false
	}
	return true
}

func serveImageFile(w http.ResponseWriter, r *http.Request, filePath string) {
	contentType := "image/jpeg"
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filePath)
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

func (h *TaxonomyHandler) ListTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	topics, err := h.repo.ListTopics(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list topics: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"topics": topics,
	})
}

