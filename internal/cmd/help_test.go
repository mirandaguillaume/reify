package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorHelp_ContainsExample(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"doctor", "--help"})
	err := rootCmd.Execute()
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "Examples:", "doctor --help must contain an Examples section")
	assert.Contains(t, out, "reify doctor", "doctor --help must show at least one reify doctor example")
}

func TestDoctorHelp_ContainsFlags(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"doctor", "--help"})
	require.NoError(t, rootCmd.Execute())
	out := buf.String()
	assert.Contains(t, out, "--provider", "doctor --help must document --provider flag")
	assert.Contains(t, out, "--model", "doctor --help must document --model flag")
	assert.Contains(t, out, "--debug", "doctor --help must document --debug flag")
	assert.Contains(t, out, "--export-yaml", "doctor --help must document --export-yaml flag")
}

func TestBuildHelp_ContainsExample(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"build", "--help"})
	require.NoError(t, rootCmd.Execute())
	out := buf.String()
	assert.Contains(t, out, "Examples:", "build --help must contain an Examples section")
	assert.Contains(t, out, "reify build", "build --help must show at least one reify build example")
}

func TestRootHelp_ContainsCompletionHint(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})
	require.NoError(t, rootCmd.Execute())
	out := buf.String()
	assert.Contains(t, out, "completion", "root --help must mention the completion command")
}

func TestBuildCmd_UnknownTargetReturnsError(t *testing.T) {
	cleanupRootCmd(t)
	rootCmd.SetArgs([]string{"build", "--target", "nonexistent-xyz"})
	err := rootCmd.Execute()
	require.Error(t, err, "build with unknown target must return an error")
	assert.Contains(t, err.Error(), "Available:", "error must list available targets")
}

func TestBuildCmd_UnknownTargetErrorContainsTarget(t *testing.T) {
	cleanupRootCmd(t)
	rootCmd.SetArgs([]string{"build", "--target", "bogus-target"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus-target", "error must reference the invalid target name")
}
