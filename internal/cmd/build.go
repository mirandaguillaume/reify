package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/mirandaguillaume/reify/internal/builder"
	"github.com/mirandaguillaume/reify/internal/importer"
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

// PortProjectConfig reads a source project's harness runtime config (hooks,
// MCP servers, native skills) from inputDir — a project root holding a
// .claude/ directory and/or a .mcp.json — and re-emits it for target into
// outputDir, degrading per-pillar where the target lacks a native channel.
// Returned warnings describe what degraded (e.g. a hook became prose). This
// is the inter-harness "port" leg: import populates the model, build re-emits.
func PortProjectConfig(inputDir, target, outputDir string) ([]string, error) {
	claudeDir := filepath.Join(inputDir, ".claude")
	mcpFile := filepath.Join(inputDir, ".mcp.json")
	cfg, err := importer.ImportProjectConfig(claudeDir, mcpFile)
	if err != nil {
		return nil, fmt.Errorf("reading source config from %s: %w", inputDir, err)
	}
	return builder.EmitProjectConfig(target, outputDir, cfg)
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
	var target, skillsDir, agentsDir, outputDirFlag, enrichFlag, inputDir string
	var watchFlag, compactFlag bool

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Generate skills and agents for a target framework",
		Long: `Compile Reify skill YAML specs into framework-native files.

Supported targets:
  claude             — Claude Code (.claude/, .mcp.json)
  copilot            — GitHub Copilot, generic (.github/)
  copilot-vscode     — Copilot in VS Code (.vscode/mcp.json, servers schema)
  copilot-cli        — Copilot CLI (~/.copilot/mcp-config.json schema)
  copilot-jetbrains  — Copilot in JetBrains/Eclipse/Xcode (UI-managed MCP)
  cursor             — Cursor (.cursor/, .cursorrules)
  agents             — AGENTS.md (Codex, Zed, recent Aider)
  reify              — Standalone Go runtime binary

With --input <project>, also ports runtime config (hooks, MCP servers,
native skills) from the source project, degrading per-pillar where the
target lacks a native channel and warning about each loss.

Examples:
  reify build --target claude
  reify build --target cursor --input .            # port .claude config to Cursor
  reify build --target copilot-vscode --input .
  reify build --target reify --output ./out`,
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

			// Optional: port runtime config (hooks/MCP/native skills) from a
			// source project. A failure to port is a warning, not a build
			// failure — the instruction compilation already succeeded.
			if inputDir != "" {
				warns, err := PortProjectConfig(inputDir, target, outputDir)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("project config not ported: %v", err))
				} else {
					result.Warnings = append(result.Warnings, warns...)
				}
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
	buildCmd.Flags().StringVar(&inputDir, "input", "", "source project to port hooks/MCP/native skills from (dir with .claude/ and/or .mcp.json)")

	if err := buildCmd.RegisterFlagCompletionFunc("target", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return spec.Available(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic("build --target completion registration failed: " + err.Error())
	}

	rootCmd.AddCommand(buildCmd)
}
