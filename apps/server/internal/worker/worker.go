package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

// ProcessBatch fetches a batch of unindexed items and persists embeddings or summaries.
// It prioritizes embedding unindexed paragraphs (for fast vector search),
// and secondarily generates chapter summaries when paragraphs are indexed.
func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	// 1. Process unindexed paragraphs first (fast vector embedding)
	unindexedParas, err := w.repo.GetUnindexedParagraphs(ctx, w.batchSize)
	if err == nil && len(unindexedParas) > 0 {
		defer func() {
			w.statusMu.Lock()
			w.isBusy = false
			w.currentBook = ""
			w.currentChapter = ""
			w.statusMu.Unlock()
			w.broadcast()
		}()

		bookTitles := make(map[string]string)
		processed := 0
		for _, p := range unindexedParas {
			if err := ctx.Err(); err != nil {
				return processed, err
			}

			bookTitle, ok := bookTitles[p.BookID]
			if !ok {
				if book, err := w.repo.GetBookByID(ctx, p.BookID); err == nil && book != nil && book.Title != "" {
					bookTitle = book.Title
				} else {
					bookTitle = p.BookID
				}
				bookTitles[p.BookID] = bookTitle
			}

			w.statusMu.Lock()
			w.isBusy = true
			w.currentBook = bookTitle
			w.currentChapter = fmt.Sprintf("Chapter %d (p.%d-%d)", p.ChapterIndex, p.StartParagraph, p.EndParagraph)
			w.statusMu.Unlock()
			w.broadcast()

			embedding, err := w.aiClient.GenerateEmbedding(ctx, p.Content)
			if err != nil {
				w.logger.Error("failed to generate embedding for paragraph", "paragraph_id", p.ID, "error", err)
				continue
			}

			if err := w.repo.InsertParagraphVector(ctx, p.ID, embedding); err != nil {
				w.logger.Error("failed to insert paragraph vector", "paragraph_id", p.ID, "error", err)
				continue
			}

			w.statusMu.Lock()
			w.lastProcessed = time.Now()
			w.statusMu.Unlock()
			w.broadcast()

			processed++
		}
		return processed, nil
	}

	// 2. Process unindexed chapters (generative summarization)
	chapters, err := w.repo.GetUnindexedChapters(ctx, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("getting unindexed chapters: %w", err)
	}
	if len(chapters) == 0 {
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

	bookTitles := make(map[string]string)
	processed := 0
	for _, ch := range chapters {
		if err := ctx.Err(); err != nil {
			return processed, err
		}

		title := ""
		if ch.Title != nil {
			title = *ch.Title
		}

		bookTitle, ok := bookTitles[ch.BookID]
		if !ok {
			if book, err := w.repo.GetBookByID(ctx, ch.BookID); err == nil && book != nil && book.Title != "" {
				bookTitle = book.Title
			} else {
				bookTitle = ch.BookID
			}
			bookTitles[ch.BookID] = bookTitle
		}

		w.statusMu.Lock()
		w.isBusy = true
		w.currentBook = bookTitle
		w.currentChapter = title
		w.statusMu.Unlock()
		w.broadcast()

		content := strings.TrimSpace(ch.ContentPlain)
		if content == "" {
			if title != "" {
				content = title
			} else {
				content = "Untitled section"
			}
		}

		summary, err := w.aiClient.SummarizeChapter(ctx, title, content)
		if err != nil {
			w.logger.Error("failed to summarize chapter", "chapter_id", ch.ID, "error", err)
			continue
		}
		summary = strings.TrimSpace(summary)
		if summary == "" {
			summary = "No summary available."
		}

		if err := w.repo.UpdateChapterSummary(ctx, ch.ID, summary); err != nil {
			w.logger.Error("failed to update chapter summary", "chapter_id", ch.ID, "error", err)
			continue
		}

		w.statusMu.Lock()
		w.lastProcessed = time.Now()
		w.statusMu.Unlock()
		w.broadcast()

		processed++
	}

	return processed, nil
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
