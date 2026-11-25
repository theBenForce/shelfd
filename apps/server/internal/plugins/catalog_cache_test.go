package plugins

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"

	"github.com/shelfd/shelfd/internal/database"
	"github.com/shelfd/shelfd/internal/repository"
)

func init() {
	sqlite_vec.Auto()
}

func setupCatalogCacheTestRepo(t *testing.T) repository.StorageEngine {
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

func TestMemoryCatalogCache_HitExpiryEvictionIsolationAndInvalidation(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCatalogCache("testpref", 2)

	curTime := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return curTime }

	k1 := buildCatalogCacheKey(cache.KeyPrefix(), "gutenberg", "user-1", "browse", "", "", "", "", nil)
	k2 := buildCatalogCacheKey(cache.KeyPrefix(), "gutenberg", "user-2", "browse", "", "", "", "", nil)
	k3 := buildCatalogCacheKey(cache.KeyPrefix(), "other-store", "user-1", "browse", "", "", "", "", nil)

	total := 10
	cache.Set(ctx, k1, CachedCatalogPage{
		Page: CatalogPage{
			Items:      []CatalogItem{{ExternalID: "1", Title: "Book One", Authors: []string{"Alice"}}},
			TotalCount: &total,
		},
		CachedAt: curTime,
	}, 5*time.Minute)

	// 1. Hit + deep-copy isolation
	got, ok := cache.Get(ctx, k1)
	if !ok {
		t.Fatalf("expected cache hit for k1")
	}
	got.Page.Items[0].Title = "MUTATED"
	got.Page.Items[0].Status = "in_library"
	got2, ok := cache.Get(ctx, k1)
	if !ok || got2.Page.Items[0].Title != "Book One" || got2.Page.Items[0].Status != "" {
		t.Fatalf("cache entry was mutated by caller: %+v", got2.Page.Items[0])
	}

	// 2. Capacity eviction (maxEntries = 2): access k1 at t+1s, set k2 at t+2s, set k3 at t+3s -> k1 is LRU and evicted
	curTime = curTime.Add(time.Second)
	_, _ = cache.Get(ctx, k1)
	curTime = curTime.Add(time.Second)
	cache.Set(ctx, k2, CachedCatalogPage{
		Page:     CatalogPage{Items: []CatalogItem{{ExternalID: "2", Title: "Book Two"}}},
		CachedAt: curTime,
	}, 5*time.Minute)
	curTime = curTime.Add(time.Second)
	cache.Set(ctx, k3, CachedCatalogPage{
		Page:     CatalogPage{Items: []CatalogItem{{ExternalID: "3", Title: "Book Three"}}},
		CachedAt: curTime,
	}, 5*time.Minute)

	if _, ok := cache.Get(ctx, k1); ok {
		t.Fatalf("expected k1 to be evicted when capacity=2 exceeded")
	}
	if _, ok := cache.Get(ctx, k2); !ok {
		t.Fatalf("expected k2 to remain cached")
	}
	if _, ok := cache.Get(ctx, k3); !ok {
		t.Fatalf("expected k3 to remain cached")
	}

	// 3. InvalidatePluginUser only removes matching plugin+user
	cache.InvalidatePluginUser(ctx, "gutenberg", "user-2")
	if _, ok := cache.Get(ctx, k2); ok {
		t.Fatalf("expected k2 to be invalidated by InvalidatePluginUser")
	}
	if _, ok := cache.Get(ctx, k3); !ok {
		t.Fatalf("expected k3 (other-store) to remain cached")
	}

	// 4. InvalidatePlugin removes all entries for plugin
	cache.InvalidatePlugin(ctx, "other-store")
	if _, ok := cache.Get(ctx, k3); ok {
		t.Fatalf("expected k3 to be invalidated by InvalidatePlugin")
	}

	// 5. TTL expiration
	cache.Set(ctx, k1, CachedCatalogPage{
		Page:     CatalogPage{Items: []CatalogItem{{ExternalID: "1", Title: "Expiring"}}},
		CachedAt: curTime,
	}, 30*time.Second)
	curTime = curTime.Add(31 * time.Second)
	if _, ok := cache.Get(ctx, k1); ok {
		t.Fatalf("expected k1 to expire after TTL")
	}
}

func TestNewRedisCatalogCache_InvalidAndUnreachable(t *testing.T) {
	ctx := context.Background()
	if _, err := NewRedisCatalogCache(ctx, "not-a-valid-url://", "shelfd", nil); err == nil {
		t.Fatalf("expected error for invalid redis URL")
	}
	if _, err := NewRedisCatalogCache(ctx, "redis://127.0.0.1:1/0", "shelfd", nil); err == nil {
		t.Fatalf("expected error for unreachable redis server")
	}
}

const mockCacheCounterPluginScript = `
import readline from 'node:readline';

let browseCalls = 0;
let searchCalls = 0;

function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n');
}

const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
rl.on('line', (line) => {
  const trimmed = line.trim();
  if (!trimmed) return;
  let msg;
  try { msg = JSON.parse(trimmed); } catch { return; }
  if (msg.method === 'initialize') {
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        protocol_version: '2026-11-01',
        name: 'mock-cache-store',
        version: '0.1.0',
        capabilities: ['catalog'],
        catalog: { modes: ['browse', 'search'] }
      }
    });
    return;
  }
  if (msg.method === 'catalog/browse') {
    browseCalls++;
    const userId = msg.params?.auth?.user_id || '';
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        items: [
          {
            external_id: 'ext-1',
            title: 'Browse Call #' + browseCalls + ' for ' + userId,
            authors: ['Test Author']
          },
          {
            external_id: 'ext-2',
            title: 'Second Book',
            authors: ['Author B']
          }
        ],
        total_count: 2
      }
    });
    return;
  }
  if (msg.method === 'catalog/search') {
    searchCalls++;
    send({
      jsonrpc: '2.0',
      id: msg.id,
      result: {
        items: [
          {
            external_id: 'search-1',
            title: 'Search Call #' + searchCalls + ' (' + (msg.params?.query || '') + ')',
            authors: ['Search Author']
          }
        ],
        total_count: 1
      }
    });
    return;
  }
  if (msg.method === 'shutdown') {
    send({ jsonrpc: '2.0', id: msg.id, result: { ok: true } });
    process.exit(0);
  }
  send({ jsonrpc: '2.0', id: msg.id, result: {} });
});
`

func TestHost_CatalogCachingForceRefreshFreshEnrichmentAndInvalidation(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed in PATH")
	}

	ctx := context.Background()
	repo := setupCatalogCacheTestRepo(t)

	userA := &repository.User{ID: "u-cache-a", Username: "alice", PasswordHash: "x"}
	userB := &repository.User{ID: "u-cache-b", Username: "bob", PasswordHash: "x"}
	if err := repo.CreateUser(ctx, userA); err != nil {
		t.Fatalf("create userA: %v", err)
	}
	if err := repo.CreateUser(ctx, userB); err != nil {
		t.Fatalf("create userB: %v", err)
	}

	pluginsDir := t.TempDir()
	pDir := filepath.Join(pluginsDir, "mock-cache-store")
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("mkdir plugin: %v", err)
	}
	manifestJSON := `{
  "name": "mock-cache-store",
  "display_name": "Mock Cache Store",
  "version": "0.1.0",
  "entrypoint": { "command": "node", "args": ["index.mjs"] },
  "capabilities": ["catalog"],
  "catalog_modes": ["browse", "search"],
  "config_schema": {
    "type": "object",
    "properties": {
      "region": { "type": "string" }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(manifestJSON), 0644); err != nil {
		t.Fatalf("write plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "index.mjs"), []byte(mockCacheCounterPluginScript), 0644); err != nil {
		t.Fatalf("write index.mjs: %v", err)
	}

	host := NewHost(repo, HostConfig{
		PluginsDir:      pluginsDir,
		APIBase:         "http://127.0.0.1:8080",
		ShelfdVersion:   "0.1.0-test",
		MasterSecret:    "cache-test-secret",
		CatalogCacheTTL: 10 * time.Minute,
	})
	host.Start(ctx)
	defer host.Stop()

	// 1. First browse for Alice -> cache miss (RPC call #1)
	p1, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userA.ID, userA.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog #1: %v", err)
	}
	if p1.Cached {
		t.Fatalf("expected first browse to be Cached=false")
	}
	if p1.Items[0].Title != "Browse Call #1 for u-cache-a" {
		t.Fatalf("unexpected title on call #1: %q", p1.Items[0].Title)
	}
	if p1.Items[0].Status != "available" {
		t.Fatalf("expected ext-1 status=available, got %q", p1.Items[0].Status)
	}

	// 2. Import ext-1 into Shelfd library, then call BrowseCatalog again -> MUST be cache hit AND freshly enriched as in_library!
	book := &repository.Book{
		ID:       "book-ext-1",
		Title:    "Imported Book",
		FilePath: "/tmp/imported.epub",
	}
	if err := repo.CreateBook(ctx, book); err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	bookID := book.ID
	if err := repo.UpsertBookSource(ctx, &repository.BookSource{
		ID:         "src-ext-1",
		PluginName: "mock-cache-store",
		ExternalID: "ext-1",
		BookID:     &bookID,
	}); err != nil {
		t.Fatalf("UpsertBookSource: %v", err)
	}

	p2, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userA.ID, userA.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog #2: %v", err)
	}
	if !p2.Cached {
		t.Fatalf("expected second browse to be Cached=true")
	}
	if p2.Items[0].Title != "Browse Call #1 for u-cache-a" {
		t.Fatalf("expected cached raw title from call #1, got %q", p2.Items[0].Title)
	}
	if p2.Items[0].Status != "in_library" || p2.Items[0].BookID == nil || *p2.Items[0].BookID != bookID {
		t.Fatalf("expected fresh DB enrichment status=in_library on cache hit, got status=%q bookID=%v", p2.Items[0].Status, p2.Items[0].BookID)
	}

	// 3. Per-user isolation: Bob must not see Alice's cached catalog (triggers RPC call #2)
	pBob, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userB.ID, userB.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog Bob: %v", err)
	}
	if pBob.Cached {
		t.Fatalf("expected Bob's first browse to be Cached=false")
	}
	if pBob.Items[0].Title != "Browse Call #2 for u-cache-b" {
		t.Fatalf("unexpected title for Bob: %q", pBob.Items[0].Title)
	}

	// 4. ForceRefresh for Alice bypasses cache (triggers RPC call #3) and updates cached entry
	p3, err := host.BrowseCatalogWithOptions(ctx, "mock-cache-store", "", "", "", userA.ID, userA.Username, CatalogQueryOptions{ForceRefresh: true})
	if err != nil {
		t.Fatalf("BrowseCatalog ForceRefresh: %v", err)
	}
	if p3.Cached {
		t.Fatalf("expected ForceRefresh response to have Cached=false")
	}
	if p3.Items[0].Title != "Browse Call #3 for u-cache-a" {
		t.Fatalf("expected ForceRefresh to invoke RPC #3, got %q", p3.Items[0].Title)
	}

	// Subsequent normal browse for Alice returns the newly cached call #3
	p4, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userA.ID, userA.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog #4: %v", err)
	}
	if !p4.Cached || p4.Items[0].Title != "Browse Call #3 for u-cache-a" {
		t.Fatalf("expected cached call #3, got cached=%v title=%q", p4.Cached, p4.Items[0].Title)
	}

	// 5. UpdateUserConfig invalidates Alice's cache but not Bob's
	if _, _, err := host.UpdateUserConfig(ctx, userA.ID, "mock-cache-store", map[string]any{"region": "uk"}); err != nil {
		t.Fatalf("UpdateUserConfig: %v", err)
	}
	p5, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userA.ID, userA.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog after config update: %v", err)
	}
	if p5.Cached {
		t.Fatalf("expected Alice's cache to be invalidated after UpdateUserConfig")
	}
	pBob2, err := host.BrowseCatalog(ctx, "mock-cache-store", "", "", "", userB.ID, userB.Username)
	if err != nil {
		t.Fatalf("BrowseCatalog Bob #2: %v", err)
	}
	if !pBob2.Cached {
		t.Fatalf("expected Bob's cache to remain valid after Alice's config update")
	}

	// 6. SearchCatalog caching + ForceRefresh
	s1, err := host.SearchCatalog(ctx, "mock-cache-store", "dune", "", nil, userA.ID, userA.Username)
	if err != nil || s1.Cached {
		t.Fatalf("SearchCatalog #1: err=%v cached=%v", err, s1.Cached)
	}
	s2, err := host.SearchCatalog(ctx, "mock-cache-store", "dune", "", nil, userA.ID, userA.Username)
	if err != nil || !s2.Cached || s2.Items[0].Title != s1.Items[0].Title {
		t.Fatalf("SearchCatalog #2 expected cache hit: err=%v cached=%v", err, s2.Cached)
	}
	s3, err := host.SearchCatalogWithOptions(ctx, "mock-cache-store", "dune", "", nil, userA.ID, userA.Username, CatalogQueryOptions{ForceRefresh: true})
	if err != nil || s3.Cached || s3.Items[0].Title == s1.Items[0].Title {
		t.Fatalf("SearchCatalog ForceRefresh expected fresh call: err=%v cached=%v title=%q", err, s3.Cached, s3.Items[0].Title)
	}
}
