package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/builder"
	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/spf13/cobra"
)

// BuildResult is an alias for builder.BuildResult for backward compatibility.
type BuildResult = builder.BuildResult

// RunBuild delegates to builder.RunBuild.
func RunBuild(skillsDir, agentsDir, outputDir, target string, enrichMode scanner.EnrichMode) BuildResult {
	return builder.RunBuild(skillsDir, agentsDir, outputDir, target, enrichMode)
}

// RunBuildWithOptions delegates to builder.RunBuildWithOptions.
func RunBuildWithOptions(skillsDir, agentsDir, outputDir, target string, enrichMode scanner.EnrichMode, compact bool) BuildResult {
	return builder.RunBuildWithOptions(skillsDir, agentsDir, outputDir, target, enrichMode, compact)
}

// GetOutputDir delegates to builder.GetOutputDir.
func GetOutputDir(target, override string) string {
	return builder.GetOutputDir(target, override)
}

// PrintBuildResult prints the build result to stdout with colored output.
func PrintBuildResult(result BuildResult) {
	if !result.Success {
		fmt.Println(color.RedString("Build failed: %s", result.Error))
		return
	}

	fmt.Println(color.GreenString("Build complete (target: %s):", result.Target))
	fmt.Printf("  Output: %s\n", result.OutputDir)
	fmt.Printf("  Skills generated: %d\n", result.SkillsGenerated)
	fmt.Printf("  Agents generated: %d\n", result.AgentsGenerated)

	if len(result.Warnings) > 0 {
		fmt.Println(color.YellowString("\nWarnings:"))
		for _, w := range result.Warnings {
			fmt.Printf("  %s %s\n", color.YellowString("!"), w)
		}
	}
}

func init() {
	var target, skillsDir, agentsDir, outputDirFlag, enrichFlag string
	var watchFlag, compactFlag bool

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Generate skills and agents for a target framework",
		Long: `Compile Reify skill YAML specs into framework-native files.

Supported targets:
  claude    — Claude Code (.claude/skills/, .claude/agents/)
  copilot   — GitHub Copilot (.github/skills/, .github/agents/)
  reify   — Standalone Go runtime binary

Examples:
  reify build --target claude
  reify build --target copilot --compact
  reify build --target reify --output ./out
  reify build --target claude --watch`,
		RunE: func(cmd *cobra.Command, args []string) error {
			available := spec.Available()
			found := false
			for _, a := range available {
				if a == target {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("unknown target %q. Available: %s", target, strings.Join(available, ", "))
			}

			enrichMode := scanner.EnrichMode(enrichFlag)

			outputDir := GetOutputDir(target, outputDirFlag)

			if watchFlag {
				controller := CreateWatcher(WatchOptions{
					SkillsDir:  skillsDir,
					AgentsDir:  agentsDir,
					OutputDir:  outputDir,
					Target:     target,
					EnrichMode: enrichMode,
				})
				defer controller.Stop()
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
				<-sigCh
				return nil
			}

			result := RunBuildWithOptions(skillsDir, agentsDir, outputDir, target, enrichMode, compactFlag)
			if !result.Success {
				return fmt.Errorf("build failed: %s", result.Error)
			}
			PrintBuildResult(result)
			return nil
		},
	}

	buildCmd.Flags().StringVarP(&target, "target", "t", "claude", "target framework")
	buildCmd.Flags().StringVarP(&skillsDir, "skills", "s", "skills", "skills directory")
	buildCmd.Flags().StringVarP(&agentsDir, "agents", "a", "agents", "agents directory")
	buildCmd.Flags().StringVarP(&outputDirFlag, "output", "o", "", "output directory")
	buildCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "watch for changes")
	buildCmd.Flags().BoolVar(&compactFlag, "compact", false, "inline skills into agent file for lower token overhead")
	buildCmd.Flags().StringVar(&enrichFlag, "enrich", "", "enrich skills with codebase context (index|full)")
	buildCmd.Flag("enrich").NoOptDefVal = "index"

	if err := buildCmd.RegisterFlagCompletionFunc("target", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return spec.Available(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic("build --target completion registration failed: " + err.Error())
	}

	rootCmd.AddCommand(buildCmd)
}
