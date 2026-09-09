package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

type LibraryHandler struct {
	repo     repository.StorageEngine
	scanner  *scanner.Scanner
	ingester *scanner.Ingester
	worker   *worker.Worker
	logger   *slog.Logger
	mu       sync.Mutex
	scanning bool
}

func NewLibraryHandler(
	repo repository.StorageEngine,
	s *scanner.Scanner,
	ingester *scanner.Ingester,
	w *worker.Worker,
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

		// Backfill any database chapters that haven't been chunked into paragraphs yet
		if backfilled, err := h.repo.BackfillParagraphs(ctx); err == nil && backfilled > 0 {
			h.logger.Info("backfilled paragraphs from existing chapters", "count", backfilled)
		}

		discovered, err := h.scanner.Scan()
		if err != nil {
			h.logger.Error("background scan failed", "error", err)
			return
		}

		h.logger.Info("scan discovered files", "count", len(discovered))
		for _, f := range discovered {
			_, err := h.ingester.IngestFile(ctx, f.FullPath, f.RelativePath)
			if err != nil {
				h.logger.Error("failed to ingest discovered file", "file", f.RelativePath, "error", err)
				continue
			}
		}

		if h.worker != nil {
			h.worker.Trigger()
		}
		h.logger.Info("background scan and ingestion finished")
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "scan_started",
		"message": "Library scan initiated in background",
	})
}
