package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/epub"
	"github.com/shelfd/shelfd/internal/repository"
)

func buildLargeAudioEPUB(t *testing.T, audioBytes []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeStored := func(name string, data []byte) {
		hdr := &zip.FileHeader{Name: name, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("create zip header %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	writeStored("mimetype", []byte("application/epub+zip"))
	writeStored("META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))
	writeStored("OEBPS/content.opf", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">urn:uuid:large-audio-test</dc:identifier>
    <dc:title>Large Embedded Audio Book</dc:title>
    <dc:creator>Test Author</dc:creator>
    <meta property="media:duration">00:00:09.000</meta>
  </metadata>
  <manifest>
    <item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml" media-overlay="smil1"/>
    <item id="smil1" href="ch1.smil" media-type="application/smil+xml"/>
    <item id="aud1" href="audio/ch1.mp3" media-type="audio/mpeg"/>
    <item id="aud2" href="audio/ch2.mp3" media-type="audio/mpeg"/>
  </manifest>
  <spine><itemref idref="ch1"/></spine>
</package>`))
	writeStored("OEBPS/ch1.xhtml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <p><span id="s1">Hello world.</span> <span id="s2">Second track sentence.</span></p>
</body></html>`))
	writeStored("OEBPS/ch1.smil", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0">
  <body>
    <par id="p1"><text src="ch1.xhtml#s1"/><audio src="audio/ch1.mp3" clipBegin="0s" clipEnd="4s"/></par>
    <par id="p2"><text src="ch1.xhtml#s2"/><audio src="audio/ch2.mp3" clipBegin="0s" clipEnd="5s"/></par>
  </body>
</smil>`))
	writeStored("OEBPS/audio/ch1.mp3", audioBytes)
	writeStored("OEBPS/audio/ch2.mp3", []byte("ID3-second-track-payload"))

	if err := zw.Close(); err != nil {
		t.Fatalf("close epub zip: %v", err)
	}
	return buf.Bytes()
}

func TestCastMediaJWTScopeAndEndpoints(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	_ = f.loginAndGetToken(t)
	user, err := f.repo.GetUserByUsername(context.Background(), "admin")
	if err != nil || user == nil {
		t.Fatalf("get admin user: %v", err)
	}

	// Seed a read-along EPUB with an embedded 16 MB audio file (> 15 MB text limit) and a cover image.
	largeAudio := make([]byte, 16*1024*1024+128)
	for i := range largeAudio {
		largeAudio[i] = byte((i * 31) % 251)
	}
	epubBytes := buildLargeAudioEPUB(t, largeAudio)

	bookRel := filepath.Join("Test Author", "Large Audio Book", "Large Audio Book.epub")
	bookFull := filepath.Join(f.libDir, bookRel)
	if err := os.MkdirAll(filepath.Dir(bookFull), 0755); err != nil {
		t.Fatalf("mkdir book dir: %v", err)
	}
	if err := os.WriteFile(bookFull, epubBytes, 0644); err != nil {
		t.Fatalf("write epub: %v", err)
	}

	coverBytes := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0}, 128)
	coverRel := filepath.Join("Test Author", "Large Audio Book", "cover.jpg")
	if err := os.WriteFile(filepath.Join(f.libDir, coverRel), coverBytes, 0644); err != nil {
		t.Fatalf("write cover: %v", err)
	}

	book := &repository.Book{
		Title:     "Large Audio Book",
		FilePath:  bookRel,
		CoverPath: &coverRel,
		BookType:  "ebook",
	}
	if err := f.repo.CreateBook(context.Background(), book); err != nil {
		t.Fatalf("create book: %v", err)
	}

	otherBook := &repository.Book{
		Title:    "Other Book",
		FilePath: "Other/Other.epub",
		BookType: "ebook",
	}
	if err := f.repo.CreateBook(context.Background(), otherBook); err != nil {
		t.Fatalf("create other book: %v", err)
	}

	castJWT, _, err := api.GenerateCastMediaJWT(user.ID, user.Username, book.ID, f.jwtSecret, 12*time.Hour)
	if err != nil {
		t.Fatalf("GenerateCastMediaJWT: %v", err)
	}

	// 1. Cover HEAD and byte-range GET with scoped ?token= query auth.
	req := httptest.NewRequest(http.MethodHead, fmt.Sprintf("/api/v1/books/%s/cover?token=%s", book.ID, castJWT), nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD cover expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("expected Accept-Ranges: bytes on HEAD cover, got %q", rec.Header().Get("Accept-Ranges"))
	}

	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/cover?token=%s", book.ID, castJWT), nil)
	req.Header.Set("Range", "bytes=0-63")
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != 64 {
		t.Fatalf("GET cover range expected 206 len=64, got %d len=%d", rec.Code, rec.Body.Len())
	}

	// 2. Asset HEAD and > 15 MB tail range GET without truncation.
	assetURL := fmt.Sprintf("/api/v1/books/%s/assets/OEBPS/audio/ch1.mp3?token=%s", book.ID, castJWT)
	req = httptest.NewRequest(http.MethodHead, assetURL, nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD asset expected 200, got %d", rec.Code)
	}

	tailStart := len(largeAudio) - 128
	tailEnd := len(largeAudio) - 1
	req = httptest.NewRequest(http.MethodGet, assetURL, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", tailStart, tailEnd))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("GET large audio tail expected 206, got %d", rec.Code)
	}
	wantSum := sha256.Sum256(largeAudio[tailStart:])
	gotSum := sha256.Sum256(rec.Body.Bytes())
	if wantSum != gotSum {
		t.Fatalf("tail bytes mismatch: audio truncated or corrupted")
	}

	// 3. Read-along WebVTT global and per-track endpoints.
	vttURL := fmt.Sprintf("/api/v1/books/%s/readalong.vtt?token=%s", book.ID, castJWT)
	req = httptest.NewRequest(http.MethodHead, vttURL, nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/vtt") {
		t.Fatalf("HEAD readalong.vtt expected 200 text/vtt, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}

	req = httptest.NewRequest(http.MethodGet, vttURL, nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET readalong.vtt expected 200, got %d", rec.Code)
	}
	vttBody := rec.Body.String()
	if !strings.HasPrefix(vttBody, "WEBVTT\n\n") || !strings.Contains(vttBody, "00:00:00.000 --> 00:00:04.000\nHello world.") || !strings.Contains(vttBody, "00:00:04.000 --> 00:00:09.000\nSecond track sentence.") {
		t.Fatalf("unexpected global WebVTT body:\n%s", vttBody)
	}

	trackVttURL := fmt.Sprintf("/api/v1/books/%s/readalong.vtt?track=OEBPS/audio/ch2.mp3&token=%s", book.ID, castJWT)
	req = httptest.NewRequest(http.MethodGet, trackVttURL, nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET per-track readalong.vtt expected 200, got %d", rec.Code)
	}
	trackBody := rec.Body.String()
	if strings.Contains(trackBody, "Hello world.") || !strings.Contains(trackBody, "00:00:00.000 --> 00:00:05.000\nSecond track sentence.") {
		t.Fatalf("unexpected per-track WebVTT body:\n%s", trackBody)
	}

	// 4. Scope deny matrix: cross-book, catalog list, and mutations must return 403.
	denied := []struct {
		method string
		path   string
	}{
		{http.MethodGet, fmt.Sprintf("/api/v1/books/%s/cover?token=%s", otherBook.ID, castJWT)},
		{http.MethodGet, fmt.Sprintf("/api/v1/books?token=%s", castJWT)},
		{http.MethodPost, fmt.Sprintf("/api/v1/audiobooks/%s/progress?token=%s", book.ID, castJWT)},
		{http.MethodDelete, fmt.Sprintf("/api/v1/audiobooks/%s/progress?token=%s", book.ID, castJWT)},
	}
	for _, tc := range denied {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s expected 403 Forbidden, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func BenchmarkCastMediaEndpoints(b *testing.B) {
	segments := make([]epub.MediaOverlaySegment, 1000)
	for i := range segments {
		start := float64(i) * 1.5
		segments[i] = epub.MediaOverlaySegment{
			ChapterIndex: i / 50,
			FragmentID:   fmt.Sprintf("w%d", i+1),
			Text:         fmt.Sprintf("Narration cue number %d for read-along playback.", i+1),
			AudioSrc:     "OEBPS/audio/book.mp3",
			ClipBegin:    start,
			ClipEnd:      start + 1.5,
			GlobalBegin:  start,
			GlobalEnd:    start + 1.5,
		}
	}

	b.Run("RenderWebVTT1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			out := api.RenderReadalongWebVTT(segments, "")
			if len(out) == 0 {
				b.Fatal("empty vtt")
			}
		}
	})

	b.Run("CastScopeAllowsRequest", func(b *testing.B) {
		scopes := []string{"cast:media:01ARZ3NDEKTSV4RRFFQ69G5FAV"}
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if !api.CastScopeAllowsRequest(scopes, http.MethodGet, "/api/v1/audiobooks/01ARZ3NDEKTSV4RRFFQ69G5FAV/stream") {
				b.Fatal("expected allowed")
			}
		}
	})
}
