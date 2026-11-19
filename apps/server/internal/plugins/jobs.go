package plugins

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shelfd/shelfd/internal/events"
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
	if j.Result != nil {
		resCp := *j.Result
		cp.Result = &resCp
	}
	return &cp
}

// PluginJobManager manages in-flight and completed background plugin jobs and broadcasts progress events.
type PluginJobManager struct {
	mu   sync.RWMutex
	jobs map[string]*PluginJob
	seq  atomic.Uint64
	hub  *events.Hub
}

// NewPluginJobManager creates a new PluginJobManager.
func NewPluginJobManager(hub *events.Hub) *PluginJobManager {
	return &PluginJobManager{
		jobs: make(map[string]*PluginJob),
		hub:  hub,
	}
}

// CreateJob creates and registers a new running PluginJob.
func (m *PluginJobManager) CreateJob(userID, pluginName, actionName, title string, target ActionTarget) (*PluginJob, <-chan struct{}) {
	now := time.Now().UTC()
	id := fmt.Sprintf("pjob-%d-%d", now.UnixNano(), m.seq.Add(1))
	if title == "" {
		if target.Title != "" {
			title = target.Title
		} else {
			title = actionName
		}
	}
	touchCh := make(chan struct{}, 1)
	job := &PluginJob{
		ID:         id,
		UserID:     userID,
		Plugin:     pluginName,
		Action:     actionName,
		Title:      title,
		TargetType: target.Type,
		TargetID:   target.ID,
		Status:     "running",
		Current:    0,
		Total:      1,
		Message:    "Starting...",
		StartedAt:  now,
		UpdatedAt:  now,
		touchCh:    touchCh,
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
	defer m.mu.Unlock()
	j, ok := m.jobs[jobID]
	if !ok {
		return false
	}
	if userID != "" && j.UserID != "" && j.UserID != userID {
		return false
	}
	delete(m.jobs, jobID)
	return true
}

func (m *PluginJobManager) broadcast(job *PluginJob) {
	if m.hub == nil || job == nil {
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
