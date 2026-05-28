package claude_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/claude"
	"github.com/stretchr/testify/assert"
)

func TestMapToolsToClaude_KnownTools(t *testing.T) {
	result := claude.MapToolsToClaude([]string{"read_file", "write_file", "bash"})
	assert.Equal(t, []string{"Read", "Write", "Bash"}, result)
}

func TestMapToolsToClaude_CaseInsensitive(t *testing.T) {
	result := claude.MapToolsToClaude([]string{"READ", "BASH", "Grep"})
	assert.Equal(t, []string{"Read", "Bash", "Grep"}, result)
}

func TestMapToolsToClaude_Deduplicates(t *testing.T) {
	result := claude.MapToolsToClaude([]string{"read_file", "read"})
	assert.Equal(t, []string{"Read"}, result)
}

func TestMapToolsToClaude_IgnoresUnknown(t *testing.T) {
	result := claude.MapToolsToClaude([]string{"read_file", "unknown_tool", "bash"})
	assert.Equal(t, []string{"Read", "Bash"}, result)
}

func TestMapToolsToClaude_EmptyInput(t *testing.T) {
	result := claude.MapToolsToClaude([]string{})
	assert.Nil(t, result)
}

func TestInferToolsFromSecurity_ReadOnly(t *testing.T) {
	tools := claude.InferToolsFromSecurity("read-only", "none")
	assert.Equal(t, []string{"Read", "Glob", "Grep"}, tools)
}

func TestInferToolsFromSecurity_ReadWrite(t *testing.T) {
	tools := claude.InferToolsFromSecurity("read-write", "none")
	assert.Equal(t, []string{"Read", "Glob", "Grep", "Write", "Edit"}, tools)
}

func TestInferToolsFromSecurity_Full(t *testing.T) {
	tools := claude.InferToolsFromSecurity("full", "full")
	assert.Equal(t, []string{"Read", "Glob", "Grep", "Write", "Edit", "Bash", "WebFetch", "WebSearch"}, tools)
}

func TestInferToolsFromSecurity_NetworkAllowlist(t *testing.T) {
	tools := claude.InferToolsFromSecurity("none", "allowlist")
	assert.Equal(t, []string{"WebFetch"}, tools)
}

func TestInferToolsFromSecurity_NetworkFull(t *testing.T) {
	tools := claude.InferToolsFromSecurity("none", "full")
	assert.Equal(t, []string{"WebFetch", "WebSearch"}, tools)
}

func TestInferToolsFromSecurity_None(t *testing.T) {
	tools := claude.InferToolsFromSecurity("none", "none")
	assert.Nil(t, tools)
}

func TestMergeToolLists_DeduplicatesAndOrders(t *testing.T) {
	result := claude.MergeToolLists(
		[]string{"Bash", "Read"},
		[]string{"Read", "Write", "Grep"},
	)
	assert.Equal(t, []string{"Grep", "Read", "Write", "Bash"}, result)
}

func TestMergeToolLists_Empty(t *testing.T) {
	result := claude.MergeToolLists()
	assert.Nil(t, result)
}

func TestMergeToolLists_SingleList(t *testing.T) {
	result := claude.MergeToolLists([]string{"Write", "Read"})
	assert.Equal(t, []string{"Read", "Write"}, result)
}
