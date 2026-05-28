// Package schema defines typed field contracts for inter-node slot validation.
// FieldSchema / FieldSpec are kept in a zero-dependency package so both
// pkg/qualitygate and internal/bench/experiment can import them without cycles.
package schema

// FieldSchema maps field name → FieldSpec contract.
// A nil/empty FieldSchema means no typed constraint (pass-through).
type FieldSchema map[string]FieldSpec

// FieldSpec declares the type contract for a single output field.
type FieldSpec struct {
	Type        string   `yaml:"type"`                // "string" | "number" | "boolean"
	Enum        []string `yaml:"enum,omitempty"`      // value must be one of these when non-empty
	MinLength   int      `yaml:"min_length,omitempty"` // for strings: minimum length
	Description string   `yaml:"description,omitempty"` // shown to LLM in format instruction
}
