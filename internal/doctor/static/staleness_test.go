package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestStaleness_NilAnalysis(t *testing.T) {
	check := &stalenessCheck{}
	assert.Nil(t, check.Run(nil))
}

func TestStaleness_NoFilePath(t *testing.T) {
	check := &stalenessCheck{}
	analysis := &parser.AgentAnalysis{
		Frontmatter: map[string]interface{}{},
	}
	assert.Nil(t, check.Run(analysis))
}

func TestStaleness_EmptyFilePath(t *testing.T) {
	check := &stalenessCheck{}
	analysis := &parser.AgentAnalysis{
		Frontmatter: map[string]interface{}{"_file_path": ""},
	}
	assert.Nil(t, check.Run(analysis))
}

func TestStaleness_WrongFilePathType(t *testing.T) {
	check := &stalenessCheck{}
	analysis := &parser.AgentAnalysis{
		Frontmatter: map[string]interface{}{"_file_path": 42},
	}
	assert.Nil(t, check.Run(analysis))
}

func TestStaleness_TrackedFile(t *testing.T) {
	// This test runs inside a git repo. Use a known tracked file.
	// The result depends on how recently the file was modified:
	// - If recent (<90 days): no findings
	// - If old (>90 days): one finding
	// Either way, it should not panic.
	check := &stalenessCheck{}
	analysis := &parser.AgentAnalysis{
		Frontmatter: map[string]interface{}{"_file_path": "go.mod"},
	}
	findings := check.Run(analysis)
	// go.mod exists in git — result is either nil (fresh) or 1 finding (stale)
	if findings != nil {
		assert.Len(t, findings, 1)
		assert.Equal(t, "version_drift", findings[0].Category)
	}
}

func TestStaleness_UntrackedFile(t *testing.T) {
	// A file that doesn't exist in git history returns zero time → no findings.
	check := &stalenessCheck{}
	analysis := &parser.AgentAnalysis{
		Frontmatter: map[string]interface{}{"_file_path": "this-file-does-not-exist-in-git.md"},
	}
	findings := check.Run(analysis)
	assert.Nil(t, findings, "untracked file should produce no findings")
}

func TestStaleness_CheckMetadata(t *testing.T) {
	check := &stalenessCheck{}
	assert.Equal(t, "git-staleness", check.ID())
	assert.Equal(t, "version_drift", check.Category())
	assert.Equal(t, "low", check.DefaultSeverity())
	assert.Contains(t, check.Tags(), "git-aware")
}
