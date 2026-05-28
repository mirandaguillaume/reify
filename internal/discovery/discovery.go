// Package discovery walks a filesystem path and returns agent definition
// files recognized by the parser registry. It accepts a single file, an
// agent directory (e.g. .claude/, .github/), or a full repo root.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

// skipDirs are directories ignored during recursive discovery.
// .claude and .github are NOT skipped because they hold agent files.
var skipDirs = map[string]bool{
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

// Resolve returns the list of agent files to process for the given path.
//
// Behaviour:
//   - path is a file → returns [path] without checking format (caller decides)
//   - path is a directory → returns every file matched by DiscoverAgentFiles
//   - path does not exist → returns an error
func Resolve(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	return DiscoverAgentFiles(path)
}

// DiscoverAgentFiles walks root recursively and returns paths to files
// recognized by any registered parser's Detect(). Infrastructure directories
// are skipped but .claude and .github are preserved for agent file discovery.
func DiscoverAgentFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if _, detectErr := parser.DetectFormat(path, content); detectErr == nil {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
