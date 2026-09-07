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
	}
}

// ProcessBatch fetches a batch of unindexed chapters, generates summaries and embeddings, and persists them.
// Returns the number of successfully indexed chapters.
func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	chapters, err := w.repo.GetUnindexedChapters(ctx, w.batchSize)
	if err != nil {
		return 0, fmt.Errorf("getting unindexed chapters: %w", err)
	}
	if len(chapters) == 0 {
		return 0, nil
	}

	processed := 0
	for _, ch := range chapters {
		if err := ctx.Err(); err != nil {
			return processed, err
		}

		title := ""
		if ch.Title != nil {
			title = *ch.Title
		}

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

		embedding, err := w.aiClient.GenerateEmbedding(ctx, summary)
		if err != nil {
			w.logger.Error("failed to generate embedding for chapter summary", "chapter_id", ch.ID, "error", err)
			continue
		}

		if err := w.repo.InsertChapterVector(ctx, ch.ID, embedding); err != nil {
			w.logger.Error("failed to insert chapter vector", "chapter_id", ch.ID, "error", err)
			continue
		}

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
