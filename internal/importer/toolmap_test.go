package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReverseMapTools_Claude(t *testing.T) {
	tools := []string{"Read", "Write", "Bash", "WebFetch"}
	result := ReverseMapTools(tools, FrameworkClaude)

	assert.Equal(t, []string{"read_file", "write_file", "bash", "web_fetch"}, result)
}

func TestReverseMapTools_Claude_DropsUnknown(t *testing.T) {
	tools := []string{"Read", "UnknownTool", "Bash"}
	result := ReverseMapTools(tools, FrameworkClaude)

	assert.Equal(t, []string{"read_file", "bash"}, result)
}

func TestReverseMapTools_Copilot(t *testing.T) {
	tools := []string{"read", "edit", "execute", "web"}
	result := ReverseMapTools(tools, FrameworkCopilot)

	assert.Equal(t, []string{"read_file", "write_file", "bash", "web_search"}, result)
}

func TestReverseMapTools_Copilot_DropsUnknown(t *testing.T) {
	tools := []string{"read", "fly"}
	result := ReverseMapTools(tools, FrameworkCopilot)

	assert.Equal(t, []string{"read_file"}, result)
}

func TestReverseMapTools_Unknown(t *testing.T) {
	// "Read" exists only in Claude map, "execute" only in Copilot map.
	tools := []string{"Read", "execute"}
	result := ReverseMapTools(tools, FrameworkUnknown)

	assert.Equal(t, []string{"read_file", "bash"}, result)
}

func TestReverseMapTools_Unknown_Deduplicates(t *testing.T) {
	// "read" maps to "read_file" in Copilot, "Read" maps to "read_file" in Claude.
	tools := []string{"Read", "read"}
	result := ReverseMapTools(tools, FrameworkUnknown)

	assert.Equal(t, []string{"read_file"}, result)
}

func TestReverseMapTools_EmptyInput(t *testing.T) {
	result := ReverseMapTools([]string{}, FrameworkClaude)
	assert.Nil(t, result)
}

func TestReverseMapTools_AllUnmapped(t *testing.T) {
	result := ReverseMapTools([]string{"foo", "bar"}, FrameworkClaude)
	assert.Nil(t, result)
}
