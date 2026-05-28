package cmd

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/color"
	fconfig "github.com/mirandaguillaume/reify/internal/config"
	"github.com/mirandaguillaume/reify/internal/discovery"
	"github.com/mirandaguillaume/reify/internal/doctor"
	"github.com/mirandaguillaume/reify/internal/doctor/analyzer"
	doctorctx "github.com/mirandaguillaume/reify/internal/doctor/context"
	"github.com/mirandaguillaume/reify/internal/doctor/export"
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
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
	var noCacheFlag bool
	var ciFlag bool
	var formatFlag string
	var concurrencyFlag int

	doctorCmd := &cobra.Command{
		Use:   "doctor <file|directory>",
		Short: "Analyze and improve agent definition files",
		Long: `Analyze any agent file (Claude Code, GitHub Copilot, Reify YAML) and
produce research-backed recommendations to improve it.

Accepts a single file, an agent directory, or a repo root — recognized agent
files are discovered automatically. All analysis is LLM-driven (an API key
for Anthropic, OpenRouter, or a running Ollama instance is required).

Configuration (ADR-7 precedence: flags > env > config file > defaults):
  Config file : .reify/config.yaml (searched from CWD up to filesystem root)
  Env vars    : REIFY_PROVIDER, REIFY_MODEL, REIFY_DEBUG

Sample .reify/config.yaml:
  doctor:
    provider: ollama
    model: llama4-scout
    debug: false

Examples:
  reify doctor .claude/agents/code-reviewer.md
  reify doctor .github/agents/dash.agent.md
  reify doctor .claude/agents/                      # directory mode
  reify doctor agent.md --provider openrouter
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

			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", target, err)
			}

			// CI mode: force plain text
			if ciFlag {
				color.NoColor = true
			}

			cmdCtx := cmd.Context()
			var analysisErr error
			if info.IsDir() {
				analysisErr = runDirectoryMode(cmdCtx, target, providerFlag, modelFlag, debugFlag, noCacheFlag, concurrencyFlag, reg)
			} else {
				analysisErr = runPipelineMode(cmdCtx, target, providerFlag, modelFlag, debugFlag, noCacheFlag, ciFlag, formatFlag, reg)
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
	doctorCmd.Flags().BoolVar(&noCacheFlag, "no-cache", false, "Bypass LLM response cache")
	doctorCmd.Flags().BoolVar(&ciFlag, "ci", false, "CI mode: plain text, quality gate, GitHub annotations")
	doctorCmd.Flags().StringVar(&formatFlag, "format", "", "Output format (json)")
	doctorCmd.Flags().IntVar(&concurrencyFlag, "concurrency", concurrencyDefault(), "directory mode: number of files analyzed in parallel (default: 1 for Ollama, 8 otherwise; env REIFY_CONCURRENCY overrides)")
	rootCmd.AddCommand(doctorCmd)
}

// concurrencyDefault is read by the flag declaration so REIFY_CONCURRENCY=N
// works as a default without requiring --concurrency on every invocation.
// A zero return value triggers provider-aware auto-detection in resolveConcurrency.
func concurrencyDefault() int {
	if v := os.Getenv("REIFY_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// resolveConcurrency picks the per-file worker count for directory mode.
// Precedence: explicit flag/env (>0) → provider auto-detect → cloud default.
// Ollama is serialized (1) because local inference can't parallelize requests
// usefully; cloud providers default to 8 in-flight requests.
func resolveConcurrency(requested int, providerFlag, modelFlag string, debugFlag bool) int {
	if requested > 0 {
		return requested
	}
	_, providerName, err := selectProvider(providerFlag, modelFlag, debugFlag)
	if err == nil && isOllamaProvider(providerName) {
		return 1
	}
	return 8
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

// runPipelineMode runs the LLM analysis pipeline + quality gate on one file.
func runPipelineMode(ctx context.Context, filePath, providerFlag, modelFlag string, debugFlag, noCacheFlag, ciFlag bool, formatFlag string, reg *registry.Registry) error {
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

	// LLM analysis (required — static fallback was removed)
	var llmFindings []llmutil.Finding
	_, _, provErr := selectProvider(providerFlag, modelFlag, debugFlag)
	if provErr != nil {
		if ciFlag {
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] CI mode: no LLM available, no findings produced\n")
			}
		} else if tty {
			fmt.Fprintf(os.Stderr, "\nNo LLM provider available — analysis cannot proceed.\n")
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
	}

	report := doctor.RunPipeline(analysis, llmFindings, doctor.PipelineOpts{})
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

// runDirectoryMode discovers agent files, analyzes each in parallel, and
// renders an aggregate summary. Per-file analysis is bounded by the
// concurrency level (auto-detected from the provider when concurrency=0).
func runDirectoryMode(ctx context.Context, dirPath, providerFlag, modelFlag string, debugFlag, noCacheFlag bool, concurrency int, reg *registry.Registry) error {
	files, err := discovery.DiscoverAgentFiles(dirPath)
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

	effective := resolveConcurrency(concurrency, providerFlag, modelFlag, debugFlag)
	if effective > len(files) {
		effective = len(files)
	}
	if debugFlag {
		fmt.Fprintf(os.Stderr, "[DEBUG] Directory mode: %d files, concurrency=%d\n", len(files), effective)
	}

	tty := doctor.IsTTY()
	results := make([]doctor.FileResult, len(files))
	var totalFindings int64
	var totalMu sync.Mutex
	var outMu sync.Mutex // serializes stdout/stderr writes so per-file blocks stay coherent

	sem := make(chan struct{}, effective)
	var wg sync.WaitGroup

	for i, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, path string) {
			defer wg.Done()
			defer func() { <-sem }()

			findings, format, ferr := analyzeFile(ctx, path, providerFlag, modelFlag, debugFlag, noCacheFlag, reg)
			r := doctor.FileResult{Path: path, Format: format, Error: ferr}
			if ferr == nil {
				r.Findings = findings
			}
			results[idx] = r

			outMu.Lock()
			defer outMu.Unlock()
			if debugFlag {
				fmt.Fprintf(os.Stderr, "[DEBUG] Analyzed file %d of %d: %s\n", idx+1, len(files), path)
			} else {
				fmt.Fprintf(os.Stderr, "Analyzed file %d of %d: %s\n", idx+1, len(files), path)
			}
			if ferr != nil {
				fmt.Fprintf(os.Stderr, "  Error: %s\n", ferr)
				return
			}
			doctor.RenderFindings(findings, path, tty, reg)
			totalMu.Lock()
			totalFindings += int64(len(findings))
			totalMu.Unlock()
		}(i, f)
	}
	wg.Wait()

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
					totalFindings += int64(len(crossFindings))
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


