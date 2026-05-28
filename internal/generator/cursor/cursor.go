package cursor

import (
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

type cursorGenerator struct{}

func (g *cursorGenerator) Target() string           { return "cursor" }
func (g *cursorGenerator) DefaultOutputDir() string { return ".cursor" }

func (g *cursorGenerator) GenerateSkill(skill model.SkillBehavior) string {
	return GenerateCursorSkillMdc(skill)
}

func (g *cursorGenerator) SetOptions(_ spec.GeneratorOptions) {}

func (g *cursorGenerator) GenerateAgent(agent model.AgentComposition, skills []model.SkillBehavior, _ string) string {
	// Cursor has no dedicated agent format — agents are expressed as rules.
	return ""
}

func (g *cursorGenerator) GenerateInstructions(skills []model.SkillBehavior, agents []model.AgentComposition) string {
	return GenerateCursorRules(skills, agents)
}

func (g *cursorGenerator) SkillPath(name string) string { return "rules/" + name + ".mdc" }
func (g *cursorGenerator) AgentPath(name string) string { return "rules/" + name + ".mdc" }
func (g *cursorGenerator) ContextDir() string           { return "context" }
func (g *cursorGenerator) InstructionsPath() string     { return "../.cursorrules" }

func init() {
	spec.Register("cursor", func() spec.Generator { return &cursorGenerator{} })
}
