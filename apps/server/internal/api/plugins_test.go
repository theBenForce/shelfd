package api_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shelfd/shelfd/internal/api"
	"github.com/shelfd/shelfd/internal/mcp"
	"github.com/shelfd/shelfd/internal/plugins"
	"github.com/shelfd/shelfd/internal/repository"
)

func buildMinimalEPUBWithoutAuthors(t *testing.T, title string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, _ := zw.Create("META-INF/container.xml")
	_, _ = w.Write([]byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	opf, _ := zw.Create("OEBPS/content.opf")
	_, _ = opf.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + title + `</dc:title>
  </metadata>
  <manifest>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>`))

	ch1, _ := zw.Create("OEBPS/ch1.xhtml")
	_, _ = ch1.Write([]byte(`<!DOCTYPE html><html><head><title>Chapter 1</title></head><body><p>Call me Ishmael.</p></body></html>`))
	_ = zw.Close()
	return buf.Bytes()
}

func TestAPI_ScopedPluginJWT_And_ProvenanceStagingLifecycle(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	// Mint a short-lived scoped plugin invocation JWT with ONLY "uploads:stage"
	scopedToken, _, err := api.GeneratePluginInvocationJWT(
		f.user.ID,
		f.user.Username,
		"project-gutenberg",
		[]string{"uploads:stage"},
		f.jwtSecret,
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("GeneratePluginInvocationJWT failed: %v", err)
	}

	// 1. Scoped token MUST be rejected (403 Forbidden) on unrelated endpoints like /api/v1/auth/tokens or /api/v1/books
	for _, blocked := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/auth/tokens"},
		{http.MethodPost, "/api/v1/auth/tokens"},
		{http.MethodGet, "/api/v1/books"},
		{http.MethodGet, "/api/v1/plugins"},
	} {
		req := httptest.NewRequest(blocked.method, blocked.path, nil)
		req.Header.Set("Authorization", "Bearer "+scopedToken)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for scoped token on %s %s, got %d: %s", blocked.method, blocked.path, rec.Code, rec.Body.String())
		}
	}

	// 2. Scoped token with "uploads:stage" IS permitted on POST /api/v1/books/upload/stage
	epubBytes := buildMinimalEPUBWithoutAuthors(t, "Moby Dick")
	body := &bytes.Buffer{}
	mpw := multipart.NewWriter(body)
	part, _ := mpw.CreateFormFile("file", "pg2701.epub")
	_, _ = part.Write(epubBytes)
	covPart, _ := mpw.CreateFormFile("cover", "cover.jpg")
	_, _ = covPart.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
	_ = mpw.WriteField("source_plugin", "project-gutenberg")
	_ = mpw.WriteField("source_external_id", "2701")
	_ = mpw.WriteField("metadata_defaults", `{"authors":["Herman Melville"],"publisher":"Project Gutenberg","genres":["Sea Stories","Classics"]}`)
	_ = mpw.Close()

	stageReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/stage", body)
	stageReq.Header.Set("Authorization", "Bearer "+scopedToken)
	stageReq.Header.Set("Content-Type", mpw.FormDataContentType())
	stageRec := httptest.NewRecorder()
	f.handler.ServeHTTP(stageRec, stageReq)

	if stageRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging book with scoped JWT, got %d: %s", stageRec.Code, stageRec.Body.String())
	}

	var stageResp struct {
		JobID    string `json:"job_id"`
		Status   string `json:"status"`
		HasCover bool   `json:"has_cover"`
		Metadata struct {
			Title     string   `json:"title"`
			Authors   []string `json:"authors"`
			Publisher string   `json:"publisher"`
			Genres    []string `json:"genres"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(stageRec.Body.Bytes(), &stageResp); err != nil {
		t.Fatalf("unmarshal stageResp: %v", err)
	}
	if !stageResp.HasCover {
		t.Errorf("expected HasCover=true from multipart cover part")
	}
	if len(stageResp.Metadata.Authors) != 1 || stageResp.Metadata.Authors[0] != "Herman Melville" {
		t.Errorf("expected fallback author Herman Melville from metadata_defaults, got %+v", stageResp.Metadata.Authors)
	}
	if stageResp.Metadata.Publisher != "Project Gutenberg" {
		t.Errorf("expected fallback publisher Project Gutenberg, got %q", stageResp.Metadata.Publisher)
	}

	// Verify book_sources provenance row points to staged upload job
	sources, err := f.repo.GetBookSourcesByExternalIDs(context.Background(), "project-gutenberg", []string{"2701"})
	if err != nil || len(sources) != 1 {
		t.Fatalf("expected 1 book_source row, got %d (err=%v)", len(sources), err)
	}
	if sources[0].UploadJobID == nil || *sources[0].UploadJobID != stageResp.JobID {
		t.Fatalf("expected book_source linked to upload_job_id %s, got %+v", stageResp.JobID, sources[0])
	}

	// Scoped token CANNOT commit the upload job itself (requires human user session)
	commitForbiddenReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+stageResp.JobID+"/commit", strings.NewReader(`{}`))
	commitForbiddenReq.Header.Set("Authorization", "Bearer "+scopedToken)
	commitForbiddenRec := httptest.NewRecorder()
	f.handler.ServeHTTP(commitForbiddenRec, commitForbiddenReq)
	if commitForbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when plugin token attempts commit, got %d", commitForbiddenRec.Code)
	}

	// 3. Human user commits the staged upload -> book_sources links to final BookID!
	userToken := f.loginAndGetToken(t)
	commitReq := httptest.NewRequest(http.MethodPost, "/api/v1/books/upload/jobs/"+stageResp.JobID+"/commit", strings.NewReader(`{}`))
	commitReq.Header.Set("Authorization", "Bearer "+userToken)
	commitRec := httptest.NewRecorder()
	f.handler.ServeHTTP(commitRec, commitReq)
	if commitRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on human CommitUploadJob, got %d: %s", commitRec.Code, commitRec.Body.String())
	}

	var committedBook struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(commitRec.Body.Bytes(), &committedBook)
	if committedBook.ID == "" {
		t.Fatalf("expected committed book ID in response: %s", commitRec.Body.String())
	}

	sourcesAfter, err := f.repo.GetBookSourcesByExternalIDs(context.Background(), "project-gutenberg", []string{"2701"})
	if err != nil || len(sourcesAfter) != 1 || sourcesAfter[0].BookID == nil || *sourcesAfter[0].BookID != committedBook.ID {
		t.Fatalf("expected book_source linked to final BookID %s, got %+v", committedBook.ID, sourcesAfter)
	}
}

func TestAPI_PluginsREST_InstallConfigureCatalogAndStage(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	pluginsDir := t.TempDir()
	var ts *httptest.Server

	pluginHost := plugins.NewHost(f.repo, plugins.HostConfig{
		PluginsDir:    pluginsDir,
		APIBase:       "http://127.0.0.1:0", // updated once httptest starts
		ShelfdVersion: "0.1.0-test",
		MasterSecret:  f.jwtSecret,
		IdleTimeout:   30 * time.Second,
		TokenIssuer: func(userID, username, pluginName string, scopes []string) (string, error) {
			tok, _, err := api.GeneratePluginInvocationJWT(userID, username, pluginName, scopes, f.jwtSecret, 5*time.Minute)
			return tok, err
		},
		Hub:    f.hub,
		Logger: slog.Default(),
	})
	defer pluginHost.Stop()

	router := api.NewRouter(api.RouterConfig{
		Repo:            f.repo,
		Ingester:        f.ingester,
		Scanner:         f.scanner,
		UploadWorker:    f.uploadWorker,
		AIClient:        f.aiClient,
		ToolInvoker:     mcp.NewToolExecutor(f.repo, f.aiClient, slog.Default()),
		PluginHost:      pluginHost,
		Hub:             f.hub,
		DataDir:         f.dataDir,
		LibraryDir:      f.libDir,
		JWTSecret:       f.jwtSecret,
		Host:            "127.0.0.1",
		Port:            8080,
		Version:         "0.1.0-test",
		DefaultUsername: "admin",
	})
	ts = httptest.NewServer(router)
	defer ts.Close()

	userToken := f.loginAndGetToken(t)

	// Build a zip plugin bundle that stages a synthetic EPUB to ts.URL when catalog/stage is called
	epubBytes := buildMinimalEPUBWithoutAuthors(t, "Pride and Prejudice")
	epubJSON, _ := json.Marshal(epubBytes) // base64 string in Go encoding/json

	pluginJS := `
import readline from 'node:readline';
const epubBytes = Buffer.from(` + string(epubJSON) + `, 'base64');
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
function send(msg) { process.stdout.write(JSON.stringify(msg) + '\n'); }

rl.on('line', async (line) => {
  if (!line.trim()) return;
  const msg = JSON.parse(line);
  if (msg.method === 'initialize') {
    send({ jsonrpc: '2.0', id: msg.id, result: { name: 'zip-catalog', version: '1.0.0' } });
  } else if (msg.method === 'shutdown') {
    send({ jsonrpc: '2.0', id: msg.id, result: { ok: true } });
    process.exit(0);
  } else if (msg.method === 'catalog/browse') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        groups: [{ id: 'classics', label: 'Classics', count: 1 }],
        items: [{ external_id: '1342', title: 'Pride and Prejudice', authors: ['Jane Austen'], formats: [{ format: 'epub' }] }]
      }
    });
  } else if (msg.method === 'catalog/stage') {
    const form = new FormData();
    form.append('file', new Blob([epubBytes], { type: 'application/epub+zip' }), 'pg1342.epub');
    form.append('source_plugin', 'zip-catalog');
    form.append('source_external_id', '1342');
    form.append('metadata_defaults', JSON.stringify({ authors: ['Jane Austen'] }));
    const res = await fetch('` + ts.URL + `/api/v1/books/upload/stage', {
      method: 'POST',
      headers: { Authorization: 'Bearer ' + msg.params.auth.invocation_token },
      body: form
    });
    const data = await res.json();
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        results: [{ external_id: '1342', title: 'Pride and Prejudice', upload_job_id: data.job_id, status: 'staged' }]
      }
    });
  }
});
`
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	mfW, _ := zw.Create("plugin.json")
	_, _ = mfW.Write([]byte(`{
  "name": "zip-catalog",
  "version": "1.0.0",
  "display_name": "Zip Catalog",
  "entrypoint": { "command": "node", "args": ["index.mjs"] },
  "capabilities": ["catalog"],
  "catalog_modes": ["browse"],
  "api_scopes": ["uploads:stage"],
  "config_schema": {
    "type": "object",
    "properties": {
      "token": { "type": "string", "x-shelfd-secret": true, "x-shelfd-curl-paste": true }
    }
  }
}`))
	jsW, _ := zw.Create("index.mjs")
	_, _ = jsW.Write([]byte(pluginJS))
	_ = zw.Close()

	// 1. POST /api/v1/plugins/install
	var installBody bytes.Buffer
	mpw := multipart.NewWriter(&installBody)
	fw, _ := mpw.CreateFormFile("file", "zip-catalog.zip")
	_, _ = fw.Write(zipBuf.Bytes())
	_ = mpw.Close()

	installReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/plugins/install", &installBody)
	installReq.Header.Set("Authorization", "Bearer "+userToken)
	installReq.Header.Set("Content-Type", mpw.FormDataContentType())
	installResp, err := http.DefaultClient.Do(installReq)
	if err != nil {
		t.Fatalf("install request failed: %v", err)
	}
	defer installResp.Body.Close()
	if installResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created on plugin install, got %d", installResp.StatusCode)
	}

	// 2. PUT & GET /api/v1/plugins/zip-catalog/config (verify secret redaction)
	cfgReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/plugins/zip-catalog/config", strings.NewReader(`{"token":"Bearer my-secret-tok"}`))
	cfgReq.Header.Set("Authorization", "Bearer "+userToken)
	cfgResp, err := http.DefaultClient.Do(cfgReq)
	if err != nil || cfgResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT config failed: status=%v err=%v", cfgResp.StatusCode, err)
	}
	var cfgPayload struct {
		Config          map[string]any `json:"config"`
		SecretFieldsSet []string       `json:"secret_fields_set"`
	}
	_ = json.NewDecoder(cfgResp.Body).Decode(&cfgPayload)
	cfgResp.Body.Close()
	if cfgPayload.Config["token"] != plugins.SecretSentinel {
		t.Fatalf("expected masked token in PUT response, got %+v", cfgPayload)
	}

	// 3. GET /api/v1/plugins/zip-catalog/catalog -> status "available"
	catReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/plugins/zip-catalog/catalog?mode=browse", nil)
	catReq.Header.Set("Authorization", "Bearer "+userToken)
	catResp, err := http.DefaultClient.Do(catReq)
	if err != nil || catResp.StatusCode != http.StatusOK {
		t.Fatalf("GET catalog failed: %v", err)
	}
	var pageBefore plugins.CatalogPage
	_ = json.NewDecoder(catResp.Body).Decode(&pageBefore)
	catResp.Body.Close()
	if len(pageBefore.Items) != 1 || pageBefore.Items[0].Status != "available" {
		t.Fatalf("expected 1 available item before staging, got %+v", pageBefore.Items)
	}

	// 4. POST /api/v1/plugins/zip-catalog/catalog/stage -> stages EPUB via loopback API!
	stageReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/plugins/zip-catalog/catalog/stage", strings.NewReader(`{"items":[{"external_id":"1342"}]}`))
	stageReq.Header.Set("Authorization", "Bearer "+userToken)
	stageResp, err := http.DefaultClient.Do(stageReq)
	if err != nil || stageResp.StatusCode != http.StatusOK {
		t.Fatalf("POST catalog/stage failed: status=%v err=%v", stageResp.StatusCode, err)
	}
	var stageRes plugins.CatalogStageResult
	_ = json.NewDecoder(stageResp.Body).Decode(&stageRes)
	stageResp.Body.Close()
	if len(stageRes.Results) != 1 || stageRes.Results[0].Status != "staged" || stageRes.Results[0].UploadJobID == "" {
		t.Fatalf("expected staged entry with job ID, got %+v", stageRes)
	}

	// 5. Re-query GET /api/v1/plugins/zip-catalog/catalog -> status is now "staged"!
	catReq2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/plugins/zip-catalog/catalog?mode=browse", nil)
	catReq2.Header.Set("Authorization", "Bearer "+userToken)
	catResp2, _ := http.DefaultClient.Do(catReq2)
	var pageAfter plugins.CatalogPage
	_ = json.NewDecoder(catResp2.Body).Decode(&pageAfter)
	catResp2.Body.Close()
	if len(pageAfter.Items) != 1 || pageAfter.Items[0].Status != "staged" {
		t.Fatalf("expected item status 'staged' after catalog/stage, got %+v", pageAfter.Items)
	}
}

func TestAPI_RealProjectGutenbergPlugin_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not found on PATH; skipping real TypeScript plugin E2E test")
	}

	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	pgSrcDir := filepath.Join(repoRoot, "plugins", "project-gutenberg")
	sdkCli := filepath.Join(repoRoot, "packages", "plugin-sdk", "cli", "shelfd-plugin.mjs")
	pgDist := filepath.Join(pgSrcDir, "dist", "index.mjs")

	if _, err := os.Stat(pgDist); err != nil {
		cmd := exec.Command("node", sdkCli, "build", pgSrcDir)
		if out, bErr := cmd.CombinedOutput(); bErr != nil {
			t.Fatalf("building project-gutenberg plugin failed: %v\n%s", bErr, string(out))
		}
	}

	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	// Spin up mock Gutendex server serving Frankenstein (#84)
	epubBytes := buildMinimalEPUBWithoutAuthors(t, "Frankenstein; Or, The Modern Prometheus")
	var mockGutendex *httptest.Server
	mockGutendex = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/books":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"count": 1,
				"next":  nil,
				"results": []map[string]any{
					{
						"id":          84,
						"title":       "Frankenstein; Or, The Modern Prometheus",
						"authors":     []map[string]any{{"name": "Shelley, Mary Wollstonecraft"}},
						"summaries":   []string{"Victor Frankenstein creates a sentient creature in an unorthodox scientific experiment."},
						"bookshelves": []string{"Gothic Fiction", "Science Fiction"},
						"languages":   []string{"en"},
						"formats": map[string]string{
							"application/epub+zip": mockGutendex.URL + "/files/pg84.epub",
							"image/jpeg":           mockGutendex.URL + "/files/pg84.jpg",
						},
					},
				},
			})
		case r.URL.Path == "/books/84":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":          84,
				"title":       "Frankenstein; Or, The Modern Prometheus",
				"authors":     []map[string]any{{"name": "Shelley, Mary Wollstonecraft"}},
				"summaries":   []string{"Victor Frankenstein creates a sentient creature in an unorthodox scientific experiment."},
				"bookshelves": []string{"Gothic Fiction", "Science Fiction"},
				"languages":   []string{"en"},
				"formats": map[string]string{
					"application/epub+zip": mockGutendex.URL + "/files/pg84.epub",
					"image/jpeg":           mockGutendex.URL + "/files/pg84.jpg",
				},
			})
		case r.URL.Path == "/files/pg84.epub":
			w.Header().Set("Content-Type", "application/epub+zip")
			_, _ = w.Write(epubBytes)
		case r.URL.Path == "/files/pg84.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockGutendex.Close()

	// Install compiled project-gutenberg plugin into temp plugins directory
	pluginsDir := t.TempDir()
	installedDir := filepath.Join(pluginsDir, "project-gutenberg")
	if err := os.MkdirAll(filepath.Join(installedDir, "dist"), 0755); err != nil {
		t.Fatalf("mkdir installed plugin: %v", err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(pgSrcDir, "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	distBytes, err := os.ReadFile(pgDist)
	if err != nil {
		t.Fatalf("read dist/index.mjs: %v", err)
	}
	_ = os.WriteFile(filepath.Join(installedDir, "plugin.json"), manifestBytes, 0644)
	_ = os.WriteFile(filepath.Join(installedDir, "dist", "index.mjs"), distBytes, 0644)

	// Start temporary listener first so pluginHost gets real loopback APIBase URL
	var shelfdServer *httptest.Server
	toolExec := mcp.NewToolExecutor(f.repo, f.aiClient, slog.Default())

	// Create server mux placeholder so we know shelfdServer.URL before starting Host
	var activeHandler http.Handler
	shelfdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeHandler.ServeHTTP(w, r)
	}))
	defer shelfdServer.Close()

	pluginHost := plugins.NewHost(f.repo, plugins.HostConfig{
		PluginsDir:    pluginsDir,
		APIBase:       shelfdServer.URL,
		ShelfdVersion: "0.1.0-test",
		MasterSecret:  f.jwtSecret,
		IdleTimeout:   30 * time.Second,
		TokenIssuer: func(userID, username, pluginName string, scopes []string) (string, error) {
			tok, _, err := api.GeneratePluginInvocationJWT(userID, username, pluginName, scopes, f.jwtSecret, 5*time.Minute)
			return tok, err
		},
		Hub:    f.hub,
		Logger: slog.Default(),
	})
	toolExec.SetPluginProvider(pluginHost)

	activeHandler = api.NewRouter(api.RouterConfig{
		Repo:            f.repo,
		Ingester:        f.ingester,
		Scanner:         f.scanner,
		UploadWorker:    f.uploadWorker,
		AIClient:        f.aiClient,
		ToolInvoker:     toolExec,
		PluginHost:      pluginHost,
		Hub:             f.hub,
		DataDir:         f.dataDir,
		LibraryDir:      f.libDir,
		JWTSecret:       f.jwtSecret,
		Host:            "127.0.0.1",
		Port:            8080,
		Version:         "0.1.0-test",
		DefaultUsername: "admin",
	})

	pluginHost.Start(context.Background())
	defer pluginHost.Stop()

	userToken := f.loginAndGetToken(t)

	// 1. Point project-gutenberg base_url at mockGutendex
	cfgReq, _ := http.NewRequest(http.MethodPut, shelfdServer.URL+"/api/v1/plugins/project-gutenberg/config", strings.NewReader(`{"base_url":"`+mockGutendex.URL+`"}`))
	cfgReq.Header.Set("Authorization", "Bearer "+userToken)
	cfgResp, err := http.DefaultClient.Do(cfgReq)
	if err != nil || cfgResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT project-gutenberg config failed: %v", err)
	}
	cfgResp.Body.Close()

	// 2. Verify MCP tool project-gutenberg_search_books works through ToolExecutor!
	mcpCtx := context.WithValue(context.Background(), mcp.TokenContextKey, &repository.APIToken{UserID: f.user.ID})
	mcpRes, err := toolExec.ExecuteInProcess(mcpCtx, "project-gutenberg_search_books", map[string]any{"query": "frankenstein"})
	if err != nil || mcpRes.IsError {
		t.Fatalf("MCP tool project-gutenberg_search_books failed: err=%v res=%+v", err, mcpRes)
	}
	if !strings.Contains(mcpRes.Content, "[pg:84]") || !strings.Contains(mcpRes.Content, "Mary Wollstonecraft Shelley") {
		t.Fatalf("unexpected MCP tool output: %s", mcpRes.Content)
	}

	// 3. Search catalog via REST -> status "available"
	searchReq, _ := http.NewRequest(http.MethodGet, shelfdServer.URL+"/api/v1/plugins/project-gutenberg/catalog?mode=search&q=frankenstein", nil)
	searchReq.Header.Set("Authorization", "Bearer "+userToken)
	searchResp, err := http.DefaultClient.Do(searchReq)
	if err != nil || searchResp.StatusCode != http.StatusOK {
		t.Fatalf("GET catalog search failed: %v", err)
	}
	var page1 plugins.CatalogPage
	_ = json.NewDecoder(searchResp.Body).Decode(&page1)
	searchResp.Body.Close()
	if len(page1.Items) != 1 || page1.Items[0].ExternalID != "84" || page1.Items[0].Status != "available" {
		t.Fatalf("expected item 84 available, got %+v", page1.Items)
	}

	// 4. Stage item #84 -> downloads EPUB + cover, uploads via loopback StageUploadBook
	stageReq, _ := http.NewRequest(http.MethodPost, shelfdServer.URL+"/api/v1/plugins/project-gutenberg/catalog/stage", strings.NewReader(`{"items":[{"external_id":"84"}]}`))
	stageReq.Header.Set("Authorization", "Bearer "+userToken)
	stageResp, err := http.DefaultClient.Do(stageReq)
	if err != nil || stageResp.StatusCode != http.StatusOK {
		t.Fatalf("POST catalog/stage failed: %v", err)
	}
	var stageRes plugins.CatalogStageResult
	_ = json.NewDecoder(stageResp.Body).Decode(&stageRes)
	stageResp.Body.Close()
	if len(stageRes.Results) != 1 || stageRes.Results[0].Status != "staged" || stageRes.Results[0].UploadJobID == "" {
		t.Fatalf("expected staged result for #84, got %+v", stageRes)
	}
	jobID := stageRes.Results[0].UploadJobID

	// 5. Commit staged upload -> transitions catalog status to "in_library"!
	commitReq, _ := http.NewRequest(http.MethodPost, shelfdServer.URL+"/api/v1/books/upload/jobs/"+jobID+"/commit", strings.NewReader(`{}`))
	commitReq.Header.Set("Authorization", "Bearer "+userToken)
	commitResp, err := http.DefaultClient.Do(commitReq)
	if err != nil || commitResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST commit failed: status=%v err=%v", commitResp.StatusCode, err)
	}
	commitResp.Body.Close()

	searchReq2, _ := http.NewRequest(http.MethodGet, shelfdServer.URL+"/api/v1/plugins/project-gutenberg/catalog?mode=search&q=frankenstein", nil)
	searchReq2.Header.Set("Authorization", "Bearer "+userToken)
	searchResp2, _ := http.DefaultClient.Do(searchReq2)
	var page2 plugins.CatalogPage
	_ = json.NewDecoder(searchResp2.Body).Decode(&page2)
	searchResp2.Body.Close()
	if len(page2.Items) != 1 || page2.Items[0].Status != "in_library" || page2.Items[0].BookID == nil {
		t.Fatalf("expected item 84 status 'in_library' after commit, got %+v", page2.Items)
	}
}
