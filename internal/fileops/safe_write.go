// Package fileops provides defensive file operations for the doctor --fix flow.
// All user file modifications go through SafeWrite, which creates a backup,
// writes to a temp file, and atomically renames to the target.
package fileops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// SafeWrite writes content to path atomically with a backup.
// Flow: (a) copy original to path.bak, (b) write to path.tmp, (c) rename tmp → path.
// If ctx is cancelled at any step, the operation aborts without partial writes.
func SafeWrite(ctx context.Context, path string, content []byte) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	backupPath := absPath + ".bak"
	tmpPath := absPath + ".tmp"

	// Acquire advisory lock
	lock, err := Lock(absPath)
	if err != nil {
		return fmt.Errorf("lock file: %w", err)
	}
	defer lock.Unlock()

	// Capture original file permissions before backup
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat original for permissions: %w", err)
	}
	perm := info.Mode().Perm()

	// Step (a): backup original
	original, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read original for backup: %w", err)
	}
	if err := os.WriteFile(backupPath, original, perm); err != nil {
		return fmt.Errorf("create backup at %s: %w", backupPath, err)
	}

	// Step (b): write to temp file (same directory for same-filesystem rename)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, content, perm); err != nil {
		os.Remove(tmpPath) // clean up partial write
		return fmt.Errorf("write temp file: %w", err)
	}

	// Step (c): atomic rename
	if err := ctx.Err(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		return fmt.Errorf("atomic rename %s → %s: %w (manual rename may be needed)", tmpPath, absPath, err)
	}

	return nil
}

// Restore restores a file from its .bak backup.
func Restore(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	backupPath := absPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found at %s", backupPath)
	}

	return os.Rename(backupPath, absPath)
}
