// Package doctor provides the top-level doctor analysis orchestration.
package doctor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

// DefaultCacheTTL is the time-to-live for cache entries.
const DefaultCacheTTL = 7 * 24 * time.Hour

// CacheEntry stores a cached LLM analysis result.
type CacheEntry struct {
	ContentHash     string           `json:"content_hash"`
	PromptHash      string           `json:"prompt_hash"`
	Model           string           `json:"model"`
	RegistryVersion string           `json:"registry_version"`
	Timestamp       time.Time        `json:"timestamp"`
	Findings        []llmutil.Finding `json:"findings"`
}

// Cache manages LLM response caching for doctor analysis.
type Cache struct {
	dir string
	ttl time.Duration
}

// NewCache creates a cache backed by the given directory.
// The directory is created on first write if it does not exist.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir, ttl: DefaultCacheTTL}
}

// contentHashPrefixLen is the number of hex characters from sha256(content)
// embedded as a prefix in the cache key. 8 hex chars = 32 bits = ~4 billion
// possible prefixes — collision risk is negligible at realistic cache sizes
// and the prefix lets Purge match by filename without opening any cache files.
// Story 4-0 AC #5.
const contentHashPrefixLen = 8

// CacheKey computes a deterministic key from file content and analysis configuration.
// Fields are separated by null bytes to prevent concatenation collisions.
//
// The returned key starts with a content-hash prefix (`{prefix}-{composite}`)
// so Purge can find all cache entries for a given file by matching filenames
// rather than parsing every cache file. Story 4-0 AC #5.
func CacheKey(content []byte, promptHash, model, registryVersion string) string {
	contentSum := sha256.Sum256(content)
	prefix := fmt.Sprintf("%x", contentSum)[:contentHashPrefixLen]

	h := sha256.New()
	h.Write(content)
	h.Write([]byte{0})
	h.Write([]byte(promptHash))
	h.Write([]byte{0})
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(registryVersion))
	composite := fmt.Sprintf("%x", h.Sum(nil))[:16]

	return prefix + "-" + composite
}

// Get returns cached findings if valid, or nil on cache miss.
func (c *Cache) Get(key string) []llmutil.Finding {
	path := c.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}

	if time.Since(entry.Timestamp) > c.ttl {
		return nil
	}

	return entry.Findings
}

// Put stores findings in the cache.
func (c *Cache) Put(key string, entry CacheEntry) error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	return os.WriteFile(c.path(key), data, 0600)
}

// Purge removes all cache entries for the given content hash.
//
// Matches by filename prefix (the first contentHashPrefixLen hex chars of the
// content's SHA256, embedded in the key by CacheKey). This avoids opening
// every JSON file in the cache directory — O(directory listing) instead of
// O(n parses). Story 4-0 AC #5.
//
// The contentHash parameter is the hex string of sha256(content); only its
// first contentHashPrefixLen characters are used for matching. Callers
// historically pass the first 16 characters; that still works since we only
// look at the first 8.
//
// Note: cache entries written by older versions of reify (before this
// change) used a different key format and won't be matched by prefix. They
// expire naturally via TTL (DefaultCacheTTL = 7 days).
func (c *Cache) Purge(contentHash string) {
	if len(contentHash) < contentHashPrefixLen {
		return
	}
	prefix := contentHash[:contentHashPrefixLen] + "-"

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		if err := os.Remove(filepath.Join(c.dir, e.Name())); err != nil && !os.IsNotExist(err) {
			// Log but don't stop — a permission failure on one entry shouldn't
			// prevent other stale entries from being purged.
			_, _ = fmt.Fprintf(os.Stderr, "doctor: cache purge: %v\n", err)
		}
	}
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".json")
}
