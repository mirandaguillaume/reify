// Package export converts AgentAnalysis to Reify skill YAML.
package export

import (
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/pkg/model"
	"gopkg.in/yaml.v3"
)

// knownExts are the outermost file extensions stripped when deriving a skill name from a path.
var knownExts = map[string]bool{
	".md": true, ".yaml": true, ".yml": true, ".json": true,
}

// ToSkillYAML converts an AgentAnalysis to a minimal valid Reify skill YAML.
// The filePath is used as a name fallback when frontmatter has no name field.
func ToSkillYAML(analysis *parser.AgentAnalysis, filePath string) ([]byte, error) {
	skill := build(analysis, filePath)
	return yaml.Marshal(skill)
}

func build(analysis *parser.AgentAnalysis, filePath string) model.SkillBehavior {
	skill := model.SkillBehavior{
		Version: "0.1.0",
		Context: model.ContextFacet{
			Memory: model.MemoryShortTerm,
		},
		Strategy: model.StrategyFacet{
			Approach: "sequential",
			Effort:   model.EffortMedium,
		},
		Observability: model.ObservabilityFacet{
			TraceLevel: model.TraceLevelMinimal,
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessNone,
			Network:    model.NetworkNone,
		},
		Negotiation: model.NegotiationFacet{
			FileConflicts: model.NegotiationYield,
		},
	}

	// P2: nil guard — analysis can be nil when called defensively
	if analysis == nil {
		skill.Skill = nameFromPath(filePath)
		return skill
	}

	// Name: frontmatter > filename
	if v, ok := analysis.Frontmatter["name"]; ok {
		if s, _ := v.(string); s != "" {
			skill.Skill = s
		}
	}
	if skill.Skill == "" {
		skill.Skill = nameFromPath(filePath)
	}

	// Description/steps: frontmatter description > first section content
	desc := ""
	if v, ok := analysis.Frontmatter["description"]; ok {
		desc, _ = v.(string)
	}
	if desc == "" && len(analysis.Sections) > 0 {
		content := strings.TrimSpace(analysis.Sections[0].Content)
		// P6: truncate at 500 runes (not bytes) to avoid splitting multi-byte UTF-8 sequences
		if runes := []rune(content); len(runes) > 500 {
			content = string(runes[:500])
		}
		desc = content
	}
	if desc != "" {
		skill.Strategy.Steps = []string{desc}
	}

	// Tools from analysis
	if len(analysis.Tools) > 0 {
		skill.Strategy.Tools = analysis.Tools
	}

	return skill
}

// nameFromPath derives a slug from the file path when frontmatter has no name.
// Only the outermost known file type extension is stripped to avoid over-stripping
// multi-dot names like a.b.c.md → a.b.c (not a).
func nameFromPath(filePath string) string {
	base := filepath.Base(filePath)
	// P5: strip only the outermost known extension, not iteratively
	if ext := filepath.Ext(base); knownExts[ext] {
		base = strings.TrimSuffix(base, ext)
	}
	// Strip second-level semantic marker (.skill, .agent) if present
	if ext := filepath.Ext(base); ext == ".skill" || ext == ".agent" {
		base = strings.TrimSuffix(base, ext)
	}
	slug := strings.ToLower(strings.NewReplacer("_", "-", " ", "-").Replace(base))
	// P1: guard against empty slug (e.g. degenerate filenames)
	if slug == "" {
		return "unknown-agent"
	}
	return slug
}
