package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/audio"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/events"
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
	toolInvoker  ai.ToolInvoker
	dataDir      string
	libraryDir   string
	hub          *events.Hub
	logger       *slog.Logger
}

// SetToolInvoker sets the in-process tool invoker for agentic book chat.
func (h *BookHandler) SetToolInvoker(invoker ai.ToolInvoker) {
	h.toolInvoker = invoker
}

func NewBookHandler(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	worker *worker.Worker,
	uploadWorker *worker.UploadWorker,
	aiClient ai.Client,
	dataDir string,
	libraryDir string,
	hub *events.Hub,
	logger *slog.Logger,
) *BookHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BookHandler{
		repo:         repo,
		ingester:     ingester,
		worker:       worker,
		uploadWorker: uploadWorker,
		aiClient:     aiClient,
		dataDir:      dataDir,
		libraryDir:   libraryDir,
		hub:          hub,
		logger:       logger,
	}
}

type BookListItem struct {
	ID                       string                         `json:"id"`
	Title                    string                         `json:"title"`
	Description              *string                        `json:"description,omitempty"`
	Language                 *string                        `json:"language,omitempty"`
	Publisher                *string                        `json:"publisher,omitempty"`
	Identifier               *string                        `json:"identifier,omitempty"`
	FilePath                 string                         `json:"file_path"`
	CoverPath                *string                        `json:"cover_path,omitempty"`
	FileSizeBytes            *int64                         `json:"file_size_bytes,omitempty"`
	FileModifiedAt           *time.Time                     `json:"file_modified_at,omitempty"`
	PublishedDate            *string                        `json:"published_date,omitempty"`
	Layout                   string                         `json:"layout"`
	RenditionSpread          string                         `json:"rendition_spread"`
	RenditionOrientation     string                         `json:"rendition_orientation"`
	PageProgressionDirection string                         `json:"page_progression_direction"`
	BookType                 string                         `json:"book_type,omitempty"`
	DurationSeconds          *float64                       `json:"duration_seconds,omitempty"`
	HasReadalong             bool                           `json:"has_readalong"`
	Authors                  []*repository.Author           `json:"authors"`
	Genres                   []*repository.Genre            `json:"genres"`
	Topics                   []*repository.Topic            `json:"topics"`
	Series                   []*repository.BookSeriesDetail `json:"series"`
}

type BookDetailResponse struct {
	BookListItem
	Spine         []*repository.SpineItem    `json:"spine"`
	Chapters      []*repository.Chapter      `json:"chapters"`
	Bookmarks     []*repository.Bookmark     `json:"bookmarks"`
	Highlights    []*repository.Highlight    `json:"highlights"`
	Files         []*repository.BookFile     `json:"files"`
	AudioChapters []*repository.AudioChapter `json:"audio_chapters,omitempty"`
}

// resolveReadalongStatus determines whether a book has read-along audio and resolves its duration.
func resolveReadalongStatus(ctx context.Context, repo repository.StorageEngine, b *repository.Book, files []*repository.BookFile) (bool, *float64) {
	if b == nil {
		return false, nil
	}
	dur := b.DurationSeconds
	if b.BookType == "audiobook" {
		return false, dur
	}
	if files == nil && repo != nil {
		files, _ = repo.GetBookFilesByBookID(ctx, b.ID)
	}
	hasAudioFile := false
	var filesDuration float64
	for _, f := range files {
		if f != nil && (f.FileType == "audiobook" || f.FileType == "audio") {
			hasAudioFile = true
			if f.DurationSeconds != nil && *f.DurationSeconds > 0 {
				filesDuration += *f.DurationSeconds
			}
		}
	}
	if (dur == nil || *dur <= 0) && filesDuration > 0 {
		dur = &filesDuration
	}
	hasReadalong := hasAudioFile || (dur != nil && *dur > 0)
	return hasReadalong, dur
}

// BuildBookListItem loads relational metadata for a book and constructs a BookListItem DTO.
func BuildBookListItem(ctx context.Context, repo repository.StorageEngine, b *repository.Book) BookListItem {
	authors, _ := repo.GetBookAuthors(ctx, b.ID)
	genres, _ := repo.GetBookGenres(ctx, b.ID)
	topics, _ := repo.GetBookTopics(ctx, b.ID)
	seriesList, _ := repo.GetBookSeries(ctx, b.ID)
	hasReadalong, durationSeconds := resolveReadalongStatus(ctx, repo, b, nil)

	return BookListItem{
		ID:                       b.ID,
		Title:                    b.Title,
		Description:              b.Description,
		Language:                 b.Language,
		Publisher:                b.Publisher,
		Identifier:               b.Identifier,
		FilePath:                 b.FilePath,
		CoverPath:                b.CoverPath,
		FileSizeBytes:            b.FileSizeBytes,
		FileModifiedAt:           b.FileModifiedAt,
		PublishedDate:            b.PublishedDate,
		Layout:                   b.Layout,
		RenditionSpread:          b.RenditionSpread,
		RenditionOrientation:     b.RenditionOrientation,
		PageProgressionDirection: b.PageProgressionDirection,
		BookType:                 b.BookType,
		DurationSeconds:          durationSeconds,
		HasReadalong:             hasReadalong,
		Authors:                  authors,
		Genres:                   genres,
		Topics:                   topics,
		Series:                   seriesList,
	}
}

type BookCitation struct {
	BookID         string  `json:"book_id,omitempty"`
	BookTitle      string  `json:"book_title,omitempty"`
	AuthorName     string  `json:"author_name,omitempty"`
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

var tokenRe = regexp.MustCompile(`(?i)\b(author|genre|topic|isbn|title):\s*(?:"([^"]*)"|(\S+))`)
var spaceRe = regexp.MustCompile(`\s+`)

func parseBookSearchTokens(input string) (cleanSearch string, author *string, genre *string, topic *string, isbn *string, title *string) {
	matches := tokenRe.FindAllStringSubmatch(input, -1)
	for _, m := range matches {
		tag := strings.ToLower(m[1])
		val := m[2]
		if val == "" {
			val = m[3]
		}
		val = strings.TrimSpace(val)
		if val != "" {
			switch tag {
			case "author":
				author = &val
			case "genre":
				genre = &val
			case "topic":
				topic = &val
			case "isbn":
				isbn = &val
			case "title":
				title = &val
			}
		}
	}
	clean := tokenRe.ReplaceAllString(input, " ")
	clean = strings.TrimSpace(spaceRe.ReplaceAllString(clean, " "))
	return clean, author, genre, topic, isbn, title
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
	if author := strings.TrimSpace(q.Get("author")); author != "" {
		filter.AuthorName = &author
	} else if authorName := strings.TrimSpace(q.Get("author_name")); authorName != "" {
		filter.AuthorName = &authorName
	}
	if genreID := strings.TrimSpace(q.Get("genre_id")); genreID != "" {
		filter.GenreID = &genreID
	}
	if genre := strings.TrimSpace(q.Get("genre")); genre != "" {
		filter.GenreName = &genre
	} else if genreName := strings.TrimSpace(q.Get("genre_name")); genreName != "" {
		filter.GenreName = &genreName
	}
	if topicID := strings.TrimSpace(q.Get("topic_id")); topicID != "" {
		filter.TopicID = &topicID
	}
	if topic := strings.TrimSpace(q.Get("topic")); topic != "" {
		filter.TopicName = &topic
	} else if topicName := strings.TrimSpace(q.Get("topic_name")); topicName != "" {
		filter.TopicName = &topicName
	}
	if title := strings.TrimSpace(q.Get("title")); title != "" {
		filter.Title = &title
	}
	if isbn := strings.TrimSpace(q.Get("isbn")); isbn != "" {
		filter.ISBN = &isbn
	}
	if seriesID := strings.TrimSpace(q.Get("series_id")); seriesID != "" {
		filter.SeriesID = &seriesID
	}
	if rawSearch := strings.TrimSpace(q.Get("search")); rawSearch != "" {
		cleanSearch, parsedAuthor, parsedGenre, parsedTopic, parsedISBN, parsedTitle := parseBookSearchTokens(rawSearch)
		if cleanSearch != "" {
			filter.Search = &cleanSearch
		}
		if filter.AuthorName == nil && parsedAuthor != nil {
			filter.AuthorName = parsedAuthor
		}
		if filter.GenreName == nil && parsedGenre != nil {
			filter.GenreName = parsedGenre
		}
		if filter.TopicName == nil && parsedTopic != nil {
			filter.TopicName = parsedTopic
		}
		if filter.ISBN == nil && parsedISBN != nil {
			filter.ISBN = parsedISBN
		}
		if filter.Title == nil && parsedTitle != nil {
			filter.Title = parsedTitle
		}
	}
	if sortBy := strings.TrimSpace(q.Get("sort_by")); sortBy != "" {
		filter.SortBy = sortBy
	}
	if sortOrder := strings.TrimSpace(q.Get("sort_order")); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	limit := 50
	if rawLimit := q.Get("limit"); rawLimit != "" {
		if val, err := strconv.Atoi(rawLimit); err == nil && val > 0 {
			limit = val
		}
	} else if rawPerPage := q.Get("per_page"); rawPerPage != "" {
		if val, err := strconv.Atoi(rawPerPage); err == nil && val > 0 {
			limit = val
		}
	}
	if limit > 1000 {
		limit = 1000
	}
	filter.Limit = limit

	offset := 0
	if rawOffset := q.Get("offset"); rawOffset != "" {
		if val, err := strconv.Atoi(rawOffset); err == nil && val >= 0 {
			offset = val
		}
	} else if rawPage := q.Get("page"); rawPage != "" {
		if val, err := strconv.Atoi(rawPage); err == nil && val > 0 {
			offset = (val - 1) * limit
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

	if len(books) == 0 && filter.Offset == 0 && filter.Title != nil && strings.TrimSpace(*filter.Title) != "" {
		normTitle := repository.NormalizeReadalongMatchText(*filter.Title)
		var normAuthor string
		if filter.AuthorName != nil {
			normAuthor = repository.NormalizeReadalongMatchText(*filter.AuthorName)
		}
		if normTitle != "" {
			if allBooks, err := h.repo.ListBooks(r.Context(), repository.BookFilter{Limit: 1000}); err == nil {
				for _, cand := range allBooks {
					if cand == nil || repository.NormalizeReadalongMatchText(cand.Title) != normTitle {
						continue
					}
					if normAuthor != "" && normAuthor != "unknown" {
						candAuthors, _ := h.repo.GetBookAuthors(r.Context(), cand.ID)
						authorMatched := false
						for _, ca := range candAuthors {
							if ca == nil {
								continue
							}
							na := repository.NormalizeReadalongMatchText(ca.Name)
							if na == normAuthor || strings.Contains(na, normAuthor) || strings.Contains(normAuthor, na) {
								authorMatched = true
								break
							}
						}
						if !authorMatched {
							continue
						}
					}
					books = append(books, cand)
				}
				total = len(books)
			}
		}
	}

	var items []BookListItem
	for _, b := range books {
		items = append(items, BuildBookListItem(r.Context(), h.repo, b))
	}

	h.logger.Debug("Listed books", "count", len(items), "total", total, "limit", limit, "offset", offset)

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
	topics, _ := h.repo.GetBookTopics(r.Context(), book.ID)
	seriesList, _ := h.repo.GetBookSeries(r.Context(), book.ID)
	spine, _ := h.repo.GetBookSpine(r.Context(), book.ID)
	chapters, _ := h.repo.GetChaptersByBookID(r.Context(), book.ID)
	// Clear ContentPlain on chapters in BookDetailResponse to keep payload lightweight
	for _, c := range chapters {
		c.ContentPlain = ""
	}
	bookmarks, _ := h.repo.ListBookmarksByBookID(r.Context(), book.ID)
	highlights, _ := h.repo.ListHighlightsByBookID(r.Context(), book.ID)
	files, _ := h.repo.GetBookFilesByBookID(r.Context(), book.ID)
	audioChapters, _ := h.repo.GetAudioChaptersByBookID(r.Context(), book.ID)
	hasReadalong, durationSeconds := resolveReadalongStatus(r.Context(), h.repo, book, files)

	writeJSON(w, http.StatusOK, BookDetailResponse{
		BookListItem: BookListItem{
			ID:                       book.ID,
			Title:                    book.Title,
			Description:              book.Description,
			Language:                 book.Language,
			Publisher:                book.Publisher,
			Identifier:               book.Identifier,
			FilePath:                 book.FilePath,
			CoverPath:                book.CoverPath,
			FileSizeBytes:            book.FileSizeBytes,
			FileModifiedAt:           book.FileModifiedAt,
			PublishedDate:            book.PublishedDate,
			Layout:                   book.Layout,
			RenditionSpread:          book.RenditionSpread,
			RenditionOrientation:     book.RenditionOrientation,
			PageProgressionDirection: book.PageProgressionDirection,
			BookType:                 book.BookType,
			DurationSeconds:          durationSeconds,
			HasReadalong:             hasReadalong,
			Authors:                  authors,
			Genres:                   genres,
			Topics:                   topics,
			Series:                   seriesList,
		},
		Spine:         spine,
		Chapters:      chapters,
		Bookmarks:     bookmarks,
		Highlights:    highlights,
		Files:         files,
		AudioChapters: audioChapters,
	})
}

func (h *BookHandler) GetBookCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
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

	f, err := os.Open(coverFile)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Cover image not found")
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to stat cover image")
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
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
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

func (h *BookHandler) resolveBookFilePath(b *repository.Book) string {
	if b == nil {
		return ""
	}
	return resolveStoredPath(h.libraryDir, h.dataDir, b.FilePath)
}

func sanitizeDownloadFilename(name string) string {
	name = strings.TrimSpace(name)
	invalidChars := regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
	cleaned := invalidChars.ReplaceAllString(name, "_")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.Trim(cleaned, "._")
	return cleaned
}

func (h *BookHandler) DownloadBook(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	filePath := h.resolveBookFilePath(book)
	if filePath == "" {
		writeJSONError(w, http.StatusNotFound, "Book file not found on disk")
		return
	}

	cleanTitle := sanitizeDownloadFilename(book.Title)
	if cleanTitle == "" {
		cleanTitle = sanitizeDownloadFilename(book.ID)
		if cleanTitle == "" {
			cleanTitle = "book"
		}
	}
	filename := fmt.Sprintf("%s.epub", cleanTitle)

	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-transform")

	http.ServeFile(w, r, filePath)
}

func bookMarkdownFilename(book *repository.Book) string {
	cleanTitle := ""
	if book != nil {
		cleanTitle = sanitizeDownloadFilename(book.Title)
		if cleanTitle == "" {
			cleanTitle = sanitizeDownloadFilename(book.ID)
		}
	}
	if cleanTitle == "" {
		cleanTitle = "book"
	}
	return fmt.Sprintf("%s.md", cleanTitle)
}

func (h *BookHandler) compileBookMarkdown(ctx context.Context, book *repository.Book) string {
	var sb strings.Builder

	title := strings.TrimSpace(book.Title)
	if title == "" {
		title = "Untitled Book"
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	authors, _ := h.repo.GetBookAuthors(ctx, book.ID)
	genres, _ := h.repo.GetBookGenres(ctx, book.ID)
	topics, _ := h.repo.GetBookTopics(ctx, book.ID)
	seriesList, _ := h.repo.GetBookSeries(ctx, book.ID)
	highlights, _ := h.repo.ListHighlightsByBookID(ctx, book.ID)
	chapters, _ := h.repo.GetChaptersByBookID(ctx, book.ID)

	sb.WriteString("## Book Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **Title**: %s\n", title))
	if len(authors) > 0 {
		names := make([]string, 0, len(authors))
		for _, a := range authors {
			if strings.TrimSpace(a.Name) != "" {
				names = append(names, strings.TrimSpace(a.Name))
			}
		}
		if len(names) > 0 {
			sb.WriteString(fmt.Sprintf("- **Author(s)**: %s\n", strings.Join(names, ", ")))
		}
	}
	if len(seriesList) > 0 {
		parts := make([]string, 0, len(seriesList))
		for _, s := range seriesList {
			if strings.TrimSpace(s.Name) == "" {
				continue
			}
			if s.SequenceNumber != nil {
				parts = append(parts, fmt.Sprintf("%s (#%g)", strings.TrimSpace(s.Name), *s.SequenceNumber))
			} else {
				parts = append(parts, strings.TrimSpace(s.Name))
			}
		}
		if len(parts) > 0 {
			sb.WriteString(fmt.Sprintf("- **Series**: %s\n", strings.Join(parts, ", ")))
		}
	}
	if book.Publisher != nil && strings.TrimSpace(*book.Publisher) != "" {
		sb.WriteString(fmt.Sprintf("- **Publisher**: %s\n", strings.TrimSpace(*book.Publisher)))
	}
	if book.PublishedDate != nil && strings.TrimSpace(*book.PublishedDate) != "" {
		sb.WriteString(fmt.Sprintf("- **Published**: %s\n", strings.TrimSpace(*book.PublishedDate)))
	}
	if book.Language != nil && strings.TrimSpace(*book.Language) != "" {
		sb.WriteString(fmt.Sprintf("- **Language**: %s\n", strings.TrimSpace(*book.Language)))
	}
	if book.Identifier != nil && strings.TrimSpace(*book.Identifier) != "" {
		sb.WriteString(fmt.Sprintf("- **Identifier**: %s\n", strings.TrimSpace(*book.Identifier)))
	}
	if len(genres) > 0 {
		names := make([]string, 0, len(genres))
		for _, g := range genres {
			if strings.TrimSpace(g.Name) != "" {
				names = append(names, strings.TrimSpace(g.Name))
			}
		}
		if len(names) > 0 {
			sb.WriteString(fmt.Sprintf("- **Genres**: %s\n", strings.Join(names, ", ")))
		}
	}
	if len(topics) > 0 {
		names := make([]string, 0, len(topics))
		for _, t := range topics {
			if strings.TrimSpace(t.Name) != "" {
				names = append(names, strings.TrimSpace(t.Name))
			}
		}
		if len(names) > 0 {
			sb.WriteString(fmt.Sprintf("- **Topics**: %s\n", strings.Join(names, ", ")))
		}
	}

	if book.Description != nil && strings.TrimSpace(*book.Description) != "" {
		sb.WriteString("\n## Synopsis\n\n")
		sb.WriteString(strings.TrimSpace(*book.Description))
		sb.WriteString("\n")
	}

	if len(highlights) > 0 {
		sb.WriteString("\n## Highlights & Notes\n\n")
		for _, hl := range highlights {
			text := strings.TrimSpace(hl.SelectedText)
			if text == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("> %s\n", strings.ReplaceAll(text, "\n", "\n> ")))
			if hl.Note != nil && strings.TrimSpace(*hl.Note) != "" {
				sb.WriteString(fmt.Sprintf("\n- **Note**: %s\n", strings.TrimSpace(*hl.Note)))
			}
			sb.WriteString("\n")
		}
	}

	// Check if DB chapters have non-empty content; otherwise fall back to live EPUB extraction
	hasChapterContent := false
	for _, ch := range chapters {
		if strings.TrimSpace(ch.ContentPlain) != "" {
			hasChapterContent = true
			break
		}
	}

	if !hasChapterContent {
		if epubPath := h.resolveBookFilePath(book); epubPath != "" {
			if reader, err := epub.Open(epubPath); err == nil {
				if parsedChapters, pErr := reader.ExtractChapters(); pErr == nil && len(parsedChapters) > 0 {
					chapters = make([]*repository.Chapter, 0, len(parsedChapters))
					for _, pc := range parsedChapters {
						chapters = append(chapters, &repository.Chapter{
							BookID:       book.ID,
							ChapterIndex: pc.Index,
							Title:        pc.Title,
							ContentPlain: pc.ContentPlain,
						})
					}
				}
				reader.Close()
			}
		}
	}

	if len(chapters) > 0 {
		sb.WriteString("\n## Chapters\n")
		for _, ch := range chapters {
			content := strings.TrimSpace(ch.ContentPlain)
			if content == "" && ch.Summary == "" {
				continue
			}
			chHeading := fmt.Sprintf("Chapter %d", ch.ChapterIndex)
			if ch.Title != nil && strings.TrimSpace(*ch.Title) != "" {
				chHeading = fmt.Sprintf("Chapter %d: %s", ch.ChapterIndex, strings.TrimSpace(*ch.Title))
			}
			sb.WriteString(fmt.Sprintf("\n### %s\n\n", chHeading))
			if strings.TrimSpace(ch.Summary) != "" {
				sb.WriteString(fmt.Sprintf("*Summary: %s*\n\n", strings.TrimSpace(ch.Summary)))
			}
			if content != "" {
				sb.WriteString(content)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// ExportBookMarkdown handles GET /api/v1/books/{id}/markdown and serves a compiled
// Markdown document containing metadata, highlights, and chapter content.
func (h *BookHandler) ExportBookMarkdown(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	markdown := h.compileBookMarkdown(r.Context(), book)
	filename := bookMarkdownFilename(book)

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-transform")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(markdown))
}

func (h *BookHandler) GetBookAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	assetPath := r.PathValue("path")
	if bookID == "" || assetPath == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "books" && i+1 < len(parts) {
				bookID = parts[i+1]
			}
			if p == "assets" && i+1 < len(parts) {
				assetPath = strings.Join(parts[i+1:], "/")
				break
			}
		}
	}

	if bookID == "" || assetPath == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID and asset path required")
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

	epubPath := h.resolveBookFilePath(book)
	if epubPath == "" {
		writeJSONError(w, http.StatusNotFound, "Book file not found on disk")
		return
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open epub: %v", err))
		return
	}
	defer reader.Close()

	var (
		assetData []byte
		mimeType  string
	)
	if isAudioAssetPath(assetPath) {
		assetData, mimeType, err = reader.ExtractMediaAsset(assetPath)
	} else {
		assetData, mimeType, err = reader.ExtractAsset(assetPath)
	}
	if err != nil {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("Asset not found: %v", err))
		return
	}

	modTime := time.Time{}
	if fi, sErr := os.Stat(epubPath); sErr == nil {
		modTime = fi.ModTime()
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, filepath.Base(assetPath), modTime, bytes.NewReader(assetData))
}

func isAudioAssetPath(assetPath string) bool {
	switch strings.ToLower(filepath.Ext(assetPath)) {
	case ".mp3", ".m4a", ".m4b", ".mp4", ".ogg", ".oga", ".opus", ".flac", ".wav", ".aac":
		return true
	default:
		return false
	}
}

func (h *BookHandler) GetChapterHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bookID := r.PathValue("id")
	identifier := r.PathValue("index")
	if bookID == "" || identifier == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "books" && i+1 < len(parts) {
				bookID = parts[i+1]
			}
			if p == "chapters" && i+1 < len(parts) {
				identifier = parts[i+1]
			}
		}
	}

	if bookID == "" || identifier == "" {
		writeJSONError(w, http.StatusBadRequest, "Book ID and chapter index required")
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

	var chapter *repository.Chapter
	if index, parseErr := strconv.Atoi(identifier); parseErr == nil {
		if index == 0 {
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

	epubPath := h.resolveBookFilePath(book)
	if epubPath == "" {
		writeJSONError(w, http.StatusNotFound, "Book file not found on disk")
		return
	}

	reader, err := epub.Open(epubPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open epub: %v", err))
		return
	}
	defer reader.Close()

	var rawHTML []byte
	var docHref string
	if chapter.Href != nil && *chapter.Href != "" {
		docHref = *chapter.Href
		rawHTML, err = reader.ExtractRawDocument(docHref)
	}

	if len(rawHTML) == 0 {
		parsedChapters, pErr := reader.ExtractChapters()
		if pErr == nil {
			for _, pc := range parsedChapters {
				if pc.Index == chapter.ChapterIndex {
					docHref = pc.Href
					rawHTML, err = reader.ExtractRawDocument(docHref)
					break
				}
			}
		}
	}

	if err != nil || len(rawHTML) == 0 {
		writeJSONError(w, http.StatusNotFound, "Chapter document not found in epub")
		return
	}

	rewrittenHTML := epub.RewritePageHTML(string(rawHTML), bookID, docHref, chapter.PageWidth, chapter.PageHeight)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.Header().Set("Content-Length", strconv.Itoa(len(rewrittenHTML)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(rewrittenHTML))
	}
}

const (
	maxUploadCoverBytes = 10 << 20 // 10 MB max cover image in multipart upload
	maxUploadFieldBytes = 1 << 20  // 1 MB max text form field (e.g. metadata_defaults)
)

type parsedBookUpload struct {
	jobID            string
	uploadsDir       string
	stagedPath       string
	filename         string
	ext              string
	fileType         string
	sizeBytes        int64
	coverData        []byte
	sourcePlugin     string
	sourceExternalID string
	rawMetaDefaults  string
}

func writeUploadedCoverIfValid(uploadsDir, jobID string, coverData []byte) bool {
	if len(coverData) == 0 {
		return false
	}
	ct := http.DetectContentType(coverData)
	if !strings.HasPrefix(ct, "image/") {
		return false
	}
	coverPath := filepath.Join(uploadsDir, jobID+".cover")
	return os.WriteFile(coverPath, coverData, 0644) == nil
}

// parseStreamingBookUpload streams a multipart book upload directly to <dataDir>/uploads/<jobID><ext>
// in O(1) memory regardless of part ordering, rejecting unsupported extensions before streaming the body
// and capping non-book parts with io.LimitReader.
func parseStreamingBookUpload(w http.ResponseWriter, r *http.Request, dataDir string) (*parsedBookUpload, bool) {
	// Clear any connection-level read deadline so slow multi-hundred-MB audiobook uploads
	// are not cut off mid-stream if an http.Server ReadTimeout is configured.
	_ = http.NewResponseController(w).SetReadDeadline(time.Time{})

	mr, err := r.MultipartReader()
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
		return nil, false
	}

	jobID := uuid.NewString()
	uploadsDir := filepath.Join(dataDir, "uploads")
	res := &parsedBookUpload{
		jobID:      jobID,
		uploadsDir: uploadsDir,
	}
	seenFile := false

	cleanup := func() {
		if res.stagedPath != "" {
			_ = os.Remove(res.stagedPath)
		}
	}

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cleanup()
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse multipart form: %v", err))
			return nil, false
		}

		formName := part.FormName()
		switch formName {
		case "file":
			if seenFile {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, "Duplicate 'file' field in multipart upload")
				return nil, false
			}
			filename := part.FileName()
			if strings.TrimSpace(filename) == "" {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in multipart upload")
				return nil, false
			}

			ext := strings.ToLower(filepath.Ext(filename))
			fileType, isPrimary := scanner.DetectFileType(filename)
			if !isPrimary || (fileType != "epub" && fileType != "audiobook") {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, "Only .epub, .m4b, .mp3, .m4a, and .flac files are accepted")
				return nil, false
			}

			if err := os.MkdirAll(uploadsDir, 0755); err != nil {
				cleanup()
				writeJSONError(w, http.StatusInternalServerError, "Failed to create uploads directory")
				return nil, false
			}

			stagedPath := filepath.Join(uploadsDir, jobID+ext)
			stagedFile, err := os.OpenFile(stagedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				cleanup()
				writeJSONError(w, http.StatusInternalServerError, "Failed to create staged upload file")
				return nil, false
			}
			res.stagedPath = stagedPath

			writtenBytes, copyErr := io.Copy(stagedFile, part)
			closeErr := stagedFile.Close()
			if copyErr != nil || closeErr != nil {
				cleanup()
				if copyErr != nil && (errors.Is(copyErr, io.ErrUnexpectedEOF) || strings.Contains(copyErr.Error(), "multipart:")) {
					writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read uploaded file stream: %v", copyErr))
					return nil, false
				}
				writeJSONError(w, http.StatusInternalServerError, "Failed to write staged upload file")
				return nil, false
			}

			res.filename = filename
			res.ext = ext
			res.fileType = fileType
			res.sizeBytes = writtenBytes
			seenFile = true

		case "cover":
			data, readErr := io.ReadAll(io.LimitReader(part, maxUploadCoverBytes+1))
			if readErr != nil {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read cover image: %v", readErr))
				return nil, false
			}
			if len(data) > maxUploadCoverBytes {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, "Cover image exceeds maximum size of 10 MB")
				return nil, false
			}
			if len(data) > 0 {
				res.coverData = data
			}

		case "source_plugin", "source_external_id", "metadata_defaults":
			data, readErr := io.ReadAll(io.LimitReader(part, maxUploadFieldBytes+1))
			if readErr != nil {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read form field %q: %v", formName, readErr))
				return nil, false
			}
			if len(data) > maxUploadFieldBytes {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Form field %q exceeds maximum size of 1 MB", formName))
				return nil, false
			}
			val := strings.TrimSpace(string(data))
			switch formName {
			case "source_plugin":
				res.sourcePlugin = val
			case "source_external_id":
				res.sourceExternalID = val
			case "metadata_defaults":
				res.rawMetaDefaults = val
			}

		default:
			n, readErr := io.Copy(io.Discard, io.LimitReader(part, maxUploadFieldBytes+1))
			if readErr != nil {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to read multipart field: %v", readErr))
				return nil, false
			}
			if n > maxUploadFieldBytes {
				cleanup()
				writeJSONError(w, http.StatusBadRequest, "Multipart form field exceeds maximum size of 1 MB")
				return nil, false
			}
		}
	}

	if !seenFile {
		cleanup()
		writeJSONError(w, http.StatusBadRequest, "Missing 'file' field in multipart upload")
		return nil, false
	}

	return res, true
}

func (h *BookHandler) UploadBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if r.URL.Query().Get("stage") == "true" {
		h.StageUploadBook(w, r)
		return
	}

	// 1. Stream upload directly to dataDir/uploads/<job_id><ext>
	up, ok := parseStreamingBookUpload(w, r, h.dataDir)
	if !ok {
		return
	}

	// 2. Validate zip archive header if EPUB
	if up.fileType == "epub" {
		zipReader, err := zip.OpenReader(up.stagedPath)
		if err != nil {
			_ = os.Remove(up.stagedPath)
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid EPUB archive: %v", err))
			return
		}
		zipReader.Close()
	}

	// Save optional cover provided in multipart form
	hasCover := writeUploadedCoverIfValid(up.uploadsDir, up.jobID, up.coverData)

	// 3. Create upload job in database
	job := &repository.UploadJob{
		ID:         up.jobID,
		Filename:   up.filename,
		StagedPath: up.stagedPath,
		Status:     "queued",
		HasCover:   hasCover,
	}
	if err := h.repo.CreateUploadJob(r.Context(), job); err != nil {
		_ = os.Remove(up.stagedPath)
		_ = os.Remove(filepath.Join(up.uploadsDir, up.jobID+".cover"))
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to enqueue upload job: %v", err))
		return
	}

	// 4. Trigger upload background worker
	if h.uploadWorker != nil {
		h.uploadWorker.Trigger()
	}
	h.broadcastQueueStatus(r.Context())

	h.logger.Info("Enqueued book upload job", "job_id", job.ID, "filename", job.Filename, "size_bytes", up.sizeBytes)

	// 5. Return 202 Accepted
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     job.ID,
		"status":     job.Status,
		"filename":   job.Filename,
		"has_cover":  hasCover,
		"message":    "Upload enqueued for processing",
		"created_at": job.CreatedAt,
	})
}

// StagedMetadataDTO represents parsed EPUB or Audiobook metadata for user inspection before saving.
type StagedMetadataDTO struct {
	Title           string   `json:"title"`
	Authors         []string `json:"authors"`
	Series          *string  `json:"series,omitempty"`
	SequenceNumber  *float64 `json:"sequence_number,omitempty"`
	Description     *string  `json:"description,omitempty"`
	Publisher       *string  `json:"publisher,omitempty"`
	Language        *string  `json:"language,omitempty"`
	Genres          []string `json:"genres,omitempty"`
	IsDuplicate     bool     `json:"is_duplicate,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	BookType        string   `json:"book_type,omitempty"`
	Narrator        *string  `json:"narrator,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
}

// CommitUploadRequest represents the user-confirmed or edited metadata to commit to /library.
type CommitUploadRequest struct {
	Title          string   `json:"title"`
	Author         string   `json:"author"`
	Authors        []string `json:"authors,omitempty"`
	Series         *string  `json:"series,omitempty"`
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Publisher      *string  `json:"publisher,omitempty"`
	Language       *string  `json:"language,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	BookType       string   `json:"book_type,omitempty"`
	Narrator       *string  `json:"narrator,omitempty"`
}

// StageUploadBook stages an EPUB or Audiobook file and extracts its metadata and cover preview without committing to /library.
func (h *BookHandler) StageUploadBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	up, ok := parseStreamingBookUpload(w, r, h.dataDir)
	if !ok {
		return
	}

	jobID := up.jobID
	uploadsDir := up.uploadsDir
	stagedPath := up.stagedPath
	ext := up.ext
	fileType := up.fileType
	hasCover := writeUploadedCoverIfValid(uploadsDir, jobID, up.coverData)

	var title string
	var authors []string
	var series *string
	var seqNum *float64
	var desc *string
	var pub *string
	var lang *string
	var genres []string
	var narrator *string
	var durationSeconds *float64

	if fileType == "epub" {
		epubReader, err := epub.Open(stagedPath)
		if err != nil {
			_ = os.Remove(stagedPath)
			_ = os.Remove(filepath.Join(uploadsDir, jobID+".cover"))
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid EPUB archive: %v", err))
			return
		}
		parsed, err := epubReader.ParseBook()
		if err != nil {
			epubReader.Close()
			_ = os.Remove(stagedPath)
			_ = os.Remove(filepath.Join(uploadsDir, jobID+".cover"))
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse EPUB metadata: %v", err))
			return
		}

		if !hasCover {
			coverBytes, _, coverErr := epubReader.ExtractCoverImage()
			if coverErr == nil && len(coverBytes) > 0 {
				hasCover = true
				coverPath := filepath.Join(uploadsDir, jobID+".cover")
				_ = os.WriteFile(coverPath, coverBytes, 0644)
			}
		}
		epubReader.Close()

		authors = make([]string, 0, len(parsed.Authors))
		for _, a := range parsed.Authors {
			if strings.TrimSpace(a.Name) != "" {
				authors = append(authors, strings.TrimSpace(a.Name))
			}
		}
		title = strings.TrimSpace(parsed.Title)
		if parsed.Series != nil && strings.TrimSpace(parsed.Series.Name) != "" {
			s := strings.TrimSpace(parsed.Series.Name)
			series = &s
			seqNum = parsed.Series.SequenceNumber
		}
		if strings.TrimSpace(parsed.Description) != "" {
			d := strings.TrimSpace(parsed.Description)
			desc = &d
		}
		if strings.TrimSpace(parsed.Publisher) != "" {
			p := strings.TrimSpace(parsed.Publisher)
			pub = &p
		}
		if strings.TrimSpace(parsed.Language) != "" {
			l := strings.TrimSpace(parsed.Language)
			lang = &l
		}
		genres = parsed.Genres
	} else {
		// Audiobook file (.m4b, .mp3, .m4a, .flac)
		meta, err := audio.ExtractMetadata(stagedPath)
		if err != nil {
			_ = os.Remove(stagedPath)
			_ = os.Remove(filepath.Join(uploadsDir, jobID+".cover"))
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to extract audio metadata: %v", err))
			return
		}

		if !hasCover && len(meta.CoverData) > 0 {
			hasCover = true
			coverPath := filepath.Join(uploadsDir, jobID+".cover")
			_ = os.WriteFile(coverPath, meta.CoverData, 0644)
		}

		title = strings.TrimSpace(meta.Title)
		if title == "" && meta.Album != "" {
			title = strings.TrimSpace(meta.Album)
		}
		if meta.Author != "" {
			for _, a := range strings.Split(meta.Author, ",") {
				clean := strings.TrimSpace(a)
				if clean != "" {
					authors = append(authors, clean)
				}
			}
		}
		if meta.Narrator != "" {
			n := strings.TrimSpace(meta.Narrator)
			narrator = &n
		}
		if strings.TrimSpace(meta.Description) != "" {
			d := strings.TrimSpace(meta.Description)
			desc = &d
		}
		if meta.DurationSeconds > 0 {
			dur := meta.DurationSeconds
			durationSeconds = &dur
		}
	}

	// Apply optional fallback metadata provided by an importer plugin (metadata_defaults JSON)
	sourcePlugin := up.sourcePlugin
	sourceExternalID := up.sourceExternalID
	rawMetaDefaults := up.rawMetaDefaults
	if rawMetaDefaults != "" {
		var defaults struct {
			Title          string   `json:"title"`
			Authors        []string `json:"authors"`
			Series         *string  `json:"series"`
			SequenceNumber *float64 `json:"sequence_number"`
			Description    *string  `json:"description"`
			Publisher      *string  `json:"publisher"`
			Language       *string  `json:"language"`
			Genres         []string `json:"genres"`
			Narrator       *string  `json:"narrator"`
		}
		if err := json.Unmarshal([]byte(rawMetaDefaults), &defaults); err == nil {
			if (title == "" || strings.EqualFold(title, "Untitled")) && strings.TrimSpace(defaults.Title) != "" {
				title = strings.TrimSpace(defaults.Title)
			}
			isGenericAuthor := len(authors) == 0 || (len(authors) == 1 && (strings.EqualFold(authors[0], "Unknown") || (defaults.Publisher != nil && strings.EqualFold(authors[0], strings.TrimSpace(*defaults.Publisher)))))
			if isGenericAuthor && len(defaults.Authors) > 0 {
				var cleanAuthors []string
				for _, a := range defaults.Authors {
					if trimmed := strings.TrimSpace(a); trimmed != "" {
						cleanAuthors = append(cleanAuthors, trimmed)
					}
				}
				if len(cleanAuthors) > 0 {
					authors = cleanAuthors
				}
			}
			if (series == nil || strings.TrimSpace(*series) == "") && defaults.Series != nil && strings.TrimSpace(*defaults.Series) != "" {
				s := strings.TrimSpace(*defaults.Series)
				series = &s
				if seqNum == nil {
					seqNum = defaults.SequenceNumber
				}
			}
			if (desc == nil || strings.TrimSpace(*desc) == "") && defaults.Description != nil && strings.TrimSpace(*defaults.Description) != "" {
				d := strings.TrimSpace(*defaults.Description)
				desc = &d
			}
			if (pub == nil || strings.TrimSpace(*pub) == "") && defaults.Publisher != nil && strings.TrimSpace(*defaults.Publisher) != "" {
				p := strings.TrimSpace(*defaults.Publisher)
				pub = &p
			}
			if (lang == nil || strings.TrimSpace(*lang) == "") && defaults.Language != nil && strings.TrimSpace(*defaults.Language) != "" {
				l := strings.TrimSpace(*defaults.Language)
				lang = &l
			}
			if len(genres) == 0 && len(defaults.Genres) > 0 {
				for _, g := range defaults.Genres {
					if trimmed := strings.TrimSpace(g); trimmed != "" {
						genres = append(genres, trimmed)
					}
				}
			}
			if (narrator == nil || strings.TrimSpace(*narrator) == "") && defaults.Narrator != nil && strings.TrimSpace(*defaults.Narrator) != "" {
				n := strings.TrimSpace(*defaults.Narrator)
				narrator = &n
			}
		}
	}

	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}
	if title == "" {
		title = strings.TrimSuffix(up.filename, filepath.Ext(up.filename))
	}

	var warnings []string
	if len(authors) == 1 && authors[0] == "Unknown" {
		warnings = append(warnings, "No author found in metadata")
	}
	if title == "Untitled" {
		warnings = append(warnings, "No title found in metadata")
	}

	// If staging an audio file, drain any pending queued EPUB uploads first so a just-uploaded
	// EPUB from Step 1 is in the catalog, then try to match an existing ebook on the shelf.
	if fileType == "audiobook" {
		h.drainPendingUploads(r.Context())

		matchedBook, matchErr := repository.MatchBookForReadalong(r.Context(), h.repo, title, authors)
		if matchErr == nil && matchedBook != nil && h.ingester != nil {
			var coverBytes []byte
			if hasCover {
				coverBytes, _ = os.ReadFile(filepath.Join(uploadsDir, jobID+".cover"))
			}

			audioFile, openErr := os.Open(stagedPath)
			if openErr == nil {
				updatedBook, wasDuplicate, attachErr := h.ingester.AttachReadalongAudio(
					r.Context(),
					matchedBook,
					up.filename,
					audioFile,
					authors,
					narrator,
					coverBytes,
				)
				audioFile.Close()
				if attachErr == nil && updatedBook != nil {
					if wasDuplicate {
						warnings = append(warnings, "Book already exists in library")
					}
					dto := StagedMetadataDTO{
						Title:           updatedBook.Title,
						Authors:         authors,
						Description:     desc,
						Publisher:       pub,
						Language:        lang,
						Genres:          genres,
						IsDuplicate:     wasDuplicate,
						Warnings:        warnings,
						BookType:        "ebook",
						Narrator:        narrator,
						DurationSeconds: updatedBook.DurationSeconds,
					}
					metaBytes, _ := json.Marshal(dto)
					metaStr := string(metaBytes)

					job := &repository.UploadJob{
						ID:         jobID,
						Filename:   up.filename,
						StagedPath: stagedPath,
						Status:     "completed",
						BookID:     &updatedBook.ID,
						Metadata:   &metaStr,
						HasCover:   hasCover,
					}
					_ = h.repo.CreateUploadJob(r.Context(), job)
					_ = h.repo.UpdateUploadJobStatus(r.Context(), job.ID, "completed", &updatedBook.ID, nil)
					if sourcePlugin != "" && sourceExternalID != "" {
						_ = h.repo.UpsertBookSource(r.Context(), &repository.BookSource{
							PluginName:  sourcePlugin,
							ExternalID:  sourceExternalID,
							UploadJobID: &job.ID,
							BookID:      &updatedBook.ID,
							MetadataJSON: func() *string {
								if rawMetaDefaults == "" {
									return nil
								}
								return &rawMetaDefaults
							}(),
						})
					}
					_ = os.Remove(stagedPath)

					if h.hub != nil {
						h.hub.Broadcast(events.Event{
							Type: events.EventBookAdded,
							Data: BuildBookListItem(r.Context(), h.repo, updatedBook),
						})
					}
					h.broadcastQueueStatus(r.Context())

					h.logger.Info("Linked staged audio as read-along to existing ebook",
						"job_id", job.ID,
						"book_id", updatedBook.ID,
						"title", updatedBook.Title,
						"is_duplicate", wasDuplicate,
					)

					writeJSON(w, http.StatusOK, map[string]any{
						"id":           updatedBook.ID,
						"book_id":      updatedBook.ID,
						"job_id":       job.ID,
						"status":       job.Status,
						"filename":     job.Filename,
						"metadata":     dto,
						"has_cover":    hasCover,
						"warnings":     warnings,
						"is_duplicate": wasDuplicate,
						"created_at":   job.CreatedAt,
					})
					return
				}
			}
		}
	}

	primaryAuthor := authors[0]
	isDuplicate := false
	targetRelPath := filepath.Join(scanner.SanitizeAuthorSegment(primaryAuthor), scanner.SanitizePathSegment(title), scanner.SanitizePathSegment(title)+ext)
	if existing, err := h.repo.GetBookByFilePath(r.Context(), targetRelPath); err == nil && existing != nil {
		isDuplicate = true
	} else {
		books, err := h.repo.ListBooks(r.Context(), repository.BookFilter{
			Search: &title,
			Limit:  50,
		})
		if err == nil {
			for _, b := range books {
				if fileType == "audiobook" && b.BookType != "audiobook" {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(b.Title), title) {
					bAuthors, err := h.repo.GetBookAuthors(r.Context(), b.ID)
					if err == nil {
						for _, ba := range bAuthors {
							if strings.EqualFold(strings.TrimSpace(ba.Name), primaryAuthor) {
								isDuplicate = true
								break
							}
						}
					}
					if isDuplicate {
						break
					}
				}
			}
		}
	}

	if isDuplicate {
		warnings = append(warnings, "Book already exists in library")
	}

	dto := StagedMetadataDTO{
		Title:           title,
		Authors:         authors,
		Series:          series,
		SequenceNumber:  seqNum,
		Description:     desc,
		Publisher:       pub,
		Language:        lang,
		Genres:          genres,
		IsDuplicate:     isDuplicate,
		Warnings:        warnings,
		BookType:        fileType,
		Narrator:        narrator,
		DurationSeconds: durationSeconds,
	}

	metaBytes, _ := json.Marshal(dto)
	metaStr := string(metaBytes)

	job := &repository.UploadJob{
		ID:         jobID,
		Filename:   up.filename,
		StagedPath: stagedPath,
		Status:     "staged",
		Metadata:   &metaStr,
		HasCover:   hasCover,
	}
	if err := h.repo.CreateUploadJob(r.Context(), job); err != nil {
		_ = os.Remove(stagedPath)
		_ = os.Remove(filepath.Join(uploadsDir, jobID+".cover"))
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to record staged upload: %v", err))
		return
	}

	if sourcePlugin != "" && sourceExternalID != "" {
		var metaPtr *string
		if rawMetaDefaults != "" {
			metaPtr = &rawMetaDefaults
		}
		_ = h.repo.UpsertBookSource(r.Context(), &repository.BookSource{
			PluginName:   sourcePlugin,
			ExternalID:   sourceExternalID,
			UploadJobID:  &job.ID,
			MetadataJSON: metaPtr,
		})
	}

	h.broadcastQueueStatus(r.Context())

	h.logger.Info("Staged book upload for review", "job_id", job.ID, "filename", job.Filename, "title", title, "type", fileType)

	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":       job.ID,
		"status":       job.Status,
		"filename":     job.Filename,
		"metadata":     dto,
		"has_cover":    hasCover,
		"warnings":     warnings,
		"is_duplicate": isDuplicate,
		"created_at":   job.CreatedAt,
	})
}

func (h *BookHandler) drainPendingUploads(ctx context.Context) {
	if h.uploadWorker != nil {
		h.uploadWorker.DrainPending(ctx)
		return
	}
	if h.ingester != nil && h.repo != nil {
		tmpWorker := worker.NewUploadWorker(h.repo, h.ingester, h.worker, worker.UploadWorkerConfig{
			Hub:    h.hub,
			Logger: h.logger,
		})
		tmpWorker.DrainPending(ctx)
	}
}

// ReadalongResponse represents the synchronized Media Overlays / read-along payload for a book.
type ReadalongResponse struct {
	BookID              string                     `json:"book_id"`
	HasReadalong        bool                       `json:"has_readalong"`
	HasMediaOverlays    bool                       `json:"has_media_overlays"`
	ActiveClass         string                     `json:"active_class"`
	PlaybackActiveClass string                     `json:"playback_active_class"`
	Narrator            string                     `json:"narrator,omitempty"`
	DurationSeconds     *float64                   `json:"duration_seconds,omitempty"`
	AudioStreamURL      string                     `json:"audio_stream_url,omitempty"`
	Segments            []epub.MediaOverlaySegment `json:"segments"`
}

// GetBookReadalong returns the EPUB 3 Media Overlays (SMIL) timeline or synthesized chapter/paragraph
// read-along segments for synchronized audio playback, word highlighting, and page turning.
func (h *BookHandler) GetBookReadalong(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) || book == nil {
		writeJSONError(w, http.StatusNotFound, "Book not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get book: %v", err))
		return
	}

	resp := h.buildReadalongResponse(r.Context(), book)
	writeJSON(w, http.StatusOK, resp)
}

func (h *BookHandler) buildReadalongResponse(ctx context.Context, book *repository.Book) ReadalongResponse {
	files, _ := h.repo.GetBookFilesByBookID(ctx, book.ID)
	hasReadalong, durationSeconds := resolveReadalongStatus(ctx, h.repo, book, files)

	resp := ReadalongResponse{
		BookID:              book.ID,
		HasReadalong:        hasReadalong,
		HasMediaOverlays:    false,
		ActiveClass:         "-epub-media-overlay-active",
		PlaybackActiveClass: "-epub-media-overlay-playing",
		DurationSeconds:     durationSeconds,
		Segments:            []epub.MediaOverlaySegment{},
	}
	if hasReadalong || book.BookType == "audiobook" {
		resp.AudioStreamURL = fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID)
	}

	// 1. Attempt to extract SMIL Media Overlays from the EPUB archive if available
	if epubPath := h.resolveBookFilePath(book); epubPath != "" && strings.HasSuffix(strings.ToLower(epubPath), ".epub") {
		if reader, err := epub.Open(epubPath); err == nil {
			if manifest, mErr := reader.ExtractMediaOverlays(); mErr == nil && manifest != nil {
				if manifest.ActiveClass != "" {
					resp.ActiveClass = manifest.ActiveClass
				}
				if manifest.PlaybackActiveClass != "" {
					resp.PlaybackActiveClass = manifest.PlaybackActiveClass
				}
				if manifest.Narrator != "" {
					resp.Narrator = manifest.Narrator
				}
				if resp.DurationSeconds == nil && manifest.DurationSeconds != nil {
					resp.DurationSeconds = manifest.DurationSeconds
				}
				if len(manifest.Segments) > 0 {
					resp.HasMediaOverlays = true
					resp.HasReadalong = true
					resp.Segments = manifest.Segments
					if resp.AudioStreamURL == "" {
						resp.AudioStreamURL = fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID)
					}
				}
			}
			reader.Close()
		}
	}

	// 2. Fallback: if the book has read-along audio linked but no SMIL segments inside the EPUB,
	// synthesize timeline segments from the book's chapters and paragraphs so the reader can still
	// highlight phrases and auto-turn pages across the audio timeline.
	if len(resp.Segments) == 0 && resp.HasReadalong {
		chapters, _ := h.repo.GetChaptersByBookID(ctx, book.ID)
		audioChapters, _ := h.repo.GetAudioChaptersByBookID(ctx, book.ID)

		totalDur := 0.0
		if resp.DurationSeconds != nil && *resp.DurationSeconds > 0 {
			totalDur = *resp.DurationSeconds
		} else {
			for _, ac := range audioChapters {
				end := ac.StartOffsetSec + ac.DurationSec
				if end > totalDur {
					totalDur = end
				}
			}
		}

		if len(chapters) > 0 && totalDur > 0 {
			totalChars := 0
			for _, ch := range chapters {
				cLen := len(strings.TrimSpace(ch.ContentPlain))
				if cLen < 20 {
					cLen = 20
				}
				totalChars += cLen
			}

			cursor := 0.0
			for i, ch := range chapters {
				var chapStart, chapEnd float64
				if len(audioChapters) == len(chapters) && audioChapters[i].DurationSec > 0 {
					chapStart = audioChapters[i].StartOffsetSec
					chapEnd = chapStart + audioChapters[i].DurationSec
				} else {
					cLen := len(strings.TrimSpace(ch.ContentPlain))
					if cLen < 20 {
						cLen = 20
					}
					dur := totalDur * (float64(cLen) / float64(totalChars))
					chapStart = cursor
					chapEnd = cursor + dur
					cursor = chapEnd
				}

				href := ""
				if ch.Href != nil {
					href = *ch.Href
				}

				var blocks []string
				for _, line := range strings.Split(ch.ContentPlain, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" {
						blocks = append(blocks, trimmed)
					}
				}
				if len(blocks) == 0 && ch.Title != nil && strings.TrimSpace(*ch.Title) != "" {
					blocks = []string{strings.TrimSpace(*ch.Title)}
				}
				if len(blocks) == 0 {
					resp.Segments = append(resp.Segments, epub.MediaOverlaySegment{
						ChapterIndex: ch.ChapterIndex,
						ChapterHref:  href,
						FragmentID:   fmt.Sprintf("ch-%d", ch.ChapterIndex),
						ClipBegin:    0,
						ClipEnd:      chapEnd - chapStart,
						GlobalBegin:  chapStart,
						GlobalEnd:    chapEnd,
					})
					continue
				}

				blockChars := 0
				for _, b := range blocks {
					blockChars += len(b)
				}
				if blockChars == 0 {
					blockChars = 1
				}

				bCursor := chapStart
				chapDur := chapEnd - chapStart
				for bIdx, bText := range blocks {
					bDur := chapDur * (float64(len(bText)) / float64(blockChars))
					bStart := bCursor
					bEnd := bCursor + bDur
					bCursor = bEnd
					resp.Segments = append(resp.Segments, epub.MediaOverlaySegment{
						ChapterIndex: ch.ChapterIndex,
						ChapterHref:  href,
						ParID:        fmt.Sprintf("ch%d_p%d", ch.ChapterIndex, bIdx+1),
						FragmentID:   fmt.Sprintf("p%d", bIdx+1),
						Text:         bText,
						ClipBegin:    bStart - chapStart,
						ClipEnd:      bEnd - chapStart,
						GlobalBegin:  bStart,
						GlobalEnd:    bEnd,
					})
				}
			}
		}
	}

	return resp
}

// GetBookReadalongVTT serves synchronized read-along segments as WebVTT captions for Cast TVs.
func (h *BookHandler) GetBookReadalongVTT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
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

	resp := h.buildReadalongResponse(r.Context(), book)
	trackFilter := strings.TrimSpace(r.URL.Query().Get("track"))
	vtt := RenderReadalongWebVTT(resp.Segments, trackFilter)

	modTime := time.Time{}
	if book.FileModifiedAt != nil {
		modTime = *book.FileModifiedAt
	}

	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, book.ID+".vtt", modTime, bytes.NewReader([]byte(vtt)))
}

// RenderReadalongWebVTT formats MediaOverlaySegments into WebVTT cues.
// When trackFilter is non-empty, only matching AudioSrc segments are included using ClipBegin/ClipEnd.
func RenderReadalongWebVTT(segments []epub.MediaOverlaySegment, trackFilter string) string {
	var b strings.Builder
	b.Grow(len(segments)*64 + 16)
	b.WriteString("WEBVTT\n\n")

	cleanFilter := strings.TrimPrefix(strings.TrimSpace(trackFilter), "/")
	cueNum := 0
	for _, seg := range segments {
		startSec := seg.GlobalBegin
		endSec := seg.GlobalEnd
		if cleanFilter != "" {
			segSrc := strings.TrimPrefix(strings.TrimSpace(seg.AudioSrc), "/")
			if segSrc != cleanFilter {
				continue
			}
			startSec = seg.ClipBegin
			endSec = seg.ClipEnd
		}

		cueText := strings.TrimSpace(seg.Text)
		if cueText == "" {
			cueText = strings.TrimSpace(seg.FragmentID)
		}
		if cueText == "" {
			continue
		}
		cueText = strings.ReplaceAll(cueText, "-->", "->")
		if endSec <= startSec {
			endSec = startSec + 0.1
		}

		cueNum++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", cueNum, formatVTTTimestamp(startSec), formatVTTTimestamp(endSec), cueText)
	}

	return b.String()
}

func formatVTTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMillis := int64(seconds*1000.0 + 0.5)
	hours := totalMillis / 3600000
	rem := totalMillis % 3600000
	mins := rem / 60000
	rem %= 60000
	secs := rem / 1000
	millis := rem % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, mins, secs, millis)
}

// GetUploadJobCover returns the extracted cover image for a staged upload job.
func (h *BookHandler) GetUploadJobCover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
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

	writeCoverBytes := func(data []byte) {
		w.Header().Set("Content-Type", http.DetectContentType(data))
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	}

	coverPath := filepath.Join(h.dataDir, "uploads", jobID+".cover")
	if data, err := os.ReadFile(coverPath); err == nil && len(data) > 0 {
		writeCoverBytes(data)
		return
	}

	// Fallback: extract directly from staged EPUB or Audiobook file
	job, err := h.repo.GetUploadJob(r.Context(), jobID)
	if err == nil && job != nil && job.StagedPath != "" {
		fileType, _ := scanner.DetectFileType(job.Filename)
		if fileType == "epub" {
			if reader, err := epub.Open(job.StagedPath); err == nil {
				defer reader.Close()
				if data, _, err := reader.ExtractCoverImage(); err == nil && len(data) > 0 {
					_ = os.WriteFile(coverPath, data, 0644)
					writeCoverBytes(data)
					return
				}
			}
		} else {
			if meta, err := audio.ExtractMetadata(job.StagedPath); err == nil && len(meta.CoverData) > 0 {
				_ = os.WriteFile(coverPath, meta.CoverData, 0644)
				writeCoverBytes(meta.CoverData)
				return
			}
		}
	}

	writeJSONError(w, http.StatusNotFound, "Cover image not found for staged upload")
}

// UploadJobCover handles uploading a replacement cover image for a staged upload job.
func (h *BookHandler) UploadJobCover(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) {
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

	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid form data: max 10MB cover image")
		return
	}

	file, header, err := r.FormFile("cover")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "No image file provided (field 'cover' or 'file')")
		return
	}
	defer file.Close()

	coverData, err := io.ReadAll(io.LimitReader(file, maxUploadCoverBytes+1))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read cover image")
		return
	}
	if len(coverData) > maxUploadCoverBytes {
		writeJSONError(w, http.StatusBadRequest, "Invalid form data: max 10MB cover image")
		return
	}

	contentType := http.DetectContentType(coverData)
	if !strings.HasPrefix(contentType, "image/") {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid file format (%s): must be an image (JPEG, PNG, WebP)", contentType))
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

	h.broadcastQueueStatus(r.Context())
	h.logger.Info("Uploaded replacement cover for staged book", "job_id", job.ID, "filename", header.Filename, "bytes", len(coverData))

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "Cover image updated successfully",
		"job_id":    job.ID,
		"has_cover": true,
	})
}

// CommitUploadJob applies any edited metadata to the EPUB on disk and finalizes ingestion into /library.
func (h *BookHandler) CommitUploadJob(w http.ResponseWriter, r *http.Request) {
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
	if errors.Is(err, repository.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "Upload job not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get upload job: %v", err))
		return
	}

	if job.Status == "completed" && job.BookID != nil {
		if existingBook, bErr := h.repo.GetBookByID(r.Context(), *job.BookID); bErr == nil && existingBook != nil {
			writeJSON(w, http.StatusCreated, existingBook)
			return
		}
	}

	if job.Status != "staged" {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Upload job is not in 'staged' status (current status: %s)", job.Status))
		return
	}

	var req CommitUploadRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request body: %v", err))
			return
		}
	}

	if strings.TrimSpace(req.Title) == "" && job.Metadata != nil && *job.Metadata != "" {
		var stagedMeta StagedMetadataDTO
		if json.Unmarshal([]byte(*job.Metadata), &stagedMeta) == nil {
			req.Title = stagedMeta.Title
			if len(req.Authors) == 0 && strings.TrimSpace(req.Author) == "" {
				req.Authors = stagedMeta.Authors
			}
			if req.Series == nil {
				req.Series = stagedMeta.Series
			}
			if req.SequenceNumber == nil {
				req.SequenceNumber = stagedMeta.SequenceNumber
			}
			if req.Description == nil {
				req.Description = stagedMeta.Description
			}
			if req.Publisher == nil {
				req.Publisher = stagedMeta.Publisher
			}
			if req.Language == nil {
				req.Language = stagedMeta.Language
			}
			if len(req.Genres) == 0 {
				req.Genres = stagedMeta.Genres
			}
			if req.Narrator == nil {
				req.Narrator = stagedMeta.Narrator
			}
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeJSONError(w, http.StatusBadRequest, "Book title is required")
		return
	}

	authors := req.Authors
	if len(authors) == 0 && strings.TrimSpace(req.Author) != "" {
		authors = []string{strings.TrimSpace(req.Author)}
	}
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}
	primaryAuthor := authors[0]

	fileType, _ := scanner.DetectFileType(job.Filename)
	if fileType == "epub" {
		// Update the EPUB file package metadata on disk
		update := epub.MetadataUpdate{
			Title:          title,
			Authors:        authors,
			Series:         req.Series,
			SequenceNumber: req.SequenceNumber,
			Description:    req.Description,
			Publisher:      req.Publisher,
			Language:       req.Language,
			Genres:         req.Genres,
		}
		if err := epub.UpdateMetadata(job.StagedPath, update); err != nil {
			h.logger.Error("Failed to update EPUB metadata", "job_id", job.ID, "error", err)
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update EPUB metadata: %v", err))
			return
		}
	}

	// Ingest the updated book into /library/<Author>/<Title>/<Title><ext>
	stagedFile, err := os.Open(job.StagedPath)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to open staged file: %v", err))
		return
	}
	defer stagedFile.Close()

	stagedCoverPath := filepath.Join(h.dataDir, "uploads", job.ID+".cover")
	coverBytes, _ := os.ReadFile(stagedCoverPath)

	// If committing an audiobook that matches an existing ebook on the shelf, attach as read-along
	if fileType == "audiobook" {
		if matchedBook, mErr := repository.MatchBookForReadalong(r.Context(), h.repo, title, authors); mErr == nil && matchedBook != nil {
			updatedBook, _, attachErr := h.ingester.AttachReadalongAudio(
				r.Context(),
				matchedBook,
				job.Filename,
				stagedFile,
				authors,
				req.Narrator,
				coverBytes,
			)
			stagedFile.Close()
			if attachErr == nil && updatedBook != nil {
				_ = os.Remove(job.StagedPath)
				_ = os.Remove(stagedCoverPath)
				_ = h.repo.UpdateUploadJobStatus(r.Context(), job.ID, "completed", &updatedBook.ID, nil)
				_ = h.repo.LinkBookSourceByUploadJob(r.Context(), job.ID, updatedBook.ID)
				if h.hub != nil {
					h.hub.Broadcast(events.Event{
						Type: events.EventBookAdded,
						Data: BuildBookListItem(r.Context(), h.repo, updatedBook),
					})
				}
				h.broadcastQueueStatus(r.Context())
				writeJSON(w, http.StatusCreated, updatedBook)
				return
			}
		}
	}

	// If a staged cover file exists, write it into /library/<Author>/<Title>/cover.<ext>
	// so Audiobookshelf immediately detects it as the book cover.
	if len(coverBytes) > 0 {
		ext := ".jpg"
		ct := http.DetectContentType(coverBytes)
		if strings.Contains(ct, "png") {
			ext = ".png"
		} else if strings.Contains(ct, "webp") {
			ext = ".webp"
		}
		bookDir := filepath.Join(h.libraryDir, scanner.SanitizeAuthorSegment(primaryAuthor), scanner.SanitizePathSegment(title))
		_ = os.MkdirAll(bookDir, 0755)
		_ = os.WriteFile(filepath.Join(bookDir, "cover"+ext), coverBytes, 0644)
	}

	book, err := h.ingester.SaveUpload(r.Context(), primaryAuthor, title, job.Filename, stagedFile)
	stagedFile.Close()
	if err != nil {
		h.logger.Error("Failed to ingest book into library", "job_id", job.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to ingest book into library: %v", err))
		return
	}

	// For standalone audiobooks, update book metadata with user-confirmed details
	if fileType == "audiobook" && book.BookType == "audiobook" {
		book.Title = title
		book.Description = req.Description
		book.Publisher = req.Publisher
		book.Language = req.Language
		_ = h.repo.UpdateBook(r.Context(), book)

		for _, a := range authors {
			if author, err := h.repo.UpsertAuthor(r.Context(), a); err == nil {
				_ = h.repo.LinkBookAuthor(r.Context(), book.ID, author.ID, "Author")
			}
		}
		if req.Narrator != nil && strings.TrimSpace(*req.Narrator) != "" {
			if narrator, err := h.repo.UpsertAuthor(r.Context(), strings.TrimSpace(*req.Narrator)); err == nil {
				_ = h.repo.LinkBookAuthor(r.Context(), book.ID, narrator.ID, "Narrator")
			}
		}
		if req.Series != nil && strings.TrimSpace(*req.Series) != "" {
			seriesName := scanner.CleanSeriesName(strings.TrimSpace(*req.Series))
			if series, err := h.repo.UpsertSeries(r.Context(), seriesName, nil); err == nil {
				_ = h.repo.LinkBookSeries(r.Context(), book.ID, series.ID, req.SequenceNumber)
			}
		}
	}

	// Clean up staged file and cached cover
	_ = os.Remove(job.StagedPath)
	_ = os.Remove(filepath.Join(h.dataDir, "uploads", job.ID+".cover"))

	_ = h.repo.UpdateUploadJobStatus(r.Context(), job.ID, "completed", &book.ID, nil)
	_ = h.repo.LinkBookSourceByUploadJob(r.Context(), job.ID, book.ID)

	if h.worker != nil {
		h.worker.Trigger()
	}

	if h.hub != nil && book != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookAdded,
			Data: BuildBookListItem(r.Context(), h.repo, book),
		})
	}
	h.broadcastQueueStatus(r.Context())

	h.logger.Info("Committed book upload to library", "job_id", job.ID, "book_id", book.ID, "title", book.Title, "author", primaryAuthor)

	writeJSON(w, http.StatusCreated, book)
}

// DeleteUploadJob discards a staged upload job and removes temporary files.
func (h *BookHandler) DeleteUploadJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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
	if err == nil && job != nil {
		_ = os.Remove(job.StagedPath)
		_ = os.Remove(filepath.Join(h.dataDir, "uploads", job.ID+".cover"))
		_ = h.repo.DeleteUncommittedBookSourceByUploadJob(r.Context(), jobID)
		_ = h.repo.DeleteUploadJob(r.Context(), jobID)
		h.broadcastQueueStatus(r.Context())
	}

	w.WriteHeader(http.StatusNoContent)
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

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	jobs, err := h.repo.ListUploadJobs(r.Context(), limit, status)
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

func (h *BookHandler) broadcastQueueStatus(ctx context.Context) {
	if h.hub == nil {
		return
	}
	status, err := h.repo.GetQueueStatus(ctx)
	if err == nil && status != nil {
		if h.worker != nil {
			rt := h.worker.GetRuntimeStatus()
			if rt.IsBusy {
				status.IsActive = true
				status.CurrentBook = rt.CurrentBook
				status.CurrentChapter = rt.CurrentChapter
			}
		}
		h.hub.Broadcast(events.Event{
			Type: events.EventQueueStatus,
			Data: status,
		})
	}
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

	var desc string
	if book.Description != nil && strings.TrimSpace(*book.Description) != "" {
		desc = strings.TrimSpace(*book.Description)
	} else {
		desc = "No description available."
	}

	systemPrompt := fmt.Sprintf(
		"You are an insightful literary companion assisting a reader with the book \"%s\".\n"+
			"Book ID: %s\n"+
			"Description: %s\n\n"+
			"Primarily answer questions from this book. You have tools available to search the library and read chapter content. "+
			"If you need details, summaries, or specific passages from the book, invoke the available tools using book_id \"%s\". "+
			"Ground your answers in the book text and cite chapters or sections where appropriate.",
		book.Title, book.ID, desc, book.ID,
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
			role := strings.ToLower(m.Role)
			if role == "user" || role == "assistant" {
				messages = append(messages, ai.ChatMessage{
					Role:    role,
					Content: m.Content,
				})
			}
		}
	}
	messages = append(messages, ai.ChatMessage{Role: "user", Content: message})

	var invoker ai.ToolInvoker = h.toolInvoker
	agent := ai.NewChatAgent(h.aiClient, invoker)
	agentResult, err := agent.Execute(r.Context(), bookID, messages)
	if err != nil {
		h.logger.Error("AI chat agent failed", "book_id", bookID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("AI chat failed: %v", err))
		return
	}

	var fallbackAuthor string
	if authors, err := h.repo.GetBookAuthors(r.Context(), book.ID); err == nil && len(authors) > 0 {
		fallbackAuthor = authors[0].Name
	}

	citations := make([]BookCitation, 0, len(agentResult.Citations))
	for _, c := range agentResult.Citations {
		title := c.ChapterTitle
		citBookID := c.BookID
		if citBookID == "" {
			citBookID = book.ID
		}
		citBookTitle := c.BookTitle
		if citBookTitle == "" {
			citBookTitle = book.Title
		}
		citAuthorName := c.AuthorName
		if citAuthorName == "" {
			citAuthorName = fallbackAuthor
		}
		citations = append(citations, BookCitation{
			BookID:         citBookID,
			BookTitle:      citBookTitle,
			AuthorName:     citAuthorName,
			ChapterID:      c.ChapterID,
			ChapterIndex:   c.ChapterIndex,
			ChapterTitle:   &title,
			StartParagraph: c.StartParagraph,
			EndParagraph:   c.EndParagraph,
			Excerpt:        c.Excerpt,
			Summary:        c.Excerpt,
		})
	}

	writeJSON(w, http.StatusOK, BookChatResponse{
		Reply:     agentResult.Reply,
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

// UpdateBookMetadataRequest defines the payload for modifying existing book metadata.
type UpdateBookMetadataRequest struct {
	Title          string   `json:"title"`
	Authors        []string `json:"authors,omitempty"`
	Author         string   `json:"author,omitempty"`
	Series         *string  `json:"series,omitempty"`
	SequenceNumber *float64 `json:"sequence_number,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Publisher      *string  `json:"publisher,omitempty"`
	Language       *string  `json:"language,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	Topics         []string `json:"topics,omitempty"`
}

// UpdateBookMetadata handles updating an existing book's metadata in the database and in-place in its EPUB file.
func (h *BookHandler) UpdateBookMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
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

	var req UpdateBookMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON request body: %v", err))
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeJSONError(w, http.StatusBadRequest, "Book title is required")
		return
	}

	authors := req.Authors
	if len(authors) == 0 && strings.TrimSpace(req.Author) != "" {
		authors = []string{strings.TrimSpace(req.Author)}
	}
	if len(authors) == 0 {
		authors = []string{"Unknown"}
	}

	// 1. Update EPUB package metadata on disk if the file exists
	epubPath := h.resolveBookFilePath(book)
	if epubPath != "" {
		update := epub.MetadataUpdate{
			Title:          title,
			Authors:        authors,
			Series:         req.Series,
			SequenceNumber: req.SequenceNumber,
			Description:    req.Description,
			Publisher:      req.Publisher,
			Language:       req.Language,
			Genres:         req.Genres,
		}
		if err := epub.UpdateMetadata(epubPath, update); err != nil {
			h.logger.Warn("Failed to update EPUB file metadata on disk", "book_id", book.ID, "file_path", epubPath, "error", err)
		} else {
			// Update file size and modtime
			if fi, err := os.Stat(epubPath); err == nil {
				size := fi.Size()
				modTime := fi.ModTime().UTC().Truncate(time.Second)
				book.FileSizeBytes = &size
				book.FileModifiedAt = &modTime
			}
		}
	}

	// 2. Update DB book entity
	book.Title = title
	book.Description = req.Description
	book.Publisher = req.Publisher
	book.Language = req.Language

	if err := h.repo.UpdateBook(r.Context(), book); err != nil {
		h.logger.Error("Failed to update book in database", "book_id", book.ID, "error", err)
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update book: %v", err))
		return
	}

	// 3. Update junction tables: authors
	_ = h.repo.ClearBookAuthors(r.Context(), book.ID)
	for _, a := range authors {
		trimmedName := strings.TrimSpace(a)
		if trimmedName == "" {
			continue
		}
		authorEntity, err := h.repo.UpsertAuthor(r.Context(), trimmedName)
		if err == nil {
			_ = h.repo.LinkBookAuthor(r.Context(), book.ID, authorEntity.ID, "author")
		}
	}

	// 4. Update junction tables: genres
	_ = h.repo.ClearBookGenres(r.Context(), book.ID)
	for _, g := range req.Genres {
		trimmedGenre := strings.TrimSpace(g)
		if trimmedGenre == "" {
			continue
		}
		genreEntity, err := h.repo.UpsertGenre(r.Context(), trimmedGenre)
		if err == nil {
			_ = h.repo.LinkBookGenre(r.Context(), book.ID, genreEntity.ID)
		}
	}

	// 5. Update junction tables: topics
	if len(req.Topics) > 0 {
		_ = h.repo.ClearBookTopics(r.Context(), book.ID)
		for _, t := range req.Topics {
			trimmedTopic := strings.TrimSpace(t)
			if trimmedTopic == "" {
				continue
			}
			topicEntity, err := h.repo.UpsertTopic(r.Context(), trimmedTopic)
			if err == nil {
				_ = h.repo.LinkBookTopic(r.Context(), book.ID, topicEntity.ID)
			}
		}
	}

	// 6. Update junction tables: series
	_ = h.repo.ClearBookSeries(r.Context(), book.ID)
	if req.Series != nil && strings.TrimSpace(*req.Series) != "" {
		seriesName := scanner.CleanSeriesName(strings.TrimSpace(*req.Series))
		seriesEntity, err := h.repo.UpsertSeries(r.Context(), seriesName, nil)
		if err == nil {
			_ = h.repo.LinkBookSeries(r.Context(), book.ID, seriesEntity.ID, req.SequenceNumber)
		}
	}

	// Broadcast updated book event
	item := BuildBookListItem(r.Context(), h.repo, book)
	if h.hub != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookUpdated,
			Data: item,
		})
	}

	// Return updated BookDetailResponse
	authorsList, _ := h.repo.GetBookAuthors(r.Context(), book.ID)
	genresList, _ := h.repo.GetBookGenres(r.Context(), book.ID)
	topicsList, _ := h.repo.GetBookTopics(r.Context(), book.ID)
	seriesList, _ := h.repo.GetBookSeries(r.Context(), book.ID)
	spine, _ := h.repo.GetBookSpine(r.Context(), book.ID)
	chapters, _ := h.repo.GetChaptersByBookID(r.Context(), book.ID)
	for _, c := range chapters {
		c.ContentPlain = ""
	}
	bookmarks, _ := h.repo.ListBookmarksByBookID(r.Context(), book.ID)
	highlights, _ := h.repo.ListHighlightsByBookID(r.Context(), book.ID)

	resp := BookDetailResponse{
		BookListItem: BookListItem{
			ID:                       book.ID,
			Title:                    book.Title,
			Description:              book.Description,
			Language:                 book.Language,
			Publisher:                book.Publisher,
			Identifier:               book.Identifier,
			FilePath:                 book.FilePath,
			CoverPath:                book.CoverPath,
			FileSizeBytes:            book.FileSizeBytes,
			FileModifiedAt:           book.FileModifiedAt,
			PublishedDate:            book.PublishedDate,
			Layout:                   book.Layout,
			RenditionSpread:          book.RenditionSpread,
			RenditionOrientation:     book.RenditionOrientation,
			PageProgressionDirection: book.PageProgressionDirection,
			Authors:                  authorsList,
			Genres:                   genresList,
			Topics:                   topicsList,
			Series:                   seriesList,
		},
		Spine:      spine,
		Chapters:   chapters,
		Bookmarks:  bookmarks,
		Highlights: highlights,
	}

	h.logger.Info("Updated book metadata", "book_id", book.ID, "title", book.Title)
	writeJSON(w, http.StatusOK, resp)
}

// UploadBookCover handles uploading a replacement cover image for an existing book in the library.
func (h *BookHandler) UploadBookCover(w http.ResponseWriter, r *http.Request) {
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

	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid form data: max 10MB cover image")
		return
	}

	file, _, err := r.FormFile("cover")
	if err != nil {
		file, _, err = r.FormFile("file")
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "No image file provided (field 'cover' or 'file')")
		return
	}
	defer file.Close()

	coverData, err := io.ReadAll(io.LimitReader(file, maxUploadCoverBytes+1))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read cover image")
		return
	}
	if len(coverData) > maxUploadCoverBytes {
		writeJSONError(w, http.StatusBadRequest, "Invalid form data: max 10MB cover image")
		return
	}

	contentType := http.DetectContentType(coverData)
	if !strings.HasPrefix(contentType, "image/") {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid file format (%s): must be an image (JPEG, PNG, WebP)", contentType))
		return
	}

	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}

	// 1. Save cover file adjacent to EPUB in book directory for Audiobookshelf compatibility
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

		// Update internal EPUB cover
		if err := epub.UpdateCover(epubPath, coverData, contentType); err != nil {
			h.logger.Warn("Failed to update EPUB embedded cover", "book_id", book.ID, "error", err)
		}
	}

	// 2. Fallback save in private dataDir/covers/<bookID>.<ext>
	if relCoverPath == "" {
		coversDir := filepath.Join(h.dataDir, "covers")
		_ = os.MkdirAll(coversDir, 0755)
		fullCoverPath := filepath.Join(coversDir, bookID+ext)
		_ = os.WriteFile(fullCoverPath, coverData, 0644)
		relCoverPath = filepath.Join("covers", bookID+ext)
	}

	book.CoverPath = &relCoverPath
	_ = h.repo.UpdateBook(r.Context(), book)

	if h.hub != nil {
		h.hub.Broadcast(events.Event{
			Type: events.EventBookUpdated,
			Data: BuildBookListItem(r.Context(), h.repo, book),
		})
	}

	h.logger.Info("Updated book cover", "book_id", book.ID, "cover_path", relCoverPath)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "success",
		"cover_path": relCoverPath,
	})
}
