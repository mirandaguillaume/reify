package reify_test

import (
	"testing"

	_ "github.com/mirandaguillaume/reify/internal/generator/reify"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReifyTarget_Registered(t *testing.T) {
	gen, err := spec.Get("reify")
	require.NoError(t, err)
	assert.Equal(t, "reify", gen.Target())
	assert.Equal(t, ".reify", gen.DefaultOutputDir())
}

func TestReifyTarget_ImplementsAgentGenerator(t *testing.T) {
	gen, _ := spec.Get("reify")
	_, ok := gen.(spec.AgentGenerator)
	assert.True(t, ok, "reify generator must implement AgentGenerator")
}

func TestReifyTarget_NotSkillGenerator(t *testing.T) {
	gen, _ := spec.Get("reify")
	_, ok := gen.(spec.SkillGenerator)
	assert.False(t, ok, "reify generator should NOT implement SkillGenerator (skills are inlined)")
}

func TestReifyTarget_AgentPath(t *testing.T) {
	gen, _ := spec.Get("reify")
	ag := gen.(spec.AgentGenerator)
	assert.Equal(t, "my_agent/main.go", ag.AgentPath("my-agent"))
	assert.Equal(t, "ci_reviewer/main.go", ag.AgentPath("ci-reviewer"))
}
