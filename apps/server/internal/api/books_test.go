package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/repository"
)

func seedMarkdownTestBook(t *testing.T, f *testFixture) *repository.Book {
	t.Helper()
	ctx := context.Background()

	book := &repository.Book{
		ID:            "book-nlm-1",
		Title:         "Solaris: The Novel?",
		Description:   strPtr("A psychological sci-fi classic about a sentient ocean."),
		Publisher:     strPtr("Walker & Co."),
		PublishedDate: strPtr("1961"),
		Language:      strPtr("en"),
		Identifier:    strPtr("ISBN-9780156027601"),
		FilePath:      "Stanislaw Lem/Solaris/Solaris.epub",
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}

	author, err := f.repo.UpsertAuthor(ctx, "Stanislaw Lem")
	if err != nil {
		t.Fatalf("UpsertAuthor failed: %v", err)
	}
	if err := f.repo.LinkBookAuthor(ctx, book.ID, author.ID, "author"); err != nil {
		t.Fatalf("LinkBookAuthor failed: %v", err)
	}

	series, err := f.repo.UpsertSeries(ctx, "SF Masterworks", nil)
	if err != nil {
		t.Fatalf("UpsertSeries failed: %v", err)
	}
	seq := 12.0
	if err := f.repo.LinkBookSeries(ctx, book.ID, series.ID, &seq); err != nil {
		t.Fatalf("LinkBookSeries failed: %v", err)
	}

	genre, err := f.repo.UpsertGenre(ctx, "Science Fiction")
	if err != nil {
		t.Fatalf("UpsertGenre failed: %v", err)
	}
	if err := f.repo.LinkBookGenre(ctx, book.ID, genre.ID); err != nil {
		t.Fatalf("LinkBookGenre failed: %v", err)
	}

	topic, err := f.repo.UpsertTopic(ctx, "First Contact")
	if err != nil {
		t.Fatalf("UpsertTopic failed: %v", err)
	}
	if err := f.repo.LinkBookTopic(ctx, book.ID, topic.ID); err != nil {
		t.Fatalf("LinkBookTopic failed: %v", err)
	}

	ch1 := &repository.Chapter{
		ID:           "ch-nlm-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        strPtr("Arrival"),
		Summary:      "Kelvin arrives aboard the Prometheus station.",
		ContentPlain: "At 19:00 hours, Kris Kelvin entered the arrival capsule.",
	}
	ch2 := &repository.Chapter{
		ID:           "ch-nlm-2",
		BookID:       book.ID,
		ChapterIndex: 2,
		Title:        strPtr("The Ocean"),
		Summary:      "Kelvin observes the cytoplasmic ocean of Solaris.",
		ContentPlain: "The shifting mimoids rose above the horizon of the living sea.",
	}
	if err := f.repo.CreateChapter(ctx, ch1); err != nil {
		t.Fatalf("CreateChapter 1 failed: %v", err)
	}
	if err := f.repo.CreateChapter(ctx, ch2); err != nil {
		t.Fatalf("CreateChapter 2 failed: %v", err)
	}

	hl := &repository.Highlight{
		ID:           "hl-nlm-1",
		BookID:       book.ID,
		ChapterID:    strPtr(ch1.ID),
		SelectedText: "We don't need other worlds. We need mirrors.",
		Note:         strPtr("Key philosophical theme of the book"),
		Color:        "yellow",
	}
	if err := f.repo.CreateHighlight(ctx, hl); err != nil {
		t.Fatalf("CreateHighlight failed: %v", err)
	}

	return book
}

func seedNotebookLMTestBook(t *testing.T, f *testFixture) *repository.Book {
	t.Helper()
	return seedMarkdownTestBook(t, f)
}

func TestAPI_ExportBookMarkdown(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	seedMarkdownTestBook(t, f)

	// 1. Unauthenticated request returns 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/markdown", nil)
	unauthRec := httptest.NewRecorder()
	f.handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated export, got %d", unauthRec.Code)
	}

	// 2. Authenticated GET request returns compiled Markdown with expected headers
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/markdown", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Errorf("expected Content-Type text/markdown; charset=utf-8, got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="Solaris_ The Novel.md"` {
		t.Errorf("expected Content-Disposition attachment; filename=\"Solaris_ The Novel.md\", got %q", cd)
	}

	body := rec.Body.String()
	expectedSubstrings := []string{
		"# Solaris: The Novel?",
		"- **Author(s)**: Stanislaw Lem",
		"- **Series**: SF Masterworks (#12)",
		"- **Publisher**: Walker & Co.",
		"- **Published**: 1961",
		"- **Language**: en",
		"- **Identifier**: ISBN-9780156027601",
		"- **Genres**: Science Fiction",
		"- **Topics**: First Contact",
		"## Synopsis",
		"A psychological sci-fi classic about a sentient ocean.",
		"## Highlights & Notes",
		"> We don't need other worlds. We need mirrors.",
		"- **Note**: Key philosophical theme of the book",
		"## Chapters",
		"### Chapter 1: Arrival",
		"*Summary: Kelvin arrives aboard the Prometheus station.*",
		"At 19:00 hours, Kris Kelvin entered the arrival capsule.",
		"### Chapter 2: The Ocean",
		"The shifting mimoids rose above the horizon of the living sea.",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(body, sub) {
			t.Errorf("expected Markdown export to contain %q, got:\n%s", sub, body)
		}
	}

	// 3. Authenticated via ?token= query parameter
	reqQuery := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/markdown?token="+token, nil)
	recQuery := httptest.NewRecorder()
	f.handler.ServeHTTP(recQuery, reqQuery)
	if recQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ?token= query export, got %d", recQuery.Code)
	}

	// 4. Nonexistent book returns 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/books/nonexistent-book/markdown", nil)
	req404.Header.Set("Authorization", "Bearer "+token)
	rec404 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent book, got %d", rec404.Code)
	}
}

func TestAPI_ExportBookMarkdown_EPUBFallback(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	token := f.loginAndGetToken(t)

	// Create valid EPUB archive on disk without any rows in the chapters table
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	m, _ := zw.Create("mimetype")
	_, _ = m.Write([]byte("application/epub+zip"))
	c, _ := zw.Create("META-INF/container.xml")
	_, _ = c.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
	opf, _ := zw.Create("OEBPS/content.opf")
	_, _ = opf.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Fallback EPUB Book</dc:title>
  </metadata>
  <manifest>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
  </spine>
</package>`))
	ch1, _ := zw.Create("OEBPS/ch1.xhtml")
	_, _ = ch1.Write([]byte(`<!DOCTYPE html><html><head><title>Chapter One</title></head><body><h1>Chapter One</h1><p>Extracted live from EPUB archive on disk.</p></body></html>`))
	_ = zw.Close()

	relPath := "FallbackAuthor/Fallback.epub"
	fullPath := filepath.Join(f.libDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write epub: %v", err)
	}

	book := &repository.Book{
		ID:       "book-nlm-fallback",
		Title:    "Fallback EPUB Book",
		FilePath: relPath,
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-fallback/markdown", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Extracted live from EPUB archive on disk.") {
		t.Errorf("expected live EPUB chapter extraction fallback in markdown, got:\n%s", rec.Body.String())
	}
}

func TestAPI_ExportBookMarkdown_And_GetSeries(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	token := f.loginAndGetToken(t)
	book1 := seedMarkdownTestBook(t, f)

	mdReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book1.ID+"/markdown", nil)
	mdReq.Header.Set("Authorization", "Bearer "+token)
	mdRec := httptest.NewRecorder()
	f.handler.ServeHTTP(mdRec, mdReq)
	if mdRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/books/{id}/markdown, got %d: %s", mdRec.Code, mdRec.Body.String())
	}
	if ct := mdRec.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Errorf("expected Content-Type text/markdown; charset=utf-8, got %q", ct)
	}
	mdBody := mdRec.Body.String()
	for _, sub := range []string{
		"# Solaris: The Novel?",
		"We don't need other worlds. We need mirrors.",
		"Key philosophical theme of the book",
		"### Chapter 1: Arrival",
	} {
		if !strings.Contains(mdBody, sub) {
			t.Errorf("expected /markdown export to contain %q, got:\n%s", sub, mdBody)
		}
	}

	seriesList, err := f.repo.ListSeries(ctx)
	if err != nil || len(seriesList) == 0 {
		t.Fatalf("expected seeded series SF Masterworks: err=%v len=%d", err, len(seriesList))
	}
	seriesID := seriesList[0].ID

	book2 := &repository.Book{
		ID:       "book-nlm-2",
		Title:    "Roadside Picnic",
		BookType: "audiobook",
		FilePath: "Strugatsky/Roadside Picnic/Roadside.m4b",
	}
	if err := f.repo.CreateBook(ctx, book2); err != nil {
		t.Fatalf("CreateBook 2 failed: %v", err)
	}
	seq2 := 2.0
	if err := f.repo.LinkBookSeries(ctx, book2.ID, seriesID, &seq2); err != nil {
		t.Fatalf("LinkBookSeries 2 failed: %v", err)
	}
	if err := f.repo.CreateBookFiles(ctx, []*repository.BookFile{
		{
			ID:       "bf-series-2",
			BookID:   book2.ID,
			FilePath: book2.FilePath,
			FileType: "audiobook",
		},
	}); err != nil {
		t.Fatalf("CreateBookFiles failed: %v", err)
	}

	serReq := httptest.NewRequest(http.MethodGet, "/api/v1/series/"+seriesID, nil)
	serReq.Header.Set("Authorization", "Bearer "+token)
	serRec := httptest.NewRecorder()
	f.handler.ServeHTTP(serRec, serReq)
	if serRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/series/{id}, got %d: %s", serRec.Code, serRec.Body.String())
	}

	var serResp struct {
		Series struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"series"`
		Books []struct {
			ID             string   `json:"id"`
			Title          string   `json:"title"`
			SequenceNumber *float64 `json:"sequence_number"`
			Files          []struct {
				ID string `json:"id"`
			} `json:"files"`
		} `json:"books"`
	}
	if err := json.Unmarshal(serRec.Body.Bytes(), &serResp); err != nil {
		t.Fatalf("decode GET /api/v1/series/{id} response: %v", err)
	}
	if serResp.Series.ID != seriesID || serResp.Series.Name != "SF Masterworks" {
		t.Fatalf("unexpected series payload: %+v", serResp.Series)
	}
	if len(serResp.Books) != 2 {
		t.Fatalf("expected 2 books in series, got %d", len(serResp.Books))
	}
	if serResp.Books[0].ID != "book-nlm-2" || serResp.Books[1].ID != "book-nlm-1" {
		t.Fatalf("expected books ordered by sequence_number (2 then 12), got %+v", serResp.Books)
	}
	if len(serResp.Books[0].Files) != 1 || serResp.Books[0].Files[0].ID != "bf-series-2" {
		t.Fatalf("expected files array populated on series book entry, got %+v", serResp.Books[0].Files)
	}
}

func assertNoFilesInUploadsDir(t *testing.T, dataDir string) {
	t.Helper()
	uploadsDir := filepath.Join(dataDir, "uploads")
	entries, err := os.ReadDir(uploadsDir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("failed to read uploadsDir %s: %v", uploadsDir, err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected no leftover files in %s, found: %v", uploadsDir, names)
	}
}

func TestAPI_StageUploadBook_MultipartPartOrderings(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	ctx := context.Background()

	// Audio with blank title/author so metadata_defaults populates them
	audioBytes := createSyntheticMP3("", "", "", 120000, nil)
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x01, 0x02, 0xAA, 0xBB}

	cases := []struct {
		name       string
		externalID string
		title      string
		buildParts func(t *testing.T, mpw *multipart.Writer, metaJSON string)
	}{
		{
			name:       "file_first_then_cover_and_metadata",
			externalID: "odyssey-order-1",
			title:      "The Odyssey (File First)",
			buildParts: func(t *testing.T, mpw *multipart.Writer, metaJSON string) {
				fp, err := mpw.CreateFormFile("file", "odyssey_first.m4b")
				if err != nil {
					t.Fatalf("create file part: %v", err)
				}
				_, _ = fp.Write(audioBytes)

				cp, err := mpw.CreateFormFile("cover", "cover.jpg")
				if err != nil {
					t.Fatalf("create cover part: %v", err)
				}
				_, _ = cp.Write(coverBytes)

				_ = mpw.WriteField("source_plugin", "overdrive")
				_ = mpw.WriteField("source_external_id", "odyssey-order-1")
				_ = mpw.WriteField("metadata_defaults", metaJSON)
			},
		},
		{
			name:       "metadata_and_cover_first_then_file_last",
			externalID: "odyssey-order-2",
			title:      "The Odyssey (File Last)",
			buildParts: func(t *testing.T, mpw *multipart.Writer, metaJSON string) {
				_ = mpw.WriteField("source_plugin", "overdrive")
				_ = mpw.WriteField("source_external_id", "odyssey-order-2")
				_ = mpw.WriteField("metadata_defaults", metaJSON)

				cp, err := mpw.CreateFormFile("cover", "cover.jpg")
				if err != nil {
					t.Fatalf("create cover part: %v", err)
				}
				_, _ = cp.Write(coverBytes)

				fp, err := mpw.CreateFormFile("file", "odyssey_last.m4b")
				if err != nil {
					t.Fatalf("create file part: %v", err)
				}
				_, _ = fp.Write(audioBytes)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			metaDefaults, _ := json.Marshal(map[string]any{
				"title":           tc.title,
				"authors":         []string{"Homer", "Emily Wilson"},
				"series":          "Greek Epics",
				"sequence_number": 2,
				"description":     "Translated by Emily Wilson.",
				"publisher":       "W. W. Norton & Company",
				"language":        "en",
				"genres":          []string{"Classics", "Poetry"},
				"narrator":        "Claire Danes",
			})

			var body bytes.Buffer
			mpw := multipart.NewWriter(&body)
			tc.buildParts(t, mpw, string(metaDefaults))
			_ = mpw.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", mpw.FormDataContentType())
			rec := httptest.NewRecorder()
			f.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp struct {
				JobID    string `json:"job_id"`
				Status   string `json:"status"`
				HasCover bool   `json:"has_cover"`
				Metadata struct {
					Title       string   `json:"title"`
					Authors     []string `json:"authors"`
					Series      *string  `json:"series"`
					Description *string  `json:"description"`
					Publisher   *string  `json:"publisher"`
					Language    *string  `json:"language"`
					Genres      []string `json:"genres"`
					Narrator    *string  `json:"narrator"`
					BookType    string   `json:"book_type"`
				} `json:"metadata"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.Status != "staged" || resp.JobID == "" {
				t.Fatalf("expected staged job, got %+v", resp)
			}
			if !resp.HasCover {
				t.Errorf("expected has_cover=true")
			}
			if resp.Metadata.Title != tc.title {
				t.Errorf("expected title %q, got %q", tc.title, resp.Metadata.Title)
			}
			if len(resp.Metadata.Authors) != 2 || resp.Metadata.Authors[0] != "Homer" || resp.Metadata.Authors[1] != "Emily Wilson" {
				t.Errorf("unexpected authors: %+v", resp.Metadata.Authors)
			}
			if resp.Metadata.Narrator == nil || *resp.Metadata.Narrator != "Claire Danes" {
				t.Errorf("unexpected narrator: %v", resp.Metadata.Narrator)
			}
			if resp.Metadata.Publisher == nil || *resp.Metadata.Publisher != "W. W. Norton & Company" {
				t.Errorf("unexpected publisher: %v", resp.Metadata.Publisher)
			}
			if resp.Metadata.Series == nil || *resp.Metadata.Series != "Greek Epics" {
				t.Errorf("unexpected series: %v", resp.Metadata.Series)
			}

			// Verify cover bytes persisted
			covReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs/"+resp.JobID+"/cover", nil)
			covReq.Header.Set("Authorization", "Bearer "+token)
			covRec := httptest.NewRecorder()
			f.handler.ServeHTTP(covRec, covReq)
			if covRec.Code != http.StatusOK || !bytes.Equal(covRec.Body.Bytes(), coverBytes) {
				t.Fatalf("expected stored cover bytes to match, status=%d", covRec.Code)
			}

			// Verify provenance recorded in book_sources
			sources, err := f.repo.GetBookSourcesByExternalIDs(ctx, "overdrive", []string{tc.externalID})
			if err != nil || len(sources) != 1 {
				t.Fatalf("expected 1 book_source row for %s, got len=%d err=%v", tc.externalID, len(sources), err)
			}
			if sources[0].UploadJobID == nil || *sources[0].UploadJobID != resp.JobID {
				t.Errorf("expected UploadJobID %s on book_source, got %+v", resp.JobID, sources[0].UploadJobID)
			}

			// Clean up staged job so uploadsDir stays tidy
			delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/books/upload/jobs/"+resp.JobID, nil)
			delReq.Header.Set("Authorization", "Bearer "+token)
			delRec := httptest.NewRecorder()
			f.handler.ServeHTTP(delRec, delReq)
			if delRec.Code != http.StatusNoContent {
				t.Fatalf("expected 204 deleting staged job, got %d", delRec.Code)
			}
		})
	}

	assertNoFilesInUploadsDir(t, f.dataDir)
}

type failAfterHeaderReader struct {
	prefix    []byte
	offset    int
	bodyReads int
}

func (r *failAfterHeaderReader) Read(p []byte) (int, error) {
	if r.offset < len(r.prefix) {
		n := copy(p, r.prefix[r.offset:])
		r.offset += n
		return n, nil
	}
	r.bodyReads++
	if r.bodyReads > 32 {
		return 0, io.EOF
	}
	for i := range p {
		p[i] = 'X'
	}
	return len(p), nil
}

func TestAPI_StageUploadBook_RejectsUnsupportedAndMalformedWithoutLeftoverFiles(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x01, 0x02}

	t.Run("unsupported_extension_rejected_before_streaming_body", func(t *testing.T) {
		boundary := "shelfdtestboundary12345"
		headerPrefix := "--" + boundary + "\r\n" +
			`Content-Disposition: form-data; name="cover"; filename="cover.jpg"` + "\r\n" +
			"Content-Type: image/jpeg\r\n\r\n" +
			string(coverBytes) + "\r\n" +
			"--" + boundary + "\r\n" +
			`Content-Disposition: form-data; name="file"; filename="malware.exe"` + "\r\n" +
			"Content-Type: application/octet-stream\r\n\r\n"

		stream := &failAfterHeaderReader{prefix: []byte(headerPrefix)}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", stream)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for unsupported extension, got %d: %s", rec.Code, rec.Body.String())
		}
		if stream.bodyReads > 1 {
			t.Errorf("expected handler to reject unsupported extension before streaming body, but read %d body chunks", stream.bodyReads)
		}
		assertNoFilesInUploadsDir(t, f.dataDir)
	})

	t.Run("truncated_file_stream_cleans_up_partial_staged_file", func(t *testing.T) {
		boundary := "shelfdtruncatedboundary"
		partialStream := "--" + boundary + "\r\n" +
			`Content-Disposition: form-data; name="file"; filename="unfinished.m4b"` + "\r\n" +
			"Content-Type: audio/mp4\r\n\r\n" +
			strings.Repeat("A", 64*1024) // no closing boundary -> UnexpectedEOF

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", strings.NewReader(partialStream))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request on truncated stream, got %d: %s", rec.Code, rec.Body.String())
		}
		assertNoFilesInUploadsDir(t, f.dataDir)
	})

	t.Run("oversized_metadata_field_after_file_cleans_up_staged_file", func(t *testing.T) {
		audioBytes := createSyntheticMP3("Odyssey", "Homer", "", 60000, nil)
		var body bytes.Buffer
		mpw := multipart.NewWriter(&body)
		fp, _ := mpw.CreateFormFile("file", "odyssey.m4b")
		_, _ = fp.Write(audioBytes)
		// Write > 1 MB metadata_defaults field
		_ = mpw.WriteField("metadata_defaults", strings.Repeat("x", (1<<20)+1024))
		_ = mpw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", mpw.FormDataContentType())
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request on oversized metadata_defaults, got %d: %s", rec.Code, rec.Body.String())
		}
		assertNoFilesInUploadsDir(t, f.dataDir)
	})
}

func TestAPI_StageUploadBook_SlowStreamSurvivesShortServerReadTimeout(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	audioBytes := createSyntheticMP3("Slow Stream Odyssey", "Homer", "Claire Danes", 90000, nil)

	var body bytes.Buffer
	mpw := multipart.NewWriter(&body)
	fp, _ := mpw.CreateFormFile("file", "slow_odyssey.m4b")
	_, _ = fp.Write(audioBytes)
	_ = mpw.Close()
	rawPayload := body.Bytes()

	srv := httptest.NewUnstartedServer(f.handler)
	srv.Config.ReadTimeout = 40 * time.Millisecond
	srv.Start()
	defer srv.Close()

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		chunkSize := len(rawPayload)/4 + 1
		for offset := 0; offset < len(rawPayload); offset += chunkSize {
			end := offset + chunkSize
			if end > len(rawPayload) {
				end = len(rawPayload)
			}
			if _, err := pw.Write(rawPayload[offset:end]); err != nil {
				return
			}
			time.Sleep(35 * time.Millisecond) // Total ~140ms > 40ms ReadTimeout
		}
	}()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/books/upload/stage", pr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mpw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("slow upload failed despite ResponseController read deadline override: %v", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on slow streamed upload, got %d: %s", resp.StatusCode, string(respBytes))
	}
}

func TestAPI_StageUploadBook_LargeStreamBoundedHeap(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	id3Header := createSyntheticMP3("Bounded Heap Odyssey", "Homer", "Claire Danes", 3600000, nil)
	const payloadSize = 64 << 20 // 64 MiB

	srv := httptest.NewServer(f.handler)
	defer srv.Close()

	runtime.GC()
	var baseMem runtime.MemStats
	runtime.ReadMemStats(&baseMem)

	var peakHeapDelta atomic.Int64
	stopSampling := make(chan struct{})
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		var ms runtime.MemStats
		for {
			select {
			case <-stopSampling:
				return
			default:
				runtime.ReadMemStats(&ms)
				if ms.HeapAlloc > baseMem.HeapAlloc {
					delta := int64(ms.HeapAlloc - baseMem.HeapAlloc)
					for {
						cur := peakHeapDelta.Load()
						if delta <= cur || peakHeapDelta.CompareAndSwap(cur, delta) {
							break
						}
					}
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		part, err := mpw.CreateFormFile("file", "bounded_heap_odyssey.mp3")
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		if _, err := part.Write(id3Header); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		chunk := bytes.Repeat([]byte{0x5A}, 64*1024)
		remaining := payloadSize - len(id3Header)
		for remaining > 0 {
			toWrite := len(chunk)
			if remaining < toWrite {
				toWrite = remaining
			}
			if _, err := part.Write(chunk[:toWrite]); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			remaining -= toWrite
		}
		_ = mpw.WriteField("source_plugin", "overdrive")
		_ = mpw.WriteField("source_external_id", "bounded-heap-64mb")
		_ = mpw.Close()
	}()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/books/upload/stage", pr)
	if err != nil {
		close(stopSampling)
		<-samplerDone
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mpw.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	close(stopSampling)
	<-samplerDone
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(respBytes))
	}

	var stageResp struct {
		JobID string `json:"job_id"`
	}
	_ = json.Unmarshal(respBytes, &stageResp)
	stagedFile := filepath.Join(f.dataDir, "uploads", stageResp.JobID+".mp3")
	fi, err := os.Stat(stagedFile)
	if err != nil {
		t.Fatalf("staged file missing: %v", err)
	}
	if fi.Size() != payloadSize {
		t.Fatalf("expected staged file size %d, got %d", payloadSize, fi.Size())
	}

	const maxAllowedHeapDelta = 24 << 20 // 24 MiB (< 64 MiB payload)
	if got := peakHeapDelta.Load(); got > maxAllowedHeapDelta {
		t.Fatalf("Go HeapAlloc grew by %.2f MiB while streaming 64 MiB upload (limit %.2f MiB)",
			float64(got)/(1<<20), float64(maxAllowedHeapDelta)/(1<<20))
	}
}




