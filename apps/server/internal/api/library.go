package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

type LibraryHandler struct {
	repo     repository.StorageEngine
	scanner  *scanner.Scanner
	ingester *scanner.Ingester
	worker   *worker.Worker
	hub      *events.Hub
	logger   *slog.Logger
	mu       sync.Mutex
	scanning bool
}

func NewLibraryHandler(
	repo repository.StorageEngine,
	s *scanner.Scanner,
	ingester *scanner.Ingester,
	w *worker.Worker,
	hub *events.Hub,
	logger *slog.Logger,
) *LibraryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LibraryHandler{
		repo:     repo,
		scanner:  s,
		ingester: ingester,
		worker:   w,
		hub:      hub,
		logger:   logger,
	}
}

func (h *LibraryHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.Lock()
	if h.scanning {
		h.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "already_scanning",
			"message": "A library scan is already in progress",
		})
		return
	}
	h.scanning = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.scanning = false
			h.mu.Unlock()
		}()

		h.logger.Info("starting background library scan")
		ctx := context.Background()

		if h.hub != nil {
			h.hub.Broadcast(events.Event{
				Type: events.EventScanStatus,
				Data: map[string]any{"status": "started"},
			})
		}

		// Backfill any database chapters that haven't been chunked into paragraphs yet
		backfilled := 0
		if n, err := h.repo.BackfillParagraphs(ctx); err == nil && n > 0 {
			backfilled = n
			h.logger.Info("backfilled paragraphs from existing chapters", "count", backfilled)
			if h.worker != nil {
				h.worker.Trigger()
			}
		}

		discovered, err := h.scanner.Scan()
		if err != nil {
			h.logger.Error("background scan failed", "error", err)
			if h.hub != nil {
				h.hub.Broadcast(events.Event{
					Type: events.EventScanStatus,
					Data: map[string]any{"status": "failed", "error": err.Error()},
				})
			}
			return
		}

		h.logger.Info("scan discovered files", "count", len(discovered))
		newCount := 0
		modifiedCount := 0
		unchangedCount := 0
		for _, f := range discovered {
			book, status, err := h.ingester.SyncFile(ctx, f.FullPath, f.RelativePath)
			if err != nil {
				h.logger.Error("failed to sync discovered file", "file", f.RelativePath, "error", err)
				continue
			}
			switch status {
			case scanner.SyncStatusNew:
				newCount++
				// Paragraphs are chunked & queued in DB. Trigger worker immediately to run in background!
				if h.worker != nil {
					h.worker.Trigger()
				}
				// Broadcast realtime book_added event to frontend immediately
				if h.hub != nil && book != nil {
					bookItem := BuildBookListItem(ctx, h.repo, book)
					h.hub.Broadcast(events.Event{
						Type: events.EventBookAdded,
						Data: bookItem,
					})
					if qStatus, err := h.repo.GetQueueStatus(ctx); err == nil {
						if h.worker != nil {
							rt := h.worker.GetRuntimeStatus()
							if rt.IsBusy {
								qStatus.IsActive = true
								qStatus.CurrentBook = rt.CurrentBook
								qStatus.CurrentChapter = rt.CurrentChapter
							}
						}
						h.hub.Broadcast(events.Event{
							Type: events.EventQueueStatus,
							Data: qStatus,
						})
					}
				}
			case scanner.SyncStatusModified:
				modifiedCount++
				if h.worker != nil {
					h.worker.Trigger()
				}
				if h.hub != nil && book != nil {
					bookItem := BuildBookListItem(ctx, h.repo, book)
					h.hub.Broadcast(events.Event{
						Type: events.EventBookUpdated,
						Data: bookItem,
					})
				}
			case scanner.SyncStatusUnchanged:
				unchangedCount++
			}
		}

		h.logger.Info("background scan and ingestion finished",
			"new", newCount,
			"modified", modifiedCount,
			"unchanged", unchangedCount,
		)

		if (newCount > 0 || modifiedCount > 0 || backfilled > 0) && h.worker != nil {
			h.worker.Trigger()
		}

		if h.hub != nil {
			h.hub.Broadcast(events.Event{
				Type: events.EventScanStatus,
				Data: map[string]any{
					"status":    "completed",
					"new":       newCount,
					"modified":  modifiedCount,
					"unchanged": unchangedCount,
				},
			})
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "scan_started",
		"message": "Library scan initiated in background",
	})
}
