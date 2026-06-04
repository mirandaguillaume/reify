// Package aider generates aider's CONVENTIONS.md plus the loading artefact
// that makes aider actually read it. Unlike AGENTS.md (auto-discovered),
// aider only loads a conventions file when it is named in `.aider.conf.yml`
// via `read:` (grounded in the current aider docs), so this target emits
// both files: CONVENTIONS.md (instructions) and ../.aider.conf.yml (loader).
package aider

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

type aiderGenerator struct{}

func (g *aiderGenerator) Target() string           { return "aider" }
func (g *aiderGenerator) DefaultOutputDir() string { return ".reify-aider" }
func (g *aiderGenerator) ContextDir() string       { return "context" }

func (g *aiderGenerator) GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	return GenerateConventionsMd(skills, agents)
}

// CONVENTIONS.md lives at the repository root; DefaultOutputDir is a throwaway
// staging dir that InstructionsPath escapes, mirroring the cursor/agents targets.
func (g *aiderGenerator) InstructionsPath() string { return "../CONVENTIONS.md" }

func init() {
	spec.Register("aider", func() spec.Generator { return &aiderGenerator{} })
}
