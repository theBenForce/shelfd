package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/jobs"
	"github.com/shelfd/shelfd/internal/repository"
)

// Config defines the configuration parameters for the background worker.
type Config struct {
	BatchSize    int
	PollInterval time.Duration
	JobManager   *jobs.Manager
	Logger       *slog.Logger
}

// RuntimeStatus holds live worker execution information.
type RuntimeStatus struct {
	IsBusy         bool      `json:"is_busy"`
	CurrentBook    string    `json:"current_book,omitempty"`
	CurrentChapter string    `json:"current_chapter,omitempty"`
	LastProcessed  time.Time `json:"last_processed,omitempty"`
}

// Worker processes unindexed chapters by summarizing them and storing vector embeddings.
type Worker struct {
	repo         repository.StorageEngine
	aiClient     ai.Client
	jobManager   *jobs.Manager
	batchSize    int
	pollInterval time.Duration
	logger       *slog.Logger
	notifyCh     chan struct{}
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup

	statusMu       sync.RWMutex
	currentBook    string
	currentChapter string
	isBusy         bool
	lastProcessed  time.Time
	activeJobID    string

	listenersMu sync.Mutex
	listeners   map[chan struct{}]struct{}
}

// NewWorker initializes a new background chapter indexing worker.
func NewWorker(repo repository.StorageEngine, aiClient ai.Client, cfg Config) *Worker {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Worker{
		repo:         repo,
		aiClient:     aiClient,
		jobManager:   cfg.JobManager,
		batchSize:    batchSize,
		pollInterval: pollInterval,
		logger:       logger,
		notifyCh:     make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		listeners:    make(map[chan struct{}]struct{}),
	}
}

// GetRuntimeStatus returns thread-safe current indexing state.
func (w *Worker) GetRuntimeStatus() RuntimeStatus {
	w.statusMu.RLock()
	defer w.statusMu.RUnlock()
	return RuntimeStatus{
		IsBusy:         w.isBusy,
		CurrentBook:    w.currentBook,
		CurrentChapter: w.currentChapter,
		LastProcessed:  w.lastProcessed,
	}
}

// Subscribe returns a channel that signals whenever a chapter is processed or status changes.
func (w *Worker) Subscribe() chan struct{} {
	w.listenersMu.Lock()
	defer w.listenersMu.Unlock()
	ch := make(chan struct{}, 1)
	if w.listeners == nil {
		w.listeners = make(map[chan struct{}]struct{})
	}
	w.listeners[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a registered listener channel.
func (w *Worker) Unsubscribe(ch chan struct{}) {
	w.listenersMu.Lock()
	defer w.listenersMu.Unlock()
	delete(w.listeners, ch)
	close(ch)
}

func (w *Worker) broadcast() {
	w.listenersMu.Lock()
	defer w.listenersMu.Unlock()
	for ch := range w.listeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// ProcessBatch fetches a batch of unindexed paragraphs and persists their vector embeddings.
func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	unindexedParas, err := w.repo.GetUnindexedParagraphs(ctx, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("getting unindexed paragraphs: %w", err)
	}
	if len(unindexedParas) == 0 {
		w.completeActiveJobIfAny(ctx)
		return 0, nil
	}

	defer func() {
		w.statusMu.Lock()
		w.isBusy = false
		w.currentBook = ""
		w.currentChapter = ""
		w.statusMu.Unlock()
		w.broadcast()
	}()

	pFirst := unindexedParas[0]
	bookTitle := pFirst.BookID
	if book, err := w.repo.GetBookByID(ctx, pFirst.BookID); err == nil && book != nil && book.Title != "" {
		bookTitle = book.Title
	}

	var chapterDesc string
	if len(unindexedParas) == 1 {
		chapterDesc = fmt.Sprintf("Chapter %d (p.%d-%d)", pFirst.ChapterIndex, pFirst.StartParagraph, pFirst.EndParagraph)
	} else {
		chapterDesc = fmt.Sprintf("Chapter %d (%d paragraphs)", pFirst.ChapterIndex, len(unindexedParas))
	}

	w.statusMu.Lock()
	w.isBusy = true
	w.currentBook = bookTitle
	w.currentChapter = chapterDesc
	w.statusMu.Unlock()
	w.broadcast()

	jobID := w.ensureAndReportJobProgress(ctx, bookTitle, chapterDesc)

	texts := make([]string, len(unindexedParas))
	for i, p := range unindexedParas {
		texts[i] = p.Content
	}

	embeddings, err := w.aiClient.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		w.logger.Error("failed to generate batch embeddings for paragraphs", "count", len(texts), "error", err)
		w.failActiveJob(jobID, fmt.Sprintf("Embedding generation failed: %v", err))
		return 0, err
	}

	if len(embeddings) != len(unindexedParas) {
		mismatchErr := fmt.Errorf("expected %d embeddings, got %d", len(unindexedParas), len(embeddings))
		w.failActiveJob(jobID, mismatchErr.Error())
		return 0, mismatchErr
	}

	vectorBatch := make([]repository.ParagraphVector, len(unindexedParas))
	for i, p := range unindexedParas {
		vectorBatch[i] = repository.ParagraphVector{
			ParagraphID: p.ID,
			Embedding:   embeddings[i],
		}
	}

	if err := w.repo.InsertParagraphVectors(ctx, vectorBatch); err != nil {
		w.logger.Error("failed to insert paragraph vectors", "count", len(vectorBatch), "error", err)
		w.failActiveJob(jobID, fmt.Sprintf("Saving embeddings failed: %v", err))
		return 0, err
	}

	w.statusMu.Lock()
	w.lastProcessed = time.Now()
	w.statusMu.Unlock()
	w.broadcast()

	w.updateOrCompleteAfterBatch(ctx, jobID, bookTitle, chapterDesc)

	return len(unindexedParas), nil
}

func (w *Worker) ensureAndReportJobProgress(ctx context.Context, bookTitle, chapterDesc string) string {
	if w.jobManager == nil {
		return ""
	}
	indexed := 0
	total := 0
	pct := 0.0
	if qStatus, err := w.repo.GetQueueStatus(ctx); err == nil && qStatus != nil {
		indexed = qStatus.IndexedChapters
		total = qStatus.TotalChapters
		pct = qStatus.ProgressPercent
	}

	w.statusMu.Lock()
	jobID := w.activeJobID
	w.statusMu.Unlock()

	if jobID == "" || w.jobManager.Get(jobID) == nil || !w.jobManager.Get(jobID).IsActive() {
		job := w.jobManager.Create(jobs.CreateOptions{
			Kind:    jobs.KindAIIndexing,
			Title:   "AI Catalog Indexing",
			Status:  jobs.StatusRunning,
			Total:   total,
			Message: chapterDesc,
		})
		jobID = job.ID
		w.statusMu.Lock()
		w.activeJobID = jobID
		w.statusMu.Unlock()
	}

	w.jobManager.UpdateProgress(jobID, jobs.ProgressUpdate{
		Current:         &indexed,
		Total:           &total,
		ItemTitle:       &bookTitle,
		ItemProgressPct: &pct,
		Message:         &chapterDesc,
	})
	return jobID
}

func (w *Worker) updateOrCompleteAfterBatch(ctx context.Context, jobID, bookTitle, chapterDesc string) {
	if w.jobManager == nil || jobID == "" {
		return
	}
	qStatus, err := w.repo.GetQueueStatus(ctx)
	if err != nil || qStatus == nil {
		return
	}
	if qStatus.PendingChapters <= 0 {
		msg := fmt.Sprintf("Indexed %d passages", qStatus.TotalChapters)
		w.jobManager.Complete(jobID, msg, map[string]any{
			"indexed_chapters": qStatus.IndexedChapters,
			"total_chapters":   qStatus.TotalChapters,
		})
		w.statusMu.Lock()
		if w.activeJobID == jobID {
			w.activeJobID = ""
		}
		w.statusMu.Unlock()
		return
	}
	indexed := qStatus.IndexedChapters
	total := qStatus.TotalChapters
	pct := qStatus.ProgressPercent
	w.jobManager.UpdateProgress(jobID, jobs.ProgressUpdate{
		Current:         &indexed,
		Total:           &total,
		ItemTitle:       &bookTitle,
		ItemProgressPct: &pct,
		Message:         &chapterDesc,
	})
}

func (w *Worker) completeActiveJobIfAny(ctx context.Context) {
	if w.jobManager == nil {
		return
	}
	w.statusMu.Lock()
	jobID := w.activeJobID
	w.activeJobID = ""
	w.statusMu.Unlock()
	if jobID == "" {
		return
	}
	msg := "AI catalog synced"
	var meta map[string]any
	if qStatus, err := w.repo.GetQueueStatus(ctx); err == nil && qStatus != nil {
		msg = fmt.Sprintf("Indexed %d passages", qStatus.TotalChapters)
		meta = map[string]any{
			"indexed_chapters": qStatus.IndexedChapters,
			"total_chapters":   qStatus.TotalChapters,
		}
	}
	w.jobManager.Complete(jobID, msg, meta)
}

func (w *Worker) failActiveJob(jobID, errMsg string) {
	if w.jobManager == nil || jobID == "" {
		return
	}
	w.jobManager.Fail(jobID, errMsg)
	w.statusMu.Lock()
	if w.activeJobID == jobID {
		w.activeJobID = ""
	}
	w.statusMu.Unlock()
}

// Trigger signals the worker to immediately process pending chapters.
func (w *Worker) Trigger() {
	select {
	case w.notifyCh <- struct{}{}:
	default:
	}
}

// Start begins the background processing loop.
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		maxIdleInterval := 60 * time.Second
		currentInterval := w.pollInterval
		ticker := time.NewTicker(currentInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				processed := w.processDrain(ctx)
				if processed > 0 {
					currentInterval = w.pollInterval
				} else {
					currentInterval = currentInterval * 2
					if currentInterval > maxIdleInterval {
						currentInterval = maxIdleInterval
					}
				}
				ticker.Reset(currentInterval)
			case <-w.notifyCh:
				currentInterval = w.pollInterval
				ticker.Reset(currentInterval)
				w.processDrain(ctx)
			}
		}
	}()
}

func (w *Worker) processDrain(ctx context.Context) int {
	total := 0
	for {
		if ctx.Err() != nil {
			return total
		}
		count, err := w.ProcessBatch(ctx)
		if err != nil || count == 0 {
			break
		}
		total += count
	}
	return total
}

// Stop stops the background worker and waits for active routines to finish.
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
}
