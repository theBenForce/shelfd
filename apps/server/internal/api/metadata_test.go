package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/metadata"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

func createTestEPUBFile(t *testing.T, targetPath, title, author string) {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	m, _ := zw.Create("mimetype")
	m.Write([]byte("application/epub+zip"))

	c, _ := zw.Create("META-INF/container.xml")
	c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	opf, _ := zw.Create("OEBPS/content.opf")
	opf.Write([]byte(fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>%s</dc:title>
  <dc:creator>%s</dc:creator>
</metadata>
<manifest>
  <item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine>
  <itemref idref="c1"/>
</spine>
</package>`, title, author)))

	ch, _ := zw.Create("OEBPS/chapter1.xhtml")
	ch.Write([]byte(`<!DOCTYPE html><html><body><h1>Chapter 1</h1><p>Test</p></body></html>`))

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to write zip: %v", err)
	}

	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test epub: %v", err)
	}
}

func TestAPI_Metadata_Search(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)

	// Missing params -> 400
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/search", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty search params, got %d", rec.Code)
	}

	// Valid query
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/metadata/search?q=dune", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var results []metadata.SearchResult
	if err := json.NewDecoder(rec2.Body).Decode(&results); err != nil {
		t.Fatalf("failed to decode search results: %v", err)
	}
}

func TestAPI_Metadata_FetchBookCover(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	// Mock image server
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
	}))
	defer imgServer.Close()

	// Create test EPUB in library
	authorDir := filepath.Join(f.libDir, "Frank Herbert")
	bookDir := filepath.Join(authorDir, "Dune")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("failed to create book directory: %v", err)
	}
	epubPath := filepath.Join(bookDir, "Dune.epub")
	createTestEPUBFile(t, epubPath, "Dune", "Frank Herbert")

	relPath, err := filepath.Rel(f.libDir, epubPath)
	if err != nil {
		t.Fatalf("relPath: %v", err)
	}

	book := &repository.Book{
		ID:        ulid.New(),
		Title:     "Dune",
		FilePath:  relPath,
		CreatedAt: time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"cover_url": imgServer.URL,
	})

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/books/%s/cover/fetch", book.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify cover file exists adjacent to EPUB
	expectedCoverPath := filepath.Join(bookDir, "cover.jpg")
	if _, err := os.Stat(expectedCoverPath); os.IsNotExist(err) {
		t.Fatalf("expected cover to be saved at %s", expectedCoverPath)
	}

	// Verify book in repo updated
	updatedBook, err := f.repo.GetBookByID(ctx, book.ID)
	if err != nil {
		t.Fatalf("get updated book: %v", err)
	}
	if updatedBook.CoverPath == nil || *updatedBook.CoverPath == "" {
		t.Fatalf("expected book CoverPath to be populated")
	}
}

func TestAPI_Metadata_FetchJobCover(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	// Mock image server
	imgServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
	}))
	defer imgServer.Close()

	uploadsDir := filepath.Join(f.dataDir, "uploads")
	_ = os.MkdirAll(uploadsDir, 0755)
	jobID := ulid.New()
	stagedPath := filepath.Join(uploadsDir, jobID+".epub")
	createTestEPUBFile(t, stagedPath, "Staged Book", "Author")

	job := &repository.UploadJob{
		ID:         jobID,
		Filename:   "test.epub",
		StagedPath: stagedPath,
		Status:     "staged",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := f.repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("create upload job: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"cover_url": imgServer.URL,
	})

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/books/upload/jobs/%s/cover/fetch", jobID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify job in repo has cover
	updatedJob, err := f.repo.GetUploadJob(ctx, jobID)
	if err != nil {
		t.Fatalf("get updated job: %v", err)
	}
	if !updatedJob.HasCover {
		t.Fatalf("expected job HasCover to be true")
	}
}
