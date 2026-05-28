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
}

func TestAggregate_SingleFileNoFindings(t *testing.T) {
	results := []FileResult{
		{Path: "agent.md", Format: "claude", Findings: nil},
	}
	report := Aggregate(results)
	assert.Equal(t, 1, report.TotalFiles)
	assert.Equal(t, 1, report.AnalyzedFiles)
	assert.Equal(t, 0, report.TotalFindings)
}

func TestAggregate_MultipleFilesWithFindings(t *testing.T) {
	results := []FileResult{
		{
			Path: "a.md", Format: "claude",
			Findings: []llmutil.Finding{
				{Category: "context", Issue: "missing context"},
				{Category: "prompt_injection", Issue: "vague directive"},
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
	assert.Equal(t, 1, report.ByCategory["context"])
	assert.Equal(t, 1, report.ByCategory["prompt_injection"])
	assert.Equal(t, 1, report.ByCategory["ordering"])
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
