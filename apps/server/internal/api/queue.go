package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/worker"
)

// QueueHandler handles inspection and streaming of background queue status.
type QueueHandler struct {
	repo         repository.StorageEngine
	worker       *worker.Worker
	uploadWorker *worker.UploadWorker
	hub          *events.Hub
}

// NewQueueHandler creates a new QueueHandler instance.
func NewQueueHandler(repo repository.StorageEngine, w *worker.Worker, uw *worker.UploadWorker, hub *events.Hub) *QueueHandler {
	return &QueueHandler{
		repo:         repo,
		worker:       w,
		uploadWorker: uw,
		hub:          hub,
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

// StreamEvents provides a Server-Sent Events (SSE) feed of real-time queue status changes and catalog events.
func (h *QueueHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "Streaming unsupported by client")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
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

	var hubCh chan events.Event
	if h.hub != nil {
		ch, unsub := h.hub.Subscribe()
		defer unsub()
		hubCh = ch
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
		case evt, ok := <-hubCh:
			if !ok {
				return
			}
			data, err := json.Marshal(evt.Data)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
