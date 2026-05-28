// Package analyzer implements the doctor's LLM-powered analysis skill.
package analyzer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
	"github.com/mirandaguillaume/reify/internal/llm"
)

// Analyze sends an agent file's parsed structure to the LLM for analysis.
// Returns structured findings with categories, confidence levels, and suggestions.
// If reg is nil, it falls back to the embedded default registry.
func Analyze(analysis *parser.AgentAnalysis, provider llm.Provider, reg *registry.Registry) ([]llmutil.Finding, error) {
	if analysis == nil {
		return nil, fmt.Errorf("nil analysis")
	}
	if provider == nil {
		return nil, fmt.Errorf("nil provider")
	}

	if reg == nil {
		var err error
		reg, err = registry.Load("")
		if err != nil {
			return nil, fmt.Errorf("load default registry: %w", err)
		}
	}

	prompt := buildPrompt(analysis, reg)

	response, err := provider.Complete(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	yamlStr, err := llmutil.ExtractYAML(response)
	if err != nil {
		return nil, fmt.Errorf("extract findings from LLM response: %w", err)
	}

	findings, err := llmutil.ParseFindings(yamlStr)
	if err != nil {
		return nil, fmt.Errorf("parse findings: %w", err)
	}

	// Enrich findings with citation_id from their category
	for i := range findings {
		if findings[i].CitationID == "" {
			findings[i].CitationID = findings[i].Category
		}
	}

	return findings, nil
}

// AnalyzeCrossFile sends multiple agent files to the LLM for cross-file consistency analysis.
// Returns findings about contradictions, persona conflicts, and permission mismatches.
func AnalyzeCrossFile(analyses []*parser.AgentAnalysis, filePaths []string, provider llm.Provider) ([]llmutil.Finding, error) {
	if len(analyses) < 2 || provider == nil {
		return nil, nil
	}
	if len(analyses) != len(filePaths) {
		return nil, fmt.Errorf("analyses and filePaths must have same length (%d vs %d)", len(analyses), len(filePaths))
	}

	prompt := buildCrossFilePrompt(analyses, filePaths)

	response, err := provider.Complete(prompt)
	if err != nil {
		return nil, fmt.Errorf("cross-file LLM analysis failed: %w", err)
	}

	yamlStr, err := llmutil.ExtractYAML(response)
	if err != nil {
		return nil, fmt.Errorf("extract cross-file findings: %w", err)
	}

	findings, err := llmutil.ParseFindings(yamlStr)
	if err != nil {
		return nil, fmt.Errorf("parse cross-file findings: %w", err)
	}

	// Mark all as cross-file
	for i := range findings {
		findings[i].Category = "consistency"
	}

	return findings, nil
}

func buildCrossFilePrompt(analyses []*parser.AgentAnalysis, filePaths []string) string {
	var b strings.Builder

	b.WriteString("Analyze these agent definition files for CROSS-FILE CONSISTENCY issues.\n\n")
	b.WriteString("Look for:\n")
	b.WriteString("- Contradictory instructions between files\n")
	b.WriteString("- Persona conflicts (different roles/identities)\n")
	b.WriteString("- Permission mismatches (one file allows what another prohibits)\n")
	b.WriteString("- Overlapping responsibilities (duplicate concerns across files)\n\n")

	// Include up to 5 files (in order provided)
	maxFiles := 5
	if len(analyses) < maxFiles {
		maxFiles = len(analyses)
	}

	for i := 0; i < maxFiles; i++ {
		b.WriteString(fmt.Sprintf("--- FILE %d: %s ---\n", i+1, filePaths[i]))
		b.Write(analyses[i].RawContent)
		b.WriteString("\n--- END FILE ---\n\n")
	}

	if len(analyses) > 5 {
		b.WriteString(fmt.Sprintf("(%d more files not shown)\n\n", len(analyses)-5))
	}

	b.WriteString(`IMPORTANT: Respond with ONLY valid YAML. No text before or after.

EXAMPLE OUTPUT:
findings:
  - category: consistency
    issue: "Contradictory permissions: file1 allows network access, file2 prohibits it"
    confidence: high
    current_state: "File 1 line 5: 'network access allowed' vs File 2 line 12: 'no network access'"
    suggested_improvement: "Align permissions across both files"

NOW ANALYZE THE FILES ABOVE. Output ONLY the YAML:
`)

	return b.String()
}

func buildPrompt(analysis *parser.AgentAnalysis, reg *registry.Registry) string {
	var b strings.Builder

	b.WriteString("Analyze this agent definition file for quality issues.\n\n")
	b.WriteString(fmt.Sprintf("File format: %s\n", analysis.Format))

	// Frontmatter summary
	if len(analysis.Frontmatter) > 0 {
		b.WriteString("Frontmatter fields: ")
		var fields []string
		for k := range analysis.Frontmatter {
			if !strings.HasPrefix(k, "_") { // Skip internal fields
				fields = append(fields, k)
			}
		}
		b.WriteString(strings.Join(fields, ", "))
		b.WriteString("\n")
	}

	// Tools
	if len(analysis.Tools) > 0 {
		b.WriteString(fmt.Sprintf("Tools declared: %s\n", strings.Join(analysis.Tools, ", ")))
	}

	// Section headers
	if len(analysis.Sections) > 0 {
		b.WriteString("Sections: ")
		var headers []string
		for _, s := range analysis.Sections {
			if s.Header != "" {
				headers = append(headers, s.Header)
			}
		}
		b.WriteString(strings.Join(headers, ", "))
		b.WriteString("\n")
	}

	// Full content (truncate to ~32K tokens to prevent OOM on very large files)
	b.WriteString("\nFull content:\n---\n")
	content := analysis.RawContent
	const maxPromptBytes = 128_000 // ~32K tokens at ~4 bytes/token
	if len(content) > maxPromptBytes {
		content = content[:maxPromptBytes]
		// Backtrack to the last complete UTF-8 rune boundary to avoid corruption.
		for len(content) > 0 && !utf8.Valid(content) {
			content = content[:len(content)-1]
		}
		b.Write(content)
		b.WriteString("\n[TRUNCATED — original file exceeds 32K token limit]\n")
	} else {
		b.Write(content)
	}
	b.WriteString("\n---\n\n")

	// Only include categories that require semantic LLM analysis.
	// Static checks (presence, vague, ordering, secrets, tokens, duplicates, drift)
	// already cover the rest — no point asking the LLM to re-check.
	llmCategories := []string{
		"decomposition", "context", "scope", "idempotency",
		"prompt_injection", "tool_usage", "examples", "error_handling",
	}

	var entries []registry.Recommendation
	for _, id := range llmCategories {
		if e, ok := reg.Get(id); ok {
			entries = append(entries, e)
		}
	}

	b.WriteString(fmt.Sprintf("TASK: Analyze this agent file for quality issues across %d categories.\n\n", len(entries)))
	b.WriteString("CATEGORIES:\n")
	for i, e := range entries {
		b.WriteString(fmt.Sprintf("%d. %s — %s (%s: %s)\n", i+1, e.ID, e.DetectionPrompt, e.Citation, e.Finding))
	}

	b.WriteString(`
Report ALL issues found. Be thorough — check every category.

IMPORTANT: For each finding, FIRST explain your reasoning, THEN state the issue and confidence.
Respond with ONLY valid YAML. No text before or after. No markdown fences. No explanations.

EXAMPLE OUTPUT:
findings:
  - category: guardrails
    reasoning: "The file has no section mentioning timeouts, output limits, or behavioral constraints. No heading matches guardrails, constraints, or limits."
    issue: "No timeout or output limits defined"
    confidence: high
    current_state: "No constraints section exists"
    suggested_improvement: "Add timeout: 5min and max output limits"
  - category: redundancy
    reasoning: "Lines 5-8 list React, TypeScript, and Tailwind CSS. This information is already present in package.json and can be inferred by the model."
    issue: "Instructions repeat the tech stack which is already in package.json"
    confidence: moderate
    current_state: "Lines 5-8 list React, TypeScript, Tailwind"
    suggested_improvement: "Remove tech stack listing — the model infers this from project files"

NOW ANALYZE THE FILE ABOVE. Output ONLY the YAML:
`)

	return b.String()
}
