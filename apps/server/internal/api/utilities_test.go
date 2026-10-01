package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/ulid"
)

func TestAPI_Utilities_FindDuplicates_EmptyAndSingle(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	// 0 books
	req := httptest.NewRequest(http.MethodGet, "/api/v1/utilities/duplicates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var res api.DuplicateScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if res.TotalGroups != 0 || len(res.Groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", res.TotalGroups)
	}

	// 1 book
	b1 := &repository.Book{
		ID:        ulid.New(),
		Title:     "Dune",
		FilePath:  "/tmp/dune.epub",
		CreatedAt: time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, b1); err != nil {
		t.Fatalf("creating book: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/utilities/duplicates", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec2.Code)
	}
	var res2 api.DuplicateScanResponse
	_ = json.NewDecoder(rec2.Body).Decode(&res2)
	if res2.TotalGroups != 0 {
		t.Fatalf("expected 0 groups for single book, got %d", res2.TotalGroups)
	}
}

func TestAPI_Utilities_FindDuplicates_MatchingISBNAndTitle(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	author, err := f.repo.UpsertAuthor(ctx, "Frank Herbert")
	if err != nil {
		t.Fatalf("upserting author: %v", err)
	}

	isbn := "978-0441172719"
	b1 := &repository.Book{
		ID:         ulid.New(),
		Title:      "Dune",
		Identifier: &isbn,
		FilePath:   "/tmp/dune_copy1.epub",
		CreatedAt:  time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, b1); err != nil {
		t.Fatalf("creating b1: %v", err)
	}
	_ = f.repo.LinkBookAuthor(ctx, b1.ID, author.ID, "author")

	isbn2 := "urn:isbn:9780441172719"
	b2 := &repository.Book{
		ID:         ulid.New(),
		Title:      "Dune (Special Edition)",
		Identifier: &isbn2,
		FilePath:   "/tmp/dune_copy2.epub",
		CreatedAt:  time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, b2); err != nil {
		t.Fatalf("creating b2: %v", err)
	}
	_ = f.repo.LinkBookAuthor(ctx, b2.ID, author.ID, "author")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/utilities/duplicates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var res api.DuplicateScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if res.TotalGroups != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", res.TotalGroups)
	}
	if res.TotalDuplicateBooks != 2 {
		t.Fatalf("expected 2 total duplicate books, got %d", res.TotalDuplicateBooks)
	}
	if res.Groups[0].Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0 for ISBN match, got %f", res.Groups[0].Confidence)
	}
}

func TestAPI_Utilities_MergeBooks_Success(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	author1, _ := f.repo.UpsertAuthor(ctx, "Isaac Asimov")
	author2, _ := f.repo.UpsertAuthor(ctx, "Robert Silverberg")
	genre1, _ := f.repo.UpsertGenre(ctx, "Science Fiction")
	genre2, _ := f.repo.UpsertGenre(ctx, "Classic Sci-Fi")
	topic1, _ := f.repo.UpsertTopic(ctx, "Robotics")

	fileA := filepath.Join(f.dataDir, "nightfall_primary.epub")
	fileB := filepath.Join(f.dataDir, "nightfall_duplicate.epub")
	_ = os.WriteFile(fileA, []byte("epub-a"), 0o644)
	_ = os.WriteFile(fileB, []byte("epub-b"), 0o644)

	bPrimary := &repository.Book{
		ID:        ulid.New(),
		Title:     "Nightfall",
		FilePath:  fileA,
		CreatedAt: time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, bPrimary); err != nil {
		t.Fatalf("creating primary book: %v", err)
	}
	_ = f.repo.LinkBookAuthor(ctx, bPrimary.ID, author1.ID, "author")
	_ = f.repo.LinkBookGenre(ctx, bPrimary.ID, genre1.ID)

	dupDesc := "A famous science fiction novel."
	dupPub := "Doubleday"
	bDup := &repository.Book{
		ID:          ulid.New(),
		Title:       "Nightfall (Expanded)",
		Description: &dupDesc,
		Publisher:   &dupPub,
		FilePath:    fileB,
		CreatedAt:   time.Now().UTC(),
	}
	if err := f.repo.CreateBook(ctx, bDup); err != nil {
		t.Fatalf("creating duplicate book: %v", err)
	}
	_ = f.repo.LinkBookAuthor(ctx, bDup.ID, author1.ID, "author")
	_ = f.repo.LinkBookAuthor(ctx, bDup.ID, author2.ID, "co-author")
	_ = f.repo.LinkBookGenre(ctx, bDup.ID, genre2.ID)
	_ = f.repo.LinkBookTopic(ctx, bDup.ID, topic1.ID)

	// Create bookmark and highlight on duplicate book
	bm := &repository.Bookmark{
		ID:        ulid.New(),
		BookID:    bDup.ID,
		Title:     "Chapter 3 Climax",
		CreatedAt: time.Now().UTC(),
	}
	_ = f.repo.CreateBookmark(ctx, bm)

	hl := &repository.Highlight{
		ID:           ulid.New(),
		BookID:       bDup.ID,
		SelectedText: "Light came slowly.",
		Color:        "yellow",
		CreatedAt:    time.Now().UTC(),
	}
	_ = f.repo.CreateHighlight(ctx, hl)

	// Merge bDup into bPrimary
	mergePayload := api.MergeBooksRequest{
		PrimaryBookID:    bPrimary.ID,
		DuplicateBookIDs: []string{bDup.ID},
		Options: &api.MergeBooksOptions{
			TransferBookmarks:  boolPtr(true),
			TransferHighlights: boolPtr(true),
			MergeMetadata:      boolPtr(true),
			DeleteFiles:        boolPtr(true),
		},
	}
	body, _ := json.Marshal(mergePayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/utilities/merge-books", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify duplicate book was deleted
	_, err := f.repo.GetBookByID(ctx, bDup.ID)
	if err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound for duplicate book, got %v", err)
	}

	// Verify duplicate file was deleted on disk
	if _, err := os.Stat(fileB); !os.IsNotExist(err) {
		t.Fatalf("expected duplicate file to be deleted from disk, but it exists")
	}

	// Verify primary book retained its file
	if _, err := os.Stat(fileA); err != nil {
		t.Fatalf("expected primary file to remain intact, got: %v", err)
	}

	// Verify primary book got merged metadata
	updatedPrimary, err := f.repo.GetBookByID(ctx, bPrimary.ID)
	if err != nil {
		t.Fatalf("fetching updated primary book: %v", err)
	}
	if updatedPrimary.Description == nil || *updatedPrimary.Description != dupDesc {
		t.Fatalf("expected primary description %q, got %v", dupDesc, updatedPrimary.Description)
	}
	if updatedPrimary.Publisher == nil || *updatedPrimary.Publisher != dupPub {
		t.Fatalf("expected primary publisher %q, got %v", dupPub, updatedPrimary.Publisher)
	}

	// Verify authors and genres merged
	pAuthors, _ := f.repo.GetBookAuthors(ctx, bPrimary.ID)
	if len(pAuthors) != 2 {
		t.Fatalf("expected 2 authors on merged primary, got %d", len(pAuthors))
	}
	pGenres, _ := f.repo.GetBookGenres(ctx, bPrimary.ID)
	if len(pGenres) != 2 {
		t.Fatalf("expected 2 genres on merged primary, got %d", len(pGenres))
	}
	pTopics, _ := f.repo.GetBookTopics(ctx, bPrimary.ID)
	if len(pTopics) != 1 {
		t.Fatalf("expected 1 topic on merged primary, got %d", len(pTopics))
	}

	// Verify bookmark transferred
	bookmarks, _ := f.repo.ListBookmarksByBookID(ctx, bPrimary.ID)
	if len(bookmarks) != 1 || bookmarks[0].ID != bm.ID {
		t.Fatalf("expected 1 transferred bookmark on primary, got %d", len(bookmarks))
	}

	// Verify highlight transferred
	highlights, _ := f.repo.ListHighlightsByBookID(ctx, bPrimary.ID)
	if len(highlights) != 1 || highlights[0].ID != hl.ID {
		t.Fatalf("expected 1 transferred highlight on primary, got %d", len(highlights))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
