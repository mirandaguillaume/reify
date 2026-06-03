// Package agents generates an AGENTS.md file — the emerging cross-tool
// standard read by Codex, Zed, Cursor, Aider (recent), and others. The
// file is auto-discovered at the repository root, so emitting it is
// sufficient (no loading-mechanism artefact needed, unlike Aider's
// CONVENTIONS.md).
package agents

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

type agentsGenerator struct{}

func (g *agentsGenerator) Target() string           { return "agents" }
func (g *agentsGenerator) DefaultOutputDir() string { return ".reify-agents" }
func (g *agentsGenerator) ContextDir() string       { return "context" }

func (g *agentsGenerator) GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	return GenerateAgentsMd(skills, agents)
}

// AGENTS.md lives at the repository root. DefaultOutputDir is a throwaway
// staging dir; InstructionsPath escapes it to the root, mirroring how the
// cursor target writes ../.cursorrules.
func (g *agentsGenerator) InstructionsPath() string { return "../AGENTS.md" }

func init() {
	spec.Register("agents", func() spec.Generator { return &agentsGenerator{} })
}
