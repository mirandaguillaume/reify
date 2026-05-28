package doctor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheKey_Deterministic(t *testing.T) {
	key1 := CacheKey([]byte("content"), "prompt", "model", "v1")
	key2 := CacheKey([]byte("content"), "prompt", "model", "v1")
	assert.Equal(t, key1, key2)
	// Format: {8-char content prefix}-{16-char composite hash} = 25 chars total
	assert.Len(t, key1, contentHashPrefixLen+1+16)
}

func TestCacheKey_StartsWithContentHashPrefix(t *testing.T) {
	// The first contentHashPrefixLen chars of CacheKey must match the first
	// chars of sha256(content). This is the contract Purge depends on.
	// Story 4-0 AC #5.
	content := []byte("my-special-content")
	key := CacheKey(content, "prompt", "model", "v1")
	expectedPrefix := fmt.Sprintf("%x", sha256.Sum256(content))[:contentHashPrefixLen]
	assert.True(t, len(key) > contentHashPrefixLen+1, "key must contain prefix + dash + composite")
	assert.Equal(t, expectedPrefix, key[:contentHashPrefixLen])
	assert.Equal(t, byte('-'), key[contentHashPrefixLen])
}

func TestCacheKey_SameContentDifferentConfigShareSamePrefix(t *testing.T) {
	// Same content with different prompt/model/registry must produce keys
	// that share the same content-hash prefix (so a single Purge call
	// removes them all when the file changes).
	content := []byte("the-file-content")
	k1 := CacheKey(content, "prompt-a", "model-a", "v1")
	k2 := CacheKey(content, "prompt-b", "model-b", "v2")
	k3 := CacheKey(content, "prompt-c", "model-a", "v1")
	assert.Equal(t, k1[:contentHashPrefixLen], k2[:contentHashPrefixLen])
	assert.Equal(t, k1[:contentHashPrefixLen], k3[:contentHashPrefixLen])
	// But composites differ
	assert.NotEqual(t, k1, k2)
	assert.NotEqual(t, k1, k3)
}

func TestCacheKey_DifferentContent(t *testing.T) {
	key1 := CacheKey([]byte("content-a"), "prompt", "model", "v1")
	key2 := CacheKey([]byte("content-b"), "prompt", "model", "v1")
	assert.NotEqual(t, key1, key2)
}

func TestCacheKey_DifferentModel(t *testing.T) {
	key1 := CacheKey([]byte("content"), "prompt", "model-a", "v1")
	key2 := CacheKey([]byte("content"), "prompt", "model-b", "v1")
	assert.NotEqual(t, key1, key2)
}

func TestCacheKey_DifferentRegistry(t *testing.T) {
	key1 := CacheKey([]byte("content"), "prompt", "model", "v1")
	key2 := CacheKey([]byte("content"), "prompt", "model", "v2")
	assert.NotEqual(t, key1, key2)
}

func TestCache_PutAndGet(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache"))

	findings := []llmutil.Finding{
		{Category: "guardrails", Issue: "No timeout", Confidence: "high"},
	}
	key := "test-key-12345678"
	entry := CacheEntry{
		ContentHash:     "abc123",
		Model:           "llama4",
		RegistryVersion: "2026.03.2",
		Timestamp:       time.Now(),
		Findings:        findings,
	}

	err := c.Put(key, entry)
	require.NoError(t, err)

	got := c.Get(key)
	require.NotNil(t, got)
	assert.Len(t, got, 1)
	assert.Equal(t, "guardrails", got[0].Category)
	assert.Equal(t, "No timeout", got[0].Issue)
}

func TestCache_Miss_NoFile(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache"))
	got := c.Get("nonexistent-key")
	assert.Nil(t, got)
}

func TestCache_Miss_Expired(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache"))

	key := "expired-key-0000"
	entry := CacheEntry{
		ContentHash: "abc",
		Timestamp:   time.Now().Add(-8 * 24 * time.Hour), // 8 days ago
		Findings:    []llmutil.Finding{{Category: "test", Issue: "old"}},
	}

	err := c.Put(key, entry)
	require.NoError(t, err)

	got := c.Get(key)
	assert.Nil(t, got, "expired entries should return nil")
}

func TestCache_Miss_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	// Write invalid JSON
	path := filepath.Join(cacheDir, "bad-json-key.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0644))

	c := NewCache(cacheDir)
	got := c.Get("bad-json-key")
	assert.Nil(t, got)
}

func TestCache_Purge(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache"))

	// Use CacheKey to generate realistic keys (with content-hash prefix).
	// Story 4-0 AC #5: Purge matches by filename prefix.
	content := []byte("my-file-content")
	otherContent := []byte("a-different-file")

	// Two cache entries for the same content (different prompt/model variants)
	key1 := CacheKey(content, "prompt-a", "model", "v1")
	key2 := CacheKey(content, "prompt-b", "model", "v1")
	// One cache entry for unrelated content
	key3 := CacheKey(otherContent, "prompt-a", "model", "v1")

	contentHash := fmt.Sprintf("%x", sha256.Sum256(content))[:16]
	otherHash := fmt.Sprintf("%x", sha256.Sum256(otherContent))[:16]

	entry1 := CacheEntry{ContentHash: contentHash, Timestamp: time.Now(), Findings: []llmutil.Finding{{Issue: "a"}}}
	entry2 := CacheEntry{ContentHash: contentHash, Timestamp: time.Now(), Findings: []llmutil.Finding{{Issue: "b"}}}
	entry3 := CacheEntry{ContentHash: otherHash, Timestamp: time.Now(), Findings: []llmutil.Finding{{Issue: "c"}}}

	require.NoError(t, c.Put(key1, entry1))
	require.NoError(t, c.Put(key2, entry2))
	require.NoError(t, c.Put(key3, entry3))

	// Purge entries for the file whose content was modified
	c.Purge(contentHash)

	assert.Nil(t, c.Get(key1), "purged entry (same content prefix) should be nil")
	assert.Nil(t, c.Get(key2), "purged entry (same content prefix) should be nil")
	assert.NotNil(t, c.Get(key3), "non-matching entry (different content prefix) should survive purge")
}

// TestCache_Purge_FilenamePrefixOptimization verifies that Purge matches by
// filename prefix (no file parsing). It plants a malformed JSON file with the
// matching prefix — Purge must still remove it without choking on the bad JSON.
// Story 4-0 AC #5.
func TestCache_Purge_FilenamePrefixOptimization(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	c := NewCache(cacheDir)
	content := []byte("file-content")
	contentHashFull := fmt.Sprintf("%x", sha256.Sum256(content))
	prefix := contentHashFull[:contentHashPrefixLen]

	// Plant a malformed JSON file with the matching prefix.
	// Old Purge would fail to unmarshal and skip removing it.
	// New Purge matches by filename prefix and removes it without parsing.
	malformedPath := filepath.Join(cacheDir, prefix+"-deadbeef12345678.json")
	require.NoError(t, os.WriteFile(malformedPath, []byte("{not-valid-json"), 0600))

	// Plant a valid file with NON-matching prefix that should survive
	survivorPath := filepath.Join(cacheDir, "01234567-87654321cafebabe.json")
	survivorEntry := CacheEntry{ContentHash: "abc", Timestamp: time.Now(), Findings: []llmutil.Finding{{Issue: "survivor"}}}
	survivorData, _ := json.Marshal(survivorEntry)
	require.NoError(t, os.WriteFile(survivorPath, survivorData, 0600))

	c.Purge(contentHashFull[:16])

	// Malformed file with matching prefix should be removed
	_, err := os.Stat(malformedPath)
	assert.True(t, os.IsNotExist(err), "malformed file with matching prefix should be removed without parsing")

	// Survivor file with different prefix should remain
	_, err = os.Stat(survivorPath)
	assert.NoError(t, err, "file with non-matching prefix should not be touched")
}

// TestCache_Purge_ShortHashIgnored verifies Purge degrades safely when given
// a too-short content hash (invariant: needs at least contentHashPrefixLen chars).
func TestCache_Purge_ShortHashIgnored(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))

	c := NewCache(cacheDir)
	content := []byte("file-content")
	key := CacheKey(content, "p", "m", "v")
	entry := CacheEntry{
		Timestamp: time.Now(),
		Findings:  []llmutil.Finding{{Issue: "still here"}},
	}
	require.NoError(t, c.Put(key, entry))

	// Short hash - Purge should no-op (returns early before any directory walk)
	c.Purge("abc")

	// Verify the file is still on disk (filesystem-level check, not Get,
	// because Get returns nil when Findings is empty even on a present entry).
	files, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Len(t, files, 1, "entry file should survive purge with too-short hash")
}

// TestCache_EmptyFindingsCacheHit verifies that an entry with zero findings
// is a valid cache hit (returns empty slice, not nil). This prevents the
// "clean file re-analyzed on every run" bug (story 4-2 review patch).
func TestCache_EmptyFindingsCacheHit(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache"))

	key := "empty-findings-key"
	entry := CacheEntry{
		ContentHash: "abc",
		Timestamp:   time.Now(),
		Findings:    []llmutil.Finding{}, // empty, not nil
	}

	require.NoError(t, c.Put(key, entry))

	got := c.Get(key)
	// Must return non-nil empty slice (valid cache hit), not nil (cache miss)
	require.NotNil(t, got, "empty findings must be a valid cache hit, not nil")
	assert.Len(t, got, 0)
}

func TestCache_CreatesDirOnPut(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "deep", "nested", "cache")
	c := NewCache(cacheDir)

	entry := CacheEntry{Timestamp: time.Now(), Findings: []llmutil.Finding{}}
	err := c.Put("test-key", entry)
	require.NoError(t, err)

	_, err = os.Stat(cacheDir)
	assert.NoError(t, err, "cache dir should be created")
}
