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

func TestAPI_PluginBooksReadScope_ActionsAuthAssetsAndJobsSSE(t *testing.T) {
	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	book := seedNotebookLMTestBook(t, f)

	seriesList, err := f.repo.ListSeries(ctx)
	if err != nil || len(seriesList) == 0 {
		t.Fatalf("ListSeries: %v", err)
	}
	seriesID := seriesList[0].ID

	booksReadToken, _, err := api.GeneratePluginInvocationJWT(
		f.user.ID,
		f.user.Username,
		"action-plugin",
		[]string{"books:read"},
		f.jwtSecret,
		5*time.Minute,
	)
	if err != nil {
		t.Fatalf("GeneratePluginInvocationJWT failed: %v", err)
	}

	for _, allowedPath := range []string{
		"/api/v1/books/" + book.ID + "/markdown",
		"/api/v1/audiobooks/" + book.ID + "/chapters",
		"/api/v1/series/" + seriesID,
	} {
		req := httptest.NewRequest(http.MethodGet, allowedPath, nil)
		req.Header.Set("Authorization", "Bearer "+booksReadToken)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for books:read token on GET %s, got %d: %s", allowedPath, rec.Code, rec.Body.String())
		}
	}

	if api.PluginScopeAllowsRequest([]string{"books:read"}, http.MethodDelete, "/api/v1/books/"+book.ID) {
		t.Fatalf("expected PluginScopeAllowsRequest to reject DELETE /api/v1/books/{id} under books:read")
	}
	for _, blocked := range []struct {
		method string
		path   string
	}{
		{http.MethodDelete, "/api/v1/books/" + book.ID + "/highlights/hl-nlm-1"},
		{http.MethodDelete, "/api/v1/audiobooks/" + book.ID + "/progress"},
		{http.MethodPut, "/api/v1/books/" + book.ID + "/metadata"},
		{http.MethodPost, "/api/v1/books/upload/stage"},
	} {
		req := httptest.NewRequest(blocked.method, blocked.path, nil)
		req.Header.Set("Authorization", "Bearer "+booksReadToken)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for books:read token on %s %s, got %d", blocked.method, blocked.path, rec.Code)
		}
	}

	pluginsDir := t.TempDir()
	pDir := filepath.Join(pluginsDir, "demo-actions")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}

	pluginScript := `
import readline from 'node:readline';
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
function send(msg) { process.stdout.write(JSON.stringify(msg) + '\n'); }

rl.on('line', async (line) => {
  if (!line.trim()) return;
  const msg = JSON.parse(line);
  if (!msg.method) return;

  if (msg.method === 'initialize') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        name: 'demo-actions',
        version: '1.0.0',
        auth: { methods: ['oauth_pkce', 'token'], title: 'Link Demo' },
        actions: [
          {
            name: 'send_card',
            title: 'Send Card',
            targets: ['book', 'series'],
            media_types: ['audiobook', 'ebook'],
            mode: 'form',
            execution: 'job'
          }
        ]
      }
    });
  } else if (msg.method === 'shutdown') {
    send({ jsonrpc: '2.0', id: msg.id, result: { ok: true } });
    process.exit(0);
  } else if (msg.method === 'auth/status') {
    send({ jsonrpc: '2.0', id: msg.id, result: { linked: true, account_label: 'demo-user' } });
  } else if (msg.method === 'auth/begin') {
    send({ jsonrpc: '2.0', id: msg.id, result: { authorize_url: 'https://example.com/auth?state=' + (msg.params?.state || '') } });
  } else if (msg.method === 'auth/complete') {
    send({ jsonrpc: '2.0', id: msg.id, result: { linked: true, account_label: 'demo-user' } });
  } else if (msg.method === 'auth/disconnect') {
    send({ jsonrpc: '2.0', id: msg.id, result: { linked: false } });
  } else if (msg.method === 'actions/prepare') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        title: 'Send ' + (msg.params?.target?.title || msg.params?.target?.id || ''),
        fields: [{ key: 'card_title', kind: 'text', title: 'Card Title' }],
        entries: [{ id: 'e1', index: 1, title: 'Entry 1' }]
      }
    });
  } else if (msg.method === 'actions/options') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        items: [{ value: 'opt-1', label: 'Option ' + (msg.params?.source || '') + ':' + (msg.params?.query || '') }]
      }
    });
  } else if (msg.method === 'actions/upload_asset') {
    const raw = Buffer.from(msg.params?.data_base64 || '', 'base64');
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        item: { value: 'asset://' + msg.params?.filename, label: msg.params?.filename + ' (' + raw.length + 'B)' }
      }
    });
  } else if (msg.method === 'actions/invoke') {
    send({
      jsonrpc: '2.0',
      method: 'progress',
      params: {
        job_id: msg.params?.job_id,
        current: 1,
        total: 1,
        percent: 100,
        title: 'Chapter 1',
        message: 'Transferring...'
      }
    });
    await new Promise((r) => setTimeout(r, 80));
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        status: 'completed',
        message: 'Card transferred'
      }
    });
  }
});
`
	manifestJSON := `{
  "name": "demo-actions",
  "version": "1.0.0",
  "display_name": "Demo Actions",
  "entrypoint": { "command": "node", "args": ["index.mjs"] },
  "capabilities": ["actions", "events"],
  "api_scopes": ["books:read"]
}`
	_ = os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(manifestJSON), 0644)
	_ = os.WriteFile(filepath.Join(pDir, "index.mjs"), []byte(pluginScript), 0644)

	eventCh, unsub := f.hub.Subscribe()
	defer unsub()

	pluginHost := plugins.NewHost(f.repo, plugins.HostConfig{
		PluginsDir:    pluginsDir,
		APIBase:       "http://127.0.0.1:8080",
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
	pluginHost.Start(ctx)
	defer pluginHost.Stop()

	router := api.NewRouter(api.RouterConfig{
		Repo:         f.repo,
		Ingester:     f.ingester,
		Scanner:      f.scanner,
		UploadWorker: f.uploadWorker,
		AIClient:     f.aiClient,
		PluginHost:   pluginHost,
		Hub:          f.hub,
		DataDir:      f.dataDir,
		LibraryDir:   f.libDir,
		JWTSecret:    f.jwtSecret,
	})

	userToken := f.loginAndGetToken(t)

	for _, step := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/plugins/demo-actions/auth/begin", `{"method":"oauth_pkce","state":"s1"}`},
		{http.MethodPost, "/api/v1/plugins/demo-actions/auth/complete", `{"method":"oauth_pkce","code":"c1"}`},
		{http.MethodGet, "/api/v1/plugins/demo-actions/auth/status", ""},
		{http.MethodDelete, "/api/v1/plugins/demo-actions/auth/disconnect", ""},
	} {
		req := httptest.NewRequest(step.method, step.path, strings.NewReader(step.body))
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on %s %s, got %d: %s", step.method, step.path, rec.Code, rec.Body.String())
		}
	}

	prepReq := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/demo-actions/actions/send_card/prepare", strings.NewReader(`{"target":{"type":"book","id":"`+book.ID+`","title":"Solaris"}}`))
	prepReq.Header.Set("Authorization", "Bearer "+userToken)
	prepRec := httptest.NewRecorder()
	router.ServeHTTP(prepRec, prepReq)
	if prepRec.Code != http.StatusOK || !strings.Contains(prepRec.Body.String(), `"Send Solaris"`) {
		t.Fatalf("unexpected actions/prepare response (%d): %s", prepRec.Code, prepRec.Body.String())
	}

	optReq := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/demo-actions/actions/send_card/options?source=targets&q=test", nil)
	optReq.Header.Set("Authorization", "Bearer "+userToken)
	optRec := httptest.NewRecorder()
	router.ServeHTTP(optRec, optReq)
	if optRec.Code != http.StatusOK || !strings.Contains(optRec.Body.String(), `"Option targets:test"`) {
		t.Fatalf("unexpected actions/options response (%d): %s", optRec.Code, optRec.Body.String())
	}

	buildAssetReq := func(payload []byte) *http.Request {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		fw, _ := mw.CreateFormFile("file", "icon.png")
		_, _ = fw.Write(payload)
		_ = mw.WriteField("field", "icon")
		_ = mw.Close()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/demo-actions/actions/send_card/assets", &b)
		r.Header.Set("Authorization", "Bearer "+userToken)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		return r
	}

	smallAssetRec := httptest.NewRecorder()
	router.ServeHTTP(smallAssetRec, buildAssetReq([]byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4}))
	if smallAssetRec.Code != http.StatusOK || !strings.Contains(smallAssetRec.Body.String(), `"asset://icon.png"`) {
		t.Fatalf("expected 200 OK for small asset upload, got %d: %s", smallAssetRec.Code, smallAssetRec.Body.String())
	}

	oversizedAsset := bytes.Repeat([]byte("A"), (5<<20)+1024)
	largeAssetRec := httptest.NewRecorder()
	router.ServeHTTP(largeAssetRec, buildAssetReq(oversizedAsset))
	if largeAssetRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 RequestEntityTooLarge for >5MB asset upload, got %d: %s", largeAssetRec.Code, largeAssetRec.Body.String())
	}

	invReq := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/demo-actions/actions/send_card/invoke", strings.NewReader(`{"target":{"type":"book","id":"`+book.ID+`","title":"Solaris"}}`))
	invReq.Header.Set("Authorization", "Bearer "+userToken)
	invRec := httptest.NewRecorder()
	router.ServeHTTP(invRec, invReq)
	if invRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on actions/invoke, got %d: %s", invRec.Code, invRec.Body.String())
	}
	var invRes plugins.ActionResult
	_ = json.Unmarshal(invRec.Body.Bytes(), &invRes)
	if invRes.Status != "job_started" || invRes.JobID == "" {
		t.Fatalf("expected job_started with JobID, got %+v", invRes)
	}

	sawJobProgress := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !sawJobProgress {
		select {
		case ev := <-eventCh:
			if m, ok := ev.Data.(map[string]any); ok && m["kind"] == "job_progress" && m["job_id"] == invRes.JobID {
				if jb, ok := m["job"].(*plugins.PluginJob); ok && jb.Status == "completed" {
					sawJobProgress = true
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	if !sawJobProgress {
		t.Fatalf("expected SSE job_progress event reaching completed status for job %s", invRes.JobID)
	}

	jobsReq := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/jobs", nil)
	jobsReq.Header.Set("Authorization", "Bearer "+userToken)
	jobsRec := httptest.NewRecorder()
	router.ServeHTTP(jobsRec, jobsReq)
	if jobsRec.Code != http.StatusOK || !strings.Contains(jobsRec.Body.String(), invRes.JobID) {
		t.Fatalf("expected GET /api/v1/plugins/jobs to list job %s, got %d: %s", invRes.JobID, jobsRec.Code, jobsRec.Body.String())
	}

	delJobReq := httptest.NewRequest(http.MethodDelete, "/api/v1/plugins/jobs/"+invRes.JobID, nil)
	delJobReq.Header.Set("Authorization", "Bearer "+userToken)
	delJobRec := httptest.NewRecorder()
	router.ServeHTTP(delJobRec, delJobReq)
	if delJobRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on DELETE /api/v1/plugins/jobs/{id}, got %d", delJobRec.Code)
	}
}

func TestAPI_RealYotoAndNotebookLMPlugins_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not found on PATH; skipping real Yoto and NotebookLM plugin E2E test")
	}

	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	sdkCli := filepath.Join(repoRoot, "packages", "plugin-sdk", "cli", "shelfd-plugin.mjs")
	pluginsDir := t.TempDir()

	for _, pname := range []string{"yoto", "notebooklm"} {
		srcDir := filepath.Join(repoRoot, "plugins", pname)
		distFile := filepath.Join(srcDir, "dist", "index.mjs")
		if _, err := os.Stat(distFile); err != nil {
			cmd := exec.Command("node", sdkCli, "build", srcDir)
			if out, bErr := cmd.CombinedOutput(); bErr != nil {
				t.Fatalf("building %s plugin failed: %v\n%s", pname, bErr, string(out))
			}
		}
		installedDir := filepath.Join(pluginsDir, pname)
		if err := os.MkdirAll(filepath.Join(installedDir, "dist"), 0755); err != nil {
			t.Fatalf("mkdir installed %s: %v", pname, err)
		}
		mfBytes, err := os.ReadFile(filepath.Join(srcDir, "plugin.json"))
		if err != nil {
			t.Fatalf("read %s plugin.json: %v", pname, err)
		}
		distBytes, err := os.ReadFile(distFile)
		if err != nil {
			t.Fatalf("read %s dist/index.mjs: %v", pname, err)
		}
		_ = os.WriteFile(filepath.Join(installedDir, "plugin.json"), mfBytes, 0644)
		_ = os.WriteFile(filepath.Join(installedDir, "dist", "index.mjs"), distBytes, 0644)
	}

	f := setupAPITest(t)
	defer f.db.Close()
	defer f.repo.Close()

	ctx := context.Background()
	ebook := seedNotebookLMTestBook(t, f)

	audioRelPath := "Beatrix Potter/The Tale of Peter Rabbit/peter.mp3"
	audioFullPath := filepath.Join(f.libDir, audioRelPath)
	_ = os.MkdirAll(filepath.Dir(audioFullPath), 0755)
	_ = os.WriteFile(audioFullPath, []byte("ID3-real-yoto-plugin-e2e-audio-stream-payload"), 0644)

	dur := 95.0
	fileSize := int64(45)
	audioBook := &repository.Book{
		ID:              "book-yoto-e2e-1",
		Title:           "The Tale of Peter Rabbit",
		BookType:        "audiobook",
		FilePath:        audioRelPath,
		FileSizeBytes:   &fileSize,
		DurationSeconds: &dur,
	}
	if err := f.repo.CreateBook(ctx, audioBook); err != nil {
		t.Fatalf("CreateBook audiobook: %v", err)
	}
	chTitle := "Chapter 1: Mr. McGregor's Garden"
	if err := f.repo.CreateAudioChapters(ctx, []*repository.AudioChapter{
		{
			ID:             "ach-yoto-1",
			BookID:         audioBook.ID,
			ChapterIndex:   1,
			Title:          chTitle,
			StartOffsetSec: 0,
			DurationSec:    95,
		},
	}); err != nil {
		t.Fatalf("CreateAudioChapters: %v", err)
	}

	const validYotoIconID = "AC6ySygbay79fIMHmMtYN9ekBo1kwMvRoi4DQW_CZW0"
	var s3BytesReceived int
	var savedCardTitle string
	var webhookReceivedMarkdown string

	var mockUpstream *httptest.Server
	mockUpstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "yoto-e2e-access-tok",
				"refresh_token": "yoto-e2e-refresh-tok",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/content/mine":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"cards": []map[string]any{
					{
						"cardId": "card-e2e-1",
						"title":  "Kids Bedtime Card",
						"content": map[string]any{
							"chapters": []map[string]any{{"key": "01", "title": "Ch 1"}},
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/media/displayIcons/user/yoto":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"displayIcons": []map[string]any{
					{
						"displayIconId": "ic-1",
						"mediaId":       validYotoIconID,
						"title":         "Peter Rabbit Icon",
						"public":        true,
						"publicTags":    []string{"rabbit", "book"},
						"url":           mockUpstream.URL + "/icons/" + validYotoIconID + ".png",
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/media/transcode/audio/uploadUrl":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upload": map[string]any{
					"uploadUrl": mockUpstream.URL + "/s3-audio-put",
					"uploadId":  "up-e2e-1",
				},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/s3-audio-put":
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r.Body)
			s3BytesReceived += buf.Len()
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/media/upload/up-e2e-1/transcoded":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transcode": map[string]any{
					"transcodedSha256": "sha256-e2e-peter",
					"transcodedInfo": map[string]any{
						"duration": 95.0,
						"fileSize": 45,
						"channels": "stereo",
						"format":   "aac",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/content":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			savedCardTitle, _ = body["title"].(string)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"card": map[string]any{
					"cardId": "card-created-e2e",
					"title":  savedCardTitle,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/notebooklm-webhook":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			webhookReceivedMarkdown, _ = body["content"].(string)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"notebook_url": "https://notebooklm.google.com/notebook/nb-e2e-1",
				"message":      "NotebookLM webhook ok",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockUpstream.Close()

	var shelfdServer *httptest.Server
	var activeHandler http.Handler
	shelfdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeHandler.ServeHTTP(w, r)
	}))
	defer shelfdServer.Close()

	eventCh, unsub := f.hub.Subscribe()
	defer unsub()

	toolExec := mcp.NewToolExecutor(f.repo, f.aiClient, slog.Default())
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

	pluginHost.Start(ctx)
	defer pluginHost.Stop()

	userToken := f.loginAndGetToken(t)

	nlmHandoffReq, _ := http.NewRequest(
		http.MethodPost,
		shelfdServer.URL+"/api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke",
		strings.NewReader(`{"target":{"type":"book","id":"`+ebook.ID+`","title":"`+ebook.Title+`"}}`),
	)
	nlmHandoffReq.Header.Set("Authorization", "Bearer "+userToken)
	nlmHandoffResp, err := http.DefaultClient.Do(nlmHandoffReq)
	if err != nil || nlmHandoffResp.StatusCode != http.StatusOK {
		t.Fatalf("NotebookLM zero-config invoke failed: status=%v err=%v", nlmHandoffResp.StatusCode, err)
	}
	var handoffRes plugins.ActionResult
	_ = json.NewDecoder(nlmHandoffResp.Body).Decode(&handoffRes)
	nlmHandoffResp.Body.Close()
	if handoffRes.Status != "client_handoff" || handoffRes.DownloadAPIPath != "/api/v1/books/"+ebook.ID+"/markdown" {
		t.Fatalf("unexpected NotebookLM handoff result: %+v", handoffRes)
	}

	nlmCfgReq, _ := http.NewRequest(
		http.MethodPut,
		shelfdServer.URL+"/api/v1/plugins/notebooklm/config",
		strings.NewReader(`{"webhook_url":"`+mockUpstream.URL+`/notebooklm-webhook","allow_local_endpoints":true}`),
	)
	nlmCfgReq.Header.Set("Authorization", "Bearer "+userToken)
	nlmCfgResp, err := http.DefaultClient.Do(nlmCfgReq)
	if err != nil || nlmCfgResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT notebooklm config failed: %v", err)
	}
	nlmCfgResp.Body.Close()

	nlmPushReq, _ := http.NewRequest(
		http.MethodPost,
		shelfdServer.URL+"/api/v1/plugins/notebooklm/actions/send_to_notebooklm/invoke",
		strings.NewReader(`{"target":{"type":"book","id":"`+ebook.ID+`"}}`),
	)
	nlmPushReq.Header.Set("Authorization", "Bearer "+userToken)
	nlmPushResp, err := http.DefaultClient.Do(nlmPushReq)
	if err != nil || nlmPushResp.StatusCode != http.StatusOK {
		t.Fatalf("NotebookLM webhook invoke failed: %v", err)
	}
	var pushRes plugins.ActionResult
	_ = json.NewDecoder(nlmPushResp.Body).Decode(&pushRes)
	nlmPushResp.Body.Close()
	if pushRes.Status != "completed" || !strings.Contains(webhookReceivedMarkdown, ebook.Title) {
		t.Fatalf("unexpected NotebookLM webhook result: res=%+v mdLen=%d", pushRes, len(webhookReceivedMarkdown))
	}

	yotoCfgReq, _ := http.NewRequest(
		http.MethodPut,
		shelfdServer.URL+"/api/v1/plugins/yoto/config",
		strings.NewReader(`{"api_base_url":"`+mockUpstream.URL+`","auth_base_url":"`+mockUpstream.URL+`","client_id":"yoto-e2e-client"}`),
	)
	yotoCfgReq.Header.Set("Authorization", "Bearer "+userToken)
	yotoCfgResp, err := http.DefaultClient.Do(yotoCfgReq)
	if err != nil || yotoCfgResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT yoto config failed: %v", err)
	}
	yotoCfgResp.Body.Close()

	yotoAuthReq, _ := http.NewRequest(
		http.MethodPost,
		shelfdServer.URL+"/api/v1/plugins/yoto/auth/complete",
		strings.NewReader(`{"method":"oauth_pkce","code":"code-e2e-1","code_verifier":"ver-e2e-1"}`),
	)
	yotoAuthReq.Header.Set("Authorization", "Bearer "+userToken)
	yotoAuthResp, err := http.DefaultClient.Do(yotoAuthReq)
	if err != nil || yotoAuthResp.StatusCode != http.StatusOK {
		t.Fatalf("POST yoto auth/complete failed: %v", err)
	}
	var authStatus plugins.AuthStatusResult
	_ = json.NewDecoder(yotoAuthResp.Body).Decode(&authStatus)
	yotoAuthResp.Body.Close()
	if !authStatus.Linked {
		t.Fatalf("expected Yoto authStatus.Linked=true after auth/complete, got %+v", authStatus)
	}

	yotoPrepReq, _ := http.NewRequest(
		http.MethodPost,
		shelfdServer.URL+"/api/v1/plugins/yoto/actions/send_to_yoto/prepare",
		strings.NewReader(`{"target":{"type":"book","id":"`+audioBook.ID+`"}}`),
	)
	yotoPrepReq.Header.Set("Authorization", "Bearer "+userToken)
	yotoPrepResp, err := http.DefaultClient.Do(yotoPrepReq)
	if err != nil || yotoPrepResp.StatusCode != http.StatusOK {
		t.Fatalf("POST yoto actions/prepare failed: %v", err)
	}
	var formSpec plugins.ActionFormSpec
	_ = json.NewDecoder(yotoPrepResp.Body).Decode(&formSpec)
	yotoPrepResp.Body.Close()
	if formSpec.AuthRequired || len(formSpec.Fields) < 5 || len(formSpec.Entries) != 1 {
		t.Fatalf("unexpected Yoto ActionFormSpec: %+v", formSpec)
	}

	yotoInvReq, _ := http.NewRequest(
		http.MethodPost,
		shelfdServer.URL+"/api/v1/plugins/yoto/actions/send_to_yoto/invoke",
		strings.NewReader(`{"target":{"type":"book","id":"`+audioBook.ID+`","title":"The Tale of Peter Rabbit"},"values":{"playlist_mode":"create","playlist_title":"Peter Rabbit E2E Card"}}`),
	)
	yotoInvReq.Header.Set("Authorization", "Bearer "+userToken)
	yotoInvResp, err := http.DefaultClient.Do(yotoInvReq)
	if err != nil || yotoInvResp.StatusCode != http.StatusOK {
		t.Fatalf("POST yoto actions/invoke failed: %v", err)
	}
	var yotoJobStart plugins.ActionResult
	_ = json.NewDecoder(yotoInvResp.Body).Decode(&yotoJobStart)
	yotoInvResp.Body.Close()
	if yotoJobStart.Status != "job_started" || yotoJobStart.JobID == "" {
		t.Fatalf("expected job_started from yoto invoke, got %+v", yotoJobStart)
	}

	jobDone := false
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && !jobDone {
		select {
		case ev := <-eventCh:
			if m, ok := ev.Data.(map[string]any); ok && m["kind"] == "job_progress" && m["job_id"] == yotoJobStart.JobID {
				if jb, ok := m["job"].(*plugins.PluginJob); ok {
					if jb.Status == "failed" {
						t.Fatalf("Yoto plugin job failed: %s", jb.Error)
					}
					if jb.Status == "completed" {
						jobDone = true
					}
				}
			}
		case <-time.After(50 * time.Millisecond):
		}
	}
	if !jobDone {
		t.Fatalf("timed out waiting for Yoto plugin job %s to complete", yotoJobStart.JobID)
	}
	if s3BytesReceived != 45 || savedCardTitle != "Peter Rabbit E2E Card" {
		t.Fatalf("expected 45 audio bytes streamed and card title 'Peter Rabbit E2E Card', got bytes=%d title=%q", s3BytesReceived, savedCardTitle)
	}

	mcpCtx := context.WithValue(ctx, mcp.TokenContextKey, &repository.APIToken{UserID: f.user.ID})
	yotoMcpRes, err := toolExec.ExecuteInProcess(mcpCtx, "yoto_list_playlists", map[string]any{})
	if err != nil || yotoMcpRes.IsError || !strings.Contains(yotoMcpRes.Content, "Kids Bedtime Card") {
		t.Fatalf("MCP tool yoto_list_playlists failed: err=%v res=%+v", err, yotoMcpRes)
	}
	nlmMcpRes, err := toolExec.ExecuteInProcess(mcpCtx, "notebooklm_send_book_to_notebooklm", map[string]any{"book_id": ebook.ID})
	if err != nil || nlmMcpRes.IsError || !strings.Contains(nlmMcpRes.Content, "NotebookLM webhook ok") {
		t.Fatalf("MCP tool notebooklm_send_book_to_notebooklm failed: err=%v res=%+v", err, nlmMcpRes)
	}
}


