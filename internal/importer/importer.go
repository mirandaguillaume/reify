package importer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/analyzer"
	"github.com/mirandaguillaume/reify/internal/classifier"
	"github.com/mirandaguillaume/reify/internal/linter"
	"github.com/mirandaguillaume/reify/internal/llm"
	"github.com/mirandaguillaume/reify/pkg/model"
	"gopkg.in/yaml.v3"
)

// ImportOptions configures the import pipeline.
type ImportOptions struct {
	Source    string       // File path, directory path, or "vercel:name"
	Provider llm.Provider // LLM provider for decomposition
	MinScore int          // Minimum quality score (0 = no minimum)
	OutputDir string      // Output directory for generated files
}

// SkillResult contains a single skill and its quality analysis.
type SkillResult struct {
	Skill      model.SkillBehavior
	RawYAML    string
	Score      analyzer.SkillScore
	LintIssues []linter.LintResult
	LoopRisks  []analyzer.LoopRisk
}

// AgentResult contains an agent composition and its quality analysis.
type AgentResult struct {
	Agent          model.AgentComposition
	RawYAML        string
	Score          analyzer.AgentScore
	DepIssues      []analyzer.DependencyIssue
	OrderingIssues []analyzer.OrderingIssue
}

// ImportResult is the output of the import pipeline.
type ImportResult struct {
	Success   bool
	Error     string
	Skills    []SkillResult
	Agent     *AgentResult
	Contracts map[string]string // produce name → output format template
	Warnings  []string
}

// llmResponse is the JSON structure returned by the LLM.
type llmResponse struct {
	Skills []struct {
		YAML string `json:"yaml"`
	} `json:"skills"`
	Agent *struct {
		YAML string `json:"yaml"`
	} `json:"agent"`
	Contracts map[string]string `json:"contracts"`
}

// RunImport executes the full import pipeline: resolve sources, extract
// frontmatter, reverse-map tools, build the LLM prompt, call the provider,
// parse and validate the response. If MinScore > 0 and issues are found it
// retries once.
func RunImport(opts ImportOptions) ImportResult {
	// 1. Resolve sources
	sources, err := ResolveSources(opts.Source)
	if err != nil {
		return ImportResult{Error: fmt.Sprintf("resolving sources: %v", err)}
	}
	if len(sources) == 0 {
		return ImportResult{Error: "no sources found"}
	}

	// Use the first source for single-file imports.
	source := sources[0]

	// 2. Extract frontmatter
	fm, body, err := ExtractFrontmatter(source.Content)
	if err != nil {
		return ImportResult{Error: fmt.Sprintf("extracting frontmatter: %v", err)}
	}

	// 3. Reverse map tools
	genericTools := ReverseMapTools(fm.Tools, source.Framework)

	// 4. Pre-classify instructions with LLM for accurate facet grounding.
	// Falls back to static classification silently if LLM fails.
	classification, _ := classifier.ClassifyLLM(body, source.Framework.String(), opts.Provider)

	// 5. Build prompt
	prompt := BuildImportPrompt(source, fm, body, genericTools, classification)

	// 6. Call LLM
	response, err := opts.Provider.Complete(prompt)
	if err != nil {
		return ImportResult{Error: fmt.Sprintf("LLM completion failed: %v", err)}
	}

	// 7. Parse response
	skills, agent, rawYAMLs, contracts, err := parseLLMResponse(response)
	if err != nil {
		return ImportResult{Error: fmt.Sprintf("parsing LLM response: %v", err)}
	}

	// Separate raw agent YAML from skill YAMLs.
	var rawAgentYAML string
	if agent != nil && len(rawYAMLs) > len(skills) {
		rawAgentYAML = rawYAMLs[len(rawYAMLs)-1]
		rawYAMLs = rawYAMLs[:len(rawYAMLs)-1]
	}

	// 7. Validate
	result := validateImport(skills, agent, rawYAMLs, rawAgentYAML)
	result.Contracts = contracts

	// 8. Retry if MinScore > 0 and feedback exists
	if opts.MinScore > 0 {
		feedback := collectFeedback(result, opts.MinScore)
		if len(feedback) > 0 {
			retryPrompt := BuildRetryPrompt(prompt, response, feedback)
			retryResponse, retryErr := opts.Provider.Complete(retryPrompt)
			if retryErr == nil {
				retrySkills, retryAgent, retryRawYAMLs, retryContracts, parseErr := parseLLMResponse(retryResponse)
				if parseErr == nil {
					var retryRawAgentYAML string
					if retryAgent != nil && len(retryRawYAMLs) > len(retrySkills) {
						retryRawAgentYAML = retryRawYAMLs[len(retryRawYAMLs)-1]
						retryRawYAMLs = retryRawYAMLs[:len(retryRawYAMLs)-1]
					}
					result = validateImport(retrySkills, retryAgent, retryRawYAMLs, retryRawAgentYAML)
					result.Contracts = retryContracts
				}
			}
		}
	}

	return result
}

// parseLLMResponse parses the JSON response from the LLM. It strips any
// markdown code fences, unmarshals the JSON, then parses each embedded
// YAML block into SkillBehavior and optionally AgentComposition.
// Returns skills, agent, all raw YAML strings (skills first, then agent if
// present), contracts map, and any error.
func parseLLMResponse(response string) ([]model.SkillBehavior, *model.AgentComposition, []string, map[string]string, error) {
	// Strip <think>...</think> blocks (Qwen3 and other chain-of-thought models).
	cleaned := stripThinkingBlocks(response)
	// Strip markdown fences.
	cleaned = stripMarkdownFences(cleaned)
	// Extract JSON object from response — local models often prepend prose.
	cleaned = extractJSON(cleaned)

	var resp llmResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("invalid JSON from LLM: %w", err)
	}

	var skills []model.SkillBehavior
	var rawYAMLs []string

	for i, s := range resp.Skills {
		var skill model.SkillBehavior
		if err := yaml.Unmarshal([]byte(s.YAML), &skill); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parsing skill YAML [%d]: %w", i, err)
		}
		skills = append(skills, skill)
		rawYAMLs = append(rawYAMLs, s.YAML)
	}

	var agent *model.AgentComposition
	if resp.Agent != nil {
		var a model.AgentComposition
		if err := yaml.Unmarshal([]byte(resp.Agent.YAML), &a); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("parsing agent YAML: %w", err)
		}
		agent = &a
		rawYAMLs = append(rawYAMLs, resp.Agent.YAML)
	}

	return skills, agent, rawYAMLs, resp.Contracts, nil
}

// extractJSON finds the outermost JSON object in s, tolerating prose
// that local models prepend or append before/after the JSON.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return s
	}
	end := strings.LastIndex(s, "}")
	if end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}

// stripThinkingBlocks removes <think>...</think> blocks emitted by
// chain-of-thought models (e.g. Qwen3) before the actual response.
func stripThinkingBlocks(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s, "</think>")
		if end == -1 {
			s = s[:start]
			break
		}
		s = s[:start] + s[end+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

// stripMarkdownFences removes markdown code fences from a string, handling
// both ```json and bare ``` markers.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	// Remove opening fence.
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// validateImport runs all validation, linting, scoring, and analysis on the
// parsed skills and optional agent. It assembles a complete ImportResult.
func validateImport(skills []model.SkillBehavior, agent *model.AgentComposition, rawSkillYAMLs []string, rawAgentYAML string) ImportResult {
	result := ImportResult{Success: true}

	var warnings []string

	// Validate and analyze each skill.
	for i, skill := range skills {
		var rawYAML string
		if i < len(rawSkillYAMLs) {
			rawYAML = rawSkillYAMLs[i]
		}

		// Model validation.
		if errs := model.ValidateSkill(skill); len(errs) > 0 {
			for _, e := range errs {
				warnings = append(warnings, fmt.Sprintf("skill %q: %s", skill.Skill, e))
			}
		}

		// Lint.
		lintIssues := linter.LintSkill(skill)

		// Loop risks.
		checker := &analyzer.DefaultGuardrailChecker{}
		loopRisks := analyzer.DetectLoopRisks(skill, checker)

		// Score.
		score := analyzer.ScoreSkill(skill)

		result.Skills = append(result.Skills, SkillResult{
			Skill:      skill,
			RawYAML:    rawYAML,
			Score:      score,
			LintIssues: lintIssues,
			LoopRisks:  loopRisks,
		})
	}

	// Validate agent if present.
	if agent != nil {
		if errs := model.ValidateAgent(*agent); len(errs) > 0 {
			for _, e := range errs {
				warnings = append(warnings, fmt.Sprintf("agent %q: %s", agent.Agent, e))
			}
		}

		depIssues := analyzer.CheckDependencies(*agent, skills)
		skillMap := make(map[string]model.SkillBehavior)
		for _, s := range skills {
			skillMap[s.Skill] = s
		}
		orderingIssues := analyzer.CheckSkillOrdering(*agent, skillMap)
		agentScore := analyzer.ScoreAgent(*agent, skills)

		result.Agent = &AgentResult{
			Agent:          *agent,
			RawYAML:        rawAgentYAML,
			Score:          agentScore,
			DepIssues:      depIssues,
			OrderingIssues: orderingIssues,
		}
	}

	result.Warnings = warnings
	return result
}

// collectFeedback gathers lint errors, loop risks, and low scores into
// feedback strings for a retry prompt.
func collectFeedback(result ImportResult, minScore int) []string {
	var feedback []string

	for _, sr := range result.Skills {
		for _, li := range sr.LintIssues {
			if li.Severity == linter.SeverityError {
				feedback = append(feedback, fmt.Sprintf("skill %q lint error [%s]: %s", sr.Skill.Skill, li.Rule, li.Message))
			}
		}
		for _, lr := range sr.LoopRisks {
			if lr.Severity == "error" {
				feedback = append(feedback, fmt.Sprintf("skill %q loop risk: %s", sr.Skill.Skill, lr.Message))
			}
		}
		if sr.Score.Total < minScore {
			feedback = append(feedback, fmt.Sprintf("skill %q scored %d/%d (minimum: %d)", sr.Skill.Skill, sr.Score.Total, 100, minScore))
		}
	}

	if result.Agent != nil {
		for _, di := range result.Agent.DepIssues {
			feedback = append(feedback, fmt.Sprintf("agent dependency issue: %s", di.Message))
		}
		if result.Agent.Score.Total < minScore {
			feedback = append(feedback, fmt.Sprintf("agent %q scored %d/%d (minimum: %d)", result.Agent.Agent.Agent, result.Agent.Score.Total, 100, minScore))
		}
	}

	feedback = append(feedback, result.Warnings...)

	return feedback
}
