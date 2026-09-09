package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/ai"
	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

func init() {
	sqlite_vec.Auto()
}

type mockAIClient struct {
	summaryResp string
	summaryErr  error
	embedResp   []float32
	embedErr    error
	chatResp    string
	chatErr     error
}

func (m *mockAIClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	if m.summaryErr != nil {
		return "", m.summaryErr
	}
	if m.summaryResp != "" {
		return m.summaryResp, nil
	}
	return "Mock summary for " + title, nil
}

func (m *mockAIClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	if len(m.embedResp) > 0 {
		return m.embedResp, nil
	}
	vec := make([]float32, 256)
	vec[0] = 0.1
	return vec, nil
}

func (m *mockAIClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := m.GenerateEmbedding(ctx, t)
		if err != nil {
			return nil, err
		}
		res[i] = emb
	}
	return res, nil
}

func (m *mockAIClient) Chat(ctx context.Context, messages []ai.ChatMessage) (string, error) {
	if m.chatErr != nil {
		return "", m.chatErr
	}
	if m.chatResp != "" {
		return m.chatResp, nil
	}
	return "Mock AI response about the book.", nil
}

type testFixture struct {
	db           *sql.DB
	repo         repository.StorageEngine
	ingester     *scanner.Ingester
	scanner      *scanner.Scanner
	worker       *worker.Worker
	uploadWorker *worker.UploadWorker
	aiClient     *mockAIClient
	handler      http.Handler
	jwtSecret    string
	dataDir      string
	libDir       string
	user         *repository.User
	password     string
}

func strPtr(s string) *string {
	return &s
}

func setupAPITest(t *testing.T) *testFixture {
	t.Helper()

	dataDir := t.TempDir()
	libDir := t.TempDir()

	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)

	password := "admin-secret-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	user := &repository.User{
		Username:     "admin",
		PasswordHash: string(hash),
	}
	if err := repo.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	ingester := scanner.NewIngester(repo, libDir, dataDir)
	s := scanner.NewScanner(libDir)
	jwtSecret := "super-secure-jwt-test-secret-key-123"

	uploadWorker := worker.NewUploadWorker(repo, ingester, nil, worker.UploadWorkerConfig{})
	aiClient := &mockAIClient{}

	handler := api.NewRouter(api.RouterConfig{
		Repo:         repo,
		Ingester:     ingester,
		Scanner:      s,
		Worker:       nil,
		UploadWorker: uploadWorker,
		AIClient:     aiClient,
		DataDir:      dataDir,
		LibraryDir:   libDir,
		JWTSecret:    jwtSecret,
		Host:            "127.0.0.1",
		Port:            8080,
		Version:         "0.1.0-test",
		DefaultUsername: "admin",
	})

	return &testFixture{
		db:           db,
		repo:         repo,
		ingester:     ingester,
		scanner:      s,
		worker:       nil,
		uploadWorker: uploadWorker,
		aiClient:     aiClient,
		handler:      handler,
		jwtSecret:    jwtSecret,
		dataDir:      dataDir,
		libDir:       libDir,
		user:         user,
		password:     password,
	}
}

func (f *testFixture) loginAndGetToken(t *testing.T) string {
	t.Helper()
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, f.user.Username, f.password)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return resp.Token
}

func TestAPI_Auth_LoginAndMe(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	// 1. Invalid password
	badLogin := `{"username":"admin","password":"wrong-password"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(badLogin))
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", rec.Code)
	}

	// 2. Non-existent username
	badUser := `{"username":"ghost","password":"wrong-password"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(badUser))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong user, got %d", rec.Code)
	}

	// 3. Valid login
	token := f.loginAndGetToken(t)
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	// 4. Access /api/v1/auth/me with token
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for me endpoint, got %d: %s", rec.Code, rec.Body.String())
	}

	var meResp struct {
		User api.UserResponse `json:"user"`
	}
	json.Unmarshal(rec.Body.Bytes(), &meResp)
	if meResp.User.Username != "admin" {
		t.Errorf("expected username admin, got %s", meResp.User.Username)
	}

	// 5. Access /api/v1/auth/me without token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAPI_Auth_ChangePassword(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// 1. Without token -> 401
	payload := `{"current_password":"admin-secret-123","new_password":"brand-new-password-456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}

	// 2. Incorrect current password -> 401
	wrongOldPass := `{"current_password":"wrong-admin-secret","new_password":"brand-new-password-456"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(wrongOldPass))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong current password, got %d", rec.Code)
	}

	// 3. Short new password (<6 chars) -> 400
	shortPass := `{"current_password":"admin-secret-123","new_password":"short"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(shortPass))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short new password, got %d", rec.Code)
	}

	// 4. Empty fields -> 400
	emptyFields := `{"current_password":"","new_password":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(emptyFields))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty fields, got %d", rec.Code)
	}

	// 5. Successful password change -> 200
	successPayload := `{"current_password":"admin-secret-123","new_password":"brand-new-password-456"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(successPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on successful password change, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Old password login now fails -> 401
	oldLogin := `{"username":"admin","password":"admin-secret-123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(oldLogin))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 using old password, got %d", rec.Code)
	}

	// 7. New password login succeeds -> 200
	newLogin := `{"username":"admin","password":"brand-new-password-456"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(newLogin))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 using new password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_Auth_Tokens(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	jwtToken := f.loginAndGetToken(t)

	// 1. Create API token
	createReq := `{"name":"Cursor Integration"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(createReq))
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create token, got %d: %s", rec.Code, rec.Body.String())
	}

	var tokenResp api.CreateTokenResponse
	json.Unmarshal(rec.Body.Bytes(), &tokenResp)
	if !strings.HasPrefix(tokenResp.Token, "shelfd_") {
		t.Errorf("expected shelfd_ prefix on token, got %s", tokenResp.Token)
	}

	// 2. List API tokens
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on list tokens, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		Tokens []api.APITokenListItem `json:"tokens"`
	}
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Tokens) != 1 || listResp.Tokens[0].ID != tokenResp.ID {
		t.Fatalf("expected 1 token matching created id, got %v", listResp.Tokens)
	}

	// 3. Use newly created API token directly on protected endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when authenticated via API token, got %d", rec.Code)
	}

	// 4. Delete token
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+tokenResp.ID, nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete token, got %d", rec.Code)
	}

	// 5. Try accessing endpoint with deleted token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on deleted token, got %d", rec.Code)
	}
}

func TestAPI_Books_CRUDAndBrowsing(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	token := f.loginAndGetToken(t)

	// Seed data
	author, _ := f.repo.UpsertAuthor(ctx, "Arthur C. Clarke")
	genre, _ := f.repo.UpsertGenre(ctx, "Hard Sci-Fi")
	series, _ := f.repo.UpsertSeries(ctx, "Space Odyssey", nil)

	book := &repository.Book{
		Title:    "2001: A Space Odyssey",
		FilePath: "Arthur C Clarke/2001/2001.epub",
	}
	f.repo.CreateBook(ctx, book)
	f.repo.LinkBookAuthor(ctx, book.ID, author.ID, "author")
	f.repo.LinkBookGenre(ctx, book.ID, genre.ID)
	seq := 1.0
	f.repo.LinkBookSeries(ctx, book.ID, series.ID, &seq)

	chTitle := "Primeval Night"
	f.repo.CreateChapter(ctx, &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &chTitle,
		Summary:      "The dawn of man and the arrival of the black monolith.",
		ContentPlain: "A drought of ten million years had come to an end.",
	})

	// 1. List books
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on list books, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		Books []api.BookListItem `json:"books"`
		Total int                `json:"total"`
	}
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if listResp.Total != 1 || len(listResp.Books) != 1 {
		t.Fatalf("expected 1 book, got total=%d len=%d", listResp.Total, len(listResp.Books))
	}
	if listResp.Books[0].Title != "2001: A Space Odyssey" {
		t.Errorf("expected title 2001, got %s", listResp.Books[0].Title)
	}
	if len(listResp.Books[0].Authors) == 0 || listResp.Books[0].Authors[0].Name != "Arthur C. Clarke" {
		t.Errorf("expected author Arthur C. Clarke, got %v", listResp.Books[0].Authors)
	}

	// 2. Filter by author
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books?author_id="+author.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Books) != 1 {
		t.Errorf("expected 1 book for matching author filter, got %d", len(listResp.Books))
	}

	// 3. Search query
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books?search=Odyssey", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if len(listResp.Books) != 1 {
		t.Errorf("expected 1 book for matching search, got %d", len(listResp.Books))
	}

	// 4. Get book detail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/"+book.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get book, got %d: %s", rec.Code, rec.Body.String())
	}

	var detailResp api.BookDetailResponse
	json.Unmarshal(rec.Body.Bytes(), &detailResp)
	if len(detailResp.Chapters) != 1 || detailResp.Chapters[0].Summary == "" {
		t.Errorf("expected 1 chapter with summary, got %v", detailResp.Chapters)
	}
	if len(detailResp.Spine) != 1 || detailResp.Spine[0].ID == "" {
		t.Errorf("expected 1 spine item with valid ID, got %v", detailResp.Spine)
	}

	chapterID := detailResp.Spine[0].ID

	// 5. Get chapter reading content by index (1)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/1", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get chapter by index, got %d: %s", rec.Code, rec.Body.String())
	}

	var chResp repository.Chapter
	json.Unmarshal(rec.Body.Bytes(), &chResp)
	if !strings.Contains(chResp.ContentPlain, "A drought of ten million years") {
		t.Errorf("unexpected chapter content: %s", chResp.ContentPlain)
	}

	// 5a. Get chapter reading content by chapter ID (ULID/UUID)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/%s", book.ID, chapterID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get chapter by ID, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5b. Get chapter reading content by index 0 (legacy fallback to first spine item)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/0", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get chapter index 0 fallback, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5c. Direct chapter endpoint GET /api/v1/chapters/{id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/chapters/%s", chapterID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on direct get chapter, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Get non-existent chapter
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/99", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing chapter, got %d", rec.Code)
	}
}

func TestAPI_Books_Reparse(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	token := f.loginAndGetToken(t)

	// Create a minimal EPUB file in f.libDir
	bookDir := filepath.Join(f.libDir, "Author", "Title")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	epubPath := filepath.Join(bookDir, "Title.epub")

	writeEPUB := func(chContent string) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		m, _ := zw.Create("mimetype")
		m.Write([]byte("application/epub+zip"))
		w, _ := zw.Create("META-INF/container.xml")
		w.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))
		opf, _ := zw.Create("content.opf")
		opf.Write([]byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Test Title</dc:title><dc:creator>Author</dc:creator></metadata><manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`))
		ch1, _ := zw.Create("ch1.xhtml")
		ch1.Write([]byte(chContent))
		zw.Close()
		os.WriteFile(epubPath, buf.Bytes(), 0644)
	}

	writeEPUB(`<html><body><h1>Old Heading</h1><p>Old text</p></body></html>`)
	relPath := filepath.Join("Author", "Title", "Title.epub")
	book, err := f.ingester.IngestFile(ctx, epubPath, relPath)
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}

	// Update EPUB content on disk
	writeEPUB(`<html><body><h2>New Subheading</h2><p>New formatted text with <em>italics</em> and <a href="#fn"><sup class="sup">1</sup></a> footnote.</p></body></html>`)

	// 1. Call POST /api/v1/books/{id}/reparse
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/books/%s/reparse", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on reparse, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Fetch chapter content via API
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/0", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get chapter, got %d: %s", rec.Code, rec.Body.String())
	}

	var chResp repository.Chapter
	if err := json.Unmarshal(rec.Body.Bytes(), &chResp); err != nil {
		t.Fatalf("unmarshal chapter: %v", err)
	}

	if !strings.Contains(chResp.ContentPlain, "## New Subheading") {
		t.Errorf("expected content to contain ## New Subheading, got: %s", chResp.ContentPlain)
	}
	if !strings.Contains(chResp.ContentPlain, "[1]") {
		t.Errorf("expected content to contain [1], got: %s", chResp.ContentPlain)
	}

	// 3. Test reparse non-existent book
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/nonexistent-id/reparse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent book reparse, got %d", rec.Code)
	}
}

func TestAPI_Books_Cover(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	bookID := "book-with-cover-123"
	coversDir := filepath.Join(f.dataDir, "covers")
	os.MkdirAll(coversDir, 0755)

	coverPath := filepath.Join(coversDir, bookID+".jpg")
	os.WriteFile(coverPath, []byte("fake-jpeg-data"), 0644)

	// Cover endpoint is public for <img> tags
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/cover", bookID), nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cover, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "public") {
		t.Errorf("expected Cache-Control public, got %s", rec.Header().Get("Cache-Control"))
	}

	// Missing cover
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/missing-book/cover", nil)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing cover, got %d", rec.Code)
	}
}

func TestAPI_Taxonomy(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	token := f.loginAndGetToken(t)

	f.repo.UpsertAuthor(ctx, "Stanislaw Lem")
	f.repo.UpsertGenre(ctx, "Philosophical Sci-Fi")
	f.repo.UpsertSeries(ctx, "Ijon Tichy", nil)

	// Authors
	req := httptest.NewRequest(http.MethodGet, "/api/v1/authors", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Stanislaw Lem") {
		t.Errorf("expected author in response, got %s", rec.Body.String())
	}

	// Genres
	req = httptest.NewRequest(http.MethodGet, "/api/v1/genres", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Philosophical Sci-Fi") {
		t.Errorf("expected genre in response, got %s", rec.Body.String())
	}

	// Series
	req = httptest.NewRequest(http.MethodGet, "/api/v1/series", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Ijon Tichy") {
		t.Errorf("expected series in response, got %s", rec.Body.String())
	}
}

func TestAPI_ConnectInfo(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/server/connect-info", nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for connect-info, got %d", rec.Code)
	}

	var resp api.ConnectInfoResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ServerName != "Shelfd" || resp.PairingPayload == "" || resp.DefaultUsername != "admin" {
		t.Errorf("unexpected connect-info response: %+v", resp)
	}
}

func TestAPI_LibraryScan(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/library/scan", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on scan trigger, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_GetBookCover(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()

	// 1. Book with JPEG cover in libraryDir
	book1 := &repository.Book{
		ID:        "book-cov-1",
		Title:     "Solaris",
		FilePath:  "Stanislaw Lem/Solaris/Solaris.epub",
		CoverPath: strPtr("Stanislaw Lem/Solaris/cover.jpg"),
	}
	if err := f.repo.CreateBook(ctx, book1); err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}

	book1CoverDir := filepath.Join(f.libDir, "Stanislaw Lem", "Solaris")
	os.MkdirAll(book1CoverDir, 0755)
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	os.WriteFile(filepath.Join(book1CoverDir, "cover.jpg"), jpegData, 0644)

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-cov-1/cover", nil)
	rec1 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 for library cover, got %d: %s", rec1.Code, rec1.Body.String())
	}
	if ct := rec1.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %s", ct)
	}
	if !bytes.Equal(rec1.Body.Bytes(), jpegData) {
		t.Errorf("served cover bytes mismatch")
	}

	// 2. Book with PNG cover in libraryDir
	book2 := &repository.Book{
		ID:        "book-cov-2",
		Title:     "Story of Your Life",
		FilePath:  "Ted Chiang/Story of Your Life/Story of Your Life.epub",
		CoverPath: strPtr("Ted Chiang/Story of Your Life/cover.png"),
	}
	if err := f.repo.CreateBook(ctx, book2); err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}

	book2CoverDir := filepath.Join(f.libDir, "Ted Chiang", "Story of Your Life")
	os.MkdirAll(book2CoverDir, 0755)
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	os.WriteFile(filepath.Join(book2CoverDir, "cover.png"), pngData, 0644)

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-cov-2/cover", nil)
	rec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for library PNG cover, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if ct := rec2.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %s", ct)
	}

	// 3. Book with fallback cover in dataDir
	book3 := &repository.Book{
		ID:        "book-cov-3",
		Title:     "Fallback Book",
		FilePath:  "Author/Fallback/Fallback.epub",
		CoverPath: strPtr("covers/book-cov-3.jpg"),
	}
	if err := f.repo.CreateBook(ctx, book3); err != nil {
		t.Fatalf("CreateBook failed: %v", err)
	}

	dataCoverDir := filepath.Join(f.dataDir, "covers")
	os.MkdirAll(dataCoverDir, 0755)
	os.WriteFile(filepath.Join(dataCoverDir, "book-cov-3.jpg"), jpegData, 0644)

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/books/book-cov-3/cover", nil)
	rec3 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for data fallback cover, got %d", rec3.Code)
	}

	// 4. Non-existent cover
	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/books/nonexistent/cover", nil)
	rec4 := httptest.NewRecorder()
	f.handler.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing cover, got %d", rec4.Code)
	}
}

func TestAPI_UploadBook(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// Create valid in-memory EPUB
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	m, _ := zw.Create("mimetype")
	m.Write([]byte("application/epub+zip"))
	w, _ := zw.Create("META-INF/container.xml")
	w.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	opf, _ := zw.Create("OEBPS/content.opf")
	opf.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Solaris</dc:title>
    <dc:creator>Stanislaw Lem</dc:creator>
  </metadata>
  <manifest>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
  </spine>
</package>`))

	ch1, _ := zw.Create("OEBPS/ch1.xhtml")
	ch1.Write([]byte(`<!DOCTYPE html><html><head><title>The Arrival</title></head><body><p>At 19:00 hours, Kelvin entered the capsule.</p></body></html>`))
	zw.Close()

	// 1. Multipart POST valid EPUB
	body := &bytes.Buffer{}
	mpw := multipart.NewWriter(body)
	part, _ := mpw.CreateFormFile("file", "solaris.epub")
	part.Write(buf.Bytes())
	mpw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mpw.FormDataContentType())
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on upload, got %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp struct {
		JobID     string `json:"job_id"`
		Status    string `json:"status"`
		Filename  string `json:"filename"`
		Message   string `json:"message"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatalf("failed to parse upload response: %v", err)
	}
	if uploadResp.JobID == "" || uploadResp.Status != "queued" || uploadResp.Filename != "solaris.epub" {
		t.Fatalf("unexpected upload response: %+v", uploadResp)
	}

	// 2. Query job status via GET /api/v1/books/upload/jobs/{id} before worker runs
	jobReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs/"+uploadResp.JobID, nil)
	jobReq.Header.Set("Authorization", "Bearer "+token)
	jobRec := httptest.NewRecorder()
	f.handler.ServeHTTP(jobRec, jobReq)
	if jobRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get job, got %d: %s", jobRec.Code, jobRec.Body.String())
	}
	var jobBefore repository.UploadJob
	json.Unmarshal(jobRec.Body.Bytes(), &jobBefore)
	if jobBefore.Status != "queued" {
		t.Fatalf("expected job status queued, got %s", jobBefore.Status)
	}

	// 3. Process the queue with worker
	processed, err := f.uploadWorker.ProcessNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("failed to process upload job: %v", err)
	}

	// 4. Query job status after processing
	jobReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs/"+uploadResp.JobID, nil)
	jobReq2.Header.Set("Authorization", "Bearer "+token)
	jobRec2 := httptest.NewRecorder()
	f.handler.ServeHTTP(jobRec2, jobReq2)
	if jobRec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get job after processing, got %d: %s", jobRec2.Code, jobRec2.Body.String())
	}
	var jobAfter repository.UploadJob
	json.Unmarshal(jobRec2.Body.Bytes(), &jobAfter)
	if jobAfter.Status != "completed" || jobAfter.BookID == nil {
		t.Fatalf("expected completed status with book_id, got %+v", jobAfter)
	}

	// 5. Verify book is cataloged in database and visible in ListBooks
	fetched, err := f.repo.GetBookByID(context.Background(), *jobAfter.BookID)
	if err != nil || fetched.Title != "Solaris" {
		t.Fatalf("book not found in database: %v", err)
	}

	// 6. Test GET /api/v1/books/upload/jobs listing
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	f.handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list upload jobs, got %d: %s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		Jobs  []*repository.UploadJob `json:"jobs"`
		Total int                     `json:"total"`
	}
	json.Unmarshal(listRec.Body.Bytes(), &listResp)
	if listResp.Total < 1 || len(listResp.Jobs) < 1 {
		t.Fatalf("expected at least 1 job in list, got %d", listResp.Total)
	}

	// 7. Nonexistent job ID returns 404
	notfoundReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/upload/jobs/nonexistent-id", nil)
	notfoundReq.Header.Set("Authorization", "Bearer "+token)
	notfoundRec := httptest.NewRecorder()
	f.handler.ServeHTTP(notfoundRec, notfoundReq)
	if notfoundRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent job, got %d", notfoundRec.Code)
	}

	// 8. Reject non-epub file
	badBody := &bytes.Buffer{}
	badMpw := multipart.NewWriter(badBody)
	badPart, _ := badMpw.CreateFormFile("file", "malicious.exe")
	badPart.Write([]byte("not an epub"))
	badMpw.Close()

	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload", badBody)
	badReq.Header.Set("Authorization", "Bearer "+token)
	badReq.Header.Set("Content-Type", badMpw.FormDataContentType())
	badRec := httptest.NewRecorder()
	f.handler.ServeHTTP(badRec, badReq)

	if badRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for non-epub upload, got %d", badRec.Code)
	}

	// 9. Reject corrupted epub (fails zip reader)
	corruptBody := &bytes.Buffer{}
	corruptMpw := multipart.NewWriter(corruptBody)
	corruptPart, _ := corruptMpw.CreateFormFile("file", "corrupt.epub")
	corruptPart.Write([]byte("definitely not a valid zip file archive"))
	corruptMpw.Close()

	corruptReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload", corruptBody)
	corruptReq.Header.Set("Authorization", "Bearer "+token)
	corruptReq.Header.Set("Content-Type", corruptMpw.FormDataContentType())
	corruptRec := httptest.NewRecorder()
	f.handler.ServeHTTP(corruptRec, corruptReq)

	if corruptRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for corrupt zip, got %d", corruptRec.Code)
	}
}

func TestAPI_EdgeCasesAndCORS(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	token := f.loginAndGetToken(t)

	// 1. CORS preflight OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/books", nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS header *, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. Bad JSON in Login
	badJSONReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{invalid"))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, badJSONReq)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json in login, got %d", rec.Code)
	}

	// 3. Empty fields in Login
	emptyReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"","password":""}`))
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, emptyReq)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty username, got %d", rec.Code)
	}

	// 4. Bad JSON in CreateToken
	badTokenReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader("{invalid"))
	badTokenReq.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, badTokenReq)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json in create token, got %d", rec.Code)
	}

	// 5. Invalid chapter index in GetChapter
	badIdxReq := httptest.NewRequest(http.MethodGet, "/api/v1/books/any-id/chapters/not-an-int", nil)
	badIdxReq.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, badIdxReq)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid chapter index, got %d", rec.Code)
	}

	// 6. Filter books by genre and series
	ctx := context.Background()
	g, _ := f.repo.UpsertGenre(ctx, "TestGenre")
	s, _ := f.repo.UpsertSeries(ctx, "TestSeries", nil)
	filterReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books?genre_id=%s&series_id=%s&limit=10&offset=0", g.ID, s.ID), nil)
	filterReq.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, filterReq)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for genre and series query, got %d", rec.Code)
	}

	// 6b. Pagination with per_page and page
	pageReq := httptest.NewRequest(http.MethodGet, "/api/v1/books?page=2&per_page=5", nil)
	pageReq.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, pageReq)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for per_page and page query, got %d", rec.Code)
	}
	var pageResp struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pageResp); err != nil {
		t.Fatalf("failed to decode page response: %v", err)
	}
	if pageResp.Limit != 5 {
		t.Errorf("expected limit 5, got %d", pageResp.Limit)
	}
	if pageResp.Offset != 5 {
		t.Errorf("expected offset 5, got %d", pageResp.Offset)
	}

	// 7. Missing 'file' in multipart upload
	emptyFormBody := &bytes.Buffer{}
	mpw := multipart.NewWriter(emptyFormBody)
	mpw.Close()
	noFileReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload", emptyFormBody)
	noFileReq.Header.Set("Authorization", "Bearer "+token)
	noFileReq.Header.Set("Content-Type", mpw.FormDataContentType())
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, noFileReq)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file field, got %d", rec.Code)
	}
}

func TestQueueAPI(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)

	// 1. Unauthenticated request to /api/v1/queue/status
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/queue/status", nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, unauthReq)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 unauthorized, got %d", rec.Code)
	}

	// 2. Authenticated request to /api/v1/queue/status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/queue/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}

	var status repository.QueueStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode queue status: %v", err)
	}

	if status.TotalChapters != 0 {
		t.Errorf("expected 0 total chapters, got %d", status.TotalChapters)
	}

	// 3. Create a book and an unindexed chapter
	ctx := context.Background()
	book := &repository.Book{
		ID:       "queue-test-book",
		Title:    "Queue Test Book",
		FilePath: "/test/book.epub",
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}
	title := "Chapter 1"
	ch := &repository.Chapter{
		ID:           "queue-test-ch1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title,
		ContentPlain: "Some content here",
	}
	if err := f.repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("failed to create chapter: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/queue/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d", rec.Code)
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode queue status: %v", err)
	}

	if status.TotalChapters != 1 || status.PendingChapters != 1 {
		t.Errorf("expected 1 total and 1 pending chapter, got total=%d, pending=%d", status.TotalChapters, status.PendingChapters)
	}
	if !status.IsActive {
		t.Errorf("expected queue to be active with pending chapters")
	}
}

func TestBookmarksEndpoints(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	book := &repository.Book{
		ID:       "bm-book-1",
		Title:    "Bookmark Book",
		FilePath: "/test/bm.epub",
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	// 1. Create Bookmark
	createPayload := `{"title": "Chapter Two", "progress": 0.45}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/bm-book-1/bookmarks", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}

	var bm repository.Bookmark
	if err := json.Unmarshal(rec.Body.Bytes(), &bm); err != nil {
		t.Fatalf("failed to decode bookmark: %v", err)
	}
	if bm.ID == "" || bm.BookID != "bm-book-1" || bm.Title != "Chapter Two" || bm.Progress != 0.45 {
		t.Fatalf("unexpected bookmark values: %+v", bm)
	}

	// 2. List Bookmarks
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/bm-book-1/bookmarks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}
	var bms []repository.Bookmark
	if err := json.Unmarshal(rec.Body.Bytes(), &bms); err != nil {
		t.Fatalf("failed to decode bookmarks: %v", err)
	}
	if len(bms) != 1 || bms[0].ID != bm.ID {
		t.Fatalf("expected 1 bookmark with id %s, got %d", bm.ID, len(bms))
	}

	// 3. GetBook includes bookmarks
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/bm-book-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}
	var bookRes api.BookDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &bookRes); err != nil {
		t.Fatalf("failed to decode book response: %v", err)
	}
	if len(bookRes.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark embedded in book, got %d", len(bookRes.Bookmarks))
	}

	// 4. Delete Bookmark
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/bookmarks/"+bm.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Verify list is empty
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/bm-book-1/bookmarks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	var emptyBms []repository.Bookmark
	json.Unmarshal(rec.Body.Bytes(), &emptyBms)
	if len(emptyBms) != 0 {
		t.Fatalf("expected 0 bookmarks, got %d", len(emptyBms))
	}
}

func TestHighlightsEndpoints(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	book := &repository.Book{
		ID:       "hl-book-1",
		Title:    "Highlight Book",
		FilePath: "/test/hl.epub",
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}

	// 1. Create Highlight
	createPayload := `{"selected_text": "Call me Ishmael.", "color": "blue", "note": "Famous opening line", "start_offset": 0, "end_offset": 16, "start_paragraph": 1, "end_paragraph": 1, "location": "ch1:0-16"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/hl-book-1/highlights", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}

	var hl repository.Highlight
	if err := json.Unmarshal(rec.Body.Bytes(), &hl); err != nil {
		t.Fatalf("failed to decode highlight: %v", err)
	}
	if hl.ID == "" || hl.BookID != "hl-book-1" || hl.SelectedText != "Call me Ishmael." || hl.Color != "blue" {
		t.Fatalf("unexpected highlight values: %+v", hl)
	}
	if hl.StartOffset == nil || *hl.StartOffset != 0 || hl.EndOffset == nil || *hl.EndOffset != 16 {
		t.Fatalf("expected start/end offset to be populated: %+v", hl)
	}

	// 2. List Highlights
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/hl-book-1/highlights", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}
	var hls []repository.Highlight
	if err := json.Unmarshal(rec.Body.Bytes(), &hls); err != nil {
		t.Fatalf("failed to decode highlights: %v", err)
	}
	if len(hls) != 1 || hls[0].ID != hl.ID {
		t.Fatalf("expected 1 highlight, got %d", len(hls))
	}

	// 3. GetBook includes highlights
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/hl-book-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	var bookRes api.BookDetailResponse
	json.Unmarshal(rec.Body.Bytes(), &bookRes)
	if len(bookRes.Highlights) != 1 {
		t.Fatalf("expected 1 highlight in book, got %d", len(bookRes.Highlights))
	}

	// 4. Delete Highlight
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/highlights/"+hl.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Verify list is empty
	req = httptest.NewRequest(http.MethodGet, "/api/v1/books/hl-book-1/highlights", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	var emptyHls []repository.Highlight
	json.Unmarshal(rec.Body.Bytes(), &emptyHls)
	if len(emptyHls) != 0 {
		t.Fatalf("expected 0 highlights, got %d", len(emptyHls))
	}
}

func TestChatBookEndpoint(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)
	ctx := context.Background()

	book := &repository.Book{
		ID:       "chat-book-1",
		Title:    "Moby Dick",
		FilePath: "/test/moby.epub",
	}
	if err := f.repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("failed to create book: %v", err)
	}
	title := "Loomings"
	summary := "Ishmael decides to go to sea and travels to New Bedford."
	ch := &repository.Chapter{
		ID:           "ch-moby-1",
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &title,
		Summary:      summary,
		ContentPlain: "Call me Ishmael. Some years ago...",
	}
	if err := f.repo.CreateChapter(ctx, ch); err != nil {
		t.Fatalf("failed to create chapter: %v", err)
	}

	// Valid chat request
	chatPayload := `{"query": "Why does Ishmael go to sea?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/books/chat-book-1/chat", strings.NewReader(chatPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}

	var chatRes map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &chatRes); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	if chatRes["response"] == "" {
		t.Errorf("expected non-empty response, got %+v", chatRes)
	}
	citations, ok := chatRes["citations"].([]interface{})
	if !ok || len(citations) == 0 {
		t.Errorf("expected citations array, got %+v", chatRes["citations"])
	}

	// Empty query error
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/chat-book-1/chat", strings.NewReader(`{"query": ""}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 bad request for empty query, got %d", rec.Code)
	}

	// Non-existent book error
	req = httptest.NewRequest(http.MethodPost, "/api/v1/books/nonexistent/chat", strings.NewReader(`{"query": "hello"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 not found for nonexistent book, got %d", rec.Code)
	}
}

func TestSearchLibraryEndpoint(t *testing.T) {
	f := setupAPITest(t)
	token := f.loginAndGetToken(t)

	// Valid search query
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=whale", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d: %s", rec.Code, rec.Body.String())
	}

	var hits []*repository.SearchHit
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	// Empty query returns empty array
	req = httptest.NewRequest(http.MethodGet, "/api/v1/search?q=", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for empty query, got %d", rec.Code)
	}
}

func TestRequestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	middleware := api.RequestLoggerMiddleware(logger)

	// 1. Regular 200 OK request with custom bytes and user in context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	user := &repository.User{ID: "usr-123", Username: "alice"}
	req = req.WithContext(context.WithValue(req.Context(), api.UserContextKey, user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("expected status 200 in log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"path":"/api/v1/test"`) {
		t.Errorf("expected path in log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Errorf("expected method GET in log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"user_id":"usr-123"`) {
		t.Errorf("expected user_id in log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"bytes":11`) {
		t.Errorf("expected bytes 11 in log, got %s", logOutput)
	}

	// 2. Health check request should be logged at DEBUG level
	buf.Reset()
	healthHandler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthRec := httptest.NewRecorder()
	healthHandler.ServeHTTP(healthRec, healthReq)

	if !strings.Contains(buf.String(), `"level":"DEBUG"`) {
		t.Errorf("expected health check to be logged at DEBUG level, got: %s", buf.String())
	}

	// 3. Error status code 500 should be logged at ERROR level
	buf.Reset()
	errHandler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	errReq := httptest.NewRequest(http.MethodPost, "/api/v1/fail", nil)
	errRec := httptest.NewRecorder()
	errHandler.ServeHTTP(errRec, errReq)

	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Errorf("expected status 500 to be logged at ERROR level, got: %s", buf.String())
	}
}


