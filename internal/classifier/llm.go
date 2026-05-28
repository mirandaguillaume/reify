package classifier

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mirandaguillaume/reify/internal/llm"
)

// ClassifyLLM extracts instructions syntactically then classifies them via
// an LLM. Item extraction is deterministic (bullets, numbered lists);
// facet assignment is the semantic problem that requires the LLM. If the
// LLM response cannot be parsed, the error is returned — no static fallback.
func ClassifyLLM(content, format string, provider llm.Provider) (Result, error) {
	items := extractItems(content)
	if len(items) == 0 {
		return Result{Format: format}, nil
	}

	prompt := buildClassifyPrompt(items)
	response, err := provider.Complete(prompt)
	if err != nil {
		return Result{}, fmt.Errorf("LLM classification failed: %w", err)
	}

	classified, err := parseClassifyResponse(response, items)
	if err != nil {
		return Result{}, fmt.Errorf("parse LLM classification response: %w", err)
	}

	return Result{Format: format, Items: classified}, nil
}

func buildClassifyPrompt(items []Item) string {
	var b strings.Builder

	b.WriteString("Classify each instruction into exactly one Reify facet.\n\n")
	b.WriteString("Facets:\n")
	b.WriteString("- context: background info, project description, tech stack, architecture, conventions\n")
	b.WriteString("- strategy: tools to use, commands to run, workflows, how to approach tasks, package managers, build steps\n")
	b.WriteString("- guardrails: prohibitions and restrictions — things the agent must NOT do (never, don't, avoid, must not)\n")
	b.WriteString("- observability: logging, metrics, monitoring, tracing, reporting\n")
	b.WriteString("- security: permissions, access control, credentials, filesystem/network rules, secrets\n\n")

	b.WriteString("Instructions to classify:\n")
	for i, item := range items {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Text))
	}

	b.WriteString("\nIMPORTANT: Your entire response must be valid JSON. Start with [ and end with ].\n")
	b.WriteString("Do NOT include any text or explanation before or after the JSON.\n\n")
	b.WriteString(`[{"i": 1, "facet": "context"}, {"i": 2, "facet": "strategy"}, ...]`)
	b.WriteString("\n")

	return b.String()
}

type llmClassifyItem struct {
	I     int    `json:"i"`
	Facet string `json:"facet"`
}

func parseClassifyResponse(response string, items []Item) ([]Item, error) {
	cleaned := extractJSONArray(stripThinkBlocks(response))

	var llmItems []llmClassifyItem
	if err := json.Unmarshal([]byte(cleaned), &llmItems); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Build index map from LLM response.
	facetByIndex := make(map[int]Facet, len(llmItems))
	for _, li := range llmItems {
		f := normalizeFacet(li.Facet)
		facetByIndex[li.I] = f
	}

	result := make([]Item, len(items))
	for i, item := range items {
		result[i] = item
		if f, ok := facetByIndex[i+1]; ok {
			result[i].Facet = f
		} else {
			// LLM omitted this index — default to context (background) rather
			// than leaving Facet empty. Better to surface "unclassified ≈ context"
			// than to render a blank cell in tables.
			result[i].Facet = FacetContext
		}
	}
	return result, nil
}

// normalizeFacet maps LLM output to a known Facet, defaulting to context.
func normalizeFacet(s string) Facet {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strategy":
		return FacetStrategy
	case "guardrails", "guardrail":
		return FacetGuardrails
	case "observability", "observ":
		return FacetObservability
	case "security":
		return FacetSecurity
	default:
		return FacetContext
	}
}

func stripThinkBlocks(s string) string {
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

func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start == -1 {
		return s
	}
	end := strings.LastIndex(s, "]")
	if end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}
