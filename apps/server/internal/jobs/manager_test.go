package jobs

import (
	"fmt"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/events"
)

func TestManagerLifecycleAndEvents(t *testing.T) {
	hub := events.NewHub()
	subCh, unsub := hub.Subscribe()
	defer unsub()

	mgr := NewManager(hub)

	created := mgr.Create(CreateOptions{
		Kind:    KindLibraryScan,
		Title:   "Library Rescan",
		Total:   4,
		Message: "Discovering files...",
	})
	if created.ID == "" || created.Status != StatusRunning || created.Kind != KindLibraryScan {
		t.Fatalf("unexpected created job: %+v", created)
	}

	select {
	case evt := <-subCh:
		if evt.Type != events.EventJobUpdated {
			t.Fatalf("expected job_updated event, got %s", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job_updated event on create")
	}

	cur := 2
	item := "dune.epub"
	msg := "Scanning 2 of 4 files..."
	updated := mgr.UpdateProgress(created.ID, ProgressUpdate{
		Current:   &cur,
		ItemTitle: &item,
		Message:   &msg,
	})
	if updated == nil || updated.Current != 2 || updated.ItemTitle != "dune.epub" || updated.ItemProgressPct != 50 {
		t.Fatalf("unexpected updated job: %+v", updated)
	}

	completed := mgr.Complete(created.ID, "Scan complete: 1 new, 0 updated, 3 unchanged", map[string]any{"new": 1})
	if completed == nil || completed.Status != StatusCompleted || completed.Current != 4 || completed.ItemProgressPct != 100 || completed.CompletedAt == nil {
		t.Fatalf("unexpected completed job: %+v", completed)
	}
	if completed.Metadata["new"] != 1 {
		t.Fatalf("expected metadata new=1, got %+v", completed.Metadata)
	}
}

func TestManagerScopingOrderingDismissAndClear(t *testing.T) {
	hub := events.NewHub()
	mgr := NewManager(hub)

	var dismissedIDs []string
	mgr.SetOnDismiss(func(userID, jobID string) {
		dismissedIDs = append(dismissedIDs, jobID)
	})

	// Global job (visible to all users)
	globalJob := mgr.Create(CreateOptions{
		Kind:   KindAIIndexing,
		Title:  "AI Catalog Indexing",
		Status: StatusRunning,
	})
	// User A plugin job (failed)
	userAJob := mgr.Create(CreateOptions{
		Kind:   KindPlugin,
		UserID: "user-a",
		Title:  "Send to Yoto",
	})
	mgr.Fail(userAJob.ID, "network timeout")

	// User B plugin job (running)
	userBJob := mgr.Create(CreateOptions{
		Kind:   KindPlugin,
		UserID: "user-b",
		Title:  "NotebookLM Export",
	})

	listA := mgr.ListForUser("user-a")
	if len(listA) != 2 {
		t.Fatalf("expected 2 jobs for user-a, got %d", len(listA))
	}
	// Active jobs must come before finished jobs
	if listA[0].ID != globalJob.ID || listA[1].ID != userAJob.ID {
		t.Fatalf("expected active job first then failed job, got %s then %s", listA[0].ID, listA[1].ID)
	}

	// User B cannot dismiss User A's job
	if mgr.Dismiss("user-b", userAJob.ID) {
		t.Fatal("user-b should not be able to dismiss user-a's job")
	}

	// ClearFinished for user-a clears user-a's failed job, leaves active globalJob and userBJob intact
	cleared := mgr.ClearFinished("user-a")
	if cleared != 1 {
		t.Fatalf("expected 1 cleared job for user-a, got %d", cleared)
	}
	if len(dismissedIDs) != 1 || dismissedIDs[0] != userAJob.ID {
		t.Fatalf("expected onDismiss for %s, got %+v", userAJob.ID, dismissedIDs)
	}
	if mgr.Get(globalJob.ID) == nil || mgr.Get(userBJob.ID) == nil {
		t.Fatal("active jobs should remain after ClearFinished")
	}
}

func TestManagerPrunesFinishedHistory(t *testing.T) {
	mgr := NewManager(nil)
	for i := 0; i < maxFinishedHistory+10; i++ {
		j := mgr.Create(CreateOptions{
			Kind:  KindUploadIngest,
			Title: fmt.Sprintf("Upload %d", i),
		})
		mgr.Complete(j.ID, "Done", nil)
	}

	all := mgr.ListForUser("")
	if len(all) != maxFinishedHistory {
		t.Fatalf("expected finished history pruned to %d, got %d", maxFinishedHistory, len(all))
	}
}
