package static

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestDrift_MissingFileReference(t *testing.T) {
	// Create a temp project with a go.mod marker
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	// Save and restore cwd
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Read the file at src/utils/auth.go for context."),
	}

	check := &driftCheck{}
	findings := check.Run(analysis)
	assert.True(t, len(findings) >= 1, "missing file reference should be flagged")
	assert.Contains(t, findings[0].Issue, "src/utils/auth.go")
}

func TestDrift_ExistingFileReference(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.MkdirAll(filepath.Join(dir, "src", "utils"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "utils", "auth.go"), []byte("package utils"), 0644)

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Read src/utils/auth.go for context."),
	}

	check := &driftCheck{}
	findings := check.Run(analysis)
	// Should not flag existing files
	for _, f := range findings {
		assert.NotContains(t, f.Issue, "src/utils/auth.go")
	}
}

func TestDrift_SkipsExampleContext(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	analysis := &parser.AgentAnalysis{
		RawContent: []byte("For example, src/utils/auth.go would be a good reference."),
	}

	check := &driftCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "example context should not be flagged")
}

func TestDrift_MakeTarget(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\tgo build\ntest:\n\tgo test"), 0644)

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Run make gen before testing."),
	}

	check := &driftCheck{}
	findings := check.Run(analysis)
	hasMakeFinding := false
	for _, f := range findings {
		if strings.Contains(f.Issue, "Make target") && strings.Contains(f.Issue, "gen") {
			hasMakeFinding = true
		}
	}
	assert.True(t, hasMakeFinding, "missing make target should be flagged")
}

func TestDrift_NilAnalysis(t *testing.T) {
	check := &driftCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestDrift_NoProjectRoot(t *testing.T) {
	// In /tmp with no project markers
	dir := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Read src/utils/auth.go"),
	}

	check := &driftCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "no project root = no drift checking")
}
