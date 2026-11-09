package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/config"
	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

func seedNotebookLMTestBook(t *testing.T, f *testFixture) *repository.Book {
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

func TestAPI_ExportBookNotebookLM(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	seedNotebookLMTestBook(t, f)

	// 1. Unauthenticated request returns 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/notebooklm", nil)
	unauthRec := httptest.NewRecorder()
	f.handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated export, got %d", unauthRec.Code)
	}

	// 2. Authenticated GET request returns compiled Markdown with expected headers
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/notebooklm", nil)
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
	reqQuery := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-1/notebooklm?token="+token, nil)
	recQuery := httptest.NewRecorder()
	f.handler.ServeHTTP(recQuery, reqQuery)
	if recQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ?token= query export, got %d", recQuery.Code)
	}

	// 4. Nonexistent book returns 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/books/nonexistent-book/notebooklm", nil)
	req404.Header.Set("Authorization", "Bearer "+token)
	rec404 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent book, got %d", rec404.Code)
	}
}

func TestAPI_ExportBookNotebookLM_EPUBFallback(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-nlm-fallback/notebooklm", nil)
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

func TestAPI_SendBookToNotebookLM_ClientHandoff(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	seedNotebookLMTestBook(t, f)

	// 1. Unauthenticated POST returns 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-nlm-1/notebooklm", strings.NewReader("{}"))
	unauthRec := httptest.NewRecorder()
	f.handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated POST, got %d", unauthRec.Code)
	}

	// 2. Authenticated POST with default zero-config returns client_handoff
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-nlm-1/notebooklm", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.NotebookLMSendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}
	if resp.Status != "client_handoff" {
		t.Errorf("expected status client_handoff, got %q", resp.Status)
	}
	if resp.NotebookURL != "https://notebooklm.google.com/" {
		t.Errorf("expected notebook_url https://notebooklm.google.com/, got %q", resp.NotebookURL)
	}
	if resp.FileName != "Solaris_ The Novel.md" {
		t.Errorf("expected file_name Solaris_ The Novel.md, got %q", resp.FileName)
	}
	if resp.DownloadPath != "/api/v1/books/book-nlm-1/notebooklm" {
		t.Errorf("expected download_path /api/v1/books/book-nlm-1/notebooklm, got %q", resp.DownloadPath)
	}

	// 3. Nonexistent book returns 404
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/books/nonexistent/notebooklm", nil)
	req404.Header.Set("Authorization", "Bearer "+token)
	rec404 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent book, got %d", rec404.Code)
	}
}

func TestAPI_SendBookToNotebookLM_WebhookAndEnterprise(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)
	seedNotebookLMTestBook(t, f)

	// Subtest 1: Webhook push success & upstream error (502)
	t.Run("webhook_push", func(t *testing.T) {
		var receivedAuth string
		var receivedPayload map[string]any
		shouldFail := false

		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldFail {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("upstream failure"))
				return
			}
			receivedAuth = r.Header.Get("Authorization")
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedPayload)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"notebook_url":"https://notebooklm.google.com/notebook/nb-webhook-99","message":"Created notebook nb-webhook-99"}`))
		}))
		defer webhookServer.Close()

		router := api.NewRouter(api.RouterConfig{
			Repo:         f.repo,
			Ingester:     f.ingester,
			Scanner:      f.scanner,
			UploadWorker: f.uploadWorker,
			AIClient:     f.aiClient,
			ToolInvoker:  mcp.NewToolExecutor(f.repo, f.aiClient, slog.Default()),
			NotebookLM: config.NotebookLMConfig{
				Enabled:    true,
				WebhookURL: webhookServer.URL,
				APIKey:     "test-webhook-secret",
			},
			Hub:        f.hub,
			DataDir:    f.dataDir,
			LibraryDir: f.libDir,
			JWTSecret:  f.jwtSecret,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-nlm-1/notebooklm", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from webhook push, got %d: %s", rec.Code, rec.Body.String())
		}
		if receivedAuth != "Bearer test-webhook-secret" {
			t.Errorf("expected Authorization Bearer test-webhook-secret, got %q", receivedAuth)
		}
		if receivedPayload["book_id"] != "book-nlm-1" {
			t.Errorf("expected book_id book-nlm-1, got %v", receivedPayload["book_id"])
		}
		contentStr, _ := receivedPayload["content"].(string)
		if !strings.Contains(contentStr, "Kris Kelvin entered the arrival capsule") {
			t.Errorf("expected webhook content to include chapter markdown, got %s", contentStr)
		}

		var resp api.NotebookLMSendResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "sent" {
			t.Errorf("expected status sent, got %q", resp.Status)
		}
		if resp.NotebookURL != "https://notebooklm.google.com/notebook/nb-webhook-99" {
			t.Errorf("expected notebook_url https://notebooklm.google.com/notebook/nb-webhook-99, got %q", resp.NotebookURL)
		}
		if resp.Message != "Created notebook nb-webhook-99" {
			t.Errorf("expected message Created notebook nb-webhook-99, got %q", resp.Message)
		}

		// Verify 502 Bad Gateway when upstream fails
		shouldFail = true
		reqFail := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-nlm-1/notebooklm", nil)
		reqFail.Header.Set("Authorization", "Bearer "+token)
		recFail := httptest.NewRecorder()
		router.ServeHTTP(recFail, reqFail)
		if recFail.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 Bad Gateway on upstream failure, got %d", recFail.Code)
		}
	})

	// Subtest 2: Google Discovery Engine / NotebookLM Enterprise push
	t.Run("enterprise_discovery_engine_push", func(t *testing.T) {
		var createCalled, uploadCalled bool
		var uploadedMarkdown string
		var uploadedFileNameHeader string

		gcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/v1alpha/projects/my-gcp-proj/locations/us-central1/notebooks":
				createCalled = true
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"name":"projects/my-gcp-proj/locations/us-central1/notebooks/nb-ent-777","notebookId":"nb-ent-777"}`))
			case "/upload/v1alpha/projects/my-gcp-proj/locations/us-central1/notebooks/nb-ent-777/sources:uploadFile":
				uploadCalled = true
				uploadedFileNameHeader = r.Header.Get("X-Goog-Upload-File-Name")
				b, _ := io.ReadAll(r.Body)
				uploadedMarkdown = string(b)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"sourceId":"src-1"}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer gcpServer.Close()

		router := api.NewRouter(api.RouterConfig{
			Repo:         f.repo,
			Ingester:     f.ingester,
			Scanner:      f.scanner,
			UploadWorker: f.uploadWorker,
			AIClient:     f.aiClient,
			ToolInvoker:  mcp.NewToolExecutor(f.repo, f.aiClient, slog.Default()),
			NotebookLM: config.NotebookLMConfig{
				Enabled:   true,
				Endpoint:  gcpServer.URL,
				ProjectID: "my-gcp-proj",
				Location:  "us-central1",
				APIKey:    "gcp-oauth-token",
			},
			Hub:        f.hub,
			DataDir:    f.dataDir,
			LibraryDir: f.libDir,
			JWTSecret:  f.jwtSecret,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/books/book-nlm-1/notebooklm", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK from enterprise push, got %d: %s", rec.Code, rec.Body.String())
		}
		if !createCalled || !uploadCalled {
			t.Fatalf("expected both createCalled (%v) and uploadCalled (%v) to be true", createCalled, uploadCalled)
		}
		if uploadedFileNameHeader != "Solaris_ The Novel.md" {
			t.Errorf("expected X-Goog-Upload-File-Name Solaris_ The Novel.md, got %q", uploadedFileNameHeader)
		}
		if !strings.Contains(uploadedMarkdown, "# Solaris: The Novel?") {
			t.Errorf("expected uploaded markdown to contain book title, got %s", uploadedMarkdown)
		}

		var resp api.NotebookLMSendResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "sent" {
			t.Errorf("expected status sent, got %q", resp.Status)
		}
		if resp.NotebookURL != "https://notebooklm.google.com/notebook/nb-ent-777" {
			t.Errorf("expected notebook_url https://notebooklm.google.com/notebook/nb-ent-777, got %q", resp.NotebookURL)
		}
	})
}
