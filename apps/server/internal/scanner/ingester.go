package scanner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/repository"
)

// Ingester coordinates parsing EPUBs and storing their metadata in the repository.
type Ingester struct {
	repo       repository.StorageEngine
	libraryDir string
	dataDir    string
}

// NewIngester creates a new Ingester instance.
func NewIngester(repo repository.StorageEngine, libraryDir, dataDir string) *Ingester {
	return &Ingester{
		repo:       repo,
		libraryDir: libraryDir,
		dataDir:    dataDir,
	}
}

// IngestFile parses an EPUB file and registers its metadata, chapters, and cover in storage.
func (in *Ingester) IngestFile(ctx context.Context, fullPath, relativePath string) (*repository.Book, error) {
	if existingBook, err := in.repo.GetBookByFilePath(ctx, relativePath); err == nil && existingBook != nil {
		_ = in.ReparseBookChapters(ctx, existingBook.ID)
		return existingBook, nil
	}

	reader, err := epub.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	parsed, err := reader.ParseBook()
	if err != nil {
		return nil, fmt.Errorf("parsing book metadata: %w", err)
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("stating book file: %w", err)
	}
	sizeBytes := fi.Size()

	bookID := uuid.NewString()

	// Extract and cache cover art to /data/covers/{book_id}.{ext}
	var coverRelPath *string
	if coverData, ext, err := reader.ExtractCoverImage(); err == nil && len(coverData) > 0 {
		coversDir := filepath.Join(in.dataDir, "covers")
		if err := os.MkdirAll(coversDir, 0755); err == nil {
			coverFilename := bookID + ext
			coverDiskPath := filepath.Join(coversDir, coverFilename)
			if err := os.WriteFile(coverDiskPath, coverData, 0644); err == nil {
				rel := filepath.Join("covers", coverFilename)
				coverRelPath = &rel
			}
		}
	}

	book := &repository.Book{
		ID:            bookID,
		Title:         parsed.Title,
		FilePath:      relativePath,
		CoverPath:     coverRelPath,
		FileSizeBytes: &sizeBytes,
	}
	if parsed.Description != "" {
		book.Description = &parsed.Description
	}
	if parsed.Publisher != "" {
		book.Publisher = &parsed.Publisher
	}
	if parsed.Language != "" {
		book.Language = &parsed.Language
	}
	if parsed.Identifier != "" {
		book.Identifier = &parsed.Identifier
	}
	if parsed.PublishedDate != "" {
		book.PublishedDate = &parsed.PublishedDate
	}

	if err := in.repo.CreateBook(ctx, book); err != nil {
		return nil, fmt.Errorf("saving book to database: %w", err)
	}

	// Link Authors
	for _, a := range parsed.Authors {
		author, err := in.repo.UpsertAuthor(ctx, a.Name)
		if err != nil {
			continue
		}
		_ = in.repo.LinkBookAuthor(ctx, book.ID, author.ID, a.Role)
	}

	// Link Genres
	for _, g := range parsed.Genres {
		genre, err := in.repo.UpsertGenre(ctx, g)
		if err != nil {
			continue
		}
		_ = in.repo.LinkBookGenre(ctx, book.ID, genre.ID)
	}

	// Link Series
	if parsed.Series != nil && parsed.Series.Name != "" {
		series, err := in.repo.UpsertSeries(ctx, parsed.Series.Name, nil)
		if err == nil {
			_ = in.repo.LinkBookSeries(ctx, book.ID, series.ID, parsed.Series.SequenceNumber)
		}
	}

	// Extract and register Chapters and Paragraphs
	chapters, err := reader.ExtractChapters()
	if err == nil {
		var allParagraphs []*repository.Paragraph
		for _, ch := range chapters {
			chapter := &repository.Chapter{
				BookID:       book.ID,
				ChapterIndex: ch.Index,
				Title:        ch.Title,
				Summary:      "", // Summaries populated by async worker in Milestone 3
				ContentPlain: ch.ContentPlain,
			}
			_ = in.repo.CreateChapter(ctx, chapter)

			chunks := epub.ChunkChapterParagraphs(ch.ContentPlain, 400, 1200)
			for _, chk := range chunks {
				allParagraphs = append(allParagraphs, &repository.Paragraph{
					BookID:         book.ID,
					ChapterID:      chapter.ID,
					ChapterIndex:   chapter.ChapterIndex,
					StartParagraph: chk.StartParagraph,
					EndParagraph:   chk.EndParagraph,
					Content:        chk.Content,
				})
			}
		}
		if len(allParagraphs) > 0 {
			_ = in.repo.CreateParagraphs(ctx, allParagraphs)
		}
	}

	return book, nil
}

// SaveUpload securely saves an uploaded EPUB stream directly into Audiobookshelf structure:
// /library/<Author>/<Title>/<Title>.epub
// It performs atomic staging (.tmp file) before final rename to ensure no partial reads by ABS scanners.
func (in *Ingester) SaveUpload(ctx context.Context, authorName, title string, r io.Reader) (*repository.Book, error) {
	sanitizedAuthor := SanitizePathSegment(authorName)
	sanitizedTitle := SanitizePathSegment(title)

	targetDir := filepath.Join(in.libraryDir, sanitizedAuthor, sanitizedTitle)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("creating book directory: %w", err)
	}

	finalFilename := sanitizedTitle + ".epub"
	finalFilePath := filepath.Join(targetDir, finalFilename)
	tmpFilePath := finalFilePath + ".tmp"

	// 1. Stage file
	f, err := os.Create(tmpFilePath)
	if err != nil {
		return nil, fmt.Errorf("creating temporary upload file: %w", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmpFilePath)
		return nil, fmt.Errorf("writing upload data: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpFilePath)
		return nil, fmt.Errorf("closing upload file: %w", err)
	}

	// 2. Atomic rename to final path
	if err := os.Rename(tmpFilePath, finalFilePath); err != nil {
		os.Remove(tmpFilePath)
		return nil, fmt.Errorf("renaming temporary upload to destination: %w", err)
	}

	// 3. Ingest newly uploaded book
	relPath := filepath.Join(sanitizedAuthor, sanitizedTitle, finalFilename)
	return in.IngestFile(ctx, finalFilePath, relPath)
}

// ReparseBookChapters reads the EPUB file on disk for a book and updates the content_plain of its chapters in the repository.
func (in *Ingester) ReparseBookChapters(ctx context.Context, bookID string) error {
	book, err := in.repo.GetBookByID(ctx, bookID)
	if err != nil {
		return fmt.Errorf("fetching book: %w", err)
	}

	fullPath := filepath.Join(in.libraryDir, book.FilePath)
	reader, err := epub.Open(fullPath)
	if err != nil {
		return fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	chapters, err := reader.ExtractChapters()
	if err != nil {
		return fmt.Errorf("extracting chapters: %w", err)
	}

	existingChapters, err := in.repo.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		return fmt.Errorf("fetching existing chapters: %w", err)
	}

	chapterMap := make(map[int]*repository.Chapter, len(existingChapters))
	for _, ch := range existingChapters {
		chapterMap[ch.ChapterIndex] = ch
	}

	// Delete existing paragraphs for this book so they are re-chunked and re-indexed
	_ = in.repo.DeleteParagraphsByBookID(ctx, bookID)

	var allParagraphs []*repository.Paragraph
	for _, ch := range chapters {
		var chapterID string
		var chapterIndex int
		if existing, ok := chapterMap[ch.Index]; ok {
			if err := in.repo.UpdateChapterContent(ctx, existing.ID, ch.ContentPlain); err != nil {
				return fmt.Errorf("updating chapter %d content: %w", ch.Index, err)
			}
			chapterID = existing.ID
			chapterIndex = existing.ChapterIndex
		} else {
			chapter := &repository.Chapter{
				BookID:       book.ID,
				ChapterIndex: ch.Index,
				Title:        ch.Title,
				Summary:      "",
				ContentPlain: ch.ContentPlain,
			}
			if err := in.repo.CreateChapter(ctx, chapter); err != nil {
				return fmt.Errorf("creating chapter %d: %w", ch.Index, err)
			}
			chapterID = chapter.ID
			chapterIndex = chapter.ChapterIndex
		}

		chunks := epub.ChunkChapterParagraphs(ch.ContentPlain, 400, 1200)
		for _, chk := range chunks {
			allParagraphs = append(allParagraphs, &repository.Paragraph{
				BookID:         book.ID,
				ChapterID:      chapterID,
				ChapterIndex:   chapterIndex,
				StartParagraph: chk.StartParagraph,
				EndParagraph:   chk.EndParagraph,
				Content:        chk.Content,
			})
		}
	}
	if len(allParagraphs) > 0 {
		_ = in.repo.CreateParagraphs(ctx, allParagraphs)
	}

	return nil
}
