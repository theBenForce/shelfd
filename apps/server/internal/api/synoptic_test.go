package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/repository"
)

func seedSynopticLibrary(t *testing.T, fx *testFixture) (book1ID, book2ID, author1ID, genreID string) {
	t.Helper()
	ctx := context.Background()

	author1, err := fx.repo.UpsertAuthor(ctx, "Frank Herbert")
	if err != nil {
		t.Fatalf("upsert author1: %v", err)
	}
	author2, err := fx.repo.UpsertAuthor(ctx, "Beatrix Potter")
	if err != nil {
		t.Fatalf("upsert author2: %v", err)
	}
	genre, err := fx.repo.UpsertGenre(ctx, "Classic Fiction")
	if err != nil {
		t.Fatalf("upsert genre: %v", err)
	}

	book1ID = "syn-book-dune"
	book2ID = "syn-book-rabbit"
	books := []struct {
		id      string
		title   string
		author  *repository.Author
		content string
	}{
		{
			id:      book1ID,
			title:   "Dune",
			author:  author1,
			content: "Fear is the mind-killer during the Gom Jabbar test of endurance.",
		},
		{
			id:      book2ID,
			title:   "The Tale of Peter Rabbit",
			author:  author2,
			content: "Peter felt terrible fear while escaping Mr. McGregor's garden.",
		},
	}

	vec := make([]float32, 256)
	vec[0] = 0.1

	for _, b := range books {
		if err := fx.repo.CreateBook(ctx, &repository.Book{
			ID:       b.id,
			Title:    b.title,
			FilePath: b.title + ".epub",
		}); err != nil {
			t.Fatalf("create book %s: %v", b.id, err)
		}
		if err := fx.repo.LinkBookAuthor(ctx, b.id, b.author.ID, "author"); err != nil {
			t.Fatalf("link author %s: %v", b.id, err)
		}
		if err := fx.repo.LinkBookGenre(ctx, b.id, genre.ID); err != nil {
			t.Fatalf("link genre %s: %v", b.id, err)
		}

		chTitle := "Chapter 1"
		chID := "ch-" + b.id
		if err := fx.repo.CreateChapter(ctx, &repository.Chapter{
			ID:           chID,
			BookID:       b.id,
			ChapterIndex: 1,
			Title:        &chTitle,
			Summary:      b.content,
			ContentPlain: b.content,
		}); err != nil {
			t.Fatalf("create chapter %s: %v", chID, err)
		}

		pid := "p-" + b.id + "-0"
		if err := fx.repo.CreateParagraphs(ctx, []*repository.Paragraph{
			{
				ID:             pid,
				BookID:         b.id,
				ChapterID:      chID,
				ChapterIndex:   1,
				StartParagraph: 0,
				EndParagraph:   1,
				Content:        b.content,
				IsEmbedded:     true,
			},
		}); err != nil {
			t.Fatalf("create paragraph %s: %v", pid, err)
		}
		if err := fx.repo.InsertParagraphVector(ctx, pid, vec); err != nil {
			t.Fatalf("insert vector %s: %v", pid, err)
		}
	}

	return book1ID, book2ID, author1.ID, genre.ID
}

func TestSynopticQueryEndpointAuthAndValidation(t *testing.T) {
	fx := setupAPITest(t)
	token := fx.loginAndGetToken(t)

	// Unauthenticated request -> 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewBufferString(`{"query":"fear","book_ids":["a","b"]}`))
	unauthRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", unauthRec.Code)
	}

	badPayloads := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{not-json`},
		{name: "empty query", body: `{"query":"   ","book_ids":["a","b"]}`},
		{name: "missing scope", body: `{"query":"fear"}`},
		{name: "single book without author or genre", body: `{"query":"fear","book_ids":["a"]}`},
	}

	for _, tc := range badPayloads {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			fx.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
			var errResp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil || errResp["error"] == "" {
				t.Fatalf("expected JSON error payload, got %s", rec.Body.String())
			}
		})
	}
}

func TestSynopticQueryEndpointTwoBooksAndScopes(t *testing.T) {
	fx := setupAPITest(t)
	token := fx.loginAndGetToken(t)
	book1ID, book2ID, author1ID, genreID := seedSynopticLibrary(t, fx)

	fx.aiClient.chatResp = `{
		"summary": "Both works depict characters navigating fear.",
		"consensus_points": [
			{"topic": "Fear under pressure", "description": "Both texts confront fear directly.", "citation_indices": [0, 1]}
		],
		"divergence_points": [
			{"topic": "Setting", "description": "Desert ritual vs garden escape.", "citation_indices": [0, 1]}
		]
	}`

	body, _ := json.Marshal(api.SynopticQueryRequest{
		Query:   "How is fear portrayed?",
		BookIDs: []string{book1ID, book2ID},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	fx.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.SynopticQueryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode synoptic response: %v", err)
	}
	if resp.BooksCovered != 2 {
		t.Fatalf("expected books_covered=2, got %d", resp.BooksCovered)
	}
	if len(resp.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(resp.Citations))
	}
	if len(resp.ConsensusPoints) != 1 || len(resp.DivergencePoints) != 1 {
		t.Fatalf("expected 1 consensus and 1 divergence point, got %+v", resp)
	}
	seenBooks := map[string]bool{}
	for _, c := range resp.Citations {
		seenBooks[c.BookID] = true
		if c.BookTitle == "" || c.AuthorName == "" || c.ChapterID == "" || c.ChapterIndex != 1 {
			t.Fatalf("incomplete citation fields: %+v", c)
		}
	}
	if !seenBooks[book1ID] || !seenBooks[book2ID] {
		t.Fatalf("expected citations from both books, got %+v", seenBooks)
	}

	// Author scope restricts citations to author1's book only
	authorBody, _ := json.Marshal(api.SynopticQueryRequest{
		Query:     "fear",
		AuthorIDs: []string{author1ID},
	})
	authorReq := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewReader(authorBody))
	authorReq.Header.Set("Authorization", "Bearer "+token)
	authorRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(authorRec, authorReq)
	if authorRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for author scope, got %d: %s", authorRec.Code, authorRec.Body.String())
	}
	var authorResp api.SynopticQueryResponse
	if err := json.Unmarshal(authorRec.Body.Bytes(), &authorResp); err != nil {
		t.Fatalf("decode author scope response: %v", err)
	}
	if len(authorResp.Citations) != 1 || authorResp.Citations[0].BookID != book1ID {
		t.Fatalf("expected only %s for author scope, got %+v", book1ID, authorResp.Citations)
	}

	// Genre scope returns both books in the genre
	genreBody, _ := json.Marshal(api.SynopticQueryRequest{
		Query:   "fear",
		GenreID: &genreID,
	})
	genreReq := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewReader(genreBody))
	genreReq.Header.Set("Authorization", "Bearer "+token)
	genreRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(genreRec, genreReq)
	if genreRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for genre scope, got %d: %s", genreRec.Code, genreRec.Body.String())
	}
	var genreResp api.SynopticQueryResponse
	if err := json.Unmarshal(genreRec.Body.Bytes(), &genreResp); err != nil {
		t.Fatalf("decode genre scope response: %v", err)
	}
	if genreResp.BooksCovered != 2 {
		t.Fatalf("expected 2 books covered for genre scope, got %d", genreResp.BooksCovered)
	}

	// Unknown book IDs return 200 with empty citations rather than 500
	emptyBody, _ := json.Marshal(api.SynopticQueryRequest{
		Query:   "fear",
		BookIDs: []string{"missing-1", "missing-2"},
	})
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/synoptic/query", bytes.NewReader(emptyBody))
	emptyReq.Header.Set("Authorization", "Bearer "+token)
	emptyRec := httptest.NewRecorder()
	fx.handler.ServeHTTP(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for zero matches, got %d: %s", emptyRec.Code, emptyRec.Body.String())
	}
	var emptyResp api.SynopticQueryResponse
	if err := json.Unmarshal(emptyRec.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("decode zero match response: %v", err)
	}
	if len(emptyResp.Citations) != 0 || emptyResp.BooksCovered != 0 || emptyResp.Summary == "" {
		t.Fatalf("unexpected zero match response: %+v", emptyResp)
	}
}
