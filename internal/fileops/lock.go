package fileops

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// FileLock represents an advisory file lock using a .lock file.
type FileLock struct {
	path string // path of the .lock file
}

// Lock acquires an advisory lock for the given file path.
// Uses O_CREATE|O_EXCL for atomic creation to avoid TOCTOU races.
// If a stale lock (dead process) exists, it is removed and re-acquired.
func Lock(path string) (*FileLock, error) {
	lockPath := path + ".lock"
	pid := os.Getpid()

	// Attempt atomic create — fails if file already exists
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		// We got the lock — write our PID
		fmt.Fprint(f, strconv.Itoa(pid))
		f.Close()
		return &FileLock{path: lockPath}, nil
	}

	// Lock file exists — check if holder is still alive
	data, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		return nil, fmt.Errorf("lock file %s exists but is unreadable: %w", lockPath, readErr)
	}

	pidStr := strings.TrimSpace(string(data))
	holderPID, parseErr := strconv.Atoi(pidStr)
	if parseErr == nil && processAlive(holderPID) {
		return nil, fmt.Errorf("file is locked by another reify process (PID: %d). Wait or remove %s", holderPID, lockPath)
	}

	// Stale lock — remove and retry once
	os.Remove(lockPath)
	f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create lock file %s after stale cleanup: %w", lockPath, err)
	}
	fmt.Fprint(f, strconv.Itoa(pid))
	f.Close()
	return &FileLock{path: lockPath}, nil
}

// Unlock releases the advisory lock by removing the .lock file.
func (fl *FileLock) Unlock() error {
	if fl == nil {
		return nil
	}
	return os.Remove(fl.path)
}

// processAlive checks if a process with the given PID is still running.
func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
