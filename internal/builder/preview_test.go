package builder_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/builder"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewProjectConfig_EmptyIsSupportedNoOp(t *testing.T) {
	files, warns, supported, err := builder.PreviewProjectConfig("cursor", model.ProjectConfig{})
	require.NoError(t, err)
	assert.True(t, supported)
	assert.Empty(t, files)
	assert.Empty(t, warns)
}

func TestPreviewProjectConfig_UnknownTargetErrors(t *testing.T) {
	cfg := model.ProjectConfig{MCPServers: map[string]model.MCPServer{"git": {Command: "npx"}}}
	_, _, supported, err := builder.PreviewProjectConfig("does-not-exist", cfg)
	require.Error(t, err)
	assert.False(t, supported)
}

func TestPreviewProjectConfig_CursorComputesWithoutWriting(t *testing.T) {
	cfg := model.ProjectConfig{
		MCPServers: map[string]model.MCPServer{"git": {Command: "npx"}},
		Hooks:      []model.Hook{{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."}},
	}
	files, warns, supported, err := builder.PreviewProjectConfig("cursor", cfg)
	require.NoError(t, err)
	assert.True(t, supported)

	// It returns what WOULD be written (mcp.json + the degraded hooks rule)...
	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	assert.True(t, paths["mcp.json"])
	assert.True(t, paths["rules/ported-hooks.mdc"])
	// ...and the degradation warning for the hook.
	assert.NotEmpty(t, warns)
}
