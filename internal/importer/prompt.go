package importer

import (
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/classifier"
)

// BuildImportPrompt constructs the LLM prompt for agent-to-skill decomposition.
// classification is optional: a zero-value Result skips the pre-classification section.
func BuildImportPrompt(source Source, fm AgentFrontmatter, body string, genericTools []string, classification classifier.Result) string {
	var b strings.Builder

	b.WriteString("You are converting an agent definition into Reify skill YAML specs.\n\n")

	// Schema reference
	b.WriteString("## Reify skill YAML schema\n\n")
	b.WriteString("Each skill MUST use YAML block style (indentation). Do NOT use flow style (curly braces {}). Example:\n\n")
	b.WriteString("skill: example-skill\n")
	b.WriteString("version: \"0.1.0\"\n")
	b.WriteString("context:\n")
	b.WriteString("  consumes: []\n")
	b.WriteString("  produces: [result]\n")
	b.WriteString("  memory: short-term\n")
	b.WriteString("strategy:\n")
	b.WriteString("  tools: [bash]\n")
	b.WriteString("  approach: Do one focused thing\n")
	b.WriteString("  steps:\n")
	b.WriteString("    - Step one\n")
	b.WriteString("    - Step two\n")
	b.WriteString("guardrails:\n")
	b.WriteString("  - \"never modify files outside the working directory\"\n")
	b.WriteString("  - timeout: 30s\n")
	b.WriteString("# IMPORTANT: guardrails MUST be a YAML list (each item starts with '-'). Never use a map.\n")
	b.WriteString("observability:\n")
	b.WriteString("  trace_level: minimal\n")
	b.WriteString("  metrics: []\n")
	b.WriteString("security:\n")
	b.WriteString("  filesystem: none\n")
	b.WriteString("  network: none\n")
	b.WriteString("  secrets: []\n")
	b.WriteString("negotiation:\n")
	b.WriteString("  file_conflicts: yield\n")
	b.WriteString("  priority: 0\n\n")
	b.WriteString("ALL fields are REQUIRED. Use these defaults when the source does not specify:\n")
	b.WriteString("  trace_level: minimal  |  filesystem: none  |  network: none  |  file_conflicts: yield  |  priority: 0\n")
	b.WriteString("memory: short-term|conversation|long-term — trace_level: minimal|standard|detailed — filesystem: none|read-only|read-write|full — network: none|allowlist|full\n\n")

	// Tool names
	b.WriteString("## Available generic tool names\n")
	b.WriteString("read_file, write_file, edit_file, grep, search, bash, web_fetch, web_search, todo, task\n\n")

	// Agent composition schema
	b.WriteString("## Agent composition schema (optional, only if input has multiple responsibilities)\n")
	b.WriteString("- agent: string (kebab-case name)\n")
	b.WriteString("- skills: [string] (skill names)\n")
	b.WriteString("- orchestration: sequential|parallel|parallel-then-merge|adaptive\n")
	b.WriteString("- description: string\n")
	b.WriteString("- consumes: [string]\n")
	b.WriteString("- produces: [string]\n\n")

	// Input
	b.WriteString("## Input to analyze\n\n")
	if fm.Name != "" {
		b.WriteString(fmt.Sprintf("Source file: %s\n", source.Name))
		b.WriteString(fmt.Sprintf("Name: %s\n", fm.Name))
		b.WriteString(fmt.Sprintf("Description: %s\n", fm.Description))
		if len(genericTools) > 0 {
			b.WriteString(fmt.Sprintf("Tools (generic): %s\n", strings.Join(genericTools, ", ")))
		}
		b.WriteString("\n")
	}
	b.WriteString("### Full agent definition\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")

	// Instructions
	// Pre-classification hint: guide the LLM with facet assignments already done.
	if len(classification.Items) > 0 {
		b.WriteString("## Pre-classified instructions\n\n")
		b.WriteString("Each instruction below has already been mapped to a Reify facet.\n")
		b.WriteString("Use these assignments when generating skill YAML fields.\n\n")
		byFacet := classification.ByFacet()
		for _, facet := range classifier.AllFacets {
			items := byFacet[facet]
			if len(items) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("%s:\n", strings.ToUpper(string(facet))))
			for _, item := range items {
				b.WriteString(fmt.Sprintf("  - %s\n", item.Text))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Analyze this agent definition and identify distinct responsibilities.\n")
	b.WriteString("2. Create one Reify skill YAML per responsibility.\n")
	b.WriteString("   Skill decomposition rules (CRITICAL — the build will REJECT skills that violate these):\n")
	b.WriteString("   - Each skill must have exactly ONE output in 'produces'\n")
	b.WriteString("   - Each step must describe ONE action (no 'and', 'then', or '&' joining different actions)\n")
	b.WriteString("   - The 'approach' field must describe a single focused activity (no conjunctions)\n")
	b.WriteString("   - If steps span different phases (e.g., input parsing, searching, analyzing),\n")
	b.WriteString("     split into separate skills connected by data flow (one skill's 'produces' feeds the next's 'consumes')\n")
	b.WriteString("   - Maximum 5 steps per skill — if you need more, the skill is too broad\n")
	b.WriteString("   - All skill fields (approach, steps, guardrails) MUST be written in English\n")
	b.WriteString("3. If the agent has multiple responsibilities, also create an agent composition.\n")
	b.WriteString("4. If the agent has a single responsibility, create only one skill (no agent).\n")
	b.WriteString("5. Set security to the minimum required permissions.\n")
	b.WriteString("6. Add meaningful guardrails (especially timeout for long-running tasks).\n")
	b.WriteString("7. Extract output format templates: if the agent defines a structured output format\n")
	b.WriteString("   (e.g., review comment format, risk score format, report structure), extract each\n")
	b.WriteString("   as a contract keyed by the matching 'produces' name. Only extract when the agent\n")
	b.WriteString("   defines an explicit, reusable output format — not for generic outputs.\n\n")

	// Output format
	b.WriteString("## Output format\n\n")
	b.WriteString("IMPORTANT: Your entire response must be valid JSON. Start with { and end with }.\n")
	b.WriteString("Do NOT include any text, explanation, or markdown before or after the JSON.\n\n")
	b.WriteString(`{"skills": [{"yaml": "skill: name\nversion: ..."}], "agent": {"yaml": "agent: name\n..."} or null, "contracts": {"produce_name": "Output format template markdown..."} or null}`)
	b.WriteString("\n")

	return b.String()
}

// BuildRetryPrompt appends validation feedback to the original prompt for a retry.
func BuildRetryPrompt(originalPrompt, originalResponse string, feedback []string) string {
	var b strings.Builder

	b.WriteString(originalPrompt)
	b.WriteString("\n\n## Previous attempt\n\n")
	b.WriteString("Your previous response:\n")
	b.WriteString(originalResponse)
	b.WriteString("\n\n## Validation feedback — Fix these issues\n\n")
	for _, f := range feedback {
		b.WriteString("- " + f + "\n")
	}
	b.WriteString("\nReturn the corrected JSON.\n")

	return b.String()
}
