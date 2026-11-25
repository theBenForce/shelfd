package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultCatalogCacheTTL        = 15 * time.Minute
	defaultMemoryCatalogCacheSize = 512
	redisPingTimeout              = 3 * time.Second
)

// CachedCatalogPage stores a raw (un-enriched) plugin catalog page and the time it was fetched.
type CachedCatalogPage struct {
	Page     CatalogPage `json:"page"`
	CachedAt time.Time   `json:"cached_at"`
}

// CatalogCache caches raw plugin storefront catalog pages keyed by plugin, user, mode, and query params.
type CatalogCache interface {
	KeyPrefix() string
	Get(ctx context.Context, key string) (*CachedCatalogPage, bool)
	Set(ctx context.Context, key string, entry CachedCatalogPage, ttl time.Duration)
	InvalidatePlugin(ctx context.Context, pluginName string)
	InvalidatePluginUser(ctx context.Context, pluginName, userID string)
	Close() error
}

func normalizeCachePrefix(prefix string) string {
	p := strings.TrimRight(strings.TrimSpace(prefix), ":")
	if p == "" {
		return "shelfd"
	}
	return p
}

func catalogPluginPrefix(prefix, pluginName string) string {
	return fmt.Sprintf("%s:plugin_catalog:%s:", normalizeCachePrefix(prefix), url.PathEscape(pluginName))
}

func catalogPluginUserPrefix(prefix, pluginName, userID string) string {
	return fmt.Sprintf("%s:plugin_catalog:%s:%s:", normalizeCachePrefix(prefix), url.PathEscape(pluginName), url.PathEscape(userID))
}

func buildCatalogCacheKey(
	prefix, pluginName, userID, mode, query, group, cursor, filter string,
	filters map[string]any,
) string {
	type canonicalParams struct {
		Query   string         `json:"q,omitempty"`
		Group   string         `json:"group,omitempty"`
		Cursor  string         `json:"cursor,omitempty"`
		Filter  string         `json:"filter,omitempty"`
		Filters map[string]any `json:"filters,omitempty"`
	}
	payload, _ := json.Marshal(canonicalParams{
		Query:   strings.TrimSpace(query),
		Group:   strings.TrimSpace(group),
		Cursor:  strings.TrimSpace(cursor),
		Filter:  strings.TrimSpace(filter),
		Filters: filters,
	})
	sum := sha256.Sum256(payload)
	return catalogPluginUserPrefix(prefix, pluginName, userID) +
		url.PathEscape(strings.ToLower(strings.TrimSpace(mode))) + ":" +
		hex.EncodeToString(sum[:16])
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneTimePtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneCatalogPage(src CatalogPage) CatalogPage {
	dst := CatalogPage{
		NextCursor: src.NextCursor,
		TotalCount: cloneIntPtr(src.TotalCount),
		Cached:     src.Cached,
		CachedAt:   cloneTimePtr(src.CachedAt),
	}
	if src.Items != nil {
		dst.Items = make([]CatalogItem, len(src.Items))
		for i, it := range src.Items {
			cp := it
			if it.Authors != nil {
				cp.Authors = append([]string(nil), it.Authors...)
			}
			if it.Genres != nil {
				cp.Genres = append([]string(nil), it.Genres...)
			}
			if it.Formats != nil {
				cp.Formats = make([]CatalogFormat, len(it.Formats))
				for j, f := range it.Formats {
					fcp := f
					fcp.SizeBytes = cloneInt64Ptr(f.SizeBytes)
					cp.Formats[j] = fcp
				}
			}
			cp.UploadJobID = cloneStringPtr(it.UploadJobID)
			cp.BookID = cloneStringPtr(it.BookID)
			dst.Items[i] = cp
		}
	}
	if src.Groups != nil {
		dst.Groups = make([]CatalogGroup, len(src.Groups))
		for i, g := range src.Groups {
			gcp := g
			gcp.Count = cloneIntPtr(g.Count)
			dst.Groups[i] = gcp
		}
	}
	return dst
}

type memoryCatalogEntry struct {
	value      CachedCatalogPage
	expiresAt  time.Time
	lastAccess time.Time
}

// MemoryCatalogCache is a bounded, concurrency-safe in-memory TTL cache for plugin catalog pages.
type MemoryCatalogCache struct {
	mu         sync.Mutex
	prefix     string
	maxEntries int
	entries    map[string]memoryCatalogEntry
	now        func() time.Time
}

// NewMemoryCatalogCache creates an in-memory catalog cache with the given prefix and max entry capacity.
func NewMemoryCatalogCache(keyPrefix string, maxEntries int) *MemoryCatalogCache {
	if maxEntries <= 0 {
		maxEntries = defaultMemoryCatalogCacheSize
	}
	return &MemoryCatalogCache{
		prefix:     normalizeCachePrefix(keyPrefix),
		maxEntries: maxEntries,
		entries:    make(map[string]memoryCatalogEntry),
		now:        time.Now,
	}
}

func (c *MemoryCatalogCache) KeyPrefix() string {
	return c.prefix
}

func (c *MemoryCatalogCache) Get(_ context.Context, key string) (*CachedCatalogPage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	now := c.now()
	if !ent.expiresAt.IsZero() && now.After(ent.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	ent.lastAccess = now
	c.entries[key] = ent

	return &CachedCatalogPage{
		Page:     cloneCatalogPage(ent.value.Page),
		CachedAt: ent.value.CachedAt,
	}, true
}

func (c *MemoryCatalogCache) Set(_ context.Context, key string, entry CachedCatalogPage, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if len(c.entries) >= c.maxEntries {
		// Evict expired entries first.
		for k, v := range c.entries {
			if !v.expiresAt.IsZero() && now.After(v.expiresAt) {
				delete(c.entries, k)
			}
		}
		// If still at capacity and inserting a new key, evict the least-recently-accessed entry.
		if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxEntries {
			var oldestKey string
			var oldestTime time.Time
			for k, v := range c.entries {
				if oldestKey == "" || v.lastAccess.Before(oldestTime) {
					oldestKey = k
					oldestTime = v.lastAccess
				}
			}
			if oldestKey != "" {
				delete(c.entries, oldestKey)
			}
		}
	}

	c.entries[key] = memoryCatalogEntry{
		value: CachedCatalogPage{
			Page:     cloneCatalogPage(entry.Page),
			CachedAt: entry.CachedAt,
		},
		expiresAt:  now.Add(ttl),
		lastAccess: now,
	}
}

func (c *MemoryCatalogCache) InvalidatePlugin(_ context.Context, pluginName string) {
	prefix := catalogPluginPrefix(c.prefix, pluginName)
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

func (c *MemoryCatalogCache) InvalidatePluginUser(_ context.Context, pluginName, userID string) {
	prefix := catalogPluginUserPrefix(c.prefix, pluginName, userID)
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

func (c *MemoryCatalogCache) Close() error {
	return nil
}

// RedisCatalogCache stores plugin catalog pages in Redis with native TTL and fail-open behavior.
type RedisCatalogCache struct {
	client redis.UniversalClient
	prefix string
	logger *slog.Logger
}

// NewRedisCatalogCache connects to Redis using redisURL (e.g., "redis://localhost:6379/0")
// and verifies connectivity with a short ping.
func NewRedisCatalogCache(ctx context.Context, redisURL, keyPrefix string, logger *slog.Logger) (*RedisCatalogCache, error) {
	if logger == nil {
		logger = slog.Default()
	}
	opts, err := redis.ParseURL(strings.TrimSpace(redisURL))
	if err != nil {
		return nil, fmt.Errorf("invalid redis_url: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(ctx, redisPingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisCatalogCache{
		client: client,
		prefix: normalizeCachePrefix(keyPrefix),
		logger: logger,
	}, nil
}

// NewRedisCatalogCacheWithClient wraps an existing Redis client (primarily for testing).
func NewRedisCatalogCacheWithClient(client redis.UniversalClient, keyPrefix string, logger *slog.Logger) *RedisCatalogCache {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisCatalogCache{
		client: client,
		prefix: normalizeCachePrefix(keyPrefix),
		logger: logger,
	}
}

func (r *RedisCatalogCache) KeyPrefix() string {
	return r.prefix
}

func (r *RedisCatalogCache) Get(ctx context.Context, key string) (*CachedCatalogPage, bool) {
	raw, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			r.logger.Warn("redis catalog cache get failed", "key", key, "error", err)
		}
		return nil, false
	}
	var entry CachedCatalogPage
	if err := json.Unmarshal(raw, &entry); err != nil {
		r.logger.Warn("redis catalog cache unmarshal failed", "key", key, "error", err)
		return nil, false
	}
	return &entry, true
}

func (r *RedisCatalogCache) Set(ctx context.Context, key string, entry CachedCatalogPage, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		r.logger.Warn("redis catalog cache marshal failed", "key", key, "error", err)
		return
	}
	if err := r.client.Set(ctx, key, raw, ttl).Err(); err != nil {
		r.logger.Warn("redis catalog cache set failed", "key", key, "error", err)
	}
}

func (r *RedisCatalogCache) InvalidatePlugin(ctx context.Context, pluginName string) {
	r.deleteByPrefix(ctx, catalogPluginPrefix(r.prefix, pluginName))
}

func (r *RedisCatalogCache) InvalidatePluginUser(ctx context.Context, pluginName, userID string) {
	r.deleteByPrefix(ctx, catalogPluginUserPrefix(r.prefix, pluginName, userID))
}

func (r *RedisCatalogCache) deleteByPrefix(ctx context.Context, prefix string) {
	pattern := prefix + "*"
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			r.logger.Warn("redis catalog cache scan failed", "pattern", pattern, "error", err)
			return
		}
		if len(keys) > 0 {
			if err := r.client.Unlink(ctx, keys...).Err(); err != nil {
				r.logger.Warn("redis catalog cache unlink failed", "pattern", pattern, "error", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

func (r *RedisCatalogCache) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}
