package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

// Config defines the configuration parameters for the background worker.
type Config struct {
	BatchSize    int
	PollInterval time.Duration
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

	w.statusMu.Lock()
	w.isBusy = true
	w.currentBook = bookTitle
	if len(unindexedParas) == 1 {
		w.currentChapter = fmt.Sprintf("Chapter %d (p.%d-%d)", pFirst.ChapterIndex, pFirst.StartParagraph, pFirst.EndParagraph)
	} else {
		w.currentChapter = fmt.Sprintf("Chapter %d (%d paragraphs)", pFirst.ChapterIndex, len(unindexedParas))
	}
	w.statusMu.Unlock()
	w.broadcast()

	texts := make([]string, len(unindexedParas))
	for i, p := range unindexedParas {
		texts[i] = p.Content
	}

	embeddings, err := w.aiClient.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		w.logger.Error("failed to generate batch embeddings for paragraphs", "count", len(texts), "error", err)
		return 0, err
	}

	if len(embeddings) != len(unindexedParas) {
		return 0, fmt.Errorf("expected %d embeddings, got %d", len(unindexedParas), len(embeddings))
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
		return 0, err
	}

	w.statusMu.Lock()
	w.lastProcessed = time.Now()
	w.statusMu.Unlock()
	w.broadcast()

	return len(unindexedParas), nil
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

func (w *Worker) processDrain(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		count, err := w.ProcessBatch(ctx)
		if err != nil || count == 0 {
			break
		}
	}
}

// Stop stops the background worker and waits for active routines to finish.
func (w *Worker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
}
