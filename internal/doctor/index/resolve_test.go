package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveIndex_ValidLinks(t *testing.T) {
	dir := t.TempDir()

	// Create index
	indexContent := []byte(`# Agent Config
| [Security](.agents/security.md) | access |
| [Testing](.agents/testing.md) | tests |
`)

	// Create linked files
	os.MkdirAll(filepath.Join(dir, ".agents"), 0755)
	os.WriteFile(filepath.Join(dir, ".agents", "security.md"), []byte("# Security\n\nFilesystem restricted."), 0644)
	os.WriteFile(filepath.Join(dir, ".agents", "testing.md"), []byte("# Testing\n\nRun go test."), 0644)

	resolved := ResolveIndex(indexContent, dir)
	assert.Len(t, resolved, 2)
	assert.False(t, resolved[0].Missing)
	assert.False(t, resolved[1].Missing)
	assert.NotNil(t, resolved[0].Analysis)
	assert.NotNil(t, resolved[1].Analysis)
}

func TestResolveIndex_MissingFile(t *testing.T) {
	dir := t.TempDir()

	indexContent := []byte(`| [Security](.agents/security.md) | access |
| [Missing](.agents/missing.md) | gone |
`)

	os.MkdirAll(filepath.Join(dir, ".agents"), 0755)
	os.WriteFile(filepath.Join(dir, ".agents", "security.md"), []byte("# Security\n\nOK."), 0644)
	// .agents/missing.md does NOT exist

	resolved := ResolveIndex(indexContent, dir)
	assert.Len(t, resolved, 2)

	// First should be fine
	assert.False(t, resolved[0].Missing)

	// Second should be missing
	assert.True(t, resolved[1].Missing)
	assert.NotNil(t, resolved[1].Error)
}

func TestResolveIndex_NoLinks(t *testing.T) {
	dir := t.TempDir()
	indexContent := []byte("# Agent Config\n\nNo links here.\n")
	resolved := ResolveIndex(indexContent, dir)
	assert.Nil(t, resolved)
}

func TestResolveIndex_SkipsExternalURLs(t *testing.T) {
	dir := t.TempDir()
	indexContent := []byte(`[Google](https://google.com)
[Local](.agents/local.md)
`)
	os.MkdirAll(filepath.Join(dir, ".agents"), 0755)
	os.WriteFile(filepath.Join(dir, ".agents", "local.md"), []byte("# Local\n"), 0644)

	resolved := ResolveIndex(indexContent, dir)
	assert.Len(t, resolved, 1)
	assert.Equal(t, ".agents/local.md", resolved[0].Path)
}

func TestResolveIndex_DeduplicatesLinks(t *testing.T) {
	dir := t.TempDir()
	indexContent := []byte(`[Security](.agents/sec.md)
[Also Security](.agents/sec.md)
`)
	os.MkdirAll(filepath.Join(dir, ".agents"), 0755)
	os.WriteFile(filepath.Join(dir, ".agents", "sec.md"), []byte("# Sec\n"), 0644)

	resolved := ResolveIndex(indexContent, dir)
	assert.Len(t, resolved, 1, "duplicate links should be deduped")
}

func TestIsIndex(t *testing.T) {
	assert.True(t, IsIndex([]byte("[A](a.md)\n[B](b.md)\n")))
	assert.False(t, IsIndex([]byte("[A](a.md)\n")))    // only 1 link
	assert.False(t, IsIndex([]byte("No links here."))) // no links
}

func TestMissingFiles(t *testing.T) {
	resolved := []ResolvedFile{
		{Path: "a.md", Missing: false},
		{Path: "b.md", Missing: true, Error: os.ErrNotExist},
		{Path: "c.md", Missing: true, Error: os.ErrNotExist},
	}

	findings := MissingFiles(resolved)
	assert.Len(t, findings, 2)
	assert.Contains(t, findings[0].Issue, "b.md")
	assert.Contains(t, findings[1].Issue, "c.md")
	assert.Equal(t, "high", findings[0].Confidence)
}
