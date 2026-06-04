package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/analyzer"
	"github.com/mirandaguillaume/reify/internal/enricher"
	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/internal/linter"
	"github.com/mirandaguillaume/reify/internal/scanner"
	yamlloader "github.com/mirandaguillaume/reify/internal/yaml"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"

	// Register generators so spec.Get works.
	_ "github.com/mirandaguillaume/reify/internal/generator/agents"
	_ "github.com/mirandaguillaume/reify/internal/generator/claude"
	_ "github.com/mirandaguillaume/reify/internal/generator/copilot"
	_ "github.com/mirandaguillaume/reify/internal/generator/cursor"
	_ "github.com/mirandaguillaume/reify/internal/generator/reify"
)

const wordLimit = 500

// CodebaseIndexKey is the consumes value that triggers codebase index generation.
const CodebaseIndexKey = "codebase_index"

// BuildResult holds the outcome of a build operation.
type BuildResult struct {
	Success         bool
	Error           string
	Target          string
	OutputDir       string
	SkillsGenerated int
	AgentsGenerated int
	Warnings        []string
}

// GetOutputDir returns the output directory, using override if set or the generator default.
func GetOutputDir(target, override string) string {
	if override != "" {
		return override
	}
	gen, err := spec.Get(target)
	if err != nil {
		return ".claude" // fallback
	}
	return gen.DefaultOutputDir()
}

// RunBuild executes the full build pipeline: parse, lint, generate skills/agents/instructions.
func RunBuild(skillsDir, agentsDir, outputDir, target string, enrichMode scanner.EnrichMode) BuildResult {
	return RunBuildWithOptions(skillsDir, agentsDir, outputDir, target, enrichMode, false)
}

// RunBuildWithOptions is like RunBuild but accepts a compact flag for inlined output.
func RunBuildWithOptions(skillsDir, agentsDir, outputDir, target string, enrichMode scanner.EnrichMode, compact bool) BuildResult {
	result := BuildResult{Target: target, OutputDir: outputDir}

	gen, err := spec.Get(target)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Load contracts (output/input format templates) from contracts/ dir
	contractsDir := filepath.Join(filepath.Dir(skillsDir), "contracts")
	contracts := generator.LoadContracts(contractsDir)

	// Resolve absolute contracts dir for file references
	absContractsDir, _ := filepath.Abs(contractsDir)

	// Apply build-time options if generator supports them
	if c, ok := gen.(spec.Configurable); ok {
		c.SetOptions(spec.GeneratorOptions{Compact: compact, Contracts: contracts, ContractsDir: absContractsDir})
	}

	// 1. Parse all skills
	skillMap := make(map[string]model.SkillBehavior)
	hasLintErrors := false

	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".skill.yaml") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(skillsDir, entry.Name()))
			if err != nil {
				continue
			}
			skill, err := yamlloader.ParseSkillYAML(string(content))
			if err != nil {
				result.Error = fmt.Sprintf("Parse error in %s: %v", entry.Name(), err)
				return result
			}
			skillMap[skill.Skill] = skill
			lintResults := linter.LintSkill(skill)
			for _, lr := range lintResults {
				if lr.Severity == linter.SeverityError {
					hasLintErrors = true
				}
			}
		}
	}

	if hasLintErrors {
		result.Error = "Build failed: lint errors found. Fix errors before building."
		return result
	}

	// 2. Scan codebase if any skill consumes codebase_index or --enrich is set
	var codebaseCtx *scanner.CodebaseContext
	hasIndexConsumer := false
	for _, skill := range skillMap {
		if skillConsumesIndex(skill) {
			hasIndexConsumer = true
			break
		}
	}

	if hasIndexConsumer || enrichMode != scanner.EnrichNone {
		codebaseCtx, err = scanner.ScanCodebase(".")
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Codebase scan failed (enrichment skipped): %v", err))
		}
	}

	// Write context files when index mode is needed
	writeContextFiles := enrichMode == scanner.EnrichIndex || (hasIndexConsumer && enrichMode != scanner.EnrichFull)
	if writeContextFiles && codebaseCtx != nil {
		contextDir := filepath.Join(outputDir, gen.ContextDir())
		if err := enricher.WriteContextFiles(codebaseCtx, contextDir); err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Failed to write context files: %v", err))
		}
	}

	// 3. Generate skills (skip when compact — skills are inlined in agent file)
	sg, hasSG := gen.(spec.SkillGenerator)
	if hasSG && !compact {
		for _, skill := range skillMap {
			md := sg.GenerateSkill(skill)
			if codebaseCtx != nil {
				switch {
				case enrichMode == scanner.EnrichFull:
					md += enricher.RenderInline(codebaseCtx)
				case enrichMode == scanner.EnrichIndex:
					md += enricher.RenderPointer(codebaseCtx, gen.ContextDir())
				case skillConsumesIndex(skill):
					md += enricher.RenderPointer(codebaseCtx, gen.ContextDir())
				}
			}
			wordCount := countWords(md)
			if wordCount > wordLimit {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Skill %q generates %d words (limit: %d). Consider simplifying.", skill.Skill, wordCount, wordLimit))
			}

			relPath := sg.SkillPath(skill.Skill)
			fullPath := filepath.Join(outputDir, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				result.Error = fmt.Sprintf("Failed to create directory for skill %q: %v", skill.Skill, err)
				return result
			}
			if err := writeFileWithBackup(fullPath, []byte(md), 0644); err != nil {
				result.Error = fmt.Sprintf("Failed to write skill %q: %v", skill.Skill, err)
				return result
			}
			result.SkillsGenerated++
		}
	}

	// 4. Generate agents
	var allAgents []model.AgentComposition
	ag, hasAG := gen.(spec.AgentGenerator)

	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".agent.yaml") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(agentsDir, entry.Name()))
			if err != nil {
				continue
			}
			agent, err := yamlloader.ParseAgentYAML(string(content))
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Agent %s: %v", entry.Name(), err))
				continue
			}

			allAgents = append(allAgents, agent)

			if !hasAG {
				continue
			}

			var resolvedSkills []model.SkillBehavior
			for _, name := range agent.AllSkills() {
				if s, ok := skillMap[name]; ok {
					resolvedSkills = append(resolvedSkills, s)
				}
			}
			if len(resolvedSkills) < len(agent.AllSkills()) {
				var missing []string
				for _, name := range agent.AllSkills() {
					if _, ok := skillMap[name]; !ok {
						missing = append(missing, name)
					}
				}
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Agent %q: unresolved skills [%s]. Tool list may be incomplete.", agent.Agent, strings.Join(missing, ", ")))
			}

			// Check ordering
			orderingIssues := analyzer.CheckSkillOrdering(agent, skillMap)
			for _, issue := range orderingIssues {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Agent %q: %s", agent.Agent, issue.Message))
			}

			md := ag.GenerateAgent(agent, resolvedSkills, outputDir)
			relPath := ag.AgentPath(agent.Agent)
			fullPath := filepath.Join(outputDir, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				result.Error = fmt.Sprintf("Failed to create directory for agent %q: %v", agent.Agent, err)
				return result
			}
			if err := writeFileWithBackup(fullPath, []byte(md), 0644); err != nil {
				result.Error = fmt.Sprintf("Failed to write agent %q: %v", agent.Agent, err)
				return result
			}
			result.AgentsGenerated++
		}
	}

	// 5. Generate instructions (optional — only if generator implements InstructionsGenerator)
	if ig, ok := gen.(spec.InstructionsGenerator); ok {
		skills := make([]model.SkillBehavior, 0, len(skillMap))
		for _, s := range skillMap {
			skills = append(skills, s)
		}
		instructions := ig.GenerateInstructions(skills, allAgents)
		instrPath := ig.InstructionsPath()
		if instructions != "" {
			fullPath := filepath.Join(outputDir, filepath.FromSlash(instrPath))
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				result.Error = fmt.Sprintf("Failed to create directory for instructions: %v", err)
				return result
			}
			if err := writeFileWithBackup(fullPath, []byte(instructions), 0644); err != nil {
				result.Error = fmt.Sprintf("Failed to write instructions: %v", err)
				return result
			}
		}
	}

	result.Success = true
	return result
}

// EmitProjectConfig compiles a project's harness config (hooks/MCP/native
// skills) for the build target and writes the resulting files under
// outputDir, backing up any overwritten file. Targets that implement
// spec.ConfigGenerator port-or-degrade; targets that don't are a no-op.
// Returned warnings (e.g. a hook degraded to prose) are for the caller to
// surface. A nil/empty config is a no-op.
func EmitProjectConfig(target, outputDir string, cfg model.ProjectConfig) (warnings []string, err error) {
	files, warns, _, err := PreviewProjectConfig(target, cfg)
	if err != nil {
		return warns, err
	}
	for _, f := range files {
		full := filepath.Join(outputDir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return warns, fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := writeFileWithBackup(full, []byte(f.Content), 0644); err != nil {
			return warns, fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return warns, nil
}

// PreviewProjectConfig computes what EmitProjectConfig would write — the
// config files and degradation warnings for target — WITHOUT touching disk.
// `supported` is false when the target has no ConfigGenerator at all (in which
// case warnings explains the total loss). A nil/empty config is a supported
// no-op. This backs `check`'s pre-build fidelity preview.
func PreviewProjectConfig(target string, cfg model.ProjectConfig) (files []spec.ConfigFile, warnings []string, supported bool, err error) {
	if cfg.IsEmpty() {
		return nil, nil, true, nil
	}
	gen, err := spec.Get(target)
	if err != nil {
		return nil, nil, false, err
	}
	cg, ok := gen.(spec.ConfigGenerator)
	if !ok {
		// Target has no config support at all — everything is lost; tell
		// the caller rather than silently dropping it.
		return nil, []string{fmt.Sprintf("target %q does not support hooks/MCP/native skills; project config not emitted", target)}, false, nil
	}
	files, warnings = cg.GenerateConfig(cfg)
	return files, warnings, true, nil
}

// writeFileWithBackup writes data to path, first preserving any existing
// file as path+".bak" — but only when the existing content actually
// differs, so repeated builds don't churn identical backups. This protects
// hand-edited files (e.g. a root AGENTS.md or CLAUDE.md) from being
// silently overwritten by a build.
func writeFileWithBackup(path string, data []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(existing, data) {
			if err := os.WriteFile(path+".bak", existing, perm); err != nil {
				return fmt.Errorf("write backup %s.bak: %w", path, err)
			}
		}
	}
	return os.WriteFile(path, data, perm)
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func skillConsumesIndex(skill model.SkillBehavior) bool {
	for _, c := range skill.Context.Consumes {
		if c == CodebaseIndexKey {
			return true
		}
	}
	return false
}
