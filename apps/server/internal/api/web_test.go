package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
)

func TestSPAHandler(t *testing.T) {
	tempWebDir := t.TempDir()

	// Setup dummy web folder structure
	indexContent := "<html><head><title>Shelf</title></head><body><h1>Shelf App</h1></body></html>"
	if err := os.WriteFile(filepath.Join(tempWebDir, "index.html"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	jsContent := "console.log('flutter loaded');"
	if err := os.WriteFile(filepath.Join(tempWebDir, "flutter.js"), []byte(jsContent), 0644); err != nil {
		t.Fatalf("failed to write flutter.js: %v", err)
	}

	handler := api.SPAHandler(tempWebDir)

	// 1. Root path serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 on root, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Shelf App") {
		t.Errorf("expected index content, got %s", rec.Body.String())
	}

	// 2. Specific file serves directly
	req = httptest.NewRequest(http.MethodGet, "/flutter.js", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 on flutter.js, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "flutter loaded") {
		t.Errorf("expected js content, got %s", rec.Body.String())
	}

	// 3. SPA route fallback serves index.html
	for _, route := range []string{"/library", "/connect", "/books/123", "/reader"} {
		req = httptest.NewRequest(http.MethodGet, route, nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 on SPA route %s, got %d", route, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Shelf App") {
			t.Errorf("expected index content for SPA route %s, got %s", route, rec.Body.String())
		}
	}

	// 4. Missing asset with extension returns 404
	req = httptest.NewRequest(http.MethodGet, "/assets/missing.png", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 on missing asset, got %d", rec.Code)
	}

	// 5. Method not allowed
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 on POST, got %d", rec.Code)
	}

	// 6. Path traversal rejection
	req = httptest.NewRequest(http.MethodGet, "/../../../etc/passwd", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Errorf("expected non-200 on path traversal, got %d", rec.Code)
	}
}
