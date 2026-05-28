package cmd

import (
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "reify",
	Short: "Reify — Forge agents from composable skill specs",
	Long: `Reify compiles agent skill specs (YAML) into framework-native formats.

Agents are compositions of Skill Behaviors — reusable behavioral units
with 5 facets: Context, Strategy, Guardrails, Observability, Security.

Examples:
  reify doctor agent.md              # analyze an agent definition
  reify build --target claude        # compile for Claude Code
  reify targets                      # list registered targets and parsers
  reify completion bash > ~/.reify-completion.bash`,
	Version: version,
}

func Execute() error {
	return rootCmd.Execute()
}
