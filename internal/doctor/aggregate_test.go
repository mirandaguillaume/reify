package doctor

import (
	"fmt"
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/stretchr/testify/assert"
)

func TestAggregate_EmptyResults(t *testing.T) {
	report := Aggregate(nil)
	assert.Equal(t, 0, report.TotalFiles)
	assert.Equal(t, 0, report.AnalyzedFiles)
	assert.Equal(t, 0, report.FailedFiles)
	assert.Equal(t, 0, report.TotalFindings)
	assert.Empty(t, report.ByCategory)
	assert.Empty(t, report.FilesNoGuardrails)
	assert.Empty(t, report.FilesNoSecurity)
}

func TestAggregate_SingleFileNoFindings(t *testing.T) {
	results := []FileResult{
		{Path: "agent.md", Format: "claude", Findings: nil},
	}
	report := Aggregate(results)
	assert.Equal(t, 1, report.TotalFiles)
	assert.Equal(t, 1, report.AnalyzedFiles)
	assert.Equal(t, 0, report.TotalFindings)
	assert.Equal(t, []string{"agent.md"}, report.FilesNoGuardrails)
	assert.Equal(t, []string{"agent.md"}, report.FilesNoSecurity)
}

func TestAggregate_MultipleFilesWithFindings(t *testing.T) {
	results := []FileResult{
		{
			Path: "a.md", Format: "claude",
			Findings: []llmutil.Finding{
				{Category: "guardrails", Issue: "no guardrails"},
				{Category: "security", Issue: "no security"},
			},
		},
		{
			Path: "b.md", Format: "copilot",
			Findings: []llmutil.Finding{
				{Category: "ordering", Issue: "bad ordering"},
			},
		},
	}
	report := Aggregate(results)
	assert.Equal(t, 2, report.TotalFiles)
	assert.Equal(t, 2, report.AnalyzedFiles)
	assert.Equal(t, 3, report.TotalFindings)
	assert.Equal(t, 1, report.ByCategory["guardrails"])
	assert.Equal(t, 1, report.ByCategory["security"])
	assert.Equal(t, 1, report.ByCategory["ordering"])

	// File b.md has no guardrails and no security findings
	assert.Contains(t, report.FilesNoGuardrails, "b.md")
	assert.Contains(t, report.FilesNoSecurity, "b.md")
	// File a.md has both
	assert.NotContains(t, report.FilesNoGuardrails, "a.md")
	assert.NotContains(t, report.FilesNoSecurity, "a.md")
}

func TestAggregate_FailedFiles(t *testing.T) {
	results := []FileResult{
		{Path: "good.md", Format: "claude", Findings: nil},
		{Path: "bad.md", Error: fmt.Errorf("parse failed")},
	}
	report := Aggregate(results)
	assert.Equal(t, 2, report.TotalFiles)
	assert.Equal(t, 1, report.AnalyzedFiles)
	assert.Equal(t, 1, report.FailedFiles)
}

func TestAggregate_CategoriesCount(t *testing.T) {
	results := []FileResult{
		{
			Path: "a.md", Format: "claude",
			Findings: []llmutil.Finding{
				{Category: "context"},
				{Category: "context"},
				{Category: "ordering"},
			},
		},
	}
	report := Aggregate(results)
	assert.Equal(t, 2, report.ByCategory["context"])
	assert.Equal(t, 1, report.ByCategory["ordering"])
}
