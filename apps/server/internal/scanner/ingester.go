package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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

// SyncStatus represents the synchronization outcome for an EPUB file.
type SyncStatus int

const (
	SyncStatusUnchanged SyncStatus = iota
	SyncStatusNew
	SyncStatusModified
)

// SyncFile synchronizes an EPUB file with the catalog: importing if new,
// updating if modified, or skipping if unchanged to preserve vector embeddings.
func (in *Ingester) SyncFile(ctx context.Context, fullPath, relativePath string) (*repository.Book, SyncStatus, error) {
	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, SyncStatusUnchanged, fmt.Errorf("stating book file: %w", err)
	}
	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)

	existingBook, err := in.repo.GetBookByFilePath(ctx, relativePath)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, SyncStatusUnchanged, fmt.Errorf("checking existing book: %w", err)
	}

	if existingBook != nil {
		isModified := false
		if existingBook.FileSizeBytes == nil || *existingBook.FileSizeBytes != sizeBytes {
			isModified = true
		} else if existingBook.FileModifiedAt != nil {
			if existingBook.FileModifiedAt.Unix() != fi.ModTime().Unix() {
				isModified = true
			}
		} else {
			// Legacy record: file size matches but FileModifiedAt is not recorded.
			// Backfill FileModifiedAt without reparsing or invalidating vector embeddings.
			existingBook.FileModifiedAt = &modTime
			_ = in.repo.UpdateBook(ctx, existingBook)
		}

		if !isModified {
			return existingBook, SyncStatusUnchanged, nil
		}

		updatedBook, err := in.updateModifiedBook(ctx, existingBook, fullPath, relativePath, fi)
		if err != nil {
			return nil, SyncStatusUnchanged, fmt.Errorf("updating modified book: %w", err)
		}
		return updatedBook, SyncStatusModified, nil
	}

	book, err := in.importNewBook(ctx, fullPath, relativePath, fi)
	if err != nil {
		return nil, SyncStatusUnchanged, err
	}
	return book, SyncStatusNew, nil
}

// IngestFile parses an EPUB file and registers its metadata, chapters, and cover in storage.
// If the book already exists and is unmodified on disk, it returns the existing book
// without re-chunking or invalidating vector embeddings.
func (in *Ingester) IngestFile(ctx context.Context, fullPath, relativePath string) (*repository.Book, error) {
	book, _, err := in.SyncFile(ctx, fullPath, relativePath)
	return book, err
}

func (in *Ingester) importNewBook(ctx context.Context, fullPath, relativePath string, fi os.FileInfo) (*repository.Book, error) {
	reader, err := epub.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	parsed, err := reader.ParseBook()
	if err != nil {
		return nil, fmt.Errorf("parsing book metadata: %w", err)
	}

	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)
	bookID := uuid.NewString()

	bookDirFull := filepath.Dir(fullPath)
	bookDirRel := filepath.Dir(relativePath)
	coverRelPath := in.resolveCover(reader, bookDirFull, bookDirRel, bookID)

	book := &repository.Book{
		ID:             bookID,
		Title:          parsed.Title,
		FilePath:       relativePath,
		CoverPath:      coverRelPath,
		FileSizeBytes:  &sizeBytes,
		FileModifiedAt: &modTime,
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
	if err := in.reparseChaptersFromReader(ctx, book.ID, reader); err != nil {
		return nil, fmt.Errorf("registering chapters and paragraphs: %w", err)
	}

	return book, nil
}

func (in *Ingester) updateModifiedBook(ctx context.Context, book *repository.Book, fullPath, relativePath string, fi os.FileInfo) (*repository.Book, error) {
	reader, err := epub.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	parsed, err := reader.ParseBook()
	if err != nil {
		return nil, fmt.Errorf("parsing book metadata: %w", err)
	}

	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)

	book.Title = parsed.Title
	book.FileSizeBytes = &sizeBytes
	book.FileModifiedAt = &modTime
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

	bookDirFull := filepath.Dir(fullPath)
	bookDirRel := filepath.Dir(relativePath)
	coverRelPath := in.resolveCover(reader, bookDirFull, bookDirRel, book.ID)
	if coverRelPath != nil {
		book.CoverPath = coverRelPath
	}

	if err := in.repo.UpdateBook(ctx, book); err != nil {
		return nil, fmt.Errorf("updating book in database: %w", err)
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

	if err := in.reparseChaptersFromReader(ctx, book.ID, reader); err != nil {
		return nil, fmt.Errorf("reparsing chapters: %w", err)
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
	if fi, err := os.Stat(fullPath); err == nil {
		modTime := fi.ModTime().UTC().Truncate(time.Second)
		sizeBytes := fi.Size()
		book.FileModifiedAt = &modTime
		book.FileSizeBytes = &sizeBytes
		_ = in.repo.UpdateBook(ctx, book)
	}

	reader, err := epub.Open(fullPath)
	if err != nil {
		return fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	return in.reparseChaptersFromReader(ctx, bookID, reader)
}

func (in *Ingester) reparseChaptersFromReader(ctx context.Context, bookID string, reader *epub.Reader) error {
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
				BookID:       bookID,
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
				BookID:         bookID,
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

var coverCandidates = []string{"cover.jpg", "cover.jpeg", "cover.png", "cover.webp"}

// resolveCover finds an existing sibling cover file or extracts from EPUB and saves alongside it in /library.
// If /library is read-only, it falls back to caching in /data/covers/.
func (in *Ingester) resolveCover(reader *epub.Reader, bookDirFull, bookDirRel, bookID string) *string {
	// 1. Check if a sibling cover file already exists in the book folder (Audiobookshelf standard)
	for _, candidate := range coverCandidates {
		candidatePath := filepath.Join(bookDirFull, candidate)
		if fi, err := os.Stat(candidatePath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			rel := filepath.Join(bookDirRel, candidate)
			return &rel
		}
	}

	// 2. Extract cover image from EPUB
	coverData, ext, err := reader.ExtractCoverImage()
	if err != nil || len(coverData) == 0 {
		return nil
	}

	if ext == "" {
		ext = ".jpg"
	}

	// 3. Attempt writing directly alongside the EPUB in /library
	targetFilename := "cover" + ext
	targetPath := filepath.Join(bookDirFull, targetFilename)
	tmpPath := filepath.Join(bookDirFull, "."+targetFilename+".tmp")

	if err := os.WriteFile(tmpPath, coverData, 0644); err == nil {
		if err := os.Rename(tmpPath, targetPath); err == nil {
			rel := filepath.Join(bookDirRel, targetFilename)
			return &rel
		}
		_ = os.Remove(tmpPath)
	}

	// 4. Fallback if /library is read-only: save to /data/covers/<book_id>.<ext>
	coversDir := filepath.Join(in.dataDir, "covers")
	if err := os.MkdirAll(coversDir, 0755); err == nil {
		fallbackFilename := bookID + ext
		fallbackDiskPath := filepath.Join(coversDir, fallbackFilename)
		if err := os.WriteFile(fallbackDiskPath, coverData, 0644); err == nil {
			rel := filepath.Join("covers", fallbackFilename)
			return &rel
		}
	}

	return nil
}
