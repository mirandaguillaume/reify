package cmd

import (
	"bytes"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompletion_SubcommandRegistered checks that the built-in completion subcommand
// is available in the command tree (Cobra adds it automatically in Execute).
func TestCompletion_SubcommandRegistered(t *testing.T) {
	// Execute any command to trigger Cobra's automatic completion registration
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "bash"})
	err := rootCmd.Execute()
	require.NoError(t, err, "completion bash must exit without error")
}

func TestCompletion_FishSubcommandNoError(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "fish"})
	err := rootCmd.Execute()
	require.NoError(t, err, "completion fish must exit without error")
}

func TestDoctorCmd_ValidArgsFunctionFiltersExtensions(t *testing.T) {
	var dc *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "doctor" {
			dc = c
			break
		}
	}
	require.NotNil(t, dc, "doctorCmd must be registered on rootCmd")
	require.NotNil(t, dc.ValidArgsFunction, "doctorCmd must have ValidArgsFunction set")

	exts, directive := dc.ValidArgsFunction(dc, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveFilterFileExt, directive)
	assert.Contains(t, exts, "md")
	assert.Contains(t, exts, "yaml")
	assert.Contains(t, exts, "yml")
}

func TestDoctorCmd_ValidArgsFunctionNoSecondArg(t *testing.T) {
	var dc *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "doctor" {
			dc = c
			break
		}
	}
	require.NotNil(t, dc)
	require.NotNil(t, dc.ValidArgsFunction)
	// Once one arg is already provided, no further file completions
	_, directive := dc.ValidArgsFunction(dc, []string{"somefile.md"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

func TestCompletion_ZshSubcommandNoError(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})
	err := rootCmd.Execute()
	require.NoError(t, err, "completion zsh must exit without error")
}

func TestCompletion_PowershellSubcommandNoError(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"completion", "powershell"})
	err := rootCmd.Execute()
	require.NoError(t, err, "completion powershell must exit without error")
}

func TestBuildCmd_TargetFlagCompletion(t *testing.T) {
	cleanupRootCmd(t)
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	// Cobra's hidden __complete command triggers flag completion and writes candidates to stdout
	rootCmd.SetArgs([]string{"__complete", "build", "--target", ""})
	_ = rootCmd.Execute()
	out := buf.String()
	for _, target := range spec.Available() {
		assert.Contains(t, out, target, "build --target completion must include registered target %q", target)
	}
}
