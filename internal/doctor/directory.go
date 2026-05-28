package doctor

import (
	"os"
	"path/filepath"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

// discoverySkipDirs are directories to skip during agent file discovery.
// Unlike scanner.SkipDirs, this does NOT skip .claude or .github since
// those directories contain agent files we want to discover.
var discoverySkipDirs = map[string]bool{
	".git":         true,
	"vendor":       true,
	"node_modules": true,
	"__pycache__":  true,
	".next":        true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
	".tox":         true,
	"target":       true,
}

// DiscoverAgentFiles walks root recursively and returns paths to files
// recognized by any registered parser's Detect(). Infrastructure directories
// are skipped but .claude and .github are preserved for agent file discovery.
func DiscoverAgentFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		if d.IsDir() {
			name := d.Name()
			if discoverySkipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		if _, detectErr := parser.DetectFormat(path, content); detectErr == nil {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
