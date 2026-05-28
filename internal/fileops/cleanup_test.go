package fileops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupBackups_RemovesOld(t *testing.T) {
	dir := t.TempDir()

	// Create an "old" .bak file
	oldBak := filepath.Join(dir, "agent.md.bak")
	require.NoError(t, os.WriteFile(oldBak, []byte("old"), 0o644))
	// Backdate it
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldBak, oldTime, oldTime))

	// Create a "new" .bak file
	newBak := filepath.Join(dir, "other.md.bak")
	require.NoError(t, os.WriteFile(newBak, []byte("new"), 0o644))

	removed, err := CleanupBackups(dir, 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	// Old one removed
	_, err = os.Stat(oldBak)
	assert.True(t, os.IsNotExist(err))

	// New one kept
	_, err = os.Stat(newBak)
	assert.NoError(t, err)
}

func TestCleanupBackups_IgnoresNonBak(t *testing.T) {
	dir := t.TempDir()

	// Create an old non-.bak file
	f := filepath.Join(dir, "agent.md")
	require.NoError(t, os.WriteFile(f, []byte("content"), 0o644))
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(f, oldTime, oldTime))

	removed, err := CleanupBackups(dir, 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)

	// File still exists
	_, err = os.Stat(f)
	assert.NoError(t, err)
}

func TestCleanupBackups_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	removed, err := CleanupBackups(dir, 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestCleanupBackups_NonexistentDir(t *testing.T) {
	_, err := CleanupBackups("/tmp/nonexistent-dir-for-test", 7*24*time.Hour)
	require.Error(t, err)
}
