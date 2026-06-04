package generator_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMCPServers_StandardSchema(t *testing.T) {
	out := generator.RenderMCPServers(map[string]model.MCPServer{
		"git": {Command: "npx", Args: []string{"-y", "x"}, Env: map[string]string{"T": "1"}},
	})
	var doc struct {
		MCPServers map[string]model.MCPServer `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Equal(t, "npx", doc.MCPServers["git"].Command)
}

func TestRenderHooksProse_NamesSystemAndKeepsCommands(t *testing.T) {
	out := generator.RenderHooksProse([]model.Hook{
		{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."},
	}, "Cursor")
	assert.Contains(t, out, "Cursor has no hook system")
	assert.Contains(t, out, "not enforced")
	assert.Contains(t, out, "On PostToolUse")
	assert.Contains(t, out, "gofmt -w .")
	assert.Contains(t, out, "Edit")
	// Strong formulation (reify-eval finding: imperative phrasing is obeyed
	// more than soft phrasing). The degraded prose must be imperative.
	assert.Contains(t, out, "Always")
	assert.Contains(t, out, "must")
	assert.Contains(t, out, "do not skip it")
}

func TestRenderNativeSkillBody_TitleDescBody(t *testing.T) {
	out := generator.RenderNativeSkillBody(model.NativeSkill{
		Name: "scaffold", Description: "scaffold a thing", Body: "## Steps\n1. do it",
	})
	assert.True(t, strings.HasPrefix(out, "# scaffold"))
	assert.Contains(t, out, "scaffold a thing")
	assert.Contains(t, out, "## Steps")
}
