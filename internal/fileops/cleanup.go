package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultBackupMaxAge is the default retention period for .bak files.
const DefaultBackupMaxAge = 7 * 24 * time.Hour

// CleanupBackups removes .bak files older than maxAge in dir (non-recursive).
// Returns the number of files removed.
func CleanupBackups(dir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bak") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(dir, e.Name())
			if err := os.Remove(fullPath); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}
