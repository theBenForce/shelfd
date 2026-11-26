package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/repository"
)

func TestBookRecapEndpoint(t *testing.T) {
	fx := setupAPITest(t)
	token := fx.loginAndGetToken(t)
	ctx := context.Background()

	book := &repository.Book{
		ID:       "recap_book_1",
		Title:    "Dune",
		FilePath: "dune.epub",
	}
	if err := fx.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	ch1Title := "Chapter 1"
	ch2Title := "Chapter 2"
	for _, ch := range []*repository.Chapter{
		{
			ID:           "recap_ch_1",
			BookID:       book.ID,
			ChapterIndex: 1,
			Title:        &ch1Title,
			Summary:      "Paul faces the Gom Jabbar.",
			ContentPlain: "Paul faces the Gom Jabbar on Caladan.",
		},
		{
			ID:           "recap_ch_2",
			BookID:       book.ID,
			ChapterIndex: 2,
			Title:        &ch2Title,
			Summary:      "House Atreides lands on Arrakis.",
			ContentPlain: "House Atreides lands on Arrakis.",
		},
	} {
		if err := fx.repo.CreateChapter(ctx, ch); err != nil {
			t.Fatalf("create chapter: %v", err)
		}
	}

	// Unauthenticated request returns 401.
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/recap", bytes.NewBufferString(`{"chapter_index":1}`))
	unauthRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", unauthRec.Code)
	}

	// Negative chapter_index returns 400.
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/recap", bytes.NewBufferString(`{"chapter_index":-1}`))
	badReq.Header.Set("Authorization", "Bearer "+token)
	badRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for negative chapter_index, got %d: %s", badRec.Code, badRec.Body.String())
	}

	// Missing book returns 404.
	missingReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/nonexistent/recap", bytes.NewBufferString(`{"chapter_index":1}`))
	missingReq.Header.Set("Authorization", "Bearer "+token)
	missingRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for missing book, got %d: %s", missingRec.Code, missingRec.Body.String())
	}

	// Valid request generates uncached recap, then second request returns cached: true.
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/recap", bytes.NewBufferString(`{"chapter_index":2,"paragraph_offset":0}`))
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on first recap, got %d: %s", rec1.Code, rec1.Body.String())
	}
	var first ai.RecapResult
	if err := json.Unmarshal(rec1.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first recap: %v", err)
	}
	if first.Cached {
		t.Fatalf("expected first recap response cached=false")
	}
	if first.ChapterIndex != 2 || first.ChaptersIncluded != 2 || first.Recap == "" {
		t.Fatalf("unexpected first recap response: %+v", first)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/recap", bytes.NewBufferString(`{"chapter_index":2,"paragraph_offset":0}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on cached recap, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var second ai.RecapResult
	if err := json.Unmarshal(rec2.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second recap: %v", err)
	}
	if !second.Cached {
		t.Fatalf("expected second recap response cached=true")
	}

	// Force refresh returns cached=false.
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/books/"+book.ID+"/recap", bytes.NewBufferString(`{"chapter_index":2,"paragraph_offset":0,"force_refresh":true}`))
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on force_refresh recap, got %d: %s", rec3.Code, rec3.Body.String())
	}
	var refreshed ai.RecapResult
	if err := json.Unmarshal(rec3.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode refreshed recap: %v", err)
	}
	if refreshed.Cached {
		t.Fatalf("expected force_refresh recap response cached=false")
	}
}
