package plugins

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/jobs"
)

// PluginJob tracks an active or completed background plugin action execution for a user.
type PluginJob struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	Plugin          string        `json:"plugin"`
	Action          string        `json:"action"`
	Title           string        `json:"title"`
	TargetType      string        `json:"target_type"`
	TargetID        string        `json:"target_id"`
	ExternalIDs     []string      `json:"external_ids,omitempty"`
	Status          string        `json:"status"` // "queued" | "running" | "completed" | "failed"
	Current         int           `json:"current"`
	Total           int           `json:"total"`
	ItemTitle       string        `json:"item_title,omitempty"`
	ItemProgressPct float64       `json:"item_progress_pct,omitempty"`
	Message         string        `json:"message,omitempty"`
	Error           string        `json:"error,omitempty"`
	Result          *ActionResult `json:"result,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	UpdatedAt       time.Time     `json:"updated_at"`

	touchCh chan struct{} `json:"-"`
}

func (j *PluginJob) clone() *PluginJob {
	if j == nil {
		return nil
	}
	cp := *j
	cp.touchCh = nil
	if len(j.ExternalIDs) > 0 {
		extCp := make([]string, len(j.ExternalIDs))
		copy(extCp, j.ExternalIDs)
		cp.ExternalIDs = extCp
	}
	if j.Result != nil {
		resCp := *j.Result
		cp.Result = &resCp
	}
	return &cp
}

// PluginJobManager manages in-flight and completed background plugin jobs and broadcasts progress events.
type PluginJobManager struct {
	mu      sync.RWMutex
	jobs    map[string]*PluginJob
	seq     atomic.Uint64
	hub     *events.Hub
	unified *jobs.Manager
}

// NewPluginJobManager creates a new PluginJobManager.
func NewPluginJobManager(hub *events.Hub) *PluginJobManager {
	return &PluginJobManager{
		jobs: make(map[string]*PluginJob),
		hub:  hub,
	}
}

// SetJobManager attaches a unified jobs.Manager so plugin jobs are mirrored into the global background job registry.
func (m *PluginJobManager) SetJobManager(jm *jobs.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unified = jm
}

// CreateJob creates and registers a new running PluginJob.
func (m *PluginJobManager) CreateJob(userID, pluginName, actionName, title string, target ActionTarget) (*PluginJob, <-chan struct{}) {
	return m.CreateJobWithTotal(userID, pluginName, actionName, title, target, 1, nil)
}

// CreateJobWithTotal creates and registers a new running PluginJob with explicit batch total and optional catalog external IDs.
func (m *PluginJobManager) CreateJobWithTotal(userID, pluginName, actionName, title string, target ActionTarget, total int, externalIDs []string) (*PluginJob, <-chan struct{}) {
	now := time.Now().UTC()
	id := fmt.Sprintf("pjob-%d-%d", now.UnixNano(), m.seq.Add(1))
	if title == "" {
		if target.Title != "" {
			title = target.Title
		} else {
			title = actionName
		}
	}
	if total <= 0 {
		total = 1
	}
	var extCopy []string
	if len(externalIDs) > 0 {
		extCopy = make([]string, len(externalIDs))
		copy(extCopy, externalIDs)
	}
	touchCh := make(chan struct{}, 1)
	job := &PluginJob{
		ID:          id,
		UserID:      userID,
		Plugin:      pluginName,
		Action:      actionName,
		Title:       title,
		TargetType:  target.Type,
		TargetID:    target.ID,
		ExternalIDs: extCopy,
		Status:      "running",
		Current:     0,
		Total:       total,
		Message:     "Starting...",
		StartedAt:   now,
		UpdatedAt:   now,
		touchCh:     touchCh,
	}

	m.mu.Lock()
	m.jobs[id] = job
	snap := job.clone()
	m.mu.Unlock()

	m.broadcast(snap)
	return snap, touchCh
}

// UpdateProgress updates a running job from a plugin progress notification and resets its watchdog timer.
func (m *PluginJobManager) UpdateProgress(pluginName, jobOrBatchID string, current, total int, itemTitle, message string, percent float64) *PluginJob {
	m.mu.Lock()
	var targetJob *PluginJob
	if jobOrBatchID != "" {
		if j, ok := m.jobs[jobOrBatchID]; ok && j.Plugin == pluginName {
			targetJob = j
		}
	}
	if targetJob == nil {
		for _, j := range m.jobs {
			if j.Plugin == pluginName && (j.Status == "running" || j.Status == "queued") {
				if targetJob == nil || j.StartedAt.After(targetJob.StartedAt) {
					targetJob = j
				}
			}
		}
	}
	if targetJob == nil {
		m.mu.Unlock()
		return nil
	}

	itemChanged := current >= 0 && current != targetJob.Current
	if current >= 0 {
		targetJob.Current = current
	}
	if total > 0 {
		targetJob.Total = total
	}
	if itemTitle != "" {
		targetJob.ItemTitle = itemTitle
	}
	if message != "" {
		targetJob.Message = message
	}
	if percent >= 0 {
		targetJob.ItemProgressPct = percent
	} else if itemChanged {
		targetJob.ItemProgressPct = 0
	}
	targetJob.UpdatedAt = time.Now().UTC()
	if targetJob.touchCh != nil {
		select {
		case targetJob.touchCh <- struct{}{}:
		default:
		}
	}
	snap := targetJob.clone()
	m.mu.Unlock()

	m.broadcast(snap)
	return snap
}

// CompleteJob marks a job as completed with its final ActionResult.
func (m *PluginJobManager) CompleteJob(jobID string, result *ActionResult) *PluginJob {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	job.Status = "completed"
	if job.Total > 0 {
		job.Current = job.Total
	}
	job.ItemProgressPct = 100
	if result != nil {
		resCopy := *result
		resCopy.Job = nil
		job.Result = &resCopy
		if result.Message != "" {
			job.Message = result.Message
		}
	}
	if job.Message == "" || job.Message == "Starting..." {
		job.Message = "Completed"
	}
	job.UpdatedAt = time.Now().UTC()
	snap := job.clone()
	m.mu.Unlock()

	m.broadcast(snap)
	return snap
}

// FailJob marks a job as failed with an error message.
func (m *PluginJobManager) FailJob(jobID string, errMsg string) *PluginJob {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	job.Status = "failed"
	job.Error = errMsg
	job.Message = errMsg
	job.UpdatedAt = time.Now().UTC()
	snap := job.clone()
	m.mu.Unlock()

	m.broadcast(snap)
	return snap
}

// ListUserJobs returns all tracked jobs for a user sorted by StartedAt descending.
func (m *PluginJobManager) ListUserJobs(userID string) []PluginJob {
	m.mu.RLock()
	out := make([]PluginJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		if userID == "" || j.UserID == userID {
			out = append(out, *j.clone())
		}
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

// GetJob returns a snapshot of a single job by ID.
func (m *PluginJobManager) GetJob(jobID string) *PluginJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if j, ok := m.jobs[jobID]; ok {
		return j.clone()
	}
	return nil
}

// DismissJob removes a job record for the owning user.
func (m *PluginJobManager) DismissJob(userID, jobID string) bool {
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
	delete(m.jobs, jobID)
	unified := m.unified
	m.mu.Unlock()

	if unified != nil {
		unified.Dismiss(userID, jobID)
	}
	return true
}

// RemoveLocal removes a plugin job from the local map without re-invoking unified.Dismiss (prevents recursion).
func (m *PluginJobManager) RemoveLocal(userID, jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return
	}
	if userID != "" && j.UserID != "" && j.UserID != userID {
		return
	}
	delete(m.jobs, jobID)
}

func (m *PluginJobManager) broadcast(job *PluginJob) {
	if job == nil {
		return
	}
	m.mu.RLock()
	unified := m.unified
	m.mu.RUnlock()

	if unified != nil {
		meta := map[string]any{
			"plugin":      job.Plugin,
			"action":      job.Action,
			"target_type": job.TargetType,
			"target_id":   job.TargetID,
		}
		if len(job.ExternalIDs) > 0 {
			meta["external_ids"] = job.ExternalIDs
		}
		if job.Result != nil {
			meta["result"] = job.Result
		}
		subtitle := job.Plugin
		if job.Action != "" {
			subtitle = fmt.Sprintf("%s • %s", job.Plugin, job.Action)
		}
		var completedAt *time.Time
		if job.Status == jobs.StatusCompleted || job.Status == jobs.StatusFailed {
			t := job.UpdatedAt
			completedAt = &t
		}
		unified.Upsert(jobs.Job{
			ID:              job.ID,
			Kind:            jobs.KindPlugin,
			UserID:          job.UserID,
			Title:           job.Title,
			Subtitle:        subtitle,
			Status:          job.Status,
			Current:         job.Current,
			Total:           job.Total,
			ItemTitle:       job.ItemTitle,
			ItemProgressPct: job.ItemProgressPct,
			Message:         job.Message,
			Error:           job.Error,
			Metadata:        meta,
			StartedAt:       job.StartedAt,
			UpdatedAt:       job.UpdatedAt,
			CompletedAt:     completedAt,
		})
	}

	if m.hub == nil {
		return
	}
	m.hub.Broadcast(events.Event{
		Type: events.EventPlugin,
		Data: map[string]any{
			"plugin":  job.Plugin,
			"kind":    "job_progress",
			"user_id": job.UserID,
			"job_id":  job.ID,
			"job":     job,
		},
	})
}
