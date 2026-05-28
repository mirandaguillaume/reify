package reify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

func init() {
	spec.Register("reify", func() spec.Generator {
		return &reifyGenerator{}
	})
}

type reifyGenerator struct{}

func (g *reifyGenerator) Target() string          { return "reify" }
func (g *reifyGenerator) DefaultOutputDir() string { return ".reify" }
func (g *reifyGenerator) ContextDir() string       { return "context" }

// AgentPath returns the relative path for the generated main.go within the output dir.
func (g *reifyGenerator) AgentPath(name string) string {
	safe := safeAgentName(name)
	return filepath.Join(safe, "main.go")
}

// GenerateAgent returns the Go main.go source code for the agent's DAG runtime.
// It also writes the go.mod file as a side effect (the builder only writes AgentPath).
func (g *reifyGenerator) GenerateAgent(agent model.AgentComposition, skills []model.SkillBehavior, outputDir string) string {
	safe := safeAgentName(agent.Agent)
	modDir := filepath.Join(outputDir, safe)
	if err := os.MkdirAll(modDir, 0755); err == nil {
		modContent := GenerateGoMod(agent.Agent)
		if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte(modContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warn: write go.mod for %s: %v\n", safe, err)
		}
		if err := os.WriteFile(filepath.Join(modDir, "go.sum"), []byte(""), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warn: write go.sum for %s: %v\n", safe, err)
		}
	}
	return GenerateAgentGo(agent, skills)
}

// safeAgentName converts an agent name to a filesystem/module-safe identifier.
func safeAgentName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}
