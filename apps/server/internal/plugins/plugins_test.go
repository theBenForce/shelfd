package plugins_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/plugins"
	"github.com/shelfd/shelfd/internal/repository"
)

func init() {
	sqlite_vec.Auto()
}

func setupTestRepo(t *testing.T) repository.StorageEngine {
	t.Helper()
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := repository.NewSQLiteStorageEngine(db)
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestCrypto_EncryptRedactPreserveAndSmartPaste(t *testing.T) {
	masterSecret := "unit-test-jwt-master-secret-key-2026"
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_cookie": map[string]any{
				"type":                 "string",
				"x-shelfd-secret":      true,
				"x-shelfd-curl-paste":  true,
				"x-shelfd-cookie-name": "_simpleauth_sess",
			},
			"access_token": map[string]any{
				"type":                "string",
				"format":              "password",
				"x-shelfd-curl-paste": true,
			},
			"preferred_format": map[string]any{
				"type": "string",
			},
		},
	}

	// 1. Pasting full DevTools cURL command extracts only target cookie and bearer token
	humbleCurl := "curl 'https://www.humblebundle.com/api/v1/user/order' \\\n  -H 'accept: application/json' \\\n  -H 'cookie: csrf_cookie=abc; _simpleauth_sess=\"eyJ1c2VyIjoiYmVuIn0=\"; other=1'"
	packtCurl := `curl 'https://services.packtpub.com/entitlements-v1/users/me/products' -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.packt.sig'`

	stored, decrypted, err := plugins.PrepareConfigForStorage(masterSecret, "store-test", schema, map[string]any{
		"session_cookie":   humbleCurl,
		"access_token":     packtCurl,
		"preferred_format": "epub",
	}, nil)
	if err != nil {
		t.Fatalf("PrepareConfigForStorage failed: %v", err)
	}

	if decrypted["session_cookie"] != `"eyJ1c2VyIjoiYmVuIn0="` {
		t.Fatalf("unexpected extracted cookie: %v", decrypted["session_cookie"])
	}
	if decrypted["access_token"] != "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.packt.sig" {
		t.Fatalf("unexpected extracted bearer token: %v", decrypted["access_token"])
	}

	// Stored values for secret fields must be encrypted with AES-256-GCM prefix
	encCookie, _ := stored["session_cookie"].(string)
	if !strings.HasPrefix(encCookie, "enc:v1:") {
		t.Fatalf("expected encrypted cookie with enc:v1: prefix, got %q", encCookie)
	}
	if stored["preferred_format"] != "epub" {
		t.Fatalf("non-secret field should stay plaintext, got %v", stored["preferred_format"])
	}

	// 2. Redaction masks secret fields with sentinel
	redacted, setFields := plugins.RedactStoredConfig(schema, stored)
	if redacted["session_cookie"] != plugins.SecretSentinel || redacted["access_token"] != plugins.SecretSentinel {
		t.Fatalf("expected secret fields redacted, got %+v", redacted)
	}
	if len(setFields) != 2 {
		t.Fatalf("expected 2 secret_fields_set entries, got %v", setFields)
	}

	// 3. Updating non-secret field while submitting SecretSentinel preserves existing ciphertext
	stored2, decrypted2, err := plugins.PrepareConfigForStorage(masterSecret, "store-test", schema, map[string]any{
		"session_cookie":   plugins.SecretSentinel,
		"access_token":     plugins.SecretSentinel,
		"preferred_format": "pdf",
	}, stored)
	if err != nil {
		t.Fatalf("PrepareConfigForStorage preserve failed: %v", err)
	}
	if stored2["session_cookie"] != encCookie {
		t.Fatalf("expected unchanged secret ciphertext to be preserved")
	}
	if decrypted2["session_cookie"] != `"eyJ1c2VyIjoiYmVuIn0="` || decrypted2["preferred_format"] != "pdf" {
		t.Fatalf("unexpected decrypted values after sentinel preserve: %+v", decrypted2)
	}

	// 4. Different plugin name derives an isolated HKDF key that cannot decrypt store-test ciphertext
	otherKey, _ := plugins.DerivePluginKey(masterSecret, "other-plugin")
	if _, err := plugins.DecryptValue(otherKey, encCookie); err == nil {
		t.Fatalf("expected cross-plugin key decryption to fail")
	}
}

func buildZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestManifestAndExtractPluginZip_ZipSlipProtection(t *testing.T) {
	pluginsDir := t.TempDir()

	// 1. Reject Zip-Slip path traversal
	maliciousZip := buildZipBytes(t, map[string]string{
		"plugin.json":       `{"name":"bad-zip","version":"1.0.0","entrypoint":{"command":"node","args":["index.mjs"]}}`,
		"../../escaped.txt": "pwned",
	})
	if _, err := plugins.ExtractPluginZip(maliciousZip, pluginsDir); err == nil {
		t.Fatalf("expected Zip-Slip archive to be rejected")
	}

	// 2. Reject unsupported API scope
	badScopeZip := buildZipBytes(t, map[string]string{
		"plugin.json": `{"name":"bad-scope","version":"1.0.0","entrypoint":{"command":"node","args":["index.mjs"]},"api_scopes":["admin:root"]}`,
	})
	if _, err := plugins.ExtractPluginZip(badScopeZip, pluginsDir); err == nil {
		t.Fatalf("expected unsupported api_scope to be rejected")
	}

	// 3. Accept nested top-level folder archive and extract cleanly into pluginsDir/<name>
	validZip := buildZipBytes(t, map[string]string{
		"demo-plugin-v1/plugin.json":    `{"name":"demo-plugin","version":"1.2.3","entrypoint":{"command":"node","args":["dist/index.mjs"]},"capabilities":["catalog"],"api_scopes":["uploads:stage"]}`,
		"demo-plugin-v1/dist/index.mjs": "console.error('ready');",
	})
	m, err := plugins.ExtractPluginZip(validZip, pluginsDir)
	if err != nil {
		t.Fatalf("ExtractPluginZip failed on valid archive: %v", err)
	}
	if m.Name != "demo-plugin" || m.Version != "1.2.3" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "demo-plugin", "dist", "index.mjs")); err != nil {
		t.Fatalf("expected extracted dist/index.mjs: %v", err)
	}
}

const mockNodePluginScript = `
import readline from 'node:readline';

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let spawnPid = process.pid;

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n');
}

rl.on('line', (line) => {
  if (!line.trim()) return;
  const msg = JSON.parse(line);
  if (!msg.method) return;

  if (msg.method === 'initialize') {
    console.error('mock-plugin initialized pid=' + spawnPid);
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        name: 'mock-store',
        version: '0.1.0',
        tools: [
          {
            name: 'check_env',
            description: 'Checks environment isolation and config',
            input_schema: { type: 'object', properties: {} }
          }
        ],
        catalog: {
          modes: ['browse', 'search'],
          supported_formats: ['epub']
        }
      }
    });
    return;
  }

  if (msg.method === 'shutdown') {
    send({ jsonrpc: '2.0', id: msg.id, result: { ok: true } });
    setTimeout(() => process.exit(0), 10);
    return;
  }

  if (msg.method === 'tools/call') {
    const leakedSecret = process.env.SHELFD_JWT_SECRET || '';
    // Also exercise plugin -> host config/patch rotation
    if (msg.params?.arguments?.rotate_token && msg.params?.auth?.user_id) {
      send({
        jsonrpc: '2.0',
        id: 'patch-1',
        method: 'config/patch',
        params: {
          user_id: msg.params.auth.user_id,
          values: { api_secret: 'rotated-secret-999' }
        }
      });
    }
    send({
      jsonrpc: '2.0',
      method: 'event',
      params: { event: 'tool_ran', pid: spawnPid }
    });
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        content: JSON.stringify({
          pid: spawnPid,
          leaked_jwt_secret: leakedSecret,
          no_proxy: process.env.NO_PROXY || '',
          data_dir: process.env.SHELFD_DATA_DIR || '',
          user_id: msg.params?.auth?.user_id || '',
          inv_token: msg.params?.auth?.invocation_token || '',
          api_secret: msg.params?.auth?.config?.api_secret || ''
        })
      }
    });
    return;
  }

  if (msg.method === 'catalog/search') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        items: [
          { external_id: 'book-1', title: 'Available Book', authors: ['Ada Lovelace'], formats: [{ format: 'epub' }] },
          { external_id: 'book-2', title: 'Staged Book', authors: ['Alan Turing'], formats: [{ format: 'epub' }] },
          { external_id: 'book-3', title: 'Library Book', authors: ['Grace Hopper'], formats: [{ format: 'epub' }] }
        ]
      }
    });
    return;
  }
});
`

func TestHost_LifecycleScaleToZeroEnvIsolationConfigPatchAndProvenance(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	user := &repository.User{Username: "reader", PasswordHash: "x"}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Set sensitive env var in parent process to verify child process does NOT inherit it
	t.Setenv("SHELFD_JWT_SECRET", "DO_NOT_LEAK_SERVER_MASTER_SECRET")

	pluginsDir := t.TempDir()
	pDir := filepath.Join(pluginsDir, "mock-store")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	manifestJSON := `{
  "name": "mock-store",
  "version": "0.1.0",
  "display_name": "Mock Store",
  "entrypoint": { "command": "node", "args": ["index.mjs"] },
  "daemon": false,
  "capabilities": ["catalog", "tools", "events"],
  "api_scopes": ["uploads:stage"],
  "config_schema": {
    "type": "object",
    "required": ["api_secret"],
    "properties": {
      "api_secret": { "type": "string", "x-shelfd-secret": true }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "index.mjs"), []byte(mockNodePluginScript), 0644); err != nil {
		t.Fatalf("write index.mjs: %v", err)
	}

	hub := events.NewHub()
	eventCh, unsub := hub.Subscribe()
	defer unsub()

	host := plugins.NewHost(repo, plugins.HostConfig{
		PluginsDir:    pluginsDir,
		APIBase:       "http://127.0.0.1:8080",
		ShelfdVersion: "0.1.0-test",
		MasterSecret:  "host-test-master-secret",
		IdleTimeout:   600 * time.Millisecond,
		TokenIssuer: func(userID, username, pluginName string, scopes []string) (string, error) {
			return "scoped-jwt:" + userID + ":" + pluginName + ":" + strings.Join(scopes, ","), nil
		},
		Hub: hub,
	})
	host.Start(ctx)
	defer host.Stop()

	// 1. Verify initial config status = false until required secret is configured
	sumBefore, err := host.GetPluginSummary(ctx, "mock-store", user.ID)
	if err != nil {
		t.Fatalf("GetPluginSummary: %v", err)
	}
	if sumBefore.Configured {
		t.Fatalf("expected Configured=false before setting required api_secret")
	}

	if _, _, err := host.UpdateUserConfig(ctx, user.ID, "mock-store", map[string]any{
		"api_secret": "initial-secret-111",
	}); err != nil {
		t.Fatalf("UpdateUserConfig: %v", err)
	}

	// 2. Call MCP tool -> check env isolation, invocation token, decrypted secret, SSE event, and config/patch
	toolRes, found, err := host.CallMCPTool(ctx, "mock-store_check_env", map[string]any{"rotate_token": true}, user.ID, user.Username)
	if err != nil || !found {
		t.Fatalf("CallMCPTool failed: found=%v err=%v", found, err)
	}

	var payload struct {
		PID             int    `json:"pid"`
		LeakedJWTSecret string `json:"leaked_jwt_secret"`
		NoProxy         string `json:"no_proxy"`
		DataDir         string `json:"data_dir"`
		UserID          string `json:"user_id"`
		InvToken        string `json:"inv_token"`
		APISecret       string `json:"api_secret"`
	}
	if err := json.Unmarshal([]byte(toolRes.Content), &payload); err != nil {
		t.Fatalf("decode tool response %q: %v", toolRes.Content, err)
	}
	if payload.LeakedJWTSecret != "" {
		t.Fatalf("SECURITY VIOLATION: child process inherited SHELFD_JWT_SECRET=%q", payload.LeakedJWTSecret)
	}
	if !strings.Contains(payload.NoProxy, "127.0.0.1") {
		t.Fatalf("expected NO_PROXY to include 127.0.0.1, got %q", payload.NoProxy)
	}
	if payload.APISecret != "initial-secret-111" {
		t.Fatalf("expected initial-secret-111 in auth.config, got %q", payload.APISecret)
	}
	if payload.InvToken != "scoped-jwt:"+user.ID+":mock-store:uploads:stage" {
		t.Fatalf("unexpected invocation token: %q", payload.InvToken)
	}
	firstPID := payload.PID

	// Verify SSE event arrived on hub
	select {
	case ev := <-eventCh:
		if ev.Type != events.EventPlugin {
			t.Fatalf("expected EventPlugin, got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for plugin SSE event")
	}

	// Wait briefly for async config/patch to persist
	time.Sleep(100 * time.Millisecond)

	// 3. Wait for scale-to-zero idle reaper to stop non-daemon process
	deadline := time.Now().Add(3 * time.Second)
	reaped := false
	for time.Now().Before(deadline) {
		s, _ := host.GetPluginSummary(ctx, "mock-store", user.ID)
		if s != nil && s.Status == plugins.StatusIdle {
			reaped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !reaped {
		t.Fatalf("expected non-daemon plugin to scale to zero (StatusIdle) after idle timeout")
	}

	// Tools must STILL be advertised while scaled to zero!
	mcpTools := host.AvailableMCPTools()
	if len(mcpTools) != 1 || mcpTools[0].Name != "mock-store_check_env" {
		t.Fatalf("expected cached MCP tool while scaled to zero, got %+v", mcpTools)
	}

	// 4. Subsequent call transparently respawns a new PID AND receives rotated secret from config/patch!
	toolRes2, _, err := host.CallMCPTool(ctx, "mock-store_check_env", map[string]any{}, user.ID, user.Username)
	if err != nil {
		t.Fatalf("second CallMCPTool after reap failed: %v", err)
	}
	var payload2 struct {
		PID       int    `json:"pid"`
		APISecret string `json:"api_secret"`
	}
	if err := json.Unmarshal([]byte(toolRes2.Content), &payload2); err != nil {
		t.Fatalf("decode second tool response: %v", err)
	}
	if payload2.PID == firstPID {
		t.Fatalf("expected respawned process to have new PID, got same PID %d", payload2.PID)
	}
	if payload2.APISecret != "rotated-secret-999" {
		t.Fatalf("expected rotated-secret-999 persisted via config/patch, got %q", payload2.APISecret)
	}

	// 5. Verify catalog provenance enrichment (available vs staged vs in_library)
	stagedJob := &repository.UploadJob{
		ID:         "job-staged-2",
		Filename:   "staged-book.epub",
		StagedPath: "/tmp/staged-book.epub",
		Status:     "staged",
	}
	if err := repo.CreateUploadJob(ctx, stagedJob); err != nil {
		t.Fatalf("CreateUploadJob: %v", err)
	}
	if err := repo.UpsertBookSource(ctx, &repository.BookSource{
		PluginName:  "mock-store",
		ExternalID:  "book-2",
		UploadJobID: &stagedJob.ID,
	}); err != nil {
		t.Fatalf("UpsertBookSource staged: %v", err)
	}

	libBook := &repository.Book{
		ID:       "book-lib-3",
		Title:    "Library Book",
		FilePath: "Grace Hopper/Library Book/book.epub",
	}
	if err := repo.CreateBook(ctx, libBook); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if err := repo.UpsertBookSource(ctx, &repository.BookSource{
		PluginName: "mock-store",
		ExternalID: "book-3",
		BookID:     &libBook.ID,
	}); err != nil {
		t.Fatalf("UpsertBookSource library: %v", err)
	}

	page, err := host.SearchCatalog(ctx, "mock-store", "computing", "", nil, user.ID, user.Username)
	if err != nil {
		t.Fatalf("SearchCatalog: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(page.Items))
	}
	if page.Items[0].Status != "available" {
		t.Errorf("book-1 expected available, got %s", page.Items[0].Status)
	}
	if page.Items[1].Status != "staged" || page.Items[1].UploadJobID == nil || *page.Items[1].UploadJobID != stagedJob.ID {
		t.Errorf("book-2 expected staged with job ID, got %+v", page.Items[1])
	}
	if page.Items[2].Status != "in_library" || page.Items[2].BookID == nil || *page.Items[2].BookID != libBook.ID {
		t.Errorf("book-3 expected in_library with book ID, got %+v", page.Items[2])
	}
}
