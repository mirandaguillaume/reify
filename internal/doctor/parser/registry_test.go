package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_And_Get(t *testing.T) {
	// Claude is registered via init()
	p, err := Get("claude")
	require.NoError(t, err)
	assert.Equal(t, "claude", p.Format())
}

func TestGet_Unknown(t *testing.T) {
	_, err := Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown parser")
}

func TestDetectFormat_Claude(t *testing.T) {
	content := []byte("---\nname: test\ntools: Read\n---\nYou are an agent.")
	p, err := DetectFormat(".claude/agents/test.md", content)
	require.NoError(t, err)
	assert.Equal(t, "claude", p.Format())
}

func TestDetectFormat_NoMatch(t *testing.T) {
	content := []byte("Just some random text")
	_, err := DetectFormat("random/file.txt", content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no parser detected")
}

func TestRegisteredFormats(t *testing.T) {
	formats := RegisteredFormats()
	assert.Contains(t, formats, "claude")
}
