package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"golang.org/x/crypto/bcrypt"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
	"github.com/shelfd/shelfd/internal/scanner"
	"github.com/shelfd/shelfd/internal/worker"
)

func init() {
	sqlite_vec.Auto()
}

type testFixture struct {
	db        *sql.DB
	repo      repository.StorageEngine
	ingester  *scanner.Ingester
	scanner   *scanner.Scanner
	worker    *worker.Worker
	handler   http.Handler
	jwtSecret string
	dataDir   string
	libDir    string
	user      *repository.User
	password  string
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

	handler := api.NewRouter(api.RouterConfig{
		Repo:       repo,
		Ingester:   ingester,
		Scanner:    s,
		Worker:     nil,
		DataDir:    dataDir,
		LibraryDir: libDir,
		JWTSecret:  jwtSecret,
		Host:       "127.0.0.1",
		Port:       8080,
		Version:    "0.1.0-test",
	})

	return &testFixture{
		db:        db,
		repo:      repo,
		ingester:  ingester,
		scanner:   s,
		worker:    nil,
		handler:   handler,
		jwtSecret: jwtSecret,
		dataDir:   dataDir,
		libDir:    libDir,
		user:      user,
		password:  password,
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

	// 5. Get chapter reading content
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/books/%s/chapters/1", book.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get chapter, got %d: %s", rec.Code, rec.Body.String())
	}

	var chResp repository.Chapter
	json.Unmarshal(rec.Body.Bytes(), &chResp)
	if !strings.Contains(chResp.ContentPlain, "A drought of ten million years") {
		t.Errorf("unexpected chapter content: %s", chResp.ContentPlain)
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
	if resp.ServerName != "Shelfd" || resp.PairingPayload == "" {
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

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on upload, got %d: %s", rec.Code, rec.Body.String())
	}

	var book repository.Book
	json.Unmarshal(rec.Body.Bytes(), &book)
	if book.Title != "Solaris" {
		t.Errorf("expected book title Solaris, got %s", book.Title)
	}

	// Verify book is cataloged in database
	fetched, err := f.repo.GetBookByID(context.Background(), book.ID)
	if err != nil || fetched.Title != "Solaris" {
		t.Fatalf("book not found in database: %v", err)
	}

	// 2. Reject non-epub file
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

