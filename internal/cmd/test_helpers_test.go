package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// resetCmdTree walks the command tree and resets every flag's Changed state and
// every flag's value back to its default. Call via t.Cleanup or at the start of
// any test that runs Execute() after another test ran "<cmd> --help" (which
// otherwise leaves help=true sticky on the subcommand and short-circuits RunE).
//
// Why a helper instead of a fresh rootCmd: each cobra.Command is registered to
// the package-level rootCmd via init(). Refactoring to a newRootCmd() factory
// would touch 8 source files + 12 test files. This helper achieves test
// isolation without that surface change. Phase 2 cleanup (full factory) is
// tracked in the Epic 9 retro action #5.
func resetCmdTree(c *cobra.Command) {
	c.Flags().VisitAll(resetFlag)
	c.PersistentFlags().VisitAll(resetFlag)
	for _, sub := range c.Commands() {
		resetCmdTree(sub)
	}
}

func resetFlag(f *pflag.Flag) {
	if !f.Changed {
		return
	}
	_ = f.Value.Set(f.DefValue)
	f.Changed = false
}

// cleanupRootCmd registers a t.Cleanup that resets the global rootCmd tree.
// Use this in tests that mutate flags or args on rootCmd to prevent leaking
// state into the next test in package order.
func cleanupRootCmd(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		resetCmdTree(rootCmd)
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
}
