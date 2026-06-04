package aider_test

import (
	"strings"
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/aider" // register the aider target
	"github.com/mirandaguillaume/reify/internal/generator/aider"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func makeGuardrailString(s string) model.GuardrailRule {
	var g model.GuardrailRule
	_ = g.UnmarshalYAML(&yaml.Node{Kind: yaml.ScalarNode, Value: s})
	return g
}

func fullSkill() model.SkillBehavior {
	return model.SkillBehavior{
		Skill: "payments-service",
		Strategy: model.StrategyFacet{
			Tools: []string{"read_file", "bash"},
			Steps: []string{"read the domain package", "run make test"},
		},
		Guardrails: []model.GuardrailRule{
			makeGuardrailString("Never log the Stripe secret"),
		},
		Observability: model.ObservabilityFacet{
			TraceLevel: model.TraceLevelStandard,
			Metrics:    []string{"settlement_outcome"},
		},
		Security: model.SecurityFacet{
			Filesystem: model.AccessReadWrite,
			Secrets:    []string{"STRIPE_SECRET_KEY"},
		},
	}
}

func TestAider_Registered(t *testing.T) {
	g, err := spec.Get("aider")
	require.NoError(t, err)
	assert.Equal(t, "aider", g.Target())
}

func TestConventions_FacetOrder(t *testing.T) {
	out := aider.GenerateConventionsMd([]model.SkillBehavior{fullSkill()}, nil)
	assert.True(t, strings.HasPrefix(out, "# Coding Conventions"))
	gIdx := strings.Index(out, "Never log the Stripe secret") // guardrail
	stepIdx := strings.Index(out, "run make test")            // strategy
	obsIdx := strings.Index(out, "Observability")
	secIdx := strings.Index(out, "Security")
	assert.Greater(t, stepIdx, gIdx, "guardrails before strategy")
	assert.Greater(t, obsIdx, stepIdx, "observability after strategy")
	assert.Greater(t, secIdx, obsIdx, "security last (recency)")
	assert.Contains(t, out, "STRIPE_SECRET_KEY")
}

func TestConventions_Empty(t *testing.T) {
	assert.Equal(t, "", aider.GenerateConventionsMd(nil, nil))
}

func TestLoadingFiles_EmitsAiderConf(t *testing.T) {
	g, _ := spec.Get("aider")
	lg, ok := g.(spec.LoadingGenerator)
	require.True(t, ok, "aider must implement LoadingGenerator")

	files, warns := lg.LoadingFiles()
	require.Len(t, files, 1)
	assert.Equal(t, "../.aider.conf.yml", files[0].Path, "conf goes to repo root")
	assert.Contains(t, files[0].Content, "read: CONVENTIONS.md", "must name the conventions file so aider loads it")
	require.NotEmpty(t, warns, "must warn about the merge/backup")
	assert.Contains(t, strings.ToLower(strings.Join(warns, " ")), "merge")
}

func TestConfig_DegradesWithWarnings(t *testing.T) {
	g, _ := spec.Get("aider")
	cg, ok := g.(spec.ConfigGenerator)
	require.True(t, ok)

	cfg := model.ProjectConfig{
		MCPServers:   map[string]model.MCPServer{"git": {Command: "npx"}},
		Hooks:        []model.Hook{{Event: "PostToolUse", Matcher: "Edit", Command: "gofmt -w ."}},
		NativeSkills: []model.NativeSkill{{Name: "scaffold", Body: "## Steps\n1. do it"}},
	}
	files, warns := cg.GenerateConfig(cfg)

	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	assert.True(t, paths["mcp.json"])
	assert.True(t, paths["ported-hooks.md"])
	assert.True(t, paths["ported-skills/scaffold.md"])
	require.Len(t, warns, 3, "MCP + hooks + skills each warn")
}
