package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargets_ListsBuildTargets(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"targets"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Build targets:")
	assert.Contains(t, out, "claude")
	assert.Contains(t, out, "copilot")
	assert.Contains(t, out, "reify")
}

func TestTargets_ListsFormatParsers(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"targets"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Format parsers:")
	// Claude Code, Copilot, and Reify parsers are always registered
	assert.True(t,
		strings.Contains(out, "claude") || strings.Contains(out, "Claude"),
		"expected claude parser in output: %s", out,
	)
}

func TestTargets_NoArgs(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"targets"})
	err := rootCmd.Execute()
	assert.NoError(t, err)
}
