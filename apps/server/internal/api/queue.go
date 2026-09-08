package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/worker"
)

// QueueHandler handles inspection and streaming of background queue status.
type QueueHandler struct {
	repo         repository.StorageEngine
	worker       *worker.Worker
	uploadWorker *worker.UploadWorker
}

// NewQueueHandler creates a new QueueHandler instance.
func NewQueueHandler(repo repository.StorageEngine, w *worker.Worker, uw *worker.UploadWorker) *QueueHandler {
	return &QueueHandler{
		repo:         repo,
		worker:       w,
		uploadWorker: uw,
	}
}

// GetStatus returns the current snapshot of queue metrics and active jobs.
func (h *QueueHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.repo.GetQueueStatus(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get queue status: %v", err))
		return
	}

	if h.worker != nil {
		rt := h.worker.GetRuntimeStatus()
		if rt.IsBusy {
			status.IsActive = true
			status.CurrentBook = rt.CurrentBook
			status.CurrentChapter = rt.CurrentChapter
		}
	}

	writeJSON(w, http.StatusOK, status)
}

// StreamEvents provides a Server-Sent Events (SSE) feed of real-time queue status changes.
func (h *QueueHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "Streaming unsupported by client")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sendStatus := func() error {
		status, err := h.repo.GetQueueStatus(r.Context())
		if err != nil {
			return err
		}
		if h.worker != nil {
			rt := h.worker.GetRuntimeStatus()
			if rt.IsBusy {
				status.IsActive = true
				status.CurrentBook = rt.CurrentBook
				status.CurrentChapter = rt.CurrentChapter
			}
		}
		data, err := json.Marshal(status)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "event: queue_status\ndata: %s\n\n", data)
		if err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	// Send initial status immediately
	if err := sendStatus(); err != nil {
		return
	}

	var subCh chan struct{}
	if h.worker != nil {
		subCh = h.worker.Subscribe()
		defer h.worker.Unsubscribe(subCh)
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if err := sendStatus(); err != nil {
				return
			}
		case <-subCh:
			if err := sendStatus(); err != nil {
				return
			}
		}
	}
}
