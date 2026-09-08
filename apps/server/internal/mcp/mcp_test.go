package mcp_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/repository"
)

func init() {
	sqlite_vec.Auto()
}

type mockAIClient struct {
	mu           sync.Mutex
	summaryText  string
	embeddingVec []float32
}

func (m *mockAIClient) SummarizeChapter(ctx context.Context, title, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.summaryText != "" {
		return m.summaryText, nil
	}
	return "Standard summary of: " + content[:min(30, len(content))], nil
}

func (m *mockAIClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.embeddingVec) > 0 {
		return m.embeddingVec, nil
	}
	vec := make([]float32, 1536)
	vec[0] = 1.0
	return vec, nil
}

func setupTestEnvironment(t *testing.T) (*sql.DB, repository.StorageEngine, string, *mcp.Server) {
	t.Helper()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)

	ctx := context.Background()
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "$2a$12$dummyhash",
	}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	rawToken := "test-secret-token-12345"
	tokenHash := mcp.HashToken(rawToken)
	apiToken := &repository.APIToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Name:      "test-agent",
	}
	if err := repo.CreateAPIToken(ctx, apiToken); err != nil {
		t.Fatalf("create token: %v", err)
	}

	aiMock := &mockAIClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := mcp.NewServer(repo, aiMock, mcp.Config{
		BasePath: "/mcp",
		Logger:   logger,
	})

	return db, repo, rawToken, server
}

func TestMCPServer_AuthMiddleware(t *testing.T) {
	db, repo, validToken, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	handler := server.Routes()

	// 1. Missing token -> 401
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", rec.Code)
	}

	// 2. Invalid token -> 401
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer invalid-token-xyz")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", rec.Code)
	}

	// 3. Valid token via Header -> succeeds
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+validToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid bearer token, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Valid token via Query Parameter -> succeeds
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages?token="+validToken, strings.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token in query param, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMCPServer_InitializeAndPing(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	handler := server.Routes()

	// 1. Initialize
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(initReq))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on initialize, got %d: %s", rec.Code, rec.Body.String())
	}

	var initResp mcp.JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("unexpected jsonrpc error: %v", initResp.Error)
	}

	resMap, ok := initResp.Result.(map[string]any)
	if !ok || resMap["protocolVersion"] != mcp.ProtocolVersion {
		t.Fatalf("expected protocolVersion %s, got %v", mcp.ProtocolVersion, resMap)
	}

	// 2. Notification initialized (no response required)
	notifReq := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(notifReq))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted on notification, got %d", rec.Code)
	}

	// 3. Ping
	pingReq := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(pingReq))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 on ping, got %d", rec.Code)
	}

	// 4. Unknown method
	unknownReq := `{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(unknownReq))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var unknownResp mcp.JSONRPCResponse
	json.Unmarshal(rec.Body.Bytes(), &unknownResp)
	if unknownResp.Error == nil || unknownResp.Error.Code != mcp.CodeMethodNotFound {
		t.Errorf("expected CodeMethodNotFound (-32601), got %v", unknownResp.Error)
	}

	// 5. Malformed JSON
	badReq := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(`{bad-json`))
	badReq.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, badReq)
	var badResp mcp.JSONRPCResponse
	json.Unmarshal(rec.Body.Bytes(), &badResp)
	if badResp.Error == nil || badResp.Error.Code != mcp.CodeParseError {
		t.Errorf("expected CodeParseError (-32700), got %v", badResp.Error)
	}
}

func TestMCPServer_ToolsList(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	handler := server.Routes()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on tools/list, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		JSONRPC string              `json:"jsonrpc"`
		ID      any                 `json:"id"`
		Result  mcp.ListToolsResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode tools/list response: %v", err)
	}

	if len(resp.Result.Tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(resp.Result.Tools))
	}

	toolNames := map[string]bool{}
	for _, tool := range resp.Result.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["search_library"] || !toolNames["get_book_metadata"] || !toolNames["read_chapter_content"] {
		t.Errorf("missing expected tools in tools/list: %v", toolNames)
	}
}

func TestMCPServer_ToolCall_SearchLibrary(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	ctx := context.Background()

	// Seed book & chapter with vector
	author, _ := repo.UpsertAuthor(ctx, "Neal Stephenson")
	genre, _ := repo.UpsertGenre(ctx, "Cyberpunk")
	series, _ := repo.UpsertSeries(ctx, "Metaverse", nil)

	book := &repository.Book{
		Title:    "Snow Crash",
		FilePath: "Neal Stephenson/Snow Crash/Snow Crash.epub",
	}
	repo.CreateBook(ctx, book)
	repo.LinkBookAuthor(ctx, book.ID, author.ID, "author")
	repo.LinkBookGenre(ctx, book.ID, genre.ID)
	seq := 1.0
	repo.LinkBookSeries(ctx, book.ID, series.ID, &seq)

	chTitle := "The Deliverator"
	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &chTitle,
		Summary:      "Hiro Protagonist drives through Los Angeles delivering pizza at supersonic speeds.",
		ContentPlain: "The Deliverator belongs to an elite order.",
	}
	repo.CreateChapter(ctx, ch)

	// Insert vector matching mock AI client embedding (dimension 0 = 1.0)
	vec := make([]float32, 1536)
	vec[0] = 1.0
	repo.InsertChapterVector(ctx, ch.ID, vec)

	handler := server.Routes()

	// 1. Successful search
	callBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_library","arguments":{"query":"high speed pizza delivery in the metaverse"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(callBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("tools/call search_library failed: %d %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result mcp.CallToolResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Result.IsError {
		t.Fatalf("search_library returned error: %v", resp.Result.Content)
	}
	if len(resp.Result.Content) == 0 || !strings.Contains(resp.Result.Content[0].Text, "Snow Crash") {
		t.Errorf("expected hit containing 'Snow Crash', got: %v", resp.Result.Content)
	}

	// 2. Search with author filter
	filterCall := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_library","arguments":{"query":"pizza delivery","author":"Neal Stephenson"}}}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(filterCall))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.Contains(resp.Result.Content[0].Text, "Snow Crash") {
		t.Errorf("expected hit for matching author, got: %s", resp.Result.Content[0].Text)
	}

	// 3. Search with non-existent author
	noAuthorCall := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_library","arguments":{"query":"pizza delivery","author":"Nonexistent Author"}}}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(noAuthorCall))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.Contains(resp.Result.Content[0].Text, "not found in library") {
		t.Errorf("expected friendly not found message, got: %s", resp.Result.Content[0].Text)
	}

	// 4. Missing query
	emptyQuery := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_library","arguments":{"query":""}}}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(emptyQuery))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Result.IsError {
		t.Errorf("expected isError: true on empty query")
	}
}

func TestMCPServer_ToolCall_GetBookMetadata(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	ctx := context.Background()

	desc := "A cyberpunk classic."
	book := &repository.Book{
		Title:       "Snow Crash",
		FilePath:    "author/snow.epub",
		Description: &desc,
	}
	repo.CreateBook(ctx, book)
	author, _ := repo.UpsertAuthor(ctx, "Neal Stephenson")
	repo.LinkBookAuthor(ctx, book.ID, author.ID, "author")

	chTitle := "Prologue"
	repo.CreateChapter(ctx, &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &chTitle,
		Summary:      "High stakes opening.",
	})

	handler := server.Routes()

	// 1. Success
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_book_metadata","arguments":{"book_id":"%s"}}}`, book.ID)
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp struct {
		Result mcp.CallToolResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Result.IsError {
		t.Fatalf("unexpected error: %v", resp.Result.Content)
	}
	text := resp.Result.Content[0].Text
	if !strings.Contains(text, "Snow Crash") || !strings.Contains(text, "Neal Stephenson") || !strings.Contains(text, "Prologue") {
		t.Errorf("expected metadata text to contain title, author, and TOC, got: %s", text)
	}

	// 2. Non-existent book ID
	body = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_book_metadata","arguments":{"book_id":"ghost-id"}}}`
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Result.IsError {
		t.Errorf("expected isError: true on missing book")
	}
}

func TestMCPServer_ToolCall_ReadChapterContent(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	ctx := context.Background()

	book := &repository.Book{Title: "Philosophy of Time", FilePath: "time.epub"}
	repo.CreateBook(ctx, book)

	paragraphs := []string{
		"Time is an illusion that manifests in sequence.",
		"The observer perceives entropy increasing in one direction.",
		"Relativity binds spacetime into a unified fabric.",
		"Quantum mechanics challenges the certainty of temporal flow.",
	}
	chTitle := "The Nature of Duration"
	ch := &repository.Chapter{
		BookID:       book.ID,
		ChapterIndex: 1,
		Title:        &chTitle,
		ContentPlain: strings.Join(paragraphs, "\n\n"),
	}
	repo.CreateChapter(ctx, ch)

	handler := server.Routes()

	// 1. Read all paragraphs (1 to 4)
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_chapter_content","arguments":{"book_id":"%s","chapter_index":1}}}`, book.ID)
	req := httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp struct {
		Result mcp.CallToolResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Result.IsError {
		t.Fatalf("unexpected error: %v", resp.Result.Content)
	}
	text := resp.Result.Content[0].Text
	if !strings.Contains(text, "Showing paragraphs 1 through 4 of 4 total") || !strings.Contains(text, "[¶1]") || !strings.Contains(text, "[¶4]") {
		t.Errorf("unexpected content response: %s", text)
	}

	// 2. Windowed read (paragraphs 2 to 3)
	body = fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_chapter_content","arguments":{"book_id":"%s","chapter_index":1,"start_paragraph":2,"end_paragraph":3}}}`, book.ID)
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	text = resp.Result.Content[0].Text
	if !strings.Contains(text, "Showing paragraphs 2 through 3 of 4 total") || strings.Contains(text, "[¶1]") || !strings.Contains(text, "[¶2]") || !strings.Contains(text, "[¶3]") || strings.Contains(text, "[¶4]") {
		t.Errorf("unexpected windowed response: %s", text)
	}

	// 3. Start paragraph exceeds total
	body = fmt.Sprintf(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_chapter_content","arguments":{"book_id":"%s","chapter_index":1,"start_paragraph":99}}}`, book.ID)
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Result.IsError {
		t.Errorf("expected isError: true when start paragraph exceeds bounds")
	}

	// 5. Read chapter using chapter_id
	body = fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_chapter_content","arguments":{"book_id":"%s","chapter_id":"%s"}}}`, book.ID, ch.ID)
	req = httptest.NewRequest(http.MethodPost, "/mcp/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp5 struct {
		Result mcp.CallToolResult `json:"result"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp5)
	if resp5.Result.IsError {
		t.Fatalf("unexpected error reading by chapter_id: %v", resp5.Result.Content)
	}
	if !strings.Contains(resp5.Result.Content[0].Text, "Showing paragraphs 1 through 4 of 4 total") {
		t.Errorf("unexpected content response for chapter_id: %s", resp5.Result.Content[0].Text)
	}
}

func TestMCPServer_SSEFlow(t *testing.T) {
	db, repo, token, server := setupTestEnvironment(t)
	defer db.Close()
	defer repo.Close()

	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Connect to SSE
	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/mcp/sse", nil)
	if err != nil {
		t.Fatalf("create sse req: %v", err)
	}
	sseReq.Header.Set("Authorization", "Bearer "+token)

	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("do sse req: %v", err)
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on sse, got %d", sseResp.StatusCode)
	}

	// 2. Read endpoint event
	reader := bufio.NewReader(sseResp.Body)
	var endpointLine string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading sse line: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			endpointLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	if !strings.Contains(endpointLine, "/messages?sessionId=") {
		t.Fatalf("expected endpoint declaring /messages?sessionId=, got: %s", endpointLine)
	}

	// 3. Post a message to that endpoint
	msgURL := ts.URL + endpointLine
	msgBody := `{"jsonrpc":"2.0","id":100,"method":"tools/list"}`
	postReq, err := http.NewRequest(http.MethodPost, msgURL, bytes.NewBufferString(msgBody))
	if err != nil {
		t.Fatalf("create post req: %v", err)
	}
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatalf("post message: %v", err)
	}
	postResp.Body.Close()

	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on post to sse session, got %d", postResp.StatusCode)
	}

	// 4. Read response message event from the SSE stream
	var responseData string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading sse response event: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			responseData = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	var jsonResp mcp.JSONRPCResponse
	if err := json.Unmarshal([]byte(responseData), &jsonResp); err != nil {
		t.Fatalf("failed to decode jsonrpc response from sse stream: %v, raw: %s", err, responseData)
	}
	if jsonResp.ID != float64(100) {
		t.Errorf("expected id 100, got %v", jsonResp.ID)
	}
}
