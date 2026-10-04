package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/shelfd/shelfd/internal/audio"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/taxonomy"
)

func naturalLess(s1, s2 string) bool {
	i, j := 0, 0
	r1, r2 := []rune(s1), []rune(s2)
	for i < len(r1) && j < len(r2) {
		if unicode.IsDigit(r1[i]) && unicode.IsDigit(r2[j]) {
			start1 := i
			for i < len(r1) && unicode.IsDigit(r1[i]) {
				i++
			}
			start2 := j
			for j < len(r2) && unicode.IsDigit(r2[j]) {
				j++
			}
			n1, _ := strconv.ParseUint(string(r1[start1:i]), 10, 64)
			n2, _ := strconv.ParseUint(string(r2[start2:j]), 10, 64)
			if n1 != n2 {
				return n1 < n2
			}
		} else {
			c1 := unicode.ToLower(r1[i])
			c2 := unicode.ToLower(r2[j])
			if c1 != c2 {
				return c1 < c2
			}
			i++
			j++
		}
	}
	return len(r1) < len(r2)
}

// TaxonomyNormalizer defines the interface for normalizing book taxonomy.
type TaxonomyNormalizer interface {
	NormalizeBookTaxonomy(ctx context.Context, bookID string, force bool) (*taxonomy.TaxonomyResult, error)
}

// Ingester coordinates parsing EPUBs and storing their metadata in the repository.
type Ingester struct {
	repo               repository.StorageEngine
	libraryDir         string
	dataDir            string
	taxonomyNormalizer TaxonomyNormalizer
}

// NewIngester creates a new Ingester instance.
func NewIngester(repo repository.StorageEngine, libraryDir, dataDir string) *Ingester {
	return &Ingester{
		repo:       repo,
		libraryDir: libraryDir,
		dataDir:    dataDir,
	}
}

// SetTaxonomyNormalizer configures a taxonomy normalizer for automatic topic/genre partitioning.
func (in *Ingester) SetTaxonomyNormalizer(normalizer TaxonomyNormalizer) {
	in.taxonomyNormalizer = normalizer
}

// SyncStatus represents the synchronization outcome for an EPUB file.
type SyncStatus int

const (
	SyncStatusUnchanged SyncStatus = iota
	SyncStatusNew
	SyncStatusModified
)

// SyncFile synchronizes an EPUB or Audiobook file with the catalog: importing if new,
// updating if modified, or skipping if unchanged to preserve vector embeddings.
func (in *Ingester) SyncFile(ctx context.Context, fullPath, relativePath string) (*repository.Book, SyncStatus, error) {
	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, SyncStatusUnchanged, fmt.Errorf("stating book file: %w", err)
	}
	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)

	fileType, isPrimary := DetectFileType(fullPath)
	if !isPrimary {
		return nil, SyncStatusUnchanged, nil
	}

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
			existingBook.FileModifiedAt = &modTime
			_ = in.repo.UpdateBook(ctx, existingBook)
		}

		if !isModified {
			_ = in.syncBookFiles(ctx, existingBook.ID, fullPath)
			return existingBook, SyncStatusUnchanged, nil
		}

		var updatedBook *repository.Book
		if fileType == "audiobook" {
			updatedBook, err = in.updateModifiedAudiobook(ctx, existingBook, fullPath, relativePath, fi)
		} else {
			updatedBook, err = in.updateModifiedBook(ctx, existingBook, fullPath, relativePath, fi)
		}
		if err != nil {
			return nil, SyncStatusUnchanged, fmt.Errorf("updating modified book: %w", err)
		}
		_ = in.syncBookFiles(ctx, updatedBook.ID, fullPath)
		if in.taxonomyNormalizer != nil {
			_, _ = in.taxonomyNormalizer.NormalizeBookTaxonomy(ctx, updatedBook.ID, false)
		}
		return updatedBook, SyncStatusModified, nil
	}

	// Check if another file in the same directory already created a Book record
	bookDirRel := filepath.Dir(relativePath)
	dirBook, err := in.repo.GetBookByDirectory(ctx, bookDirRel)
	if err == nil && dirBook != nil {
		// If the new file is an EPUB and the directory was previously tracked under an audiobook file, upgrade primary file to EPUB
		if fileType == "epub" && dirBook.BookType == "audiobook" {
			dirBook.FilePath = relativePath
			dirBook.FileSizeBytes = &sizeBytes
			dirBook.FileModifiedAt = &modTime
			dirBook.BookType = "ebook"
			updatedBook, err := in.updateModifiedBook(ctx, dirBook, fullPath, relativePath, fi)
			if err == nil {
				_ = in.syncBookFiles(ctx, updatedBook.ID, fullPath)
				if in.taxonomyNormalizer != nil {
					_, _ = in.taxonomyNormalizer.NormalizeBookTaxonomy(ctx, updatedBook.ID, false)
				}
				return updatedBook, SyncStatusModified, nil
			}
		}
		_ = in.syncBookFiles(ctx, dirBook.ID, fullPath)
		return dirBook, SyncStatusUnchanged, nil
	}

	var book *repository.Book
	if fileType == "audiobook" {
		book, err = in.importNewAudiobook(ctx, fullPath, relativePath, fi)
	} else {
		book, err = in.importNewBook(ctx, fullPath, relativePath, fi)
	}
	if err != nil {
		return nil, SyncStatusUnchanged, err
	}
	_ = in.syncBookFiles(ctx, book.ID, fullPath)
	if in.taxonomyNormalizer != nil {
		_, _ = in.taxonomyNormalizer.NormalizeBookTaxonomy(ctx, book.ID, false)
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
		ID:                       bookID,
		Title:                    parsed.Title,
		FilePath:                 relativePath,
		CoverPath:                coverRelPath,
		FileSizeBytes:            &sizeBytes,
		FileModifiedAt:           &modTime,
		Layout:                   parsed.Layout,
		RenditionSpread:          parsed.RenditionSpread,
		RenditionOrientation:     parsed.RenditionOrientation,
		PageProgressionDirection: parsed.PageProgressionDirection,
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
	for _, a := range resolveAuthors(parsed.Authors, relativePath) {
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
		seriesName := CleanSeriesName(parsed.Series.Name)
		series, err := in.repo.UpsertSeries(ctx, seriesName, nil)
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
	book.Layout = parsed.Layout
	book.RenditionSpread = parsed.RenditionSpread
	book.RenditionOrientation = parsed.RenditionOrientation
	book.PageProgressionDirection = parsed.PageProgressionDirection
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
	for _, a := range resolveAuthors(parsed.Authors, relativePath) {
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
		seriesName := CleanSeriesName(parsed.Series.Name)
		series, err := in.repo.UpsertSeries(ctx, seriesName, nil)
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

// ReparseBookChapters reads the EPUB file on disk for a book and updates its layout and chapters in the repository.
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
	}

	reader, err := epub.Open(fullPath)
	if err != nil {
		return fmt.Errorf("opening epub: %w", err)
	}
	defer reader.Close()

	if parsed, err := reader.ParseBook(); err == nil {
		book.Layout = parsed.Layout
		book.RenditionSpread = parsed.RenditionSpread
		book.RenditionOrientation = parsed.RenditionOrientation
		book.PageProgressionDirection = parsed.PageProgressionDirection
	}
	_ = in.repo.UpdateBook(ctx, book)

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
			existing.Title = ch.Title
			existing.ContentPlain = ch.ContentPlain
			existing.Href = &ch.Href
			existing.PageWidth = ch.PageWidth
			existing.PageHeight = ch.PageHeight
			existing.PageSpread = ch.PageSpread
			if err := in.repo.UpdateChapter(ctx, existing); err != nil {
				return fmt.Errorf("updating chapter %d: %w", ch.Index, err)
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
				Href:         &ch.Href,
				PageWidth:    ch.PageWidth,
				PageHeight:   ch.PageHeight,
				PageSpread:   ch.PageSpread,
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

// CleanSeriesName normalizes series names, e.g. mapping "Percy Jackson and the Olympians" to "Percy Jackson".
func CleanSeriesName(name string) string {
	clean := strings.TrimSpace(name)
	if strings.EqualFold(clean, "Percy Jackson and the Olympians") || strings.EqualFold(clean, "Percy Jackson & the Olympians") {
		return "Percy Jackson"
	}
	return clean
}

func resolveAuthors(parsedAuthors []epub.ParsedAuthor, relativePath string) []epub.ParsedAuthor {
	if len(parsedAuthors) > 0 {
		return parsedAuthors
	}
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	if len(parts) >= 2 {
		dirAuthor := strings.TrimSpace(parts[0])
		if dirAuthor != "" && !strings.EqualFold(dirAuthor, "Unknown") {
			return []epub.ParsedAuthor{{Name: dirAuthor}}
		}
	}
	return nil
}

type audioTrackCandidate struct {
	file DiscoveredFile
	meta *audio.Metadata
}

func (in *Ingester) importNewAudiobook(ctx context.Context, fullPath, relativePath string, fi os.FileInfo) (*repository.Book, error) {
	meta, err := audio.ExtractMetadata(fullPath)
	if err != nil {
		return nil, fmt.Errorf("extracting audio metadata: %w", err)
	}

	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)
	bookID := uuid.NewString()

	title := meta.Album
	if title == "" {
		title = meta.Title
	}
	if title == "" || title == meta.TrackTitle {
		parts := strings.Split(filepath.ToSlash(filepath.Dir(relativePath)), "/")
		if len(parts) >= 1 && parts[len(parts)-1] != "." && parts[len(parts)-1] != "" && !strings.EqualFold(parts[len(parts)-1], "audiobooks") {
			title = parts[len(parts)-1]
		}
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(fullPath), filepath.Ext(fullPath))
	}

	bookDirFull := filepath.Dir(fullPath)
	bookDirRel := filepath.Dir(relativePath)
	coverRelPath := in.resolveAudioCover(meta, bookDirFull, bookDirRel, bookID)

	duration := meta.DurationSeconds
	book := &repository.Book{
		ID:              bookID,
		Title:           title,
		FilePath:        relativePath,
		CoverPath:       coverRelPath,
		FileSizeBytes:   &sizeBytes,
		FileModifiedAt:  &modTime,
		BookType:        "audiobook",
		DurationSeconds: &duration,
		Layout:          "reflowable",
	}

	if meta.Description != "" {
		book.Description = &meta.Description
	}
	if meta.PublishedDate != "" {
		book.PublishedDate = &meta.PublishedDate
	}

	if err := in.repo.CreateBook(ctx, book); err != nil {
		return nil, fmt.Errorf("saving audiobook to database: %w", err)
	}

	// Link Author
	authorName := meta.Author
	if authorName == "" {
		parts := strings.Split(filepath.ToSlash(relativePath), "/")
		if len(parts) >= 2 && !strings.EqualFold(parts[0], "Unknown") {
			authorName = parts[0]
		}
	}
	if authorName != "" {
		author, err := in.repo.UpsertAuthor(ctx, authorName)
		if err == nil {
			_ = in.repo.LinkBookAuthor(ctx, book.ID, author.ID, "Author")
		}
	}

	// Link Narrator if provided
	if meta.Narrator != "" && !strings.EqualFold(meta.Narrator, authorName) {
		narrator, err := in.repo.UpsertAuthor(ctx, meta.Narrator)
		if err == nil {
			_ = in.repo.LinkBookAuthor(ctx, book.ID, narrator.ID, "Narrator")
		}
	}

	return book, nil
}

func (in *Ingester) updateModifiedAudiobook(ctx context.Context, book *repository.Book, fullPath, relativePath string, fi os.FileInfo) (*repository.Book, error) {
	meta, err := audio.ExtractMetadata(fullPath)
	if err != nil {
		return nil, fmt.Errorf("extracting audio metadata: %w", err)
	}

	sizeBytes := fi.Size()
	modTime := fi.ModTime().UTC().Truncate(time.Second)

	if meta.Album != "" {
		book.Title = meta.Album
	} else if meta.Title != "" && book.Title == "" {
		book.Title = meta.Title
	}
	book.FileSizeBytes = &sizeBytes
	book.FileModifiedAt = &modTime
	duration := meta.DurationSeconds
	book.DurationSeconds = &duration
	book.BookType = "audiobook"

	if meta.Description != "" {
		book.Description = &meta.Description
	}
	if meta.PublishedDate != "" {
		book.PublishedDate = &meta.PublishedDate
	}

	bookDirFull := filepath.Dir(fullPath)
	bookDirRel := filepath.Dir(relativePath)
	coverRelPath := in.resolveAudioCover(meta, bookDirFull, bookDirRel, book.ID)
	if coverRelPath != nil {
		book.CoverPath = coverRelPath
	}

	if err := in.repo.UpdateBook(ctx, book); err != nil {
		return nil, fmt.Errorf("updating audiobook in database: %w", err)
	}

	return book, nil
}

func (in *Ingester) resolveAudioCover(meta *audio.Metadata, bookDirFull, bookDirRel, bookID string) *string {
	// 1. Sibling cover file
	for _, candidate := range coverCandidates {
		candidatePath := filepath.Join(bookDirFull, candidate)
		if fi, err := os.Stat(candidatePath); err == nil && !fi.IsDir() && fi.Size() > 0 {
			rel := filepath.Join(bookDirRel, candidate)
			return &rel
		}
	}

	// 2. Extracted cover data from audio file
	if len(meta.CoverData) == 0 {
		return nil
	}

	ext := ".jpg"
	if meta.CoverMimeType == "image/png" {
		ext = ".png"
	}

	targetFilename := "cover" + ext
	targetPath := filepath.Join(bookDirFull, targetFilename)
	tmpPath := filepath.Join(bookDirFull, "."+targetFilename+".tmp")

	if err := os.WriteFile(tmpPath, meta.CoverData, 0644); err == nil {
		if err := os.Rename(tmpPath, targetPath); err == nil {
			rel := filepath.Join(bookDirRel, targetFilename)
			return &rel
		}
		_ = os.Remove(tmpPath)
	}

	coversDir := filepath.Join(in.dataDir, "covers")
	if err := os.MkdirAll(coversDir, 0755); err == nil {
		fallbackFilename := bookID + ext
		fallbackDiskPath := filepath.Join(coversDir, fallbackFilename)
		if err := os.WriteFile(fallbackDiskPath, meta.CoverData, 0644); err == nil {
			rel := filepath.Join("covers", fallbackFilename)
			return &rel
		}
	}

	return nil
}

func (in *Ingester) syncBookFiles(ctx context.Context, bookID, bookFullPath string) error {
	bookDirFull := filepath.Dir(bookFullPath)
	s := NewScanner(in.libraryDir)
	discovered, err := s.ScanBookDirectory(bookDirFull)
	if err != nil {
		return err
	}

	var audioTracks []audioTrackCandidate
	var nonAudioFiles []DiscoveredFile

	for _, f := range discovered {
		if f.FileType == "audiobook" {
			meta, err := audio.ExtractMetadata(f.FullPath)
			if err != nil {
				meta = &audio.Metadata{
					Title: strings.TrimSuffix(filepath.Base(f.FullPath), filepath.Ext(f.FullPath)),
				}
			}
			audioTracks = append(audioTracks, audioTrackCandidate{
				file: f,
				meta: meta,
			})
		} else {
			nonAudioFiles = append(nonAudioFiles, f)
		}
	}

	// Sort audio tracks by DiscNumber, TrackNumber, and Natural Filename
	sort.SliceStable(audioTracks, func(i, j int) bool {
		t1, t2 := audioTracks[i], audioTracks[j]
		if t1.meta.DiscNumber != t2.meta.DiscNumber {
			return t1.meta.DiscNumber < t2.meta.DiscNumber
		}
		if t1.meta.TrackNumber != t2.meta.TrackNumber && t1.meta.TrackNumber > 0 && t2.meta.TrackNumber > 0 {
			return t1.meta.TrackNumber < t2.meta.TrackNumber
		}
		return naturalLess(filepath.Base(t1.file.FullPath), filepath.Base(t2.file.FullPath))
	})

	// Sort non-audio files (EPUB first, then others by natural name)
	sort.SliceStable(nonAudioFiles, func(i, j int) bool {
		f1, f2 := nonAudioFiles[i], nonAudioFiles[j]
		if f1.FileType == "epub" && f2.FileType != "epub" {
			return true
		}
		if f1.FileType != "epub" && f2.FileType == "epub" {
			return false
		}
		return naturalLess(filepath.Base(f1.FullPath), filepath.Base(f2.FullPath))
	})

	var bookFiles []*repository.BookFile
	var audioChapters []*repository.AudioChapter
	var totalAudioDuration float64 = 0.0

	// Add non-audio companion files
	for _, f := range nonAudioFiles {
		size := f.SizeBytes
		mod := f.ModTime
		bf := &repository.BookFile{
			BookID:         bookID,
			FileType:       f.FileType,
			FilePath:       f.RelativePath,
			FileSizeBytes:  &size,
			FileModifiedAt: &mod,
		}
		bookFiles = append(bookFiles, bf)
	}

	// Add audio tracks and build combined chapter index
	for _, at := range audioTracks {
		f := at.file
		meta := at.meta
		size := f.SizeBytes
		mod := f.ModTime
		dur := meta.DurationSeconds
		bf := &repository.BookFile{
			BookID:          bookID,
			FileType:        f.FileType,
			FilePath:        f.RelativePath,
			FileSizeBytes:   &size,
			FileModifiedAt:  &mod,
			DurationSeconds: &dur,
		}
		if meta.BitrateKbps > 0 {
			br := meta.BitrateKbps
			bf.BitrateKbps = &br
		}
		bookFiles = append(bookFiles, bf)

		trackStartOffset := totalAudioDuration
		if len(meta.Chapters) > 1 {
			for _, ch := range meta.Chapters {
				audioChapters = append(audioChapters, &repository.AudioChapter{
					BookID:         bookID,
					ChapterIndex:   len(audioChapters) + 1,
					Title:          ch.Title,
					StartOffsetSec: trackStartOffset + ch.StartOffsetSec,
					DurationSec:    ch.DurationSec,
				})
			}
		} else {
			chTitle := meta.TrackTitle
			if chTitle == "" {
				chTitle = meta.Title
			}
			if chTitle == "" || chTitle == meta.Album {
				chTitle = strings.TrimSuffix(filepath.Base(f.FullPath), filepath.Ext(f.FullPath))
			}
			audioChapters = append(audioChapters, &repository.AudioChapter{
				BookID:         bookID,
				ChapterIndex:   len(audioChapters) + 1,
				Title:          chTitle,
				StartOffsetSec: trackStartOffset,
				DurationSec:    dur,
			})
		}
		totalAudioDuration += dur
	}

	// Update Book total duration and Title if appropriate
	book, err := in.repo.GetBookByID(ctx, bookID)
	if err == nil && book != nil {
		changed := false
		if totalAudioDuration > 0 && (book.DurationSeconds == nil || *book.DurationSeconds != totalAudioDuration) {
			book.DurationSeconds = &totalAudioDuration
			changed = true
		}

		if book.BookType == "audiobook" && len(audioTracks) > 0 {
			firstMeta := audioTracks[0].meta
			dirName := filepath.Base(bookDirFull)
			// If book title was set to track 1 title or placeholder, fix to album or folder name
			if book.Title == "" || (firstMeta != nil && (book.Title == firstMeta.TrackTitle || strings.HasPrefix(book.Title, "1.") || strings.HasPrefix(book.Title, "01"))) {
				if firstMeta != nil && firstMeta.Album != "" {
					book.Title = firstMeta.Album
					changed = true
				} else if dirName != "." && dirName != "" && !strings.EqualFold(dirName, "audiobooks") {
					book.Title = dirName
					changed = true
				}
			}
		}

		if changed {
			_ = in.repo.UpdateBook(ctx, book)
		}
	}

	// Update audio chapters in DB
	_ = in.repo.DeleteAudioChaptersByBookID(ctx, bookID)
	if len(audioChapters) > 0 {
		_ = in.repo.CreateAudioChapters(ctx, audioChapters)
	}

	// Update book files in DB
	_ = in.repo.DeleteBookFilesByBookID(ctx, bookID)
	if len(bookFiles) > 0 {
		return in.repo.CreateBookFiles(ctx, bookFiles)
	}
	return nil
}

