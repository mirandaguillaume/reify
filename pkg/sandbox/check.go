package sandbox

import (
	"fmt"
	"path/filepath"
)

// CheckAccess validates a file path against a FileAccessPolicy for a given action.
// action is "read", "write", or "delete" ("delete" checks against WriteGlobs).
// Returns ("", true) if allowed, or (reason, false) if denied.
// Paths are cleaned (filepath.Clean) to prevent traversal bypasses via "..".
func CheckAccess(policy *FileAccessPolicy, path string, action string) (reason string, allowed bool) {
	if policy == nil {
		return "", true
	}

	// Normalize path to prevent traversal attacks (e.g. "src/../etc/passwd")
	path = filepath.Clean(path)

	// Deny globs always take precedence
	for _, g := range policy.DenyGlobs {
		if GlobMatch(g, path) {
			return fmt.Sprintf("denied by glob %q", g), false
		}
	}

	// Determine which globs to check based on action
	var globs []string
	switch action {
	case "read":
		globs = policy.ReadGlobs
	case "write", "delete":
		globs = policy.WriteGlobs
	default:
		return fmt.Sprintf("unknown action %q", action), false
	}

	// Empty globs = no restriction (allow all)
	if len(globs) == 0 {
		return "", true
	}

	// Path must match at least one allow glob
	for _, g := range globs {
		if GlobMatch(g, path) {
			return "", true
		}
	}

	return fmt.Sprintf("%s not in allowed globs %v", action, globs), false
}
