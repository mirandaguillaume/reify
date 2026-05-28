package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScaffold_CorpusRealFiles verifies that Scaffold produces valid output
// on real agent files without panicking. This is the --fix corpus test
// carried from Epic 3: 10+ files per format pass scaffold generation.
func TestScaffold_CorpusRealFiles(t *testing.T) {
	testdataRoot := filepath.Join("..", "parser", "testdata")
	if _, err := os.Stat(testdataRoot); err != nil {
		t.Skipf("parser testdata not available: %v", err)
	}

	dirs := []struct {
		subdir  string
		simBase string
	}{
		{"claude", filepath.Join(".claude", "agents")},
		{"copilot", filepath.Join(".github", "agents")},
	}

	var totalFiles int
	for _, d := range dirs {
		dirPath := filepath.Join(testdataRoot, d.subdir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
				continue
			}

			t.Run(d.subdir+"/"+e.Name(), func(t *testing.T) {
				content, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
				require.NoError(t, err)

				simPath := filepath.Join(d.simBase, e.Name())
				p, err := parser.DetectFormat(simPath, content)
				require.NoError(t, err)

				analysis, err := p.Parse(content)
				require.NoError(t, err)

				result, err := Scaffold(analysis, nil)
				require.NoError(t, err, "Scaffold must not error on real files")
				require.NotNil(t, result)

				// Index must be generated
				assert.NotEmpty(t, result.IndexContent, "AGENTS.md index must be non-empty")

				// Must produce files for all 6 specialized categories
				assert.Len(t, result.Files, 6, "scaffold must produce 6 specialized files")

				// Each file must be non-empty
				for path, content := range result.Files {
					assert.NotEmpty(t, content, "scaffold file %s must be non-empty", path)
				}

				totalFiles++
			})
		}
	}

	require.GreaterOrEqual(t, totalFiles, 10,
		"corpus scaffold test requires >= 10 real files, found %d", totalFiles)

	t.Logf("Scaffold corpus: %d files scaffolded successfully", totalFiles)
}
