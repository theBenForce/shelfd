package plugins_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const mockActionPluginScript = `
import readline from 'node:readline';

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let pendingHost = new Map();
let seq = 0;

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n');
}

function callHost(method, params) {
  seq += 1;
  const id = 'p-call-' + seq;
  return new Promise((resolve, reject) => {
    pendingHost.set(id, { resolve, reject });
    send({ jsonrpc: '2.0', id, method, params });
  });
}

rl.on('line', async (line) => {
  if (!line.trim()) return;
  const msg = JSON.parse(line);
  if (msg.id !== undefined && !msg.method) {
    const waiter = pendingHost.get(String(msg.id));
    if (waiter) {
      pendingHost.delete(String(msg.id));
      if (msg.error) waiter.reject(new Error(msg.error.message));
      else waiter.resolve(msg.result);
    }
    return;
  }

  if (msg.method === 'initialize') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        name: 'mock-actions',
        version: '0.2.0',
        auth: {
          methods: ['oauth_pkce', 'token'],
          title: 'Link Mock Account'
        },
        actions: [
          {
            name: 'sync_act',
            title: 'Sync Action',
            targets: ['book'],
            media_types: ['ebook'],
            mode: 'form',
            execution: 'sync'
          },
          {
            name: 'bg_job',
            title: 'Background Transfer',
            targets: ['book', 'series'],
            media_types: ['audiobook'],
            mode: 'form',
            execution: 'job'
          },
          {
            name: 'stall_job',
            title: 'Stalling Transfer',
            targets: ['book'],
            mode: 'immediate',
            execution: 'job'
          }
        ]
      }
    });
    return;
  }

  if (msg.method === 'shutdown') {
    send({ jsonrpc: '2.0', id: msg.id, result: { ok: true } });
    setTimeout(() => process.exit(0), 10);
    return;
  }

  if (msg.method === 'auth/status') {
    const tok = msg.params?.auth?.config?.access_token || '';
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        linked: Boolean(tok),
        account_label: tok ? 'reader@example.com' : '',
        methods: ['oauth_pkce', 'token']
      }
    });
    return;
  }

  if (msg.method === 'auth/begin') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        authorize_url: 'https://login.example.com/oauth?state=' + (msg.params?.state || '') + '&challenge=' + (msg.params?.code_challenge || ''),
        state: msg.params?.state || ''
      }
    });
    return;
  }

  if (msg.method === 'auth/complete') {
    await callHost('config/patch', {
      user_id: msg.params.auth.user_id,
      values: { access_token: 'linked-token-' + (msg.params.code || 'direct') }
    });
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        linked: true,
        account_label: 'reader@example.com'
      }
    });
    return;
  }

  if (msg.method === 'auth/disconnect') {
    await callHost('config/patch', {
      user_id: msg.params.auth.user_id,
      values: { access_token: '' }
    });
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: { linked: false }
    });
    return;
  }

  if (msg.method === 'actions/prepare') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        title: 'Prepare ' + (msg.params?.target?.title || msg.params?.target?.id || ''),
        submit_label: 'Send Now',
        entries: [{ id: 'ch-1', index: 1, title: 'Chapter 1' }],
        fields: [
          {
            key: 'mode',
            kind: 'segmented',
            title: 'Mode',
            options: [{ value: 'create', label: 'Create' }, { value: 'append', label: 'Append' }]
          },
          {
            key: 'card_id',
            kind: 'remote_select',
            title: 'Target Card',
            options_source: 'cards',
            visible_when: { field: 'mode', equals: 'append' }
          }
        ]
      }
    });
    return;
  }

  if (msg.method === 'actions/options') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        items: [
          { value: 'card-1', label: 'Bedtime Stories (' + (msg.params?.query || 'all') + ')' }
        ]
      }
    });
    return;
  }

  if (msg.method === 'actions/upload_asset') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        item: {
          value: 'yoto://#icon-123',
          label: msg.params?.filename || 'icon.png',
          icon_uri: 'yoto://#icon-123'
        }
      }
    });
    return;
  }

  if (msg.method === 'actions/invoke') {
    const act = msg.params?.action;
    if (act === 'sync_act') {
      const refreshed = await callHost('host/refresh_token', { user_id: msg.params.auth.user_id });
      send({
        jsonrpc: '2.0',
        id: msg.id,
        result: {
          status: 'client_handoff',
          download_api_path: '/api/v1/books/' + msg.params.target.id + '/markdown',
          open_url: 'https://notebooklm.google.com/',
          data: { refreshed_token: refreshed.invocation_token }
        }
      });
      return;
    }

    if (act === 'bg_job') {
      for (let step = 1; step <= 3; step++) {
        await new Promise((r) => setTimeout(r, 220));
        send({
          jsonrpc: '2.0',
          method: 'progress',
          params: {
            job_id: msg.params.job_id,
            current: step,
            total: 3,
            percent: Math.round((step / 3) * 100),
            title: 'Chapter ' + step,
            message: 'Uploading chapter ' + step
          }
        });
      }
      const refreshed = await callHost('host/refresh_token', { user_id: msg.params.auth.user_id });
      send({
        jsonrpc: '2.0',
        id: msg.id,
        result: {
          status: 'completed',
          message: 'Uploaded 3 chapters',
          data: { refreshed_token: refreshed.invocation_token }
        }
      });
      return;
    }

    if (act === 'stall_job') {
      return;
    }
  }
});
`

func TestHost_ActionsManifestAuthRPCsJobLeaseWatchdogAndTokenRefresh(t *testing.T) {
	validManifest, err := plugins.ParseManifest([]byte(`{
		"name": "mock-actions",
		"version": "0.2.0",
		"entrypoint": {"command": "node", "args": ["index.mjs"]},
		"capabilities": ["actions", "events"],
		"api_scopes": ["books:read"]
	}`))
	if err != nil {
		t.Fatalf("ParseManifest rejected valid 'actions' capability: %v", err)
	}
	if !validManifest.HasCapability("actions") {
		t.Fatalf("expected HasCapability('actions') == true")
	}

	repo := setupTestRepo(t)
	ctx := context.Background()

	user := &repository.User{Username: "action-reader", PasswordHash: "x"}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	pluginsDir := t.TempDir()
	pDir := filepath.Join(pluginsDir, "mock-actions")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	manifestJSON := `{
  "name": "mock-actions",
  "version": "0.2.0",
  "display_name": "Mock Actions",
  "entrypoint": { "command": "node", "args": ["index.mjs"] },
  "daemon": false,
  "capabilities": ["actions", "events"],
  "api_scopes": ["books:read"],
  "config_schema": {
    "type": "object",
    "properties": {
      "access_token": { "type": "string", "x-shelfd-secret": true }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "index.mjs"), []byte(mockActionPluginScript), 0644); err != nil {
		t.Fatalf("write index.mjs: %v", err)
	}

	hub := events.NewHub()
	eventCh, unsub := hub.Subscribe()
	defer unsub()

	var tokenCounter int
	host := plugins.NewHost(repo, plugins.HostConfig{
		PluginsDir:     pluginsDir,
		APIBase:        "http://127.0.0.1:8080",
		ShelfdVersion:  "0.1.0-test",
		MasterSecret:   "actions-master-secret-key",
		IdleTimeout:    350 * time.Millisecond,
		JobIdleTimeout: 5 * time.Second,
		TokenIssuer: func(userID, username, pluginName string, scopes []string) (string, error) {
			tokenCounter++
			return "tok-" + pluginName + "-" + string(rune('0'+tokenCounter)), nil
		},
		Hub: hub,
	})
	host.Start(ctx)
	defer host.Stop()

	sum, err := host.GetPluginSummary(ctx, "mock-actions", user.ID)
	if err != nil {
		t.Fatalf("GetPluginSummary: %v", err)
	}
	if sum.Auth == nil || len(sum.Auth.Methods) != 2 {
		t.Fatalf("expected cached AuthSpec with 2 methods, got %+v", sum.Auth)
	}
	if len(sum.Actions) != 3 {
		t.Fatalf("expected 3 cached ActionDefs, got %+v", sum.Actions)
	}

	beginRes, err := host.BeginAuth(ctx, "mock-actions", plugins.AuthBeginParams{
		Method:        "oauth_pkce",
		State:         "st-123",
		CodeChallenge: "chall-456",
	}, user.ID, user.Username)
	if err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	if !strings.Contains(beginRes.AuthorizeURL, "state=st-123") || !strings.Contains(beginRes.AuthorizeURL, "challenge=chall-456") {
		t.Fatalf("unexpected AuthorizeURL: %s", beginRes.AuthorizeURL)
	}

	compRes, err := host.CompleteAuth(ctx, "mock-actions", plugins.AuthCompleteParams{
		Method: "oauth_pkce",
		Code:   "code-abc",
	}, user.ID, user.Username)
	if err != nil || !compRes.Linked {
		t.Fatalf("CompleteAuth failed: res=%+v err=%v", compRes, err)
	}

	statusRes, err := host.GetAuthStatus(ctx, "mock-actions", user.ID, user.Username)
	if err != nil || !statusRes.Linked || statusRes.AccountLabel != "reader@example.com" {
		t.Fatalf("GetAuthStatus after link failed: res=%+v err=%v", statusRes, err)
	}

	formSpec, err := host.PrepareAction(ctx, "mock-actions", "sync_act", plugins.ActionTarget{
		Type:  "book",
		ID:    "book-dune",
		Title: "Dune",
	}, nil, user.ID, user.Username)
	if err != nil {
		t.Fatalf("PrepareAction: %v", err)
	}
	if formSpec.Title != "Prepare Dune" || len(formSpec.Fields) != 2 || len(formSpec.Entries) != 1 {
		t.Fatalf("unexpected ActionFormSpec: %+v", formSpec)
	}
	if formSpec.Fields[1].VisibleWhen == nil || formSpec.Fields[1].VisibleWhen.Equals != "append" {
		t.Fatalf("expected VisibleWhen on second field, got %+v", formSpec.Fields[1])
	}

	optRes, err := host.FetchActionOptions(ctx, "mock-actions", "sync_act", "cards", "", "bedtime", "", nil, user.ID, user.Username)
	if err != nil || len(optRes.Items) != 1 || !strings.Contains(optRes.Items[0].Label, "bedtime") {
		t.Fatalf("FetchActionOptions failed: res=%+v err=%v", optRes, err)
	}

	assetRes, err := host.UploadActionAsset(ctx, "mock-actions", "sync_act", "icon", "castle.png", "image/png", "aGVsbG8=", user.ID, user.Username)
	if err != nil || assetRes.Item.Value != "yoto://#icon-123" {
		t.Fatalf("UploadActionAsset failed: res=%+v err=%v", assetRes, err)
	}

	syncRes, err := host.InvokeAction(ctx, "mock-actions", "sync_act", plugins.ActionTarget{
		Type: "book",
		ID:   "book-dune",
	}, nil, nil, false, user.ID, user.Username)
	if err != nil || syncRes.Status != "client_handoff" || syncRes.DownloadAPIPath != "/api/v1/books/book-dune/markdown" {
		t.Fatalf("InvokeAction sync failed: res=%+v err=%v", syncRes, err)
	}
	if refreshedTok, _ := syncRes.Data["refreshed_token"].(string); !strings.HasPrefix(refreshedTok, "tok-mock-actions-") {
		t.Fatalf("expected host/refresh_token to return new token, got %v", syncRes.Data)
	}

	jobStartRes, err := host.InvokeAction(ctx, "mock-actions", "bg_job", plugins.ActionTarget{
		Type:  "book",
		ID:    "book-peter-rabbit",
		Title: "Peter Rabbit",
	}, map[string]any{"mode": "create"}, nil, false, user.ID, user.Username)
	if err != nil || jobStartRes.Status != "job_started" || jobStartRes.JobID == "" {
		t.Fatalf("InvokeAction bg_job failed: res=%+v err=%v", jobStartRes, err)
	}
	jobID := jobStartRes.JobID

	time.Sleep(450 * time.Millisecond)
	midSum, _ := host.GetPluginSummary(ctx, "mock-actions", user.ID)
	if midSum == nil || midSum.Status != plugins.StatusRunning {
		t.Fatalf("expected plugin to remain StatusRunning during active background job lease, got %+v", midSum)
	}

	var sawCompletedJobEvent bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		j := host.GetJob(jobID)
		if j != nil && j.Status == "completed" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for {
		select {
		case ev := <-eventCh:
			if ev.Type == events.EventPlugin {
				if m, ok := ev.Data.(map[string]any); ok && m["kind"] == "job_progress" && m["job_id"] == jobID {
					if jb, ok := m["job"].(*plugins.PluginJob); ok && jb.Status == "completed" {
						sawCompletedJobEvent = true
					}
				}
			}
			continue
		default:
		}
		break
	}
	finalJob := host.GetJob(jobID)
	if finalJob == nil || finalJob.Status != "completed" || finalJob.Current != 3 || !sawCompletedJobEvent {
		t.Fatalf("expected completed job with 3 steps and SSE job_progress event, got job=%+v sawEvent=%v", finalJob, sawCompletedJobEvent)
	}

	reapDeadline := time.Now().Add(2500 * time.Millisecond)
	reaped := false
	for time.Now().Before(reapDeadline) {
		s, _ := host.GetPluginSummary(ctx, "mock-actions", user.ID)
		if s != nil && s.Status == plugins.StatusIdle {
			reaped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !reaped {
		t.Fatalf("expected plugin to scale to StatusIdle after background job lease released")
	}

	host.SetJobWatchdogTimeout(250 * time.Millisecond)
	stallStart, err := host.InvokeAction(ctx, "mock-actions", "stall_job", plugins.ActionTarget{
		Type:  "book",
		ID:    "book-stall",
		Title: "Stall Book",
	}, nil, nil, false, user.ID, user.Username)
	if err != nil || stallStart.JobID == "" {
		t.Fatalf("InvokeAction stall_job failed: %v", err)
	}

	stallDeadline := time.Now().Add(2 * time.Second)
	var stalledJob *plugins.PluginJob
	for time.Now().Before(stallDeadline) {
		j := host.GetJob(stallStart.JobID)
		if j != nil && j.Status == "failed" {
			stalledJob = j
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if stalledJob == nil || stalledJob.Status != "failed" || !strings.Contains(stalledJob.Error, "timed out") {
		t.Fatalf("expected stalled job to fail via watchdog timeout, got %+v", stalledJob)
	}

	if !host.DismissJob(user.ID, jobID) {
		t.Fatalf("expected DismissJob to return true")
	}
	if host.GetJob(jobID) != nil {
		t.Fatalf("expected dismissed job to be removed")
	}

	discRes, err := host.DisconnectAuth(ctx, "mock-actions", user.ID, user.Username)
	if err != nil || discRes.Linked {
		t.Fatalf("DisconnectAuth failed: res=%+v err=%v", discRes, err)
	}
}

func TestInstallOrUpdateZip_LifecycleAndSafety(t *testing.T) {
	ctx := context.Background()
	repo := setupTestRepo(t)
	pluginsDir := t.TempDir()

	host := plugins.NewHost(repo, plugins.HostConfig{
		PluginsDir:    pluginsDir,
		APIBase:       "http://127.0.0.1:18080",
		ShelfdVersion: "0.1.0-test",
		MasterSecret:  "test-master-secret",
		IdleTimeout:   30 * time.Second,
		Hub:           events.NewHub(),
	})
	defer host.Stop()

	makePluginZip := func(name, version string) []byte {
		script := `
import readline from 'node:readline';
const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on('line', (line) => {
  if (!line.trim()) return;
  const msg = JSON.parse(line);
  if (msg.method === 'initialize') {
    process.stdout.write(JSON.stringify({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        name: '` + name + `',
        version: '` + version + `',
        commands: [{ name: 'ping', title: 'Ping' }]
      }
    }) + '\n');
  } else if (msg.method === 'commands/invoke') {
    process.stdout.write(JSON.stringify({
      jsonrpc: '2.0',
      id: msg.id,
      result: { version: '` + version + `', pid: process.pid }
    }) + '\n');
  } else if (msg.method === 'shutdown') {
    process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: msg.id, result: { ok: true } }) + '\n');
    setTimeout(() => process.exit(0), 10);
  }
});
`
		return buildZipBytes(t, map[string]string{
			"plugin.json": `{
				"name": "` + name + `",
				"version": "` + version + `",
				"display_name": "Zip Update Demo",
				"entrypoint": { "command": "node", "args": ["index.mjs"] },
				"capabilities": ["commands"]
			}`,
			"index.mjs": script,
		})
	}

	// 1. Fresh install via InstallOrUpdateZip
	res1, err := host.InstallOrUpdateZip(ctx, makePluginZip("zip-demo", "1.0.0"), plugins.InstallZipOptions{})
	if err != nil {
		t.Fatalf("fresh install failed: %v", err)
	}
	if res1.Updated || res1.PreviousVersion != "" {
		t.Fatalf("expected fresh install metadata, got updated=%v prev=%q", res1.Updated, res1.PreviousVersion)
	}
	if res1.Version != "1.0.0" || !res1.Enabled {
		t.Fatalf("expected enabled v1.0.0, got %+v", res1)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "zip-demo", plugins.UserManagedMarkerFile)); err != nil {
		t.Fatalf("expected %s marker file after zip install: %v", plugins.UserManagedMarkerFile, err)
	}

	// Capture initial PID and write a .data scratch file to verify preservation across updates
	raw1, err := host.InvokeCommand(ctx, "zip-demo", "ping", nil, "user-1", "admin")
	if err != nil {
		t.Fatalf("ping v1.0.0 failed: %v", err)
	}
	var ping1 struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	if err := json.Unmarshal(raw1, &ping1); err != nil || ping1.Version != "1.0.0" {
		t.Fatalf("unexpected ping1: %s (err=%v)", string(raw1), err)
	}

	scratchPath := filepath.Join(pluginsDir, "zip-demo", ".data", "state.txt")
	if err := os.MkdirAll(filepath.Dir(scratchPath), 0755); err != nil {
		t.Fatalf("mkdir .data: %v", err)
	}
	if err := os.WriteFile(scratchPath, []byte("keep-me"), 0644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}

	// 2. Reject update when archive name does not match ExpectedName
	_, err = host.InstallOrUpdateZip(ctx, makePluginZip("wrong-plugin", "2.0.0"), plugins.InstallZipOptions{
		ExpectedName: "zip-demo",
	})
	if !errors.Is(err, plugins.ErrPluginNameMismatch) {
		t.Fatalf("expected ErrPluginNameMismatch, got %v", err)
	}
	// Reject update targeting non-existent plugin
	_, err = host.InstallOrUpdateZip(ctx, makePluginZip("ghost-plugin", "1.0.0"), plugins.InstallZipOptions{
		ExpectedName: "ghost-plugin",
	})
	if !errors.Is(err, plugins.ErrPluginNotFound) {
		t.Fatalf("expected ErrPluginNotFound, got %v", err)
	}

	// 3. In-place update (v1.0.0 -> v1.1.0) with ExpectedName
	res2, err := host.InstallOrUpdateZip(ctx, makePluginZip("zip-demo", "1.1.0"), plugins.InstallZipOptions{
		ExpectedName: "zip-demo",
	})
	if err != nil {
		t.Fatalf("update to v1.1.0 failed: %v", err)
	}
	if !res2.Updated || res2.PreviousVersion != "1.0.0" || res2.Version != "1.1.0" {
		t.Fatalf("expected update v1.0.0 -> v1.1.0, got updated=%v prev=%q ver=%q", res2.Updated, res2.PreviousVersion, res2.Version)
	}
	// Verify .data preserved
	scratchData, err := os.ReadFile(scratchPath)
	if err != nil || string(scratchData) != "keep-me" {
		t.Fatalf("expected .data/state.txt preserved, got %q (err=%v)", string(scratchData), err)
	}
	// Verify new process spawned with new version
	raw2, err := host.InvokeCommand(ctx, "zip-demo", "ping", nil, "user-1", "admin")
	if err != nil {
		t.Fatalf("ping v1.1.0 failed: %v", err)
	}
	var ping2 struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	if err := json.Unmarshal(raw2, &ping2); err != nil || ping2.Version != "1.1.0" || ping2.PID == ping1.PID {
		t.Fatalf("expected new process at v1.1.0 (old pid=%d), got %+v", ping1.PID, ping2)
	}

	// 4. Disable plugin and update to v1.2.0: must preserve disabled state and not start process
	if _, err := host.SetEnabled(ctx, "zip-demo", false, "user-1"); err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	res3, err := host.InstallOrUpdateZip(ctx, makePluginZip("zip-demo", "1.2.0"), plugins.InstallZipOptions{
		ExpectedName: "zip-demo",
	})
	if err != nil {
		t.Fatalf("update disabled plugin failed: %v", err)
	}
	if !res3.Updated || res3.PreviousVersion != "1.1.0" || res3.Version != "1.2.0" {
		t.Fatalf("unexpected res3: %+v", res3)
	}
	if res3.Enabled || res3.Status != plugins.StatusDisabled {
		t.Fatalf("expected disabled plugin to stay disabled after update, got enabled=%v status=%s", res3.Enabled, res3.Status)
	}
}

