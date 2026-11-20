package api

import (
	"net/http"
	"strings"

	"github.com/shelfd/shelfd/internal/jobs"
)

// JobsHandler serves the unified background jobs REST API.
type JobsHandler struct {
	manager *jobs.Manager
}

// NewJobsHandler creates a new JobsHandler.
func NewJobsHandler(manager *jobs.Manager) *JobsHandler {
	return &JobsHandler{manager: manager}
}

// ListJobs returns all active and recent finished background jobs visible to the current user.
func (h *JobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSON(w, http.StatusOK, map[string]any{"jobs": []jobs.Job{}})
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	list := h.manager.ListForUser(user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"jobs": list})
}

// DismissJob removes a single background job visible to the current user.
func (h *JobsHandler) DismissJob(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	jobID := strings.TrimSpace(r.PathValue("id"))
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "Job ID required")
		return
	}
	if !h.manager.Dismiss(user.ID, jobID) {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ClearFinishedJobs removes all completed and failed background jobs visible to the current user.
func (h *JobsHandler) ClearFinishedJobs(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeJSON(w, http.StatusOK, map[string]any{"cleared": 0})
		return
	}
	user := CurrentUser(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	cleared := h.manager.ClearFinished(user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"cleared": cleared})
}
