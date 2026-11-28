package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/repository"
)

func TestBookFiles_DownloadAndDelete(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	ctx := context.Background()

	// Create test files on disk in library directory
	bookDir := filepath.Join("Stephen Fry", "Odyssey")
	epubRel := filepath.Join(bookDir, "Odyssey.epub")
	audioRel := filepath.Join(bookDir, "Odyssey.m4b")
	coverRel := filepath.Join(bookDir, "cover.jpg")

	if err := os.MkdirAll(filepath.Join(f.libDir, bookDir), 0755); err != nil {
		t.Fatalf("mkdir book dir: %v", err)
	}
	epubBytes := []byte("fake epub content")
	audioBytes := []byte("fake m4b audio content")
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}

	if err := os.WriteFile(filepath.Join(f.libDir, epubRel), epubBytes, 0644); err != nil {
		t.Fatalf("write epub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.libDir, audioRel), audioBytes, 0644); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(f.libDir, coverRel), coverBytes, 0644); err != nil {
		t.Fatalf("write cover: %v", err)
	}

	epubSize := int64(len(epubBytes))
	audioSize := int64(len(audioBytes))
	coverSize := int64(len(coverBytes))
	duration := 3600.0
	bitrate := 64
	coverPath := coverRel

	book := &repository.Book{
		Title:           "Odyssey",
		FilePath:        epubRel,
		CoverPath:       &coverPath,
		BookType:        "dual",
		DurationSeconds: &duration,
		FileSizeBytes:   &epubSize,
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	otherBook := &repository.Book{
		Title:    "Other Book",
		FilePath: "Other/Book.epub",
		BookType: "ebook",
	}
	if err := f.repo.CreateBook(ctx, otherBook); err != nil {
		t.Fatalf("create other book: %v", err)
	}

	mimeEpub := "application/epub+zip"
	mimeAudio := "audio/mp4"
	mimeCover := "image/jpeg"

	bfEpub := &repository.BookFile{
		BookID:        book.ID,
		FileType:      "epub",
		FilePath:      epubRel,
		FileSizeBytes: &epubSize,
		MimeType:      &mimeEpub,
	}
	bfAudio := &repository.BookFile{
		BookID:          book.ID,
		FileType:        "audiobook",
		FilePath:        audioRel,
		FileSizeBytes:   &audioSize,
		DurationSeconds: &duration,
		BitrateKbps:     &bitrate,
		MimeType:        &mimeAudio,
	}
	bfCover := &repository.BookFile{
		BookID:        book.ID,
		FileType:      "cover",
		FilePath:      coverRel,
		FileSizeBytes: &coverSize,
		MimeType:      &mimeCover,
	}
	if err := f.repo.CreateBookFiles(ctx, []*repository.BookFile{bfEpub, bfAudio, bfCover}); err != nil {
		t.Fatalf("create book files: %v", err)
	}
	if err := f.repo.CreateAudioChapters(ctx, []*repository.AudioChapter{
		{
			BookID:         book.ID,
			ChapterIndex:   0,
			Title:          "Chapter 1",
			StartOffsetSec: 0,
			DurationSec:    3600,
		},
	}); err != nil {
		t.Fatalf("create audio chapters: %v", err)
	}

	// 1. Download cover file
	dlReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/files/%s/download", book.ID, bfCover.ID), nil)
	dlReq.Header.Set("Authorization", "Bearer "+token)
	dlRec := httptest.NewRecorder()
	f.handler.ServeHTTP(dlRec, dlReq)

	if dlRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK downloading cover, got %d: %s", dlRec.Code, dlRec.Body.String())
	}
	if got := dlRec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", got)
	}
	if got := dlRec.Header().Get("Content-Disposition"); !strings.Contains(got, `attachment; filename="cover.jpg"`) {
		t.Errorf("expected attachment filename cover.jpg, got %q", got)
	}
	if !bytes.Equal(dlRec.Body.Bytes(), coverBytes) {
		t.Errorf("downloaded cover bytes mismatch")
	}

	// 2. Download with mismatched book ID returns 404
	badDlReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/files/%s/download", otherBook.ID, bfCover.ID), nil)
	badDlReq.Header.Set("Authorization", "Bearer "+token)
	badDlRec := httptest.NewRecorder()
	f.handler.ServeHTTP(badDlRec, badDlReq)
	if badDlRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for mismatched book ID download, got %d", badDlRec.Code)
	}

	// 3. Delete with mismatched book ID returns 404
	badDelReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/books/%s/files/%s", otherBook.ID, bfCover.ID), nil)
	badDelReq.Header.Set("Authorization", "Bearer "+token)
	badDelRec := httptest.NewRecorder()
	f.handler.ServeHTTP(badDelRec, badDelReq)
	if badDelRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for mismatched book ID delete, got %d", badDelRec.Code)
	}

	// 4. Delete cover file removes from disk, DB, and clears book.CoverPath
	delCoverReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/books/%s/files/%s", book.ID, bfCover.ID), nil)
	delCoverReq.Header.Set("Authorization", "Bearer "+token)
	delCoverRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delCoverRec, delCoverReq)
	if delCoverRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting cover, got %d: %s", delCoverRec.Code, delCoverRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(f.libDir, coverRel)); !os.IsNotExist(err) {
		t.Errorf("expected cover file to be removed from disk, stat err=%v", err)
	}
	updatedBook, err := f.repo.GetBookByID(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book after cover delete: %v", err)
	}
	if updatedBook.CoverPath != nil {
		t.Errorf("expected book.CoverPath to be nil, got %q", *updatedBook.CoverPath)
	}

	// 5. Delete audiobook file clears audio chapters, duration, and reconciles BookType to ebook
	delAudioReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/books/%s/files/%s", book.ID, bfAudio.ID), nil)
	delAudioReq.Header.Set("Authorization", "Bearer "+token)
	delAudioRec := httptest.NewRecorder()
	f.handler.ServeHTTP(delAudioRec, delAudioReq)
	if delAudioRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting audio, got %d: %s", delAudioRec.Code, delAudioRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(f.libDir, audioRel)); !os.IsNotExist(err) {
		t.Errorf("expected audio file to be removed from disk, stat err=%v", err)
	}
	updatedBook, err = f.repo.GetBookByID(ctx, book.ID)
	if err != nil {
		t.Fatalf("get book after audio delete: %v", err)
	}
	if updatedBook.DurationSeconds != nil {
		t.Errorf("expected DurationSeconds to be nil, got %v", *updatedBook.DurationSeconds)
	}
	if updatedBook.BookType != "ebook" {
		t.Errorf("expected BookType to be ebook, got %q", updatedBook.BookType)
	}
	chapters, err := f.repo.GetAudioChaptersByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("get audio chapters: %v", err)
	}
	if len(chapters) != 0 {
		t.Errorf("expected 0 audio chapters, got %d", len(chapters))
	}
	remaining, err := f.repo.GetBookFilesByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("get remaining book files: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != bfEpub.ID {
		t.Errorf("expected only epub file remaining, got %d files", len(remaining))
	}
}
