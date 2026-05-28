package cmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	fconfig "github.com/mirandaguillaume/reify/internal/config"
	"github.com/mirandaguillaume/reify/internal/doctor"
	"github.com/mirandaguillaume/reify/internal/doctor/export"
	"github.com/mirandaguillaume/reify/internal/doctor/analyzer"
	doctorctx "github.com/mirandaguillaume/reify/internal/doctor/context"
	docindex "github.com/mirandaguillaume/reify/internal/doctor/index"
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
	"github.com/mirandaguillaume/reify/internal/doctor/scaffold"
	"github.com/mirandaguillaume/reify/internal/doctor/static"
	"github.com/mirandaguillaume/reify/internal/scanner"
	"github.com/spf13/cobra"
)

// ErrFindings is returned when doctor finds issues. Callers should
// inspect this with errors.Is and set exit code 2 themselves.
var ErrFindings = errors.New("doctor found issues")

func init() {
	var providerFlag string
	var modelFlag string
	var debugFlag bool
	var updateRegistryFlag bool
	var exportYAMLFlag bool
	var fixFlag bool
	var forceFlag bool
	var noCacheFlag bool
	var quickFlag bool
	var ciFlag bool
	var thoroughFlag bool
	var securityFlag bool
	var formatFlag string

	doctorCmd := &cobra.Command{
		Use:   "doctor <file|directory>",
		Short: "Analyze and improve agent definition files",
		Long: `Analyze any agent file (Claude Code, GitHub Copilot, Reify YAML) and
produce research-backed recommendations to improve it.

When given a directory, discovers all agent files recursively and
produces an aggregate summary after individual file results.

Configuration (ADR-7 precedence: flags > env > config file > defaults):
  Config file : .reify/config.yaml (searched from CWD up to filesystem root)
  Env vars    : REIFY_PROVIDER, REIFY_MODEL, REIFY_DEBUG

Sample .reify/config.yaml:
  doctor:
    provider: ollama
    model: llama4-scout
    confidence_threshold: moderate
    backup_retention: 168h
    debug: false

Examples:
  reify doctor .claude/agents/code-reviewer.md
  reify doctor .github/agents/dash.agent.md
  reify doctor skills/review-commenter.skill.yaml
  reify doctor .claude/agents/                      # directory mode
  reify doctor agent.md --provider openrouter
  reify doctor agent.md --fix                       # interactive rewrite
  reify doctor agent.md --fix --force               # skip diff-size check
  reify doctor --update-registry                    # download latest registry`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) >= 1 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return []string{"md", "yaml", "yml"}, cobra.ShellCompDirectiveFilterFileExt
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()

			// Config resolution: flags > env > config file > defaults (ADR-7).
			origProvider := providerFlag
			origModel := modelFlag
			origDebug := debugFlag
			debugChanged := cmd.Flags().Changed("debug")
			cfg, cfgErr := fconfig.Load(cwd)
			if cfgErr != nil {
				cfg = &fconfig.Config{}
			}
			cfgProvider := cfg.Doctor.Provider
			cfgModel := cfg.Doctor.Model
			cfgDebug := cfg.Doctor.Debug
			fconfig.ApplyEnv(cfg)
			fconfig.ApplyFlags(cfg, origProvider, origModel, origDebug, debugChanged)
			providerFlag = cfg.Doctor.Provider
			modelFlag = cfg.Doctor.Model
			debugFlag = cfg.Doctor.Debug

			if cfgErr != nil {
				fmt.Fprintf(os.Stderr, "reify: warning: config load error (using defaults): %v\n", cfgErr)
			}
			if debugFlag {
				logResolved := func(name, flagVal, envKey, fileVal, resolved string) {
					source := "default (auto-detect)"
					switch {
					case flagVal != "":
						source = "flag"
					case os.Getenv(envKey) != "":
						source = "env"
					case fileVal != "":
						source = "config file"
					}
					if resolved != "" {
						fmt.Fprintf(os.Stderr, "[DEBUG] Config: %s=%q (from: %s)\n", name, resolved, source)
					}
				}
				logResolved("provider", origProvider, "REIFY_PROVIDER", cfgProvider, providerFlag)
				logResolved("model", origModel, "REIFY_MODEL", cfgModel, modelFlag)

				// debug source
				debugSource := "default (false)"
				switch {
				case debugChanged:
					debugSource = "flag"
				case os.Getenv("REIFY_DEBUG") != "":
					debugSource = "env"
				case cfgDebug:
					debugSource = "config file"
				}
				fmt.Fprintf(os.Stderr, "[DEBUG] Config: debug=%v (from: %s)\n", debugFlag, debugSource)

				// config-file-only fields
				if cfg.Doctor.ConfidenceThreshold != "" {
					fmt.Fprintf(os.Stderr, "[DEBUG] Config: confidence_threshold=%q (from: config file)\n", cfg.Doctor.ConfidenceThreshold)
				}
				if cfg.Doctor.BackupRetention != 0 {
					fmt.Fprintf(os.Stderr, "[DEBUG] Config: backup_retention=%v (from: config file)\n", cfg.Doctor.BackupRetention)
				}
				if cfg.Doctor.RegistryPath != "" {
					fmt.Fprintf(os.Stderr, "[DEBUG] Config: registry_path=%q (from: config file)\n", cfg.Doctor.RegistryPath)
				}
			}

			// --export-yaml: parse agent file, emit Reify skill YAML to stdout.
			if exportYAMLFlag {
				if fixFlag {
					return fmt.Errorf("--export-yaml cannot be combined with --fix")
				}
				// P3: --update-registry would be silently ignored after export exits; reject explicitly.
				if updateRegistryFlag {
					return fmt.Errorf("--export-yaml cannot be combined with --update-registry")
				}
				if len(args) == 0 {
					return fmt.Errorf("--export-yaml requires a file argument")
				}
				target := args[0]
				info, statErr := os.Stat(target)
				if statErr != nil {
					return fmt.Errorf("cannot access %s: %w", target, statErr)
				}
				if info.IsDir() {
					return fmt.Errorf("--export-yaml requires a file, not a directory")
				}
				content, readErr := os.ReadFile(target)
				if readErr != nil {
					return fmt.Errorf("cannot read %s: %w", target, readErr)
				}
				p, parseErr := parser.DetectFormat(target, content)
				if parseErr != nil {
					return fmt.Errorf("cannot detect format of %s: %w", target, parseErr)
				}
				analysis, analysisErr := p.Parse(content)
				if analysisErr != nil {
					return fmt.Errorf("parse error in %s: %w", target, analysisErr)
				}
				yamlBytes, exportErr := export.ToSkillYAML(analysis, target)
				if exportErr != nil {
					return fmt.Errorf("export: %w", exportErr)
				}
				// P4: propagate write errors (e.g. broken pipe in CI pipelines)
				if _, err := cmd.OutOrStdout().Write(yamlBytes); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}

			// Load research registry
			reg, err := registry.Load(cwd)
			if err != nil {
				return fmt.Errorf("load research registry: %w", err)
			}

			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] Registry: version=%s source=%s\n", reg.Version, reg.Source)
				if reg.SourcePath != "" {
					fmt.Fprintf(os.Stderr, "[DEBUG] Registry path: %s\n", reg.SourcePath)
				}
			}

			// --update-registry: download and exit (no analysis)
			if updateRegistryFlag {
				return registry.Update(reg, cwd)
			}

			if len(args) == 0 {
				return fmt.Errorf("requires a file or directory argument (or --update-registry)")
			}

			target := args[0]

			// --fix: scaffold AGENTS.md index + .agents/*.md
			if fixFlag {
				return runScaffoldMode(target, debugFlag, reg)
			}

			// Determine if target is file or directory
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", target, err)
			}

			// Determine analysis mode
			mode := "default"
			if quickFlag {
				mode = "quick"
			} else if thoroughFlag {
				mode = "thorough"
			} else if securityFlag {
				mode = "security"
			}

			// CI mode: force plain text, auto-fallback to quick if no LLM
			if ciFlag {
				color.NoColor = true
			}

			// Check if target is an index file
			var indexContent []byte
			isIndex := false
			if !info.IsDir() {
				if c, err := os.ReadFile(target); err == nil {
					indexContent = c
					isIndex = docindex.IsIndex(c)
				}
			}

			cmdCtx := cmd.Context()
			var analysisErr error
			if isIndex {
				analysisErr = runIndexModeWithContent(target, indexContent, mode, formatFlag, ciFlag, reg)
			} else if mode == "quick" && formatFlag == "" && !ciFlag {
				analysisErr = runQuickMode(target, reg)
			} else if mode == "quick" && (formatFlag != "" || ciFlag) {
				// Quick mode but with structured output — use pipeline
				analysisErr = runPipelineMode(cmdCtx, target, providerFlag, modelFlag, debugFlag, noCacheFlag, ciFlag, mode, formatFlag, reg)
			} else if info.IsDir() {
				analysisErr = runDirectoryMode(cmdCtx, target, providerFlag, modelFlag, debugFlag, noCacheFlag, reg)
			} else {
				analysisErr = runPipelineMode(cmdCtx, target, providerFlag, modelFlag, debugFlag, noCacheFlag, ciFlag, mode, formatFlag, reg)
			}

			// Show registry update notification once, after all findings
			notifyRegistryUpdate(reg)

			return analysisErr
		},
	}

	doctorCmd.Flags().StringVar(&providerFlag, "provider", "", "LLM provider (ollama, openrouter, anthropic). Default: auto-detect")
	doctorCmd.Flags().StringVar(&modelFlag, "model", "", "LLM model name (provider-specific)")
	doctorCmd.Flags().BoolVar(&debugFlag, "debug", false, "Show debug output on stderr")
	doctorCmd.Flags().BoolVar(&updateRegistryFlag, "update-registry", false, "Download the latest research registry and exit")
	doctorCmd.Flags().BoolVar(&exportYAMLFlag, "export-yaml", false, "Export parsed analysis as Reify skill YAML to stdout")
	doctorCmd.Flags().BoolVar(&fixFlag, "fix", false, "Generate AGENTS.md index + specialized files from agent file")
	doctorCmd.Flags().BoolVar(&forceFlag, "force", false, "Skip diff-size sanity check (use with --fix)")
	doctorCmd.Flags().BoolVar(&noCacheFlag, "no-cache", false, "Bypass LLM response cache")
	doctorCmd.Flags().BoolVar(&quickFlag, "quick", false, "Static checks only — no LLM, instant results")
	doctorCmd.Flags().BoolVar(&ciFlag, "ci", false, "CI mode: plain text, quality gate, GitHub annotations")
	doctorCmd.Flags().BoolVar(&thoroughFlag, "thorough", false, "Run all checks including thorough-tagged")
	doctorCmd.Flags().BoolVar(&securityFlag, "security", false, "Run security-tagged checks only")
	doctorCmd.Flags().StringVar(&formatFlag, "format", "", "Output format (json)")
	rootCmd.AddCommand(doctorCmd)
}

// notifyRegistryUpdate prints a one-time notification if a newer registry is available.
func notifyRegistryUpdate(reg *registry.Registry) {
	if !reg.NeedsUpdate() {
		return
	}

	tty := doctor.IsTTY()
	if tty {
		fmt.Fprintf(os.Stderr, "\n%s Research registry update available (current: %s, latest: %s).\n",
			color.YellowString("[INFO]"), reg.Version, reg.LatestVersion)
		fmt.Fprintf(os.Stderr, "  Run: %s\n", color.CyanString("reify doctor --update-registry"))
	} else {
		fmt.Fprintf(os.Stderr, "\nINFO: Research registry update available (current: %s, latest: %s).\n", reg.Version, reg.LatestVersion)
		fmt.Fprintf(os.Stderr, "  Run: reify doctor --update-registry\n")
	}
}

// runQuickMode runs static checks only — no LLM, instant results.
func runQuickMode(target string, reg *registry.Registry) error {
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", target, err)
	}

	var files []string
	if info.IsDir() {
		files, err = doctor.DiscoverAgentFiles(target)
		if err != nil {
			return fmt.Errorf("discover agent files: %w", err)
		}
	} else {
		files = []string{target}
	}

	if len(files) == 0 {
		fmt.Println("No agent files found.")
		return nil
	}

	tty := doctor.IsTTY()
	totalFindings := 0

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			continue
		}

		p, err := parser.DetectFormat(f, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot detect format of %s: %v\n", f, err)
			continue
		}

		analysis, err := p.Parse(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Parse error in %s: %v\n", f, err)
			continue
		}

		// Run static checks only
		findings := static.RunChecks(analysis, "quick")
		structural := doctor.ComputeStructural(findings, sectionMappingCount)
		doctor.RenderStructural(structural, f, tty)

		// Also show non-presence findings (vague, ordering)
		var otherFindings []llmutil.Finding
		for _, finding := range findings {
			if !strings.Contains(finding.Issue, "Missing") {
				otherFindings = append(otherFindings, finding)
			}
		}
		if len(otherFindings) > 0 {
			doctor.RenderFindings(otherFindings, f, tty, reg)
		}
		totalFindings += len(findings)
	}

	if totalFindings > 0 {
		return ErrFindings
	}
	return nil
}

// sectionMappingCount for structural score — must match static/presence.go (15 sections).
// ComputeStructural adds +1 for secrets check internally.
const sectionMappingCount = 15

// runIndexModeWithContent analyzes an AGENTS.md index by resolving links and computing aggregate score.
func runIndexModeWithContent(target string, content []byte, mode, formatFlag string, ciFlag bool, reg *registry.Registry) error {
	baseDir := filepath.Dir(target)
	resolved := docindex.ResolveIndex(content, baseDir)
	tty := doctor.IsTTY() && !ciFlag

	// Show index resolution
	if tty {
		fmt.Printf("\nIndex: %s (%d referenced files)\n", target, len(resolved))
	} else {
		fmt.Printf("Index: %s (%d referenced files)\n", target, len(resolved))
	}

	// Show missing file findings
	missingFindings := docindex.MissingFiles(resolved)
	for _, f := range missingFindings {
		if tty {
			fmt.Printf("  %s %s\n", color.RedString("x"), f.Issue)
		} else {
			fmt.Printf("  MISSING: %s\n", f.Issue)
		}
	}

	// Compute aggregate score
	agg := docindex.ScoreIndex(resolved, mode)

	// Render
	if formatFlag == "json" {
		jsonBytes, err := docindex.AggregateToJSON(agg, target)
		if err != nil {
			return fmt.Errorf("JSON output: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Print(docindex.FormatAggregate(agg))
	}

	if len(missingFindings) > 0 || agg.Covered < agg.Total {
		return ErrFindings
	}
	return nil
}

// runScaffoldMode generates AGENTS.md index + .agents/*.md from the original file.
func runScaffoldMode(target string, debugFlag bool, reg *registry.Registry) error {
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", target, err)
	}

	// Check if this is already an index — scaffold only missing files
	if docindex.IsIndex(content) {
		return scaffoldMissing(target, content, debugFlag)
	}

	// Parse the original file
	p, pErr := parser.DetectFormat(target, content)
	var analysis *parser.AgentAnalysis
	if pErr == nil {
		analysis, _ = p.Parse(content)
	}
	if analysis == nil {
		analysis = &parser.AgentAnalysis{Format: "unknown", RawContent: content}
	}

	// Scan codebase for context
	projectRoot := doctorctx.DetectProjectRoot(filepath.Dir(target))
	var ctx *scanner.CodebaseContext
	if projectRoot != "" {
		ctx, _ = scanner.ScanCodebase(projectRoot)
	}

	// Generate scaffold
	result, err := scaffold.Scaffold(analysis, ctx)
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Write files
	baseDir := filepath.Dir(target)
	if baseDir == "." {
		baseDir, _ = os.Getwd()
	}

	tty := doctor.IsTTY()

	// Write AGENTS.md (skip if exists)
	indexPath := filepath.Join(baseDir, "AGENTS.md")
	if _, err := os.Stat(indexPath); err == nil {
		if tty {
			fmt.Printf("  %s %s (already exists, skipped)\n", color.YellowString("~"), indexPath)
		} else {
			fmt.Printf("  ~ %s (already exists, skipped)\n", indexPath)
		}
	} else {
		if err := os.WriteFile(indexPath, result.IndexContent, 0644); err != nil {
			return fmt.Errorf("write index: %w", err)
		}
		if tty {
			fmt.Printf("  %s %s\n", color.GreenString("+"), indexPath)
		} else {
			fmt.Printf("  + %s\n", indexPath)
		}
	}

	// Write .agents/*.md (skip existing)
	written := 0
	for relPath, content := range result.Files {
		fullPath := filepath.Join(baseDir, relPath)
		if _, err := os.Stat(fullPath); err == nil {
			if tty {
				fmt.Printf("  %s %s (already exists, skipped)\n", color.YellowString("~"), fullPath)
			} else {
				fmt.Printf("  ~ %s (already exists, skipped)\n", fullPath)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("create dir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
		if tty {
			fmt.Printf("  %s %s\n", color.GreenString("+"), fullPath)
		} else {
			fmt.Printf("  + %s\n", fullPath)
		}
		written++
	}

	if written == 0 {
		fmt.Println("\nAll files already exist. Nothing written.")
	} else if tty {
		color.Green("\nScaffold complete: %d files written (%d migrated, %d templated)",
			written, result.MigratedCount, result.TemplatedCount)
	} else {
		fmt.Printf("\nScaffold complete: %d files written (%d migrated, %d templated)\n",
			written, result.MigratedCount, result.TemplatedCount)
	}

	return nil
}

// scaffoldMissing generates only the missing files referenced in an existing index.
func scaffoldMissing(indexPath string, indexContent []byte, debugFlag bool) error {
	baseDir := filepath.Dir(indexPath)
	resolved := docindex.ResolveIndex(indexContent, baseDir)

	tty := doctor.IsTTY()
	created := 0

	for _, rf := range resolved {
		if !rf.Missing {
			continue
		}
		// Find matching template
		fullPath := filepath.Join(baseDir, rf.Path)
		for _, sf := range scaffold.DefaultFiles {
			if strings.Contains(rf.Path, sf.Name) {
				content := sf.Title + " — TODO: customize for your project.\n"
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					return fmt.Errorf("create dir: %w", err)
				}
				if err := os.WriteFile(fullPath, []byte("# "+content), 0644); err != nil {
					return fmt.Errorf("write %s: %w", rf.Path, err)
				}
				if tty {
					fmt.Printf("  %s %s (new)\n", color.GreenString("+"), fullPath)
				} else {
					fmt.Printf("  + %s (new)\n", fullPath)
				}
				created++
				break
			}
		}
	}

	if created == 0 {
		fmt.Println("All referenced files exist. Nothing to scaffold.")
	} else if tty {
		color.Green("\n%d missing files created.", created)
	} else {
		fmt.Printf("\n%d missing files created.\n", created)
	}

	return nil
}

// runPipelineMode runs the full pipeline: static → LLM → post-process → gate.
func runPipelineMode(ctx context.Context, filePath, providerFlag, modelFlag string, debugFlag, noCacheFlag, ciFlag bool, mode, formatFlag string, reg *registry.Registry) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", filePath, err)
	}
	p, err := parser.DetectFormat(filePath, content)
	if err != nil {
		return fmt.Errorf("cannot detect format of %s: %w", filePath, err)
	}
	analysis, err := p.Parse(content)
	if err != nil {
		return fmt.Errorf("parse error in %s: %w", filePath, err)
	}

	tty := doctor.IsTTY() && !ciFlag

	// Phase 1: Run static checks + render structural (instant)
	opts := doctor.PipelineOpts{
		Mode:         mode,
		SectionCount: sectionMappingCount,
	}

	staticFindings := static.RunChecks(analysis, mode)
	structural := doctor.ComputeStructural(staticFindings, sectionMappingCount)
	if !ciFlag || formatFlag == "" {
		doctor.RenderStructural(structural, filePath, tty)
	}

	// Phase 2: LLM analysis (unless CI with no provider)
	var llmFindings []llmutil.Finding
	_, providerName, provErr := selectProvider(providerFlag, modelFlag, debugFlag)
	if provErr != nil {
		if ciFlag {
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] CI mode: no LLM available, static only\n")
			}
		} else if tty {
			fmt.Fprintf(os.Stderr, "\nNo LLM provider available — showing static analysis only.\n")
		}
	} else {
		var spin *doctor.Spinner
		if tty {
			spin = doctor.NewSpinner("Analyzing with LLM...")
			spin.Start()
		}
		llmFindings, _, err = analyzeFile(ctx, filePath, providerFlag, modelFlag, debugFlag, noCacheFlag, reg)
		if spin != nil {
			spin.Stop()
		}
		if err != nil {
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] LLM analysis error: %v\n", err)
			}
		}
		_ = providerName // used only for debug
	}

	// Phase 3: Run full pipeline
	report := doctor.RunPipeline(analysis, llmFindings, opts)
	report.FilePath = filePath

	// Phase 4: Output
	if formatFlag == "json" {
		jsonBytes, err := doctor.ToJSON(report, filePath)
		if err != nil {
			return fmt.Errorf("JSON output: %w", err)
		}
		fmt.Println(string(jsonBytes))
	} else if ciFlag {
		doctor.RenderGitHubAnnotations(report.AllFindings, filePath)
		doctor.RenderGateAnnotation(report.GateResult)
	} else {
		doctor.RenderAntiPatterns(report.AllFindings, tty)
		doctor.RenderFindings(report.AllFindings, filePath, tty, reg)
	}

	// Gate result determines exit code — only fail on gate failure
	if !report.GateResult.Pass {
		return ErrFindings
	}
	return nil
}

// runDirectoryMode discovers agent files, analyzes each, and renders an aggregate summary.
func runDirectoryMode(ctx context.Context, dirPath, providerFlag, modelFlag string, debugFlag, noCacheFlag bool, reg *registry.Registry) error {
	files, err := doctor.DiscoverAgentFiles(dirPath)
	if err != nil {
		return fmt.Errorf("discover agent files in %s: %w", dirPath, err)
	}

	if len(files) == 0 {
		tty := doctor.IsTTY()
		if tty {
			color.Green("No agent files found in %s", dirPath)
		} else {
			fmt.Printf("No agent files found in %s\n", dirPath)
		}
		return nil
	}

	tty := doctor.IsTTY()
	var results []doctor.FileResult
	totalFindings := 0

	for i, f := range files {
		if debugFlag {
			fmt.Fprintf(os.Stderr, "[DEBUG] Analyzing file %d of %d: %s\n", i+1, len(files), f)
		} else {
			fmt.Fprintf(os.Stderr, "Analyzing file %d of %d: %s\n", i+1, len(files), f)
		}

		findings, format, err := analyzeFile(ctx, f, providerFlag, modelFlag, debugFlag, noCacheFlag, reg)
		result := doctor.FileResult{Path: f, Format: format, Error: err}
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %s\n", err)
		} else {
			result.Findings = findings
			doctor.RenderFindings(findings, f, tty, reg)
			totalFindings += len(findings)
		}
		results = append(results, result)
	}

	// Cross-file consistency analysis (if >1 file and LLM available)
	if len(results) > 1 {
		provider, _, provErr := selectProvider(providerFlag, modelFlag, debugFlag)
		if provErr == nil {
			var analyses []*parser.AgentAnalysis
			var filePaths []string
			for _, r := range results {
				if r.Error == nil && r.Findings != nil {
					// Re-parse for cross-file (lightweight — already in memory conceptually)
					content, err := os.ReadFile(r.Path)
					if err != nil {
						continue
					}
					p, err := parser.DetectFormat(r.Path, content)
					if err != nil {
						continue
					}
					a, err := p.Parse(content)
					if err != nil {
						continue
					}
					analyses = append(analyses, a)
					filePaths = append(filePaths, r.Path)
				}
			}
			if len(analyses) > 1 {
				if debugFlag {
					fmt.Fprintf(os.Stderr, "[DEBUG] Running cross-file consistency on %d files\n", len(analyses))
				}
				crossFindings, err := analyzer.AnalyzeCrossFile(analyses, filePaths, provider)
				if err != nil {
					if debugFlag {
						fmt.Fprintf(os.Stderr, "[DEBUG] Cross-file error: %v\n", err)
					}
				} else if len(crossFindings) > 0 {
					doctor.RenderConsistency(crossFindings, tty, reg)
					totalFindings += len(crossFindings)
				}
			}
		}
	}

	// Aggregate and render summary
	report := doctor.Aggregate(results)
	doctor.RenderAggregate(report, tty)

	if totalFindings > 0 {
		return ErrFindings
	}
	return nil
}

// analyzeFile runs the full analysis pipeline on a single file using the DAG engine.
// Returns findings, detected format name, and any error.
func analyzeFile(ctx context.Context, filePath, providerFlag, modelFlag string, debugFlag, noCacheFlag bool, reg *registry.Registry) ([]llmutil.Finding, string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read %s: %w", filePath, err)
	}

	if debugFlag {
		fmt.Fprintf(os.Stderr, "[DEBUG] Registered parsers: %v\n", parser.RegisteredFormats())
		for _, name := range parser.RegisteredFormats() {
			pp, _ := parser.Get(name)
			if pp != nil {
				detected := pp.Detect(filePath, content)
				fmt.Fprintf(os.Stderr, "[DEBUG] Parser %s.Detect(%s) = %v\n", name, filePath, detected)
			}
		}
	}

	// Format detection (needed early for cache check and fallback path)
	p, err := parser.DetectFormat(filePath, content)
	if err != nil {
		return nil, "", fmt.Errorf("cannot detect format of %s: %w\nSupported formats: Claude Code, GitHub Copilot, Reify YAML", filePath, err)
	}

	if debugFlag {
		fmt.Fprintf(os.Stderr, "[DEBUG] Detected format: %s\n", p.Format())
	}

	analysis, err := p.Parse(content)
	if err != nil {
		return nil, p.Format(), fmt.Errorf("parse error in %s: %w", filePath, err)
	}

	if debugFlag {
		fmt.Fprintf(os.Stderr, "[DEBUG] Parsed: %d sections, %d tools, %d frontmatter fields\n",
			len(analysis.Sections), len(analysis.Tools), len(analysis.Frontmatter))
	}

	// Select provider. Always surface unavailability so non-debug runs see why
	// LLM-dependent analysis was skipped (rather than silently producing
	// static-only results).
	provider, providerName, provErr := selectProvider(providerFlag, modelFlag, debugFlag)
	if provErr != nil {
		if debugFlag {
			fmt.Fprintf(os.Stderr, "[DEBUG] No LLM: %s\n", provErr)
		} else {
			fmt.Fprintf(os.Stderr, "warn: LLM unavailable, running static checks only: %s\n", provErr)
		}
	}

	// Cache check (only when provider is available)
	var cache *doctor.Cache
	var cacheKey string
	if provErr == nil {
		if debugFlag {
			fmt.Fprintf(os.Stderr, "[DEBUG] Using provider: %s\n", providerName)
		}
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			cwd = filepath.Dir(filePath)
		}
		cacheDir := filepath.Join(cwd, ".reify", "cache", "doctor")
		cache = doctor.NewCache(cacheDir)
		cacheProviderName := canonicalProvider(providerName)
		cacheKey = doctor.CacheKey(content, reg.PromptHash(), cacheProviderName, reg.Version)

		if !noCacheFlag {
			if cached := cache.Get(cacheKey); cached != nil {
				if debugFlag {
					fmt.Fprintf(os.Stderr, "[DEBUG] Cache hit for %s (key: %s)\n", filePath, cacheKey)
				}
				// Cache hit: still run context enrichment
				projectRoot := doctorctx.DetectProjectRoot(filepath.Dir(filePath))
				ctxFindings, _ := doctorctx.Enrich(analysis, projectRoot)
				findings := append(cached, ctxFindings...)
				return findings, p.Format(), nil
			}
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] Cache miss for %s (key: %s)\n", filePath, cacheKey)
			}
		}
	}

	// Build and execute the doctor DAG
	d, err := doctor.BuildDAG(debugFlag)
	if err != nil {
		return nil, p.Format(), fmt.Errorf("build DAG: %w", err)
	}

	projectRoot := doctorctx.DetectProjectRoot(filepath.Dir(filePath))
	if debugFlag {
		if projectRoot != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG] Project root: %s\n", projectRoot)
		} else {
			fmt.Fprintf(os.Stderr, "[DEBUG] No project root detected — skipping context enrichment\n")
		}
	}

	input := doctor.DoctorInput{
		FilePath:    filePath,
		Content:     content,
		Provider:    provider, // nil when no provider available
		Registry:    reg,
		ProjectRoot: projectRoot,
		Debug:       debugFlag,
	}

	// Concurrency=1 for Ollama (local LLM serialization)
	concurrency := 0
	if provErr == nil && isOllamaProvider(providerName) {
		concurrency = 1
	}

	result, err := doctor.RunDAG(ctx, d, input, concurrency)
	if err != nil {
		return nil, p.Format(), fmt.Errorf("analysis failed: %w", err)
	}

	// Cache LLM findings (not context findings); cache even when empty so clean files don't get re-analyzed
	if cache != nil {
		cacheEntry := doctor.CacheEntry{
			ContentHash:     fmt.Sprintf("%x", sha256.Sum256(content))[:16],
			PromptHash:      reg.PromptHash(),
			Model:           providerName,
			RegistryVersion: reg.Version,
			Timestamp:       time.Now(),
			Findings:        result.LLMFindings,
		}
		if cacheErr := cache.Put(cacheKey, cacheEntry); cacheErr != nil && debugFlag {
			fmt.Fprintf(os.Stderr, "[DEBUG] Cache write error: %v\n", cacheErr)
		}
	}

	// Show warnings
	if result.Analysis != nil {
		tty := doctor.IsTTY()
		for _, w := range result.Analysis.Warnings {
			if tty {
				fmt.Printf("%s %s\n", color.YellowString("[WARN]"), w)
			} else {
				fmt.Printf("WARN: %s\n", w)
			}
		}
	}

	if debugFlag {
		fmt.Fprintf(os.Stderr, "[DEBUG] DAG findings: %d (LLM: %d, context: %d)\n",
			len(result.AllFindings), len(result.LLMFindings), len(result.CtxFindings))
	}

	return result.AllFindings, p.Format(), nil
}


