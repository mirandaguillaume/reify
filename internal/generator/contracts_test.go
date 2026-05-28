package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContracts(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "review_comments.md"), []byte("## Review Comments\n\nStructured review output."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "risk_score.md"), []byte("Risk score from 0-10."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a contract"), 0644))

	contracts := generator.LoadContracts(dir)
	assert.Equal(t, 2, len(contracts))
	assert.Contains(t, contracts["review_comments"], "Structured review output")
	assert.Contains(t, contracts["risk_score"], "Risk score")
	assert.NotContains(t, contracts, "ignored")
}

func TestLoadContracts_TemplateMd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fault_location.template.md"), []byte("```\nfile: {path}\n```"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "repo_structure.template.md"), []byte("```\nproject: {name}\n```"), 0644))

	contracts := generator.LoadContracts(dir)
	assert.Equal(t, 2, len(contracts))
	assert.Contains(t, contracts["fault_location"], "file: {path}")
	assert.Contains(t, contracts["repo_structure"], "project: {name}")
}

func TestLoadContracts_MissingDir(t *testing.T) {
	contracts := generator.LoadContracts("/nonexistent/dir")
	assert.Empty(t, contracts)
}

func TestFormatContractSection_WithMatches(t *testing.T) {
	contracts := map[string]string{
		"review_comments": "Structured JSON with severity, location, message.",
		"risk_score":      "Integer 0-10 with justification.",
	}
	section := generator.FormatContractSection([]string{"review_comments", "risk_score"}, contracts)
	assert.Contains(t, section, "## Output Format")
	assert.Contains(t, section, "### Output: Review_comments")
	assert.Contains(t, section, "Structured JSON")
	assert.Contains(t, section, "### Output: Risk_score")
	assert.Contains(t, section, "Integer 0-10")
}

func TestFormatContractSection_NoMatches(t *testing.T) {
	contracts := map[string]string{
		"something_else": "unrelated",
	}
	section := generator.FormatContractSection([]string{"review_comments"}, contracts)
	assert.Empty(t, section)
}

func TestFormatContractSection_NilContracts(t *testing.T) {
	section := generator.FormatContractSection([]string{"review_comments"}, nil)
	assert.Empty(t, section)
}

func TestFormatContractSection_EmptyProduces(t *testing.T) {
	contracts := map[string]string{"review_comments": "something"}
	section := generator.FormatContractSection(nil, contracts)
	assert.Empty(t, section)
}
