package scanner_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
)

func createSampleEPUB(title, author, genre, seriesName string, seq float64) []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	mimetypeHeader := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	w, _ := zw.CreateHeader(mimetypeHeader)
	w.Write([]byte("application/epub+zip"))

	containerXML := `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`
	w, _ = zw.Create("META-INF/container.xml")
	w.Write([]byte(containerXML))

	opfXML := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
    <dc:creator id="aut">` + author + `</dc:creator>
    <dc:subject>` + genre + `</dc:subject>
    <dc:language>en</dc:language>
    <dc:identifier id="id">urn:uuid:test-sample-id</dc:identifier>
    <meta property="belongs-to-collection" id="c01">` + seriesName + `</meta>
    <meta refines="#c01" property="group-position">` + "1.0" + `</meta>
  </metadata>
  <manifest>
    <item id="cov" href="cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`
	w, _ = zw.Create("content.opf")
	w.Write([]byte(opfXML))

	w, _ = zw.Create("cover.jpg")
	w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})

	w, _ = zw.Create("ch1.xhtml")
	w.Write([]byte(`<html><head><title>Chapter 1</title></head><body><h1>Chapter 1</h1><p>Test text.</p></body></html>`))

	zw.Close()
	return buf.Bytes()
}

func TestScannerAudiobookshelfCoexistence(t *testing.T) {
	tempLib := t.TempDir()

	// Create an Audiobookshelf structure: /library/Frank Herbert/Dune/
	bookDir := filepath.Join(tempLib, "Frank Herbert", "Dune")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// 1. Place sidecar files that must NEVER be modified or touched
	absMetadataContent := []byte(`{"title": "Dune", "narrator": "George Guidall"}`)
	descContent := []byte("A classic sci-fi epic.")
	audioContent := []byte("fake audio bytes")
	coverContent := []byte("original abs cover")

	os.WriteFile(filepath.Join(bookDir, ".metadata.json"), absMetadataContent, 0644)
	os.WriteFile(filepath.Join(bookDir, "desc.txt"), descContent, 0644)
	os.WriteFile(filepath.Join(bookDir, "track01.m4b"), audioContent, 0644)
	os.WriteFile(filepath.Join(bookDir, "cover.jpg"), coverContent, 0644)

	// 2. Place an EPUB file
	epubBytes := createSampleEPUB("Dune", "Frank Herbert", "Sci-Fi", "Dune Chronicles", 1.0)
	epubPath := filepath.Join(bookDir, "Dune.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	// 3. Scan library
	s := scanner.NewScanner(tempLib)
	discovered, err := s.Scan()
	if err != nil {
		t.Fatalf("scanner.Scan() error: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("expected exactly 1 discovered epub, got %d: %v", len(discovered), discovered)
	}

	expectedRel := filepath.Join("Frank Herbert", "Dune", "Dune.epub")
	if discovered[0].RelativePath != expectedRel {
		t.Errorf("expected relative path %s, got %s", expectedRel, discovered[0].RelativePath)
	}

	// 4. Invariant Assertion: sidecar files must remain strictly unmodified
	metaRead, _ := os.ReadFile(filepath.Join(bookDir, ".metadata.json"))
	if !bytes.Equal(metaRead, absMetadataContent) {
		t.Errorf(".metadata.json was modified!")
	}

	descRead, _ := os.ReadFile(filepath.Join(bookDir, "desc.txt"))
	if !bytes.Equal(descRead, descContent) {
		t.Errorf("desc.txt was modified!")
	}

	audioRead, _ := os.ReadFile(filepath.Join(bookDir, "track01.m4b"))
	if !bytes.Equal(audioRead, audioContent) {
		t.Errorf("track01.m4b was modified!")
	}

	coverRead, _ := os.ReadFile(filepath.Join(bookDir, "cover.jpg"))
	if !bytes.Equal(coverRead, coverContent) {
		t.Errorf("cover.jpg was modified!")
	}
}

func TestIngesterAndUpload(t *testing.T) {
	ctx := context.Background()
	tempLib := t.TempDir()
	tempData := t.TempDir()

	// Initialize database
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	ingester := scanner.NewIngester(repo, tempLib, tempData)

	// 1. Ingest existing EPUB
	epubBytes := createSampleEPUB("Dune", "Frank Herbert", "Science Fiction", "Dune Chronicles", 1.0)
	bookDir := filepath.Join(tempLib, "Frank Herbert", "Dune")
	os.MkdirAll(bookDir, 0755)
	epubPath := filepath.Join(bookDir, "Dune.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	relPath := filepath.Join("Frank Herbert", "Dune", "Dune.epub")
	book, err := ingester.IngestFile(ctx, epubPath, relPath)
	if err != nil {
		t.Fatalf("ingester.IngestFile failed: %v", err)
	}

	if book.Title != "Dune" {
		t.Errorf("expected title Dune, got %s", book.Title)
	}
	if book.CoverPath == nil || *book.CoverPath == "" {
		t.Errorf("expected cached cover path to be set")
	}

	// Verify cover was written alongside the EPUB in tempLib/Frank Herbert/Dune/cover.jpg
	expectedCoverPath := filepath.Join(tempLib, *book.CoverPath)
	if _, err := os.Stat(expectedCoverPath); err != nil {
		t.Errorf("cover file does not exist at %s: %v", expectedCoverPath, err)
	}
	expectedCoverRel := filepath.Join("Frank Herbert", "Dune", "cover.jpg")
	if *book.CoverPath != expectedCoverRel {
		t.Errorf("expected cover path %s, got %s", expectedCoverRel, *book.CoverPath)
	}

	// Verify relational entities
	authors, err := repo.GetBookAuthors(ctx, book.ID)
	if err != nil || len(authors) != 1 || authors[0].Name != "Frank Herbert" {
		t.Errorf("unexpected authors: %v", authors)
	}

	genres, err := repo.GetBookGenres(ctx, book.ID)
	if err != nil || len(genres) != 1 || genres[0].Name != "Science Fiction" {
		t.Errorf("unexpected genres: %v", genres)
	}

	seriesList, err := repo.GetBookSeries(ctx, book.ID)
	if err != nil || len(seriesList) != 1 || seriesList[0].Name != "Dune Chronicles" {
		t.Errorf("unexpected series: %v", seriesList)
	}

	chapters, err := repo.GetChaptersByBookID(ctx, book.ID)
	if err != nil || len(chapters) != 1 {
		t.Errorf("expected 1 chapter, got %v", chapters)
	}

	// 2. Test Atomic Upload
	uploadEPUBBytes := createSampleEPUB("Neuromancer", "William Gibson", "Cyberpunk", "Sprawl", 1.0)
	uploadedBook, err := ingester.SaveUpload(ctx, "William Gibson", "Neuromancer", bytes.NewReader(uploadEPUBBytes))
	if err != nil {
		t.Fatalf("SaveUpload failed: %v", err)
	}

	// Assert uploaded book was stored strictly under /library/<Author>/<Title>/<Title>.epub
	expectedUploadedRel := filepath.Join("William Gibson", "Neuromancer", "Neuromancer.epub")
	if uploadedBook.FilePath != expectedUploadedRel {
		t.Errorf("expected uploaded path %s, got %s", expectedUploadedRel, uploadedBook.FilePath)
	}

	expectedUploadDiskPath := filepath.Join(tempLib, expectedUploadedRel)
	if _, err := os.Stat(expectedUploadDiskPath); err != nil {
		t.Errorf("uploaded file missing on disk: %v", err)
	}

	// Verify uploaded book has cover.jpg created alongside it
	if uploadedBook.CoverPath == nil || *uploadedBook.CoverPath == "" {
		t.Errorf("expected uploaded book cover path to be set")
	}
	expectedUploadedCover := filepath.Join(tempLib, "William Gibson", "Neuromancer", "cover.jpg")
	if _, err := os.Stat(expectedUploadedCover); err != nil {
		t.Errorf("uploaded cover missing on disk: %v", err)
	}

	// 3. Test ReparseBookChapters
	// Modify EPUB content on disk
	updatedEPUBBytes := createSampleEPUB("Dune", "Frank Herbert", "Science Fiction", "Dune Chronicles", 1.0)
	// Replace ch1.xhtml inside updatedEPUB with formatted markdown content
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	zr, err := zip.NewReader(bytes.NewReader(updatedEPUBBytes), int64(len(updatedEPUBBytes)))
	if err != nil {
		t.Fatalf("reading epub zip: %v", err)
	}
	for _, f := range zr.File {
		w, _ := zw.Create(f.Name)
		if f.Name == "ch1.xhtml" {
			w.Write([]byte(`<html><body><h2>Updated Heading</h2><p>Updated *italic* text.</p></body></html>`))
		} else {
			rc, _ := f.Open()
			bufCopy := new(bytes.Buffer)
			bufCopy.ReadFrom(rc)
			rc.Close()
			w.Write(bufCopy.Bytes())
		}
	}
	zw.Close()
	os.WriteFile(epubPath, buf.Bytes(), 0644)

	// Call ReparseBookChapters
	if err := ingester.ReparseBookChapters(ctx, book.ID); err != nil {
		t.Fatalf("ReparseBookChapters failed: %v", err)
	}

	// Verify chapter content was updated in repo
	updatedChapters, err := repo.GetChaptersByBookID(ctx, book.ID)
	if err != nil || len(updatedChapters) == 0 {
		t.Fatalf("GetChaptersByBookID failed: %v", err)
	}
	if !strings.Contains(updatedChapters[0].ContentPlain, "## Updated Heading") {
		t.Errorf("expected content to contain ## Updated Heading, got %s", updatedChapters[0].ContentPlain)
	}

	// Also verify IngestFile on existing file reparses without error
	reingested, err := ingester.IngestFile(ctx, epubPath, relPath)
	if err != nil {
		t.Fatalf("IngestFile on existing book failed: %v", err)
	}
	if reingested.ID != book.ID {
		t.Errorf("expected same book ID %s, got %s", book.ID, reingested.ID)
	}
}

func TestIngesterSiblingCoverPreserved(t *testing.T) {
	ctx := context.Background()
	tempLib := t.TempDir()
	tempData := t.TempDir()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	ingester := scanner.NewIngester(repo, tempLib, tempData)

	// Pre-create book folder with a custom curated cover.jpg
	bookDir := filepath.Join(tempLib, "Andy Weir", "Project Hail Mary")
	os.MkdirAll(bookDir, 0755)
	customCoverContent := []byte("custom audiobookshelf curated cover")
	coverPath := filepath.Join(bookDir, "cover.jpg")
	os.WriteFile(coverPath, customCoverContent, 0644)

	// Create and ingest EPUB
	epubBytes := createSampleEPUB("Project Hail Mary", "Andy Weir", "Hard Sci-Fi", "", 0)
	epubPath := filepath.Join(bookDir, "Project Hail Mary.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	relPath := filepath.Join("Andy Weir", "Project Hail Mary", "Project Hail Mary.epub")
	book, err := ingester.IngestFile(ctx, epubPath, relPath)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}

	// Invariant 1: CoverPath points to existing sibling cover
	expectedRel := filepath.Join("Andy Weir", "Project Hail Mary", "cover.jpg")
	if book.CoverPath == nil || *book.CoverPath != expectedRel {
		t.Errorf("expected cover path %s, got %v", expectedRel, book.CoverPath)
	}

	// Invariant 2: Custom cover content was NEVER overwritten
	data, err := os.ReadFile(coverPath)
	if err != nil || !bytes.Equal(data, customCoverContent) {
		t.Errorf("cover.jpg was mutated or unreadable: %v", err)
	}
}

func TestIngesterReadOnlyLibraryFallback(t *testing.T) {
	ctx := context.Background()
	tempLib := t.TempDir()
	tempData := t.TempDir()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(ctx, db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	defer repo.Close()

	ingester := scanner.NewIngester(repo, tempLib, tempData)

	bookDir := filepath.Join(tempLib, "Ted Chiang", "Exhalation")
	os.MkdirAll(bookDir, 0755)

	epubBytes := createSampleEPUB("Exhalation", "Ted Chiang", "Sci-Fi", "", 0)
	epubPath := filepath.Join(bookDir, "Exhalation.epub")
	os.WriteFile(epubPath, epubBytes, 0644)

	// Simulate read-only directory
	if err := os.Chmod(bookDir, 0555); err != nil {
		t.Skip("chmod not supported in environment")
	}
	defer os.Chmod(bookDir, 0755)

	relPath := filepath.Join("Ted Chiang", "Exhalation", "Exhalation.epub")
	book, err := ingester.IngestFile(ctx, epubPath, relPath)
	if err != nil {
		t.Fatalf("IngestFile failed on read-only directory: %v", err)
	}

	if book.CoverPath == nil {
		t.Fatalf("expected CoverPath to be set via fallback")
	}

	// Verify cover was saved to dataDir fallback
	expectedFallbackPath := filepath.Join(tempData, *book.CoverPath)
	if _, err := os.Stat(expectedFallbackPath); err != nil {
		t.Errorf("fallback cover missing at %s: %v", expectedFallbackPath, err)
	}
}
