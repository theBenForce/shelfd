package api

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/ai"
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
}

func NewBookHandler(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	worker *worker.Worker,
	uploadWorker *worker.UploadWorker,
	aiClient ai.Client,
	dataDir string,
	libraryDir string,
) *BookHandler {
	return &BookHandler{
		repo:         repo,
		ingester:     ingester,
		worker:       worker,
		uploadWorker: uploadWorker,
		aiClient:     aiClient,
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
	FileModifiedAt *time.Time                     `json:"file_modified_at,omitempty"`
	PublishedDate  *string                        `json:"published_date,omitempty"`
	Authors        []*repository.Author           `json:"authors"`
	Genres         []*repository.Genre            `json:"genres"`
	Series         []*repository.BookSeriesDetail `json:"series"`
}

type BookDetailResponse struct {
	BookListItem
	Spine      []*repository.SpineItem   `json:"spine"`
	Chapters   []*repository.Chapter     `json:"chapters"`
	Bookmarks  []*repository.Bookmark    `json:"bookmarks"`
	Highlights []*repository.Highlight   `json:"highlights"`
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
			FilePath:       b.FilePath,
			CoverPath:      b.CoverPath,
			FileSizeBytes:  b.FileSizeBytes,
			FileModifiedAt: b.FileModifiedAt,
			PublishedDate:  b.PublishedDate,
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
	bookmarks, _ := h.repo.ListBookmarksByBookID(r.Context(), book.ID)
	highlights, _ := h.repo.ListHighlightsByBookID(r.Context(), book.ID)

	writeJSON(w, http.StatusOK, BookDetailResponse{
		BookListItem: BookListItem{
			ID:            book.ID,
			Title:         book.Title,
			Description:   book.Description,
			Language:      book.Language,
			Publisher:     book.Publisher,
			Identifier:    book.Identifier,
			FilePath:       book.FilePath,
			CoverPath:      book.CoverPath,
			FileSizeBytes:  book.FileSizeBytes,
			FileModifiedAt: book.FileModifiedAt,
			PublishedDate:  book.PublishedDate,
			Authors:       authors,
			Genres:        genres,
			Series:        seriesList,
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


