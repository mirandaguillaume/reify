//go:build validation

// Real-harness validation for the aider target: emit CONVENTIONS.md + the
// .aider.conf.yml loader, then run the actual `aider` CLI and assert it loads
// the conventions file (harness acceptance, not self-consistency). Gated behind
// the `validation` build tag and skips when aider isn't installed.
//
//	go test -tags validation ./internal/generator/aider/
package aider_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mirandaguillaume/reify/internal/generator/aider"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidation_AiderLoadsConventions(t *testing.T) {
	bin, err := exec.LookPath("aider")
	if err != nil {
		t.Skip("aider CLI not installed")
	}

	dir := t.TempDir()
	// CONVENTIONS.md (instructions) at the project root.
	skills := []model.SkillBehavior{{
		Skill:      "payments",
		Guardrails: []model.GuardrailRule{makeGuardrailString("Never log secrets")},
	}}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CONVENTIONS.md"),
		[]byte(aider.GenerateConventionsMd(skills, nil)), 0o644))

	// .aider.conf.yml (loader) — exactly what reify emits.
	g, _ := spec.Get("aider")
	lg := g.(spec.LoadingGenerator)
	files, _ := lg.LoadingFiles()
	for _, f := range files {
		// f.Path is "../.aider.conf.yml" relative to the staging dir; here we
		// write it directly at the project root.
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".aider.conf.yml"), []byte(f.Content), 0o644))
	}

	cmd := exec.Command(bin, "--exit", "--no-git", "--yes-always")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "OPENAI_API_KEY=sk-dummy")
	out, _ := cmd.CombinedOutput() // aider may exit non-zero on the dummy key; we assert on output

	assert.Contains(t, string(out), "CONVENTIONS.md",
		"the real aider must load the conventions file via the emitted .aider.conf.yml:\n%s", out)
}
