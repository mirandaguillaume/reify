package copilot_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/copilot"
	"github.com/stretchr/testify/assert"
)

func TestMapToolsToCopilot_KnownTools(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"read_file", "write_file", "bash"})
	assert.Equal(t, []string{"read", "edit", "execute"}, result)
}

func TestMapToolsToCopilot_CaseInsensitive(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"READ", "BASH", "Grep"})
	assert.Equal(t, []string{"read", "execute", "search"}, result)
}

func TestMapToolsToCopilot_Deduplicates(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"read_file", "read"})
	assert.Equal(t, []string{"read"}, result)
}

func TestMapToolsToCopilot_IgnoresUnknown(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"read_file", "unknown_tool", "bash"})
	assert.Equal(t, []string{"read", "execute"}, result)
}

func TestMapToolsToCopilot_EmptyInput(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{})
	assert.Nil(t, result)
}

func TestMapToolsToCopilot_SearchAliases(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"grep", "search", "find", "glob"})
	assert.Equal(t, []string{"search"}, result)
}

func TestMapToolsToCopilot_WebAliases(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"web_fetch", "http", "fetch", "web_search"})
	assert.Equal(t, []string{"web"}, result)
}

func TestMapToolsToCopilot_AgentAliases(t *testing.T) {
	result := copilot.MapToolsToCopilot([]string{"task", "delegate"})
	assert.Equal(t, []string{"agent"}, result)
}

func TestInferCopilotToolsFromSecurity_ReadOnly(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("read-only", "none")
	assert.Equal(t, []string{"read", "search"}, tools)
}

func TestInferCopilotToolsFromSecurity_ReadWrite(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("read-write", "none")
	assert.Equal(t, []string{"read", "search", "edit"}, tools)
}

func TestInferCopilotToolsFromSecurity_Full(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("full", "full")
	assert.Equal(t, []string{"read", "search", "edit", "execute", "web"}, tools)
}

func TestInferCopilotToolsFromSecurity_NetworkAllowlist(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("none", "allowlist")
	assert.Equal(t, []string{"web"}, tools)
}

func TestInferCopilotToolsFromSecurity_NetworkFull(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("none", "full")
	assert.Equal(t, []string{"web"}, tools)
}

func TestInferCopilotToolsFromSecurity_None(t *testing.T) {
	tools := copilot.InferCopilotToolsFromSecurity("none", "none")
	assert.Nil(t, tools)
}

func TestMergeCopilotToolLists_DeduplicatesAndOrders(t *testing.T) {
	result := copilot.MergeCopilotToolLists(
		[]string{"execute", "read"},
		[]string{"read", "edit", "search"},
	)
	assert.Equal(t, []string{"read", "edit", "search", "execute"}, result)
}

func TestMergeCopilotToolLists_Empty(t *testing.T) {
	result := copilot.MergeCopilotToolLists()
	assert.Nil(t, result)
}

func TestMergeCopilotToolLists_SingleList(t *testing.T) {
	result := copilot.MergeCopilotToolLists([]string{"edit", "read"})
	assert.Equal(t, []string{"read", "edit"}, result)
}

func TestMergeCopilotToolLists_AllCanonical(t *testing.T) {
	result := copilot.MergeCopilotToolLists(
		[]string{"todo", "agent", "web", "execute", "search", "edit", "read"},
	)
	assert.Equal(t, []string{"read", "edit", "search", "execute", "web", "agent", "todo"}, result)
}
