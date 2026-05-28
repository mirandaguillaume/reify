package fileops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeWrite_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	original := []byte("# Original content")
	require.NoError(t, os.WriteFile(path, original, 0o644))

	rewritten := []byte("# Rewritten content\n\n## New Section")
	err := SafeWrite(context.Background(), path, rewritten)
	require.NoError(t, err)

	// Verify rewritten content
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rewritten, got)

	// Verify backup exists with original content
	backup, err := os.ReadFile(path + ".bak")
	require.NoError(t, err)
	assert.Equal(t, original, backup)
}

func TestSafeWrite_PreservesExactBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	original := []byte("hello")
	require.NoError(t, os.WriteFile(path, original, 0o644))

	// Content with emoji, CJK, CRLF — SafeWrite must preserve exact bytes
	content := []byte("# 你好世界 🎉\r\nContent with émojis")
	err := SafeWrite(context.Background(), path, content)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestSafeWrite_OriginalPreservedOnTmpWriteFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	original := []byte("# Original")
	require.NoError(t, os.WriteFile(path, original, 0o644))

	// Make the tmp file path a directory to force write failure
	tmpPath := path + ".tmp"
	require.NoError(t, os.Mkdir(tmpPath, 0o755))

	err := SafeWrite(context.Background(), path, []byte("new content"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write temp file")

	// Original must be preserved
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestSafeWrite_NoOriginalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.md")

	err := SafeWrite(context.Background(), path, []byte("content"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat original for permissions")
}

func TestSafeWrite_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	original := []byte("# Original")
	require.NoError(t, os.WriteFile(path, original, 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := SafeWrite(ctx, path, []byte("new content"))
	require.Error(t, err)

	// Original must be preserved
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestRestore_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	original := []byte("# Original")
	require.NoError(t, os.WriteFile(path, original, 0o644))

	// SafeWrite to create backup
	rewritten := []byte("# Rewritten")
	require.NoError(t, SafeWrite(context.Background(), path, rewritten))

	// Restore from backup
	err := Restore(path)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, got)
}

func TestRestore_NoBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))

	err := Restore(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backup found")
}
