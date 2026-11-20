package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/jobs"
)

func TestUnifiedJobsAPIAndLibraryRescanJob(t *testing.T) {
	fx := setupAPITest(t)
	token := fx.loginAndGetToken(t)

	// Trigger a library scan -> should return 202 with a library_scan job
	scanReq := httptest.NewRequest(http.MethodPost, "/api/v1/library/scan", nil)
	scanReq.Header.Set("Authorization", "Bearer "+token)
	scanRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(scanRec, scanReq)

	if scanRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted for scan, got %d: %s", scanRec.Code, scanRec.Body.String())
	}

	var scanResp struct {
		Status  string    `json:"status"`
		Message string    `json:"message"`
		Job     *jobs.Job `json:"job"`
	}
	if err := json.Unmarshal(scanRec.Body.Bytes(), &scanResp); err != nil {
		t.Fatalf("failed to decode scan response: %v", err)
	}
	if scanResp.Job == nil || scanResp.Job.Kind != jobs.KindLibraryScan {
		t.Fatalf("expected library_scan job in response, got %+v", scanResp.Job)
	}

	// Poll GET /api/v1/jobs until the scan completes
	var completedJob *jobs.Job
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		listReq.Header.Set("Authorization", "Bearer "+token)
		listRec := httptest.NewRecorder()
		fx.handler.ServeHTTP(listRec, listReq)

		if listRec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for GET /api/v1/jobs, got %d", listRec.Code)
		}
		var listResp struct {
			Jobs []jobs.Job `json:"jobs"`
		}
		if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to decode jobs list: %v", err)
		}
		for i := range listResp.Jobs {
			if listResp.Jobs[i].ID == scanResp.Job.ID && listResp.Jobs[i].Status == jobs.StatusCompleted {
				j := listResp.Jobs[i]
				completedJob = &j
				break
			}
		}
		if completedJob != nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if completedJob == nil {
		t.Fatal("timed out waiting for library_scan job to complete in GET /api/v1/jobs")
	}

	// Clear finished jobs via DELETE /api/v1/jobs
	clearReq := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs", nil)
	clearReq.Header.Set("Authorization", "Bearer "+token)
	clearRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(clearRec, clearReq)

	if clearRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for DELETE /api/v1/jobs, got %d", clearRec.Code)
	}
	var clearResp struct {
		Cleared int `json:"cleared"`
	}
	if err := json.Unmarshal(clearRec.Body.Bytes(), &clearResp); err != nil {
		t.Fatalf("failed to decode clear response: %v", err)
	}
	if clearResp.Cleared != 1 {
		t.Fatalf("expected 1 cleared job, got %d", clearResp.Cleared)
	}

	// Verify GET /api/v1/jobs is now empty
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(listRec, listReq)
	var listResp struct {
		Jobs []jobs.Job `json:"jobs"`
	}
	_ = json.Unmarshal(listRec.Body.Bytes(), &listResp)
	if len(listResp.Jobs) != 0 {
		t.Fatalf("expected 0 jobs after clearing finished, got %d", len(listResp.Jobs))
	}
}
