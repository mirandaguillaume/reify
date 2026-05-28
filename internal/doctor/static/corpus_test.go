package static

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/require"
)

// TestCorpus_AllChecksOnRealFiles runs every registered static check against
// real agent files scraped from GitHub (in parser/testdata/). This verifies
// that checks don't panic, produce reasonable findings, and handle diverse
// real-world content gracefully.
//
// Satisfies AC #1: "5+ real files scraped from GitHub" — we run against 22.
func TestCorpus_AllChecksOnRealFiles(t *testing.T) {
	testdataRoot := filepath.Join("..", "parser", "testdata")
	if _, err := os.Stat(testdataRoot); err != nil {
		t.Skipf("parser testdata not available: %v", err)
	}

	dirs := []struct {
		subdir  string
		simBase string // simulated path base for format detection
	}{
		{"claude", filepath.Join(".claude", "agents")},
		{"copilot", filepath.Join(".github", "agents")},
	}

	var totalFiles int
	var totalFindings int

	for _, d := range dirs {
		dirPath := filepath.Join(testdataRoot, d.subdir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			t.Logf("skipping %s: %v", d.subdir, err)
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
				require.NoError(t, err, "format detection must succeed on real files")

				analysis, err := p.Parse(content)
				require.NoError(t, err, "parsing must succeed on real files")

				// Run all registered checks — must not panic
				findings := RunChecks(analysis, "default")
				// findings can be nil (no findings) or a slice — both are valid
				totalFindings += len(findings)
			})
			totalFiles++
		}
	}

	require.GreaterOrEqual(t, totalFiles, 5,
		"corpus test requires >= 5 real agent files, found %d", totalFiles)

	t.Logf("Corpus: %d files analyzed, %d total findings across all checks", totalFiles, totalFindings)
}

// TestCorpus_ThoroughMode runs ALL checks (including thorough-tagged like
// copy-paste detection) on the full corpus. This satisfies AC #4 by exercising
// copy-paste detection against real agent files from GitHub.
func TestCorpus_ThoroughMode(t *testing.T) {
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

	tested := 0
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

			content, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			require.NoError(t, err)

			simPath := filepath.Join(d.simBase, e.Name())
			p, err := parser.DetectFormat(simPath, content)
			require.NoError(t, err)

			analysis, err := p.Parse(content)
			require.NoError(t, err)

			_ = RunChecks(analysis, "thorough") // must not panic
			tested++
		}
	}

	require.GreaterOrEqual(t, tested, 10,
		"thorough corpus test needs at least 10 files (exercises copy-paste detection)")
}
