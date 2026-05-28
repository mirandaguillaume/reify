package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToJSONSchema_IncludesMinLength(t *testing.T) {
	s := FieldSchema{
		"rationale": {Type: "string", MinLength: 20},
	}
	js := ToJSONSchema(s)
	props, ok := js["properties"].(map[string]any)
	require.True(t, ok)
	prop, ok := props["rationale"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 20, prop["minLength"], "minLength must be included in JSON Schema property")
}

func TestToJSONSchema_NoMinLength_OmitsProperty(t *testing.T) {
	s := FieldSchema{
		"ruling": {Type: "string", Enum: []string{"approved", "rejected"}},
	}
	js := ToJSONSchema(s)
	props := js["properties"].(map[string]any)
	prop := props["ruling"].(map[string]any)
	_, hasMinLength := prop["minLength"]
	assert.False(t, hasMinLength, "minLength must be absent when not declared")
}

func TestToJSONSchema_AllFieldsRequired(t *testing.T) {
	s := FieldSchema{
		"a": {Type: "string"},
		"b": {Type: "number"},
	}
	js := ToJSONSchema(s)
	required, ok := js["required"].([]string)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"a", "b"}, required)
}

func TestToPromptInstruction_IncludesMinLength(t *testing.T) {
	s := FieldSchema{
		"rationale": {Type: "string", MinLength: 20},
	}
	instr := ToPromptInstruction(s)
	assert.Contains(t, instr, "min 20 chars")
}
