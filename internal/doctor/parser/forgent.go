package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/linter"
	yamlloader "github.com/mirandaguillaume/reify/internal/yaml"
	"gopkg.in/yaml.v3"
)

type reifyParser struct{}

func init() {
	Register("reify", func() FormatParser { return &reifyParser{} })
}

func (p *reifyParser) Format() string { return "reify" }

// Detect returns true if the file is a Reify skill or agent YAML definition.
func (p *reifyParser) Detect(path string, _ []byte) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".skill.yaml") || strings.HasSuffix(base, ".agent.yaml")
}

// Parse extracts structure from a Reify skill or agent YAML file.
// For skill files, runs lint rules and includes findings as metadata.
func (p *reifyParser) Parse(content []byte) (*AgentAnalysis, error) {
	s := string(content)

	// Try skill first, then agent
	skill, skillErr := yamlloader.ParseSkillYAML(s)
	if skillErr == nil {
		analysis := buildReifyAnalysis("skill", content)
		// Run lint diagnostics — subsumes old reify doctor
		lintResults := linter.LintSkill(skill)
		if len(lintResults) > 0 {
			analysis.Frontmatter["_lint_results"] = formatLintResults(lintResults)
			analysis.Frontmatter["_lint_count"] = len(lintResults)
		}
		return analysis, nil
	}

	_, agentErr := yamlloader.ParseAgentYAML(s)
	if agentErr == nil {
		return buildReifyAnalysis("agent", content), nil
	}

	return nil, fmt.Errorf("not a valid Reify YAML: skill: %v; agent: %v", skillErr, agentErr)
}

// Validate for Reify YAML: re-parse and validate.
func (p *reifyParser) Validate(_, rewritten []byte) error {
	s := string(rewritten)
	if _, err := yamlloader.ParseSkillYAML(s); err == nil {
		return nil
	}
	if _, err := yamlloader.ParseAgentYAML(s); err == nil {
		return nil
	}
	return fmt.Errorf("rewritten file is not valid Reify YAML")
}

func formatLintResults(results []linter.LintResult) []string {
	var msgs []string
	for _, r := range results {
		msgs = append(msgs, fmt.Sprintf("[%s] %s: %s", r.Severity, r.Rule, r.Message))
	}
	return msgs
}

func buildReifyAnalysis(kind string, content []byte) *AgentAnalysis {
	var fm map[string]interface{}
	_ = yaml.Unmarshal(content, &fm)
	if fm == nil {
		fm = make(map[string]interface{})
	}

	var tools []string
	if strategy, ok := fm["strategy"].(map[string]interface{}); ok {
		if t, ok := strategy["tools"].([]interface{}); ok {
			for _, item := range t {
				if s, ok := item.(string); ok {
					tools = append(tools, s)
				}
			}
		}
	}

	// Map YAML keys to sections
	var sections []Section
	facets := []string{"context", "strategy", "guardrails", "observability", "security", "negotiation"}
	if kind == "agent" {
		facets = []string{"skills", "stages", "orchestration", "consumes", "produces"}
	}
	for _, facet := range facets {
		if val, ok := fm[facet]; ok {
			sections = append(sections, Section{
				Header:  facet,
				Content: fmt.Sprintf("%v", val),
				Level:   1,
			})
		}
	}

	return &AgentAnalysis{
		Format:      "reify",
		Frontmatter: fm,
		Sections:    sections,
		Tools:       tools,
		RawContent:  content,
	}
}
