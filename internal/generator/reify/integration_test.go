package reify_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mirandaguillaume/reify/internal/generator/reify"
)

func TestIntegration_GeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	gen, err := spec.Get("reify")
	require.NoError(t, err)
	ag := gen.(spec.AgentGenerator)

	agent := model.AgentComposition{
		Agent:    "test-agent",
		Skills:   []string{"analyzer"},
		Consumes: []string{"input_data"},
		Produces: []string{"output_data"},
	}
	skills := []model.SkillBehavior{
		{
			Skill:   "analyzer",
			Version: "1.0.0",
			Context: model.ContextFacet{
				Consumes: []string{"input_data"},
				Produces: []string{"output_data"},
			},
			Strategy: model.StrategyFacet{
				Approach: "analysis",
				Steps:    []string{"analyze the input", "produce output"},
			},
			Security: model.SecurityFacet{
				Filesystem: model.AccessReadOnly,
				Network:    model.NetworkNone,
			},
		},
	}

	tmpDir := t.TempDir()
	code := ag.GenerateAgent(agent, skills, tmpDir)

	// Write main.go
	agentDir := filepath.Join(tmpDir, "test_agent")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "main.go"), []byte(code), 0644))

	// Write go.mod pointing to actual repo root
	repoRoot, err := filepath.Abs("../../..")
	require.NoError(t, err)
	goMod := "module test_agent\n\ngo 1.22\n\nrequire github.com/mirandaguillaume/reify v0.0.0\n\nreplace github.com/mirandaguillaume/reify => " + repoRoot + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "go.mod"), []byte(goMod), 0644))

	// go mod tidy
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = agentDir
	tidyOut, tidyErr := tidy.CombinedOutput()
	require.NoError(t, tidyErr, "go mod tidy must succeed:\n%s", string(tidyOut))

	// go build
	build := exec.Command("go", "build", ".")
	build.Dir = agentDir
	buildOut, buildErr := build.CombinedOutput()

	assert.NoError(t, buildErr, "generated code must compile:\n%s", string(buildOut))
}
