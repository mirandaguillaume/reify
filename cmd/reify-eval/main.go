// Command reify-eval measures whether a rule's formulation changes an
// agent's obedience, by running the real harness against trapped git
// fixtures in a disposable Docker sandbox. See
// docs/superpowers/specs/2026-06-02-eval-harness-design.md.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "reify-eval",
		Short: "Fidelity eval harness: measure rule-formulation effectiveness",
	}
	root.AddCommand(runCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the eval matrix for a fixture",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("not implemented yet")
		},
	}
}
