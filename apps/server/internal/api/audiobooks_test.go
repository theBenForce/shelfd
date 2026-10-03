package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/repository"
)

func TestAudiobooks_StreamingAndRanges(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// Create test audio file in library directory
	relPath := filepath.Join("Author Name", "Test Audio Book", "Test Audio Book.m4b")
	fullPath := filepath.Join(f.libDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir audio dir: %v", err)
	}
	// Generate dummy audio content (10,000 bytes)
	audioData := make([]byte, 10000)
	for i := range audioData {
		audioData[i] = byte(i % 256)
	}
	if err := os.WriteFile(fullPath, audioData, 0644); err != nil {
		t.Fatalf("write audio file: %v", err)
	}

	// Create book in repository
	fileSize := int64(len(audioData))
	duration := 120.0
	bitrate := 128
	book := &repository.Book{
		Title:           "Test Audio Book",
		FilePath:        relPath,
		BookType:        "audiobook",
		DurationSeconds: &duration,
		FileSizeBytes:   &fileSize,
	}
	if err := f.repo.CreateBook(context.Background(), book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	// Add associated book files
	mimeAudio := "audio/mp4"
	bf := &repository.BookFile{
		BookID:          book.ID,
		FileType:        "audiobook",
		FilePath:        relPath,
		FileSizeBytes:   &fileSize,
		DurationSeconds: &duration,
		BitrateKbps:     &bitrate,
		MimeType:        &mimeAudio,
	}
	if err := f.repo.CreateBookFiles(context.Background(), []*repository.BookFile{bf}); err != nil {
		t.Fatalf("create book files: %v", err)
	}

	// 1. Full stream request (GET, 200 OK)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for full audio stream, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes, got %s", rec.Header().Get("Accept-Ranges"))
	}
	if rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Errorf("expected Content-Type: audio/mp4, got %s", rec.Header().Get("Content-Type"))
	}
	if rec.Body.Len() != len(audioData) {
		t.Fatalf("expected body length %d, got %d", len(audioData), rec.Body.Len())
	}

	// 2. HEAD request capability check
	headReq := httptest.NewRequest(http.MethodHead, fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID), nil)
	headReq.Header.Set("Authorization", "Bearer "+token)
	headRec := httptest.NewRecorder()
	f.handler.ServeHTTP(headRec, headReq)

	if headRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD request, got %d", headRec.Code)
	}
	if headRec.Header().Get("Accept-Ranges") != "bytes" {
		t.Errorf("expected Accept-Ranges: bytes on HEAD request")
	}
	if headRec.Body.Len() != 0 {
		t.Errorf("expected empty body on HEAD request, got %d bytes", headRec.Body.Len())
	}

	// 3. Partial Content Byte-Range Request (Range: bytes=0-499 -> 500 bytes, 206 Partial Content)
	rangeReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID), nil)
	rangeReq.Header.Set("Authorization", "Bearer "+token)
	rangeReq.Header.Set("Range", "bytes=0-499")
	rangeRec := httptest.NewRecorder()
	f.handler.ServeHTTP(rangeRec, rangeReq)

	if rangeRec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d: %s", rangeRec.Code, rangeRec.Body.String())
	}
	if rangeRec.Header().Get("Content-Range") != "bytes 0-499/10000" {
		t.Errorf("expected Content-Range: bytes 0-499/10000, got %s", rangeRec.Header().Get("Content-Range"))
	}
	if rangeRec.Body.Len() != 500 {
		t.Fatalf("expected 500 bytes body, got %d", rangeRec.Body.Len())
	}
	if !bytes.Equal(rangeRec.Body.Bytes(), audioData[0:500]) {
		t.Errorf("range response bytes mismatch")
	}

	// 4. Middle Byte-Range Request (Range: bytes=5000-5999 -> 1000 bytes)
	rangeReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID), nil)
	rangeReq2.Header.Set("Authorization", "Bearer "+token)
	rangeReq2.Header.Set("Range", "bytes=5000-5999")
	rangeRec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(rangeRec2, rangeReq2)

	if rangeRec2.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d: %s", rangeRec2.Code, rangeRec2.Body.String())
	}
	if rangeRec2.Header().Get("Content-Range") != "bytes 5000-5999/10000" {
		t.Errorf("expected Content-Range: bytes 5000-5999/10000, got %s", rangeRec2.Header().Get("Content-Range"))
	}
	if rangeRec2.Body.Len() != 1000 {
		t.Fatalf("expected 1000 bytes body, got %d", rangeRec2.Body.Len())
	}
	if !bytes.Equal(rangeRec2.Body.Bytes(), audioData[5000:6000]) {
		t.Errorf("range response bytes mismatch")
	}

	// 5. Invalid Range Request (Range: bytes=20000-30000 -> 416 Range Not Satisfiable)
	invReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/stream", book.ID), nil)
	invReq.Header.Set("Authorization", "Bearer "+token)
	invReq.Header.Set("Range", "bytes=20000-30000")
	invRec := httptest.NewRecorder()
	f.handler.ServeHTTP(invRec, invReq)

	if invRec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("expected 416 Range Not Satisfiable, got %d: %s", invRec.Code, invRec.Body.String())
	}

	// 6. Generic Book File Streaming endpoint
	bfList, _ := f.repo.GetBookFilesByBookID(context.Background(), book.ID)
	if len(bfList) == 0 {
		t.Fatalf("expected book file")
	}
	fileStreamReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/files/%s/stream", book.ID, bfList[0].ID), nil)
	fileStreamReq.Header.Set("Authorization", "Bearer "+token)
	fileStreamReq.Header.Set("Range", "bytes=100-199")
	fileStreamRec := httptest.NewRecorder()
	f.handler.ServeHTTP(fileStreamRec, fileStreamReq)

	if fileStreamRec.Code != http.StatusPartialContent {
		t.Fatalf("expected 206 on file stream, got %d: %s", fileStreamRec.Code, fileStreamRec.Body.String())
	}
	if fileStreamRec.Body.Len() != 100 {
		t.Fatalf("expected 100 bytes, got %d", fileStreamRec.Body.Len())
	}

	// 7. Non-existent book (404)
	nonReq := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/non-existent-id/stream", nil)
	nonReq.Header.Set("Authorization", "Bearer "+token)
	nonRec := httptest.NewRecorder()
	f.handler.ServeHTTP(nonRec, nonReq)

	if nonRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent book, got %d", nonRec.Code)
	}
}

func TestAudiobooks_ChaptersAndProgress(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// Create book
	duration := 3600.0
	book := &repository.Book{
		Title:           "Chaptered Audiobook",
		FilePath:        "Author/Title/Title.m4b",
		BookType:        "audiobook",
		DurationSeconds: &duration,
	}
	if err := f.repo.CreateBook(context.Background(), book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	// Create audio chapters
	chapters := []*repository.AudioChapter{
		{
			BookID:         book.ID,
			ChapterIndex:   1,
			Title:          "Prologue",
			StartOffsetSec: 0.0,
			DurationSec:    300.0,
		},
		{
			BookID:         book.ID,
			ChapterIndex:   2,
			Title:          "Chapter 1: The Beginning",
			StartOffsetSec: 300.0,
			DurationSec:    1200.0,
		},
	}
	if err := f.repo.CreateAudioChapters(context.Background(), chapters); err != nil {
		t.Fatalf("create chapters: %v", err)
	}

	// 1. Get Chapters (GET /api/v1/audiobooks/{id}/chapters)
	chReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/chapters", book.ID), nil)
	chReq.Header.Set("Authorization", "Bearer "+token)
	chRec := httptest.NewRecorder()
	f.handler.ServeHTTP(chRec, chReq)

	if chRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for chapters, got %d: %s", chRec.Code, chRec.Body.String())
	}
	var fetchedChapters []*repository.AudioChapter
	if err := json.Unmarshal(chRec.Body.Bytes(), &fetchedChapters); err != nil {
		t.Fatalf("unmarshal chapters: %v", err)
	}
	if len(fetchedChapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(fetchedChapters))
	}
	if fetchedChapters[0].Title != "Prologue" || fetchedChapters[1].Title != "Chapter 1: The Beginning" {
		t.Errorf("unexpected chapter titles: %+v", fetchedChapters)
	}

	// 2. Get Default Progress (GET /api/v1/audiobooks/{id}/progress)
	progReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/progress", book.ID), nil)
	progReq.Header.Set("Authorization", "Bearer "+token)
	progRec := httptest.NewRecorder()
	f.handler.ServeHTTP(progRec, progReq)

	if progRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for initial progress, got %d: %s", progRec.Code, progRec.Body.String())
	}
	var initialProg repository.AudiobookProgress
	if err := json.Unmarshal(progRec.Body.Bytes(), &initialProg); err != nil {
		t.Fatalf("unmarshal initial progress: %v", err)
	}
	if initialProg.PositionSeconds != 0.0 || initialProg.Speed != 1.0 {
		t.Errorf("unexpected initial progress: %+v", initialProg)
	}

	// 3. Save Progress (POST /api/v1/audiobooks/{id}/progress)
	saveBody := `{"position_seconds": 450.5, "speed": 1.25, "is_completed": false}`
	postReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/audiobooks/%s/progress", book.ID), bytes.NewBufferString(saveBody))
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")
	postRec := httptest.NewRecorder()
	f.handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on save progress, got %d: %s", postRec.Code, postRec.Body.String())
	}
	var savedProg repository.AudiobookProgress
	if err := json.Unmarshal(postRec.Body.Bytes(), &savedProg); err != nil {
		t.Fatalf("unmarshal saved progress: %v", err)
	}
	if savedProg.PositionSeconds != 450.5 || savedProg.Speed != 1.25 || savedProg.IsCompleted {
		t.Errorf("unexpected saved progress: %+v", savedProg)
	}

	// 4. Retrieve Saved Progress (GET /api/v1/audiobooks/{id}/progress)
	getReq2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audiobooks/%s/progress", book.ID), nil)
	getReq2.Header.Set("Authorization", "Bearer "+token)
	getRec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(getRec2, getReq2)

	if getRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get saved progress, got %d", getRec2.Code)
	}
	var fetchedProg repository.AudiobookProgress
	if err := json.Unmarshal(getRec2.Body.Bytes(), &fetchedProg); err != nil {
		t.Fatalf("unmarshal fetched progress: %v", err)
	}
	if fetchedProg.PositionSeconds != 450.5 || fetchedProg.Speed != 1.25 {
		t.Errorf("unexpected fetched progress: %+v", fetchedProg)
	}

	// 5. Delete Progress (DELETE /api/v1/audiobooks/{id}/progress)
	delReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/audiobooks/%s/progress", book.ID), nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete progress, got %d", delRec.Code)
	}

	// 6. Test Book Details endpoint includes Files and AudioChapters
	bookDetailReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s", book.ID), nil)
	bookDetailReq.Header.Set("Authorization", "Bearer "+token)
	bookDetailRec := httptest.NewRecorder()
	f.handler.ServeHTTP(bookDetailRec, bookDetailReq)

	if bookDetailRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for book detail, got %d: %s", bookDetailRec.Code, bookDetailRec.Body.String())
	}
	var bookDetail api.BookDetailResponse
	if err := json.Unmarshal(bookDetailRec.Body.Bytes(), &bookDetail); err != nil {
		t.Fatalf("unmarshal book detail: %v", err)
	}
	if len(bookDetail.AudioChapters) != 2 {
		t.Fatalf("expected 2 audio chapters in book detail, got %d", len(bookDetail.AudioChapters))
	}
	if bookDetail.BookType != "audiobook" {
		t.Errorf("expected book_type audiobook, got %s", bookDetail.BookType)
	}
}
