package worker_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

func createTestEPUB(title, author string) []byte {
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
    <dc:creator>` + author + `</dc:creator>
    <dc:language>en</dc:language>
    <dc:identifier id="id">urn:uuid:test-sample-id</dc:identifier>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="ch1"/>
  </spine>
</package>`
	w, _ = zw.Create("content.opf")
	w.Write([]byte(opfXML))

	w, _ = zw.Create("ch1.xhtml")
	w.Write([]byte(`<html><head><title>Chapter 1</title></head><body><h1>Chapter 1</h1><p>Test text.</p></body></html>`))

	zw.Close()
	return buf.Bytes()
}

func TestUploadWorker_ProcessNext_Success(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "library")
	dataDir := filepath.Join(tempDir, "data")
	uploadsDir := filepath.Join(dataDir, "uploads")
	_ = os.MkdirAll(uploadsDir, 0755)

	ingester := scanner.NewIngester(repo, libraryDir, dataDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{
		Logger: logger,
	})

	// Stage test epub
	stagedPath := filepath.Join(uploadsDir, "test-job-1.epub")
	epubBytes := createTestEPUB("Children of Dune", "Frank Herbert")
	if err := os.WriteFile(stagedPath, epubBytes, 0644); err != nil {
		t.Fatalf("failed to write staged file: %v", err)
	}

	job := &repository.UploadJob{
		ID:         "test-job-1",
		Filename:   "Children of Dune.epub",
		StagedPath: stagedPath,
		Status:     "queued",
	}
	if err := repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("create upload job: %v", err)
	}

	// Process
	processed, err := w.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext failed: %v", err)
	}
	if !processed {
		t.Fatalf("expected job to be processed")
	}

	// Verify job status
	updatedJob, err := repo.GetUploadJob(ctx, "test-job-1")
	if err != nil {
		t.Fatalf("get upload job: %v", err)
	}
	if updatedJob.Status != "completed" {
		t.Fatalf("expected status completed, got %s (error: %v)", updatedJob.Status, updatedJob.ErrorMessage)
	}
	if updatedJob.BookID == nil {
		t.Fatalf("expected book_id to be set")
	}

	// Verify staged file cleaned up
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatalf("expected staged file to be removed")
	}

	// Verify book exists in library
	book, err := repo.GetBookByID(ctx, *updatedJob.BookID)
	if err != nil {
		t.Fatalf("get book by id: %v", err)
	}
	if book.Title != "Children of Dune" {
		t.Fatalf("expected title 'Children of Dune', got %s", book.Title)
	}
	destPath := filepath.Join(libraryDir, book.FilePath)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatalf("expected book file at %s", destPath)
	}
}

func TestUploadWorker_ProcessNext_InvalidEPUB(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "library")
	dataDir := filepath.Join(tempDir, "data")
	uploadsDir := filepath.Join(dataDir, "uploads")
	_ = os.MkdirAll(uploadsDir, 0755)

	ingester := scanner.NewIngester(repo, libraryDir, dataDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{
		Logger: logger,
	})

	stagedPath := filepath.Join(uploadsDir, "corrupted.epub")
	if err := os.WriteFile(stagedPath, []byte("not a zip file"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	job := &repository.UploadJob{
		ID:         "bad-job-1",
		Filename:   "bad.epub",
		StagedPath: stagedPath,
		Status:     "queued",
	}
	if err := repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("create upload job: %v", err)
	}

	processed, err := w.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext failed: %v", err)
	}
	if !processed {
		t.Fatalf("expected job to be processed")
	}

	updatedJob, err := repo.GetUploadJob(ctx, "bad-job-1")
	if err != nil {
		t.Fatalf("get upload job: %v", err)
	}
	if updatedJob.Status != "failed" {
		t.Fatalf("expected status failed, got %s", updatedJob.Status)
	}
	if updatedJob.ErrorMessage == nil || *updatedJob.ErrorMessage == "" {
		t.Fatalf("expected non-empty error message")
	}

	// Verify staged file cleaned up
	if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
		t.Fatalf("expected staged file to be removed even on failure")
	}
}

func TestUploadWorker_ProcessNext_MissingStagedFile(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "library")
	dataDir := filepath.Join(tempDir, "data")

	ingester := scanner.NewIngester(repo, libraryDir, dataDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{
		Logger: logger,
	})

	job := &repository.UploadJob{
		ID:         "missing-file-job",
		Filename:   "missing.epub",
		StagedPath: filepath.Join(dataDir, "uploads", "nonexistent.epub"),
		Status:     "queued",
	}
	if err := repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("create upload job: %v", err)
	}

	processed, err := w.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext failed: %v", err)
	}
	if !processed {
		t.Fatalf("expected job to be processed")
	}

	updatedJob, _ := repo.GetUploadJob(ctx, "missing-file-job")
	if updatedJob.Status != "failed" {
		t.Fatalf("expected status failed, got %s", updatedJob.Status)
	}
}

func TestUploadWorker_EmptyQueue(t *testing.T) {
	ctx := context.Background()
	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	tempDir := t.TempDir()
	ingester := scanner.NewIngester(repo, filepath.Join(tempDir, "library"), filepath.Join(tempDir, "data"))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{
		Logger: logger,
	})

	processed, err := w.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext failed: %v", err)
	}
	if processed {
		t.Fatalf("expected false on empty queue, got true")
	}
}

func TestUploadWorker_StartupRecoveryAndTrigger(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, repo := setupTestDB(t)
	defer db.Close()
	defer repo.Close()

	tempDir := t.TempDir()
	libraryDir := filepath.Join(tempDir, "library")
	dataDir := filepath.Join(tempDir, "data")
	uploadsDir := filepath.Join(dataDir, "uploads")
	_ = os.MkdirAll(uploadsDir, 0755)

	ingester := scanner.NewIngester(repo, libraryDir, dataDir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{
		PollInterval: 500 * time.Millisecond,
		Logger:       logger,
	})

	// Simulate an orphaned job from a prior crash
	stagedPath := filepath.Join(uploadsDir, "recovered.epub")
	epubBytes := createTestEPUB("God Emperor of Dune", "Frank Herbert")
	if err := os.WriteFile(stagedPath, epubBytes, 0644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	job := &repository.UploadJob{
		ID:         "crashed-job-1",
		Filename:   "God Emperor of Dune.epub",
		StagedPath: stagedPath,
		Status:     "processing", // orphaned in processing
	}
	if err := repo.CreateUploadJob(ctx, job); err != nil {
		t.Fatalf("create upload job: %v", err)
	}

	w.Start(ctx)

	// Wait up to 2 seconds for worker to process recovered job
	deadline := time.Now().Add(2 * time.Second)
	success := false
	for time.Now().Before(deadline) {
		j, err := repo.GetUploadJob(ctx, "crashed-job-1")
		if err == nil && j.Status == "completed" {
			success = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !success {
		t.Fatalf("expected crashed job to be recovered and completed")
	}

	w.Stop()
}
