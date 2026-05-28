package schema

import (
	"fmt"
	"sort"
	"strings"
)

// ToJSONSchema converts a FieldSchema to a minimal JSON Schema object
// for constrained-decoding APIs (e.g. Anthropic Structured Outputs).
// Keys are sorted so the output is deterministic.
func ToJSONSchema(s FieldSchema) map[string]any {
	properties := make(map[string]any, len(s))
	required := make([]string, 0, len(s))

	keys := sortedKeys(s)
	for _, name := range keys {
		spec := s[name]
		prop := map[string]any{"type": spec.Type}
		if len(spec.Enum) > 0 {
			enum := make([]any, len(spec.Enum))
			for i, v := range spec.Enum {
				enum[i] = v
			}
			prop["enum"] = enum
		}
		if spec.MinLength > 0 {
			prop["minLength"] = spec.MinLength
		}
		if spec.Description != "" {
			prop["description"] = spec.Description
		}
		properties[name] = prop
		required = append(required, name)
	}

	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// ToPromptInstruction returns a natural-language format instruction for providers
// without constrained-decoding support. The instruction is appended to the prompt
// so it is the LLM's last seen constraint (highest compliance rate empirically).
//
// Example output:
//
//	Output ONLY a valid JSON object with the following fields:
//	- "ruling" (string, one of: approved, rejected)
//	- "rationale" (string, min 20 chars — Explanation of the decision)
//	Do not wrap in markdown code fences.
func ToPromptInstruction(s FieldSchema) string {
	var b strings.Builder
	b.WriteString("Output ONLY a valid JSON object with the following fields:\n")
	for _, name := range sortedKeys(s) {
		spec := s[name]
		var constraints []string
		if len(spec.Enum) > 0 {
			constraints = append(constraints, "one of: "+strings.Join(spec.Enum, ", "))
		}
		if spec.MinLength > 0 {
			constraints = append(constraints, fmt.Sprintf("min %d chars", spec.MinLength))
		}
		line := fmt.Sprintf("- %q (%s", name, spec.Type)
		if len(constraints) > 0 {
			line += ", " + strings.Join(constraints, ", ")
		}
		if spec.Description != "" {
			line += " — " + spec.Description
		}
		line += ")"
		b.WriteString(line + "\n")
	}
	b.WriteString("Do not wrap in markdown code fences.")
	return b.String()
}

func sortedKeys(s FieldSchema) []string {
	keys := make([]string, 0, len(s))
	for name := range s {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}
