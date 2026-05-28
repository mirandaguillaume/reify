package fileops

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLock_AcquireAndRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	lock, err := Lock(path)
	require.NoError(t, err)
	require.NotNil(t, lock)

	// Lock file exists with our PID
	data, err := os.ReadFile(path + ".lock")
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(os.Getpid()), string(data))

	// Release
	require.NoError(t, lock.Unlock())

	// Lock file removed
	_, err = os.Stat(path + ".lock")
	assert.True(t, os.IsNotExist(err))
}

func TestLock_Contention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	lock1, err := Lock(path)
	require.NoError(t, err)
	defer lock1.Unlock()

	// Second lock should fail (same PID is alive)
	_, err = Lock(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked by another reify process")
}

func TestLock_StaleLockCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")

	// Create a stale lock with a definitely-dead PID
	lockPath := path + ".lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("999999999"), 0o644))

	// Should acquire because PID is dead
	lock, err := Lock(path)
	require.NoError(t, err)
	require.NotNil(t, lock)
	defer lock.Unlock()
}

func TestLock_UnlockNil(t *testing.T) {
	var lock *FileLock
	assert.NoError(t, lock.Unlock())
}
