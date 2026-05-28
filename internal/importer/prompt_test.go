package importer

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/stretchr/testify/assert"
)

func TestBuildImportPrompt(t *testing.T) {
	source := Source{
		Name:    "review.md",
		Content: "# Review Agent\nReviews PRs",
	}
	fm := AgentFrontmatter{
		Name:        "review",
		Description: "Reviews PRs",
		Tools:       []string{"Read", "Grep"},
	}

	prompt := BuildImportPrompt(source, fm, "# Review Agent\nReviews PRs", []string{"read_file", "grep"}, classifier.Result{})

	assert.Contains(t, prompt, "Reify skill YAML schema")
	assert.Contains(t, prompt, "review")
	assert.Contains(t, prompt, "Reviews PRs")
	assert.Contains(t, prompt, `"skills"`)
	assert.Contains(t, prompt, "consumes")
	assert.Contains(t, prompt, "produces")
	assert.Contains(t, prompt, "read_file, grep")
}

func TestBuildImportPrompt_NoFrontmatter(t *testing.T) {
	source := Source{Name: "agent.md"}
	fm := AgentFrontmatter{}

	prompt := BuildImportPrompt(source, fm, "# My Agent", nil, classifier.Result{})

	assert.Contains(t, prompt, "# My Agent")
	assert.NotContains(t, prompt, "Source file:")
}

func TestBuildImportPrompt_ContractExtraction(t *testing.T) {
	source := Source{Name: "agent.md", Content: "# Agent"}
	fm := AgentFrontmatter{Name: "test"}

	prompt := BuildImportPrompt(source, fm, "# Agent", nil, classifier.Result{})

	assert.Contains(t, prompt, "output format templates")
	assert.Contains(t, prompt, "contracts")
	assert.Contains(t, prompt, "produce_name")
}

func TestBuildImportPrompt_SRPGuidance(t *testing.T) {
	source := Source{Name: "agent.md", Content: "# Agent"}
	fm := AgentFrontmatter{Name: "test"}

	prompt := BuildImportPrompt(source, fm, "# Agent", nil, classifier.Result{})

	assert.Contains(t, prompt, "exactly ONE output")
	assert.Contains(t, prompt, "ONE action")
	assert.Contains(t, prompt, "Maximum 5 steps")
	assert.Contains(t, prompt, "REJECT skills")
}

func TestBuildImportPrompt_WithClassification(t *testing.T) {
	source := Source{Name: "agent.md", Content: "# Agent"}
	fm := AgentFrontmatter{Name: "test"}
	cl := classifier.Result{
		Items: []classifier.Item{
			{Text: "Never expose secrets", Facet: classifier.FacetGuardrails, Section: "Rules"},
			{Text: "Use bash for commands", Facet: classifier.FacetStrategy, Section: "Commands"},
		},
	}

	prompt := BuildImportPrompt(source, fm, "# Agent", nil, cl)

	assert.Contains(t, prompt, "Pre-classified instructions")
	assert.Contains(t, prompt, "GUARDRAILS")
	assert.Contains(t, prompt, "Never expose secrets")
	assert.Contains(t, prompt, "STRATEGY")
	assert.Contains(t, prompt, "Use bash for commands")
}

func TestBuildRetryPrompt(t *testing.T) {
	feedback := []string{
		"skill 'review': missing guardrails.timeout",
		"skill 'review': score 45/100 (below 60 threshold)",
	}
	prompt := BuildRetryPrompt("original prompt", "original response", feedback)

	assert.Contains(t, prompt, "original prompt")
	assert.Contains(t, prompt, "original response")
	assert.Contains(t, prompt, "missing guardrails.timeout")
	assert.Contains(t, prompt, "score 45/100")
	assert.Contains(t, prompt, "Fix these issues")
}
