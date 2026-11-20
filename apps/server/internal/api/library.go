package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/jobs"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

type LibraryHandler struct {
	repo       repository.StorageEngine
	scanner    *scanner.Scanner
	ingester   *scanner.Ingester
	worker     *worker.Worker
	hub        *events.Hub
	jobManager *jobs.Manager
	logger     *slog.Logger
	mu         sync.Mutex
	scanning   bool
	activeJob  string
}

func NewLibraryHandler(
	repo repository.StorageEngine,
	s *scanner.Scanner,
	ingester *scanner.Ingester,
	w *worker.Worker,
	hub *events.Hub,
	jobManager *jobs.Manager,
	logger *slog.Logger,
) *LibraryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LibraryHandler{
		repo:       repo,
		scanner:    s,
		ingester:   ingester,
		worker:     w,
		hub:        hub,
		jobManager: jobManager,
		logger:     logger,
	}
}

func (h *LibraryHandler) Scan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	h.mu.Lock()
	if h.scanning {
		var existingJob *jobs.Job
		if h.jobManager != nil && h.activeJob != "" {
			existingJob = h.jobManager.Get(h.activeJob)
		}
		h.mu.Unlock()
		resp := map[string]any{
			"status":  "already_scanning",
			"message": "A library scan is already in progress",
		}
		if existingJob != nil {
			resp["job"] = existingJob
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	h.scanning = true

	var bgJob *jobs.Job
	if h.jobManager != nil {
		bgJob = h.jobManager.Create(jobs.CreateOptions{
			Kind:    jobs.KindLibraryScan,
			Title:   "Library Rescan",
			Status:  jobs.StatusRunning,
			Total:   0,
			Message: "Discovering library files...",
		})
		h.activeJob = bgJob.ID
	}
	h.mu.Unlock()

	jobID := ""
	if bgJob != nil {
		jobID = bgJob.ID
	}

	go func() {
		defer func() {
			h.mu.Lock()
			h.scanning = false
			h.activeJob = ""
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
			if h.jobManager != nil && jobID != "" {
				h.jobManager.Fail(jobID, err.Error())
			}
			if h.hub != nil {
				h.hub.Broadcast(events.Event{
					Type: events.EventScanStatus,
					Data: map[string]any{"status": "failed", "error": err.Error()},
				})
			}
			return
		}

		totalFiles := len(discovered)
		h.logger.Info("scan discovered files", "count", totalFiles)
		if h.jobManager != nil && jobID != "" {
			msg := fmt.Sprintf("Scanning %d files...", totalFiles)
			if totalFiles == 0 {
				msg = "No library files found"
			}
			zero := 0
			h.jobManager.UpdateProgress(jobID, jobs.ProgressUpdate{
				Current: &zero,
				Total:   &totalFiles,
				Message: &msg,
			})
		}

		newCount := 0
		modifiedCount := 0
		unchangedCount := 0
		for i, f := range discovered {
			if h.jobManager != nil && jobID != "" {
				cur := i + 1
				itemName := filepath.Base(f.RelativePath)
				msg := fmt.Sprintf("Scanning %d of %d files...", cur, totalFiles)
				h.jobManager.UpdateProgress(jobID, jobs.ProgressUpdate{
					Current:   &cur,
					Total:     &totalFiles,
					ItemTitle: &itemName,
					Message:   &msg,
				})
			}

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

		if h.jobManager != nil && jobID != "" {
			msg := "Reconciling authors and genres..."
			h.jobManager.UpdateProgress(jobID, jobs.ProgressUpdate{
				Message: &msg,
			})
		}

		// Reconcile authors and clean up orphaned taxonomy
		if err := h.repo.ReconcileAuthors(ctx); err != nil {
			h.logger.Error("failed to reconcile authors after scan", "error", err)
		}
		if _, err := h.repo.PruneOrphanedGenres(ctx); err != nil {
			h.logger.Error("failed to prune orphaned genres after scan", "error", err)
		}

		h.logger.Info("background scan and ingestion finished",
			"new", newCount,
			"modified", modifiedCount,
			"unchanged", unchangedCount,
		)

		if (newCount > 0 || modifiedCount > 0 || backfilled > 0) && h.worker != nil {
			h.worker.Trigger()
		}

		if h.jobManager != nil && jobID != "" {
			summary := fmt.Sprintf("Scan complete: %d new, %d updated, %d unchanged", newCount, modifiedCount, unchangedCount)
			h.jobManager.Complete(jobID, summary, map[string]any{
				"new":       newCount,
				"modified":  modifiedCount,
				"unchanged": unchangedCount,
			})
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

	resp := map[string]any{
		"status":  "scan_started",
		"message": "Library scan initiated in background",
	}
	if bgJob != nil {
		resp["job"] = bgJob
	}
	writeJSON(w, http.StatusAccepted, resp)
}
