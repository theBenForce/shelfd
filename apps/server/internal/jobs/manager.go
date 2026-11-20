package jobs

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shelfd/shelfd/internal/events"
)

// JobKind identifies the category of a standardized background task.
const (
	KindPlugin       = "plugin"
	KindLibraryScan  = "library_scan"
	KindAIIndexing   = "ai_indexing"
	KindUploadIngest = "upload_ingest"
)

// JobStatus identifies the lifecycle state of a background job.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

const maxFinishedHistory = 50

// Job represents a standardized background task tracked by the server.
type Job struct {
	ID              string         `json:"id"`
	Kind            string         `json:"kind"`
	UserID          string         `json:"user_id,omitempty"`
	Title           string         `json:"title"`
	Subtitle        string         `json:"subtitle,omitempty"`
	Status          string         `json:"status"`
	Current         int            `json:"current"`
	Total           int            `json:"total"`
	ItemTitle       string         `json:"item_title,omitempty"`
	ItemProgressPct float64        `json:"item_progress_pct,omitempty"`
	Message         string         `json:"message,omitempty"`
	Error           string         `json:"error,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	StartedAt       time.Time      `json:"started_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
}

// IsActive returns true if the job is currently queued or running.
func (j *Job) IsActive() bool {
	return j != nil && (j.Status == StatusQueued || j.Status == StatusRunning)
}

// Clone returns a deep copy of the Job safe for concurrent reads.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	cp := *j
	if j.CompletedAt != nil {
		t := *j.CompletedAt
		cp.CompletedAt = &t
	}
	if j.Metadata != nil {
		metaCopy := make(map[string]any, len(j.Metadata))
		for k, v := range j.Metadata {
			metaCopy[k] = v
		}
		cp.Metadata = metaCopy
	}
	return &cp
}

// CreateOptions configures a newly created background job.
type CreateOptions struct {
	ID       string
	Kind     string
	UserID   string
	Title    string
	Subtitle string
	Status   string
	Total    int
	Message  string
	Metadata map[string]any
}

// ProgressUpdate describes incremental progress fields for an active job.
type ProgressUpdate struct {
	Title           string
	Subtitle        string
	Current         *int
	Total           *int
	ItemTitle       *string
	ItemProgressPct *float64
	Message         *string
	Metadata        map[string]any
}

// Manager tracks active and recent finished background jobs across all subsystems
// and broadcasts standardized SSE updates via events.Hub.
type Manager struct {
	mu        sync.RWMutex
	jobs      map[string]*Job
	seq       atomic.Uint64
	hub       *events.Hub
	onDismiss func(userID, jobID string)
}

// NewManager creates a new unified background job Manager.
func NewManager(hub *events.Hub) *Manager {
	return &Manager{
		jobs: make(map[string]*Job),
		hub:  hub,
	}
}

// SetOnDismiss registers an optional callback invoked when a job is dismissed or cleared.
func (m *Manager) SetOnDismiss(fn func(userID, jobID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDismiss = fn
}

// Create creates, stores, and broadcasts a new background job.
func (m *Manager) Create(opts CreateOptions) *Job {
	now := time.Now().UTC()
	id := opts.ID
	if id == "" {
		id = fmt.Sprintf("job-%d-%d", now.UnixNano(), m.seq.Add(1))
	}
	status := opts.Status
	if status == "" {
		status = StatusRunning
	}
	msg := opts.Message
	if msg == "" && status == StatusRunning {
		msg = "Starting..."
	}

	var meta map[string]any
	if opts.Metadata != nil {
		meta = make(map[string]any, len(opts.Metadata))
		for k, v := range opts.Metadata {
			meta[k] = v
		}
	}

	job := &Job{
		ID:        id,
		Kind:      opts.Kind,
		UserID:    opts.UserID,
		Title:     opts.Title,
		Subtitle:  opts.Subtitle,
		Status:    status,
		Current:   0,
		Total:     opts.Total,
		Message:   msg,
		Metadata:  meta,
		StartedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.jobs[id] = job
	snap := job.Clone()
	m.mu.Unlock()

	m.broadcastUpdated(snap)
	return snap
}

// Upsert inserts or replaces a job snapshot and broadcasts it.
func (m *Manager) Upsert(job Job) *Job {
	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = fmt.Sprintf("job-%d-%d", now.UnixNano(), m.seq.Add(1))
	}
	if job.StartedAt.IsZero() {
		job.StartedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}
	if job.Status == "" {
		job.Status = StatusRunning
	}

	m.mu.Lock()
	cp := job.Clone()
	m.jobs[cp.ID] = cp
	if !cp.IsActive() {
		m.pruneFinishedLocked()
	}
	snap := cp.Clone()
	m.mu.Unlock()

	m.broadcastUpdated(snap)
	return snap
}

// UpdateProgress updates an existing job's progress fields and broadcasts the change.
func (m *Manager) UpdateProgress(jobID string, upd ProgressUpdate) *Job {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil
	}

	if upd.Title != "" {
		job.Title = upd.Title
	}
	if upd.Subtitle != "" {
		job.Subtitle = upd.Subtitle
	}
	if upd.Current != nil {
		job.Current = *upd.Current
	}
	if upd.Total != nil {
		job.Total = *upd.Total
	}
	if upd.ItemTitle != nil {
		job.ItemTitle = *upd.ItemTitle
	}
	if upd.ItemProgressPct != nil {
		job.ItemProgressPct = *upd.ItemProgressPct
	} else if job.Total > 0 {
		pct := (float64(job.Current) / float64(job.Total)) * 100
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		job.ItemProgressPct = pct
	}
	if upd.Message != nil {
		job.Message = *upd.Message
	}
	if upd.Metadata != nil {
		if job.Metadata == nil {
			job.Metadata = make(map[string]any, len(upd.Metadata))
		}
		for k, v := range upd.Metadata {
			job.Metadata[k] = v
		}
	}
	job.UpdatedAt = time.Now().UTC()
	snap := job.Clone()
	m.mu.Unlock()

	m.broadcastUpdated(snap)
	return snap
}

// Complete marks a job as completed and broadcasts the update.
func (m *Manager) Complete(jobID string, message string, metadata map[string]any) *Job {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	now := time.Now().UTC()
	job.Status = StatusCompleted
	if job.Total > 0 {
		job.Current = job.Total
	}
	job.ItemProgressPct = 100
	if message != "" {
		job.Message = message
	} else if job.Message == "" || job.Message == "Starting..." {
		job.Message = "Completed"
	}
	if metadata != nil {
		if job.Metadata == nil {
			job.Metadata = make(map[string]any, len(metadata))
		}
		for k, v := range metadata {
			job.Metadata[k] = v
		}
	}
	job.UpdatedAt = now
	job.CompletedAt = &now
	m.pruneFinishedLocked()
	snap := job.Clone()
	m.mu.Unlock()

	m.broadcastUpdated(snap)
	return snap
}

// Fail marks a job as failed with an error message and broadcasts the update.
func (m *Manager) Fail(jobID string, errMsg string) *Job {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	now := time.Now().UTC()
	job.Status = StatusFailed
	job.Error = errMsg
	job.Message = errMsg
	job.UpdatedAt = now
	job.CompletedAt = &now
	m.pruneFinishedLocked()
	snap := job.Clone()
	m.mu.Unlock()

	m.broadcastUpdated(snap)
	return snap
}

// Get returns a snapshot of a single job by ID.
func (m *Manager) Get(jobID string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[jobID]; ok {
		return j.Clone()
	}
	return nil
}

// FindActiveByKind returns the most recently started active job of the given kind, if any.
func (m *Manager) FindActiveByKind(kind string) *Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var best *Job
	for _, j := range m.jobs {
		if j.Kind == kind && j.IsActive() {
			if best == nil || j.StartedAt.After(best.StartedAt) {
				best = j
			}
		}
	}
	return best.Clone()
}

// ListForUser returns all active and recent finished jobs visible to userID.
// Global system jobs (UserID == "") are visible to all authenticated users.
// Results are ordered with active jobs first (newest StartedAt first),
// followed by finished jobs (newest UpdatedAt first).
func (m *Manager) ListForUser(userID string) []Job {
	m.mu.RLock()
	out := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		if userID == "" || j.UserID == "" || j.UserID == userID {
			out = append(out, *j.Clone())
		}
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		ai := out[i].IsActive()
		aj := out[j].IsActive()
		if ai != aj {
			return ai
		}
		if ai {
			return out[i].StartedAt.After(out[j].StartedAt)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// Dismiss removes a single job visible to userID and broadcasts a removal event.
func (m *Manager) Dismiss(userID, jobID string) bool {
	m.mu.Lock()
	j, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return false
	}
	if userID != "" && j.UserID != "" && j.UserID != userID {
		m.mu.Unlock()
		return false
	}
	ownerID := j.UserID
	cb := m.onDismiss
	delete(m.jobs, jobID)
	m.mu.Unlock()

	if cb != nil {
		cb(userID, jobID)
	}
	m.broadcastRemoved(jobID, ownerID)
	return true
}

// ClearFinished removes all completed and failed jobs visible to userID.
func (m *Manager) ClearFinished(userID string) int {
	type removedItem struct {
		id      string
		ownerID string
	}

	m.mu.Lock()
	var removed []removedItem
	for id, j := range m.jobs {
		if j.IsActive() {
			continue
		}
		if userID != "" && j.UserID != "" && j.UserID != userID {
			continue
		}
		removed = append(removed, removedItem{id: id, ownerID: j.UserID})
		delete(m.jobs, id)
	}
	cb := m.onDismiss
	m.mu.Unlock()

	for _, item := range removed {
		if cb != nil {
			cb(userID, item.id)
		}
		m.broadcastRemoved(item.id, item.ownerID)
	}
	return len(removed)
}

func (m *Manager) pruneFinishedLocked() {
	var finished []*Job
	for _, j := range m.jobs {
		if !j.IsActive() {
			finished = append(finished, j)
		}
	}
	if len(finished) <= maxFinishedHistory {
		return
	}
	sort.Slice(finished, func(i, j int) bool {
		return finished[i].UpdatedAt.After(finished[j].UpdatedAt)
	})
	for _, stale := range finished[maxFinishedHistory:] {
		delete(m.jobs, stale.ID)
	}
}

func (m *Manager) broadcastUpdated(job *Job) {
	if m.hub == nil || job == nil {
		return
	}
	m.hub.Broadcast(events.Event{
		Type: events.EventJobUpdated,
		Data: job,
	})
}

func (m *Manager) broadcastRemoved(jobID, userID string) {
	if m.hub == nil || jobID == "" {
		return
	}
	m.hub.Broadcast(events.Event{
		Type: events.EventJobRemoved,
		Data: map[string]any{
			"job_id":  jobID,
			"user_id": userID,
		},
	})
}
