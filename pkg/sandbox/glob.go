package sandbox

import (
	"path/filepath"
	"strings"
)

// GlobMatch checks if a path matches a glob pattern.
// Supports ** for recursive directory matching in any position.
func GlobMatch(pattern, path string) bool {
	if !strings.Contains(pattern, "**") {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
		return pattern == path
	}

	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := parts[1]

	prefixClean := strings.TrimSuffix(prefix, "/")
	suffixClean := strings.TrimPrefix(suffix, "/")

	if prefixClean != "" {
		if !strings.HasPrefix(path, prefixClean+"/") && path != prefixClean {
			return false
		}
	}

	if suffixClean == "" {
		return true
	}

	remaining := path
	if prefixClean != "" {
		remaining = strings.TrimPrefix(path, prefixClean+"/")
	}
	pathParts := strings.Split(remaining, "/")
	for i := 0; i < len(pathParts); i++ {
		candidate := strings.Join(pathParts[i:], "/")
		if matched, err := filepath.Match(suffixClean, candidate); err == nil && matched {
			return true
		}
		if strings.Contains(suffixClean, "**") && GlobMatch(suffixClean, candidate) {
			return true
		}
	}

	return false
}
