package qualitygate

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTyped_AllValid(t *testing.T) {
	s := schema.FieldSchema{
		"ruling":    {Type: "string", Enum: []string{"approved", "rejected"}},
		"score":     {Type: "number"},
		"compliant": {Type: "boolean"},
	}
	data := map[string]any{
		"ruling":    "approved",
		"score":     float64(87),
		"compliant": true,
	}
	assert.NoError(t, ValidateTyped(data, s))
}

func TestValidateTyped_EmptySchema_NoConstraint(t *testing.T) {
	assert.NoError(t, ValidateTyped(map[string]any{"x": "anything"}, schema.FieldSchema{}))
	assert.NoError(t, ValidateTyped(map[string]any{}, nil))
}

func TestValidateTyped_MissingField(t *testing.T) {
	s := schema.FieldSchema{"ruling": {Type: "string"}}
	err := ValidateTyped(map[string]any{}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "typed schema: field")
	assert.Contains(t, err.Error(), "ruling")
	assert.Contains(t, err.Error(), "missing from output")
}

func TestValidateTyped_WrongType_String(t *testing.T) {
	s := schema.FieldSchema{"ruling": {Type: "string"}}
	err := ValidateTyped(map[string]any{"ruling": float64(42)}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ruling")
	assert.Contains(t, err.Error(), "expected string")
}

func TestValidateTyped_WrongType_Number(t *testing.T) {
	s := schema.FieldSchema{"score": {Type: "number"}}
	err := ValidateTyped(map[string]any{"score": "high"}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "score")
	assert.Contains(t, err.Error(), "expected number")
}

func TestValidateTyped_WrongType_Boolean(t *testing.T) {
	s := schema.FieldSchema{"ok": {Type: "boolean"}}
	err := ValidateTyped(map[string]any{"ok": "true"}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ok")
	assert.Contains(t, err.Error(), "expected boolean")
}

func TestValidateTyped_EnumViolation(t *testing.T) {
	s := schema.FieldSchema{"ruling": {Type: "string", Enum: []string{"approved", "rejected"}}}
	err := ValidateTyped(map[string]any{"ruling": "pending"}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "typed schema: field")
	assert.Contains(t, err.Error(), "ruling")
	assert.Contains(t, err.Error(), "not in enum")
}

func TestValidateTyped_MinLengthViolation(t *testing.T) {
	s := schema.FieldSchema{"rationale": {Type: "string", MinLength: 20}}
	err := ValidateTyped(map[string]any{"rationale": "short"}, s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rationale")
	assert.Contains(t, err.Error(), "below minimum")
}

func TestValidateTyped_MinLength_Satisfied(t *testing.T) {
	s := schema.FieldSchema{"rationale": {Type: "string", MinLength: 5}}
	assert.NoError(t, ValidateTyped(map[string]any{"rationale": "long enough"}, s))
}

func TestValidateTyped_EnumSatisfied(t *testing.T) {
	s := schema.FieldSchema{"ruling": {Type: "string", Enum: []string{"approved", "rejected"}}}
	assert.NoError(t, ValidateTyped(map[string]any{"ruling": "rejected"}, s))
}
