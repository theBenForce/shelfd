package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shelfd/shelfd/internal/audio"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
)

// UploadWorkerConfig defines configuration parameters for the upload background worker.
type UploadWorkerConfig struct {
	PollInterval time.Duration
	Hub          *events.Hub
	Logger       *slog.Logger
}

// UploadWorker processes staged EPUB files sequentially and registers them in the catalog.
type UploadWorker struct {
	repo          repository.StorageEngine
	ingester      *scanner.Ingester
	chapterWorker *Worker
	hub           *events.Hub
	pollInterval  time.Duration
	logger        *slog.Logger
	notifyCh      chan struct{}
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

// NewUploadWorker initializes a new background upload worker.
func NewUploadWorker(
	repo repository.StorageEngine,
	ingester *scanner.Ingester,
	chapterWorker *Worker,
	cfg UploadWorkerConfig,
) *UploadWorker {
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 3 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &UploadWorker{
		repo:          repo,
		ingester:      ingester,
		chapterWorker: chapterWorker,
		hub:           cfg.Hub,
		pollInterval:  pollInterval,
		logger:        logger,
		notifyCh:      make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}
}

// Trigger signals the worker to immediately process pending upload jobs.
func (w *UploadWorker) Trigger() {
	select {
	case w.notifyCh <- struct{}{}:
	default:
	}
}

// ProcessNext fetches and processes a single pending upload job.
// Returns true if a job was found and processed, or false if queue is empty.
func (w *UploadWorker) ProcessNext(ctx context.Context) (bool, error) {
	jobs, err := w.repo.GetPendingUploadJobs(ctx, 1)
	if err != nil {
		return false, fmt.Errorf("getting pending upload jobs: %w", err)
	}
	if len(jobs) == 0 {
		return false, nil
	}

	job := jobs[0]
	w.logger.Info("processing upload job", "job_id", job.ID, "filename", job.Filename)

	if err := w.repo.UpdateUploadJobStatus(ctx, job.ID, "processing", nil, nil); err != nil {
		w.logger.Error("failed to mark job processing", "job_id", job.ID, "error", err)
		return false, err
	}

	// Always ensure staged file and staged cover are removed when done with processing
	defer os.Remove(job.StagedPath)
	stagedCoverPath := filepath.Join(filepath.Dir(job.StagedPath), job.ID+".cover")
	defer os.Remove(stagedCoverPath)

	// Verify staged file exists
	if _, err := os.Stat(job.StagedPath); os.IsNotExist(err) {
		errMsg := "staged file not found on disk"
		_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
		w.logger.Error("upload job failed: staged file missing", "job_id", job.ID)
		return true, nil
	}

	fileType, _ := scanner.DetectFileType(job.Filename)
	author := "Unknown"
	title := strings.TrimSuffix(job.Filename, filepath.Ext(job.Filename))

	if fileType == "epub" {
		epubReader, err := epub.Open(job.StagedPath)
		if err != nil {
			errMsg := fmt.Sprintf("invalid epub archive: %v", err)
			_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
			w.logger.Error("upload job failed: opening epub", "job_id", job.ID, "error", err)
			return true, nil
		}
		parsed, err := epubReader.ParseBook()
		epubReader.Close()
		if err != nil {
			errMsg := fmt.Sprintf("failed to parse epub metadata: %v", err)
			_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
			w.logger.Error("upload job failed: parsing metadata", "job_id", job.ID, "error", err)
			return true, nil
		}

		if len(parsed.Authors) > 0 && strings.TrimSpace(parsed.Authors[0].Name) != "" {
			author = parsed.Authors[0].Name
		}
		if strings.TrimSpace(parsed.Title) != "" {
			title = parsed.Title
		}
	} else {
		meta, err := audio.ExtractMetadata(job.StagedPath)
		if err != nil {
			errMsg := fmt.Sprintf("failed to parse audio metadata: %v", err)
			_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
			w.logger.Error("upload job failed: parsing audio metadata", "job_id", job.ID, "error", err)
			return true, nil
		}
		if meta.Author != "" {
			author = meta.Author
		}
		if meta.Title != "" {
			title = meta.Title
		} else if meta.Album != "" {
			title = meta.Album
		}
	}

	stagedFile, err := os.Open(job.StagedPath)
	if err != nil {
		errMsg := fmt.Sprintf("failed to open staged file: %v", err)
		_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
		w.logger.Error("upload job failed: opening staged file", "job_id", job.ID, "error", err)
		return true, nil
	}
	defer stagedFile.Close()

	// If staged cover exists, copy it to library folder
	if coverBytes, err := os.ReadFile(stagedCoverPath); err == nil && len(coverBytes) > 0 {
		ext := ".jpg"
		ct := http.DetectContentType(coverBytes)
		if strings.Contains(ct, "png") {
			ext = ".png"
		} else if strings.Contains(ct, "webp") {
			ext = ".webp"
		}
		bookDir := filepath.Join(w.ingester.LibraryDir(), scanner.SanitizePathSegment(author), scanner.SanitizePathSegment(title))
		_ = os.MkdirAll(bookDir, 0755)
		_ = os.WriteFile(filepath.Join(bookDir, "cover"+ext), coverBytes, 0644)
	}

	book, err := w.ingester.SaveUpload(ctx, author, title, job.Filename, stagedFile)
	if err != nil {
		errMsg := fmt.Sprintf("failed to ingest book into library: %v", err)
		_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "failed", nil, &errMsg)
		w.logger.Error("upload job failed: ingestion error", "job_id", job.ID, "error", err)
		return true, nil
	}

	if err := w.repo.UpdateUploadJobStatus(ctx, job.ID, "completed", &book.ID, nil); err != nil {
		w.logger.Error("failed to mark job completed", "job_id", job.ID, "error", err)
		return true, err
	}

	w.logger.Info("upload job completed successfully", "job_id", job.ID, "book_id", book.ID, "title", book.Title)

	if w.hub != nil && book != nil {
		w.hub.Broadcast(events.Event{
			Type: events.EventBookAdded,
			Data: book,
		})
	}

	if w.chapterWorker != nil && fileType == "epub" {
		w.chapterWorker.Trigger()
	}

	return true, nil
}

// Start begins background processing of upload jobs.
func (w *UploadWorker) Start(ctx context.Context) {
	// Startup recovery: reset any orphaned processing jobs back to queued
	pending, err := w.repo.GetPendingUploadJobs(ctx, 100)
	if err == nil {
		for _, job := range pending {
			if job.Status == "processing" {
				w.logger.Warn("re-queuing interrupted upload job from prior run", "job_id", job.ID)
				_ = w.repo.UpdateUploadJobStatus(ctx, job.ID, "queued", nil, nil)
			}
		}
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				w.processDrain(ctx)
			case <-w.notifyCh:
				w.processDrain(ctx)
			}
		}
	}()
}

func (w *UploadWorker) processDrain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		processed, err := w.ProcessNext(ctx)
		if err != nil || !processed {
			break
		}
	}
}

// Stop terminates the background upload worker and waits for current jobs to finish.
func (w *UploadWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
}
