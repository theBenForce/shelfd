package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPI_ExportStudyDeck(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	book := seedMarkdownTestBook(t, f)

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/study-deck", strings.NewReader(`{"format":"apkg"}`))
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid format returns 400 with json error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/study-deck", strings.NewReader(`{"format":"pdf"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
		var errResp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("decode error json: %v", err)
		}
		if errResp["error"] == "" {
			t.Fatalf("expected non-empty error field, got %v", errResp)
		}
	})

	t.Run("missing book returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/nonexistent-book/study-deck", strings.NewReader(`{"format":"apkg"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("apkg export returns valid zip with collection.anki2 and media", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/study-deck", strings.NewReader(`{"format":"apkg","chapter_indices":[1]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment;") || !strings.Contains(cd, ".apkg") {
			t.Fatalf("unexpected Content-Disposition: %q", cd)
		}
		zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatalf("open apkg zip: %v", err)
		}
		seen := map[string]bool{}
		for _, zf := range zr.File {
			seen[zf.Name] = true
		}
		if !seen["collection.anki2"] || !seen["media"] {
			t.Fatalf("missing required apkg entries, got %v", seen)
		}
	})

	t.Run("csv export returns valid utf-8 csv rows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/study-deck", strings.NewReader(`{"format":"csv","chapter_indices":[1,2]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
			t.Fatalf("expected text/csv content type, got %q", ct)
		}
		rows, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
		if err != nil {
			t.Fatalf("parse csv: %v", err)
		}
		if len(rows) < 2 {
			t.Fatalf("expected header and at least one card row, got %d rows", len(rows))
		}
	})

	t.Run("markdown export returns vault zip with README and concepts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/study-deck", strings.NewReader(`{"format":"markdown","chapter_indices":[1]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
			t.Fatalf("expected application/zip, got %q", ct)
		}
		zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatalf("open markdown zip: %v", err)
		}
		hasReadme := false
		hasConcept := false
		for _, zf := range zr.File {
			if zf.Name == "README.md" {
				hasReadme = true
			}
			if strings.HasPrefix(zf.Name, "concepts/") && strings.HasSuffix(zf.Name, ".md") {
				hasConcept = true
			}
		}
		if !hasReadme || !hasConcept {
			t.Fatalf("expected README.md and concepts/*.md in markdown vault zip")
		}
	})
}
