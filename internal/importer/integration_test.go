package importer

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_SimpleReviewer(t *testing.T) {
	// Read golden input
	input, err := os.ReadFile("testdata/input/simple-reviewer.md")
	require.NoError(t, err)

	// Mock provider returns a pre-built valid response
	provider := &testProvider{
		response: `{"skills": [{"yaml": "skill: code-reviewer\nversion: \"0.1.0\"\ncontext:\n  consumes: [git_diff, source_code]\n  produces: [review_comments]\n  memory: short-term\nstrategy:\n  tools: [read_file, grep, bash]\n  approach: sequential\n  steps:\n    - Read the git diff\n    - Check for security, performance, and style issues\n    - Write review comments with specific suggestions\nguardrails:\n  - timeout: 120s\n  - Be constructive and focus on bugs over style\nobservability:\n  trace_level: standard\n  metrics: [comments_count, issues_found]\nsecurity:\n  filesystem: read-only\n  network: none\n  secrets: []\nnegotiation:\n  file_conflicts: yield\n  priority: 0"}], "agent": null}`,
	}

	// Write input to temp file
	dir := t.TempDir()
	inputPath := dir + "/code-reviewer.md"
	os.WriteFile(inputPath, input, 0644)

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
		MinScore: 0,
		OutputDir: dir,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	assert.Len(t, result.Skills, 1)

	skill := result.Skills[0].Skill
	assert.Equal(t, "code-reviewer", skill.Skill)
	assert.Contains(t, skill.Context.Consumes, "git_diff")
	assert.Contains(t, skill.Context.Produces, "review_comments")
	assert.Contains(t, skill.Strategy.Tools, "read_file")
	assert.Nil(t, result.Agent)

	// Score should be reasonable
	assert.Greater(t, result.Skills[0].Score.Total, 40)

	// Write and verify output
	written, err := WriteImportResult(result, dir)
	require.NoError(t, err)
	assert.Len(t, written, 1)

	// Verify written file is valid YAML
	content, err := os.ReadFile(written[0])
	require.NoError(t, err)
	assert.Contains(t, string(content), "code-reviewer")
}

func TestIntegration_CopilotAgent(t *testing.T) {
	// Read the Copilot-style fixture.
	input, err := os.ReadFile("testdata/input/copilot-agent.agent.md")
	require.NoError(t, err)

	// Write to a temp path under .github/agents/ so DetectFramework returns FrameworkCopilot.
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".github", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
	inputPath := filepath.Join(agentDir, "code-reviewer.agent.md")
	require.NoError(t, os.WriteFile(inputPath, input, 0644))

	// Verify framework detection before running import.
	assert.Equal(t, FrameworkCopilot, DetectFramework(inputPath))

	// Mock provider returns a single skill.
	provider := &testProvider{
		response: buildMockResponse([]string{"code-reviewer"}, ""),
	}

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	require.Len(t, result.Skills, 1)

	skill := result.Skills[0].Skill
	assert.Equal(t, "code-reviewer", skill.Skill)
	assert.NotEmpty(t, skill.Context.Consumes)
	assert.NotEmpty(t, skill.Context.Produces)
	assert.NotEmpty(t, skill.Strategy.Tools)
	assert.NotEmpty(t, skill.Version)
	assert.NotEmpty(t, result.Skills[0].RawYAML)
	assert.Nil(t, result.Agent)
}

func TestIntegration_NoFrontmatter(t *testing.T) {
	// Read the no-frontmatter fixture.
	input, err := os.ReadFile("testdata/input/no-frontmatter.md")
	require.NoError(t, err)

	// Write to temp file (no special path — FrameworkUnknown).
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "research-assistant.md")
	require.NoError(t, os.WriteFile(inputPath, input, 0644))

	// Mock provider returns a single skill inferred from body.
	provider := &testProvider{
		response: buildMockResponse([]string{"research-assistant"}, ""),
	}

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "research-assistant", result.Skills[0].Skill.Skill)
	assert.Empty(t, result.Error)
	assert.Nil(t, result.Agent)
}

func TestIntegration_MultiResponsibility(t *testing.T) {
	// Read the multi-responsibility fixture.
	input, err := os.ReadFile("testdata/input/multi-responsibility.md")
	require.NoError(t, err)

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "ci-pipeline.md")
	require.NoError(t, os.WriteFile(inputPath, input, 0644))

	// Mock provider returns 4 skills + 1 agent.
	provider := &testProvider{
		response: buildMockResponse(
			[]string{"lint-checker", "test-runner", "deployer", "report-generator"},
			"ci-pipeline",
		),
	}

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	require.Len(t, result.Skills, 4)

	// Verify all 4 skills exist with non-zero scores.
	expectedSkills := []string{"lint-checker", "test-runner", "deployer", "report-generator"}
	for i, expected := range expectedSkills {
		assert.Equal(t, expected, result.Skills[i].Skill.Skill)
		assert.Greater(t, result.Skills[i].Score.Total, 0, "skill %q should have non-zero score", expected)
	}

	// Verify agent is present and references all skills.
	require.NotNil(t, result.Agent)
	assert.Equal(t, "ci-pipeline", result.Agent.Agent.Agent)
	assert.Equal(t, expectedSkills, result.Agent.Agent.Skills)
}

// retryProvider is a stateful mock that returns different responses on each call.
type retryProvider struct {
	calls     int
	responses []string
}

func (p *retryProvider) Complete(prompt string) (string, error) {
	idx := p.calls
	if idx >= len(p.responses) {
		idx = len(p.responses) - 1
	}
	p.calls++
	return p.responses[idx], nil
}

// weakSkillYAML returns a skill YAML that scores below 60 (no guardrails, no
// metrics, minimal trace, no steps).
func weakSkillYAML(name string) string {
	return "skill: " + name + "\n" +
		"version: \"1.0.0\"\n" +
		"context:\n" +
		"  consumes: [\"source-code\"]\n" +
		"  produces: [\"lint-report\"]\n" +
		"  memory: short-term\n" +
		"strategy:\n" +
		"  tools: [\"read_file\"]\n" +
		"  approach: Analyze code\n" +
		"observability:\n" +
		"  trace_level: minimal\n" +
		"security:\n" +
		"  filesystem: read-only\n" +
		"  network: none\n" +
		"  secrets: []\n" +
		"negotiation:\n" +
		"  file_conflicts: yield\n" +
		"  priority: 0\n"
}

// strongSkillYAML returns a skill YAML that scores above 60 (guardrails,
// metrics, standard trace, steps).
func strongSkillYAML(name string) string {
	return "skill: " + name + "\n" +
		"version: \"1.0.0\"\n" +
		"context:\n" +
		"  consumes: [\"source-code\"]\n" +
		"  produces: [\"lint-report\"]\n" +
		"  memory: short-term\n" +
		"strategy:\n" +
		"  tools: [\"read_file\", \"grep\"]\n" +
		"  approach: Analyze source code for issues\n" +
		"  steps:\n" +
		"    - Read the source files\n" +
		"    - Run linting rules\n" +
		"    - Produce report\n" +
		"guardrails:\n" +
		"  - \"timeout: 30s\"\n" +
		"  - \"Be constructive and focus on bugs over style\"\n" +
		"observability:\n" +
		"  trace_level: standard\n" +
		"  metrics: [\"duration\", \"issues_found\"]\n" +
		"security:\n" +
		"  filesystem: read-only\n" +
		"  network: none\n" +
		"  secrets: []\n" +
		"negotiation:\n" +
		"  file_conflicts: yield\n" +
		"  priority: 1\n"
}

func buildWeakResponse(name string) string {
	type skillEntry struct {
		YAML string `json:"yaml"`
	}
	type response struct {
		Skills []skillEntry `json:"skills"`
		Agent  *struct {
			YAML string `json:"yaml"`
		} `json:"agent"`
	}
	resp := response{
		Skills: []skillEntry{{YAML: weakSkillYAML(name)}},
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

func buildStrongResponse(name string) string {
	type skillEntry struct {
		YAML string `json:"yaml"`
	}
	type response struct {
		Skills []skillEntry `json:"skills"`
		Agent  *struct {
			YAML string `json:"yaml"`
		} `json:"agent"`
	}
	resp := response{
		Skills: []skillEntry{{YAML: strongSkillYAML(name)}},
	}
	data, _ := json.Marshal(resp)
	return string(data)
}

func TestRunImport_RetryLoop(t *testing.T) {
	// Create a simple input file.
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "retry-agent.md")
	require.NoError(t, os.WriteFile(inputPath, []byte("# Retry Agent\n\nLints code.\n"), 0644))

	// 1st call returns weak skill (score ~47, below MinScore 60).
	// 2nd call returns strong skill (score ~85, above MinScore 60).
	provider := &retryProvider{
		responses: []string{
			buildWeakResponse("code-linter"),
			buildStrongResponse("code-linter"),
		},
	}

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
		MinScore: 60,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	require.Len(t, result.Skills, 1)

	// The provider should have been called twice (initial + retry).
	assert.Equal(t, 2, provider.calls, "expected 2 provider calls (initial + retry)")

	// The result should use the 2nd (stronger) response.
	skill := result.Skills[0].Skill
	assert.Equal(t, "code-linter", skill.Skill)
	assert.Greater(t, result.Skills[0].Score.Total, 60,
		"retry result should score above MinScore threshold")
	assert.NotEmpty(t, skill.Guardrails, "retry result should have guardrails")
	assert.NotEmpty(t, skill.Observability.Metrics, "retry result should have metrics")
	assert.Len(t, skill.Strategy.Steps, 3, "retry result should have 3 steps")
}

func TestRunImport_DirectoryImport(t *testing.T) {
	// Create a temp directory with 2 .md files.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "agent-a.md"),
		[]byte("---\nname: agent-a\ndescription: First agent\ntools: [Read]\n---\n\n# Agent A\n\nDoes things.\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "agent-b.md"),
		[]byte("---\nname: agent-b\ndescription: Second agent\ntools: [Bash]\n---\n\n# Agent B\n\nDoes other things.\n"),
		0644,
	))

	// Mock provider returns a skill based on whichever file is processed.
	provider := &testProvider{
		response: buildMockResponse([]string{"agent-a-skill"}, ""),
	}

	result := RunImport(ImportOptions{
		Source:   dir,
		Provider: provider,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)

	// RunImport currently only processes the first source (sources[0]).
	// This documents the current single-file behavior for directory inputs.
	require.Len(t, result.Skills, 1,
		"RunImport processes only the first file from a directory")
	assert.Equal(t, "agent-a-skill", result.Skills[0].Skill.Skill)
	assert.Nil(t, result.Agent)
}

// --- Real Vercel skills integration tests ---
// These tests fetch real SKILL.md files from GitHub and verify our
// frontmatter extraction and source resolution work with real-world data.
// They skip automatically when there's no network access.

func canReachGitHub(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head("https://raw.githubusercontent.com")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("skipping: cannot reach raw.githubusercontent.com")
	}
}

func TestVercelSkill_FindSkills(t *testing.T) {
	canReachGitHub(t)

	sources, err := ResolveSources("vercel:find-skills")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	source := sources[0]
	assert.Equal(t, "find-skills/SKILL.md", source.Name)
	assert.Contains(t, source.Content, "find-skills")

	// Verify frontmatter extraction works with real content
	fm, body, err := ExtractFrontmatter(source.Content)
	require.NoError(t, err)
	assert.Equal(t, "find-skills", fm.Name)
	assert.NotEmpty(t, fm.Description)
	assert.NotEmpty(t, body)
}

func TestVercelSkill_UseAiSdk(t *testing.T) {
	canReachGitHub(t)

	// This skill lives in vercel/ai, not vercel-labs/skills
	sources, err := ResolveSources("vercel:use-ai-sdk")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	source := sources[0]
	assert.Contains(t, source.Content, "AI SDK")

	// Verify frontmatter extraction
	fm, body, err := ExtractFrontmatter(source.Content)
	require.NoError(t, err)
	// The name in frontmatter is "ai-sdk" not "use-ai-sdk"
	assert.NotEmpty(t, fm.Name)
	assert.NotEmpty(t, fm.Description)
	assert.NotEmpty(t, body)
}

func TestVercelSkill_AdrSkill(t *testing.T) {
	canReachGitHub(t)

	sources, err := ResolveSources("vercel:adr-skill")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	source := sources[0]
	assert.Contains(t, source.Content, "ADR")

	fm, body, err := ExtractFrontmatter(source.Content)
	require.NoError(t, err)
	assert.Equal(t, "adr-skill", fm.Name)
	assert.NotEmpty(t, body)
}

func TestVercelSkill_FullImportPipeline(t *testing.T) {
	canReachGitHub(t)

	// Fetch a real skill and run it through the full import pipeline
	// (with a mock LLM — we're testing source resolution + frontmatter, not the LLM)
	sources, err := ResolveSources("vercel:find-skills")
	require.NoError(t, err)
	require.Len(t, sources, 1)

	// Write fetched content to a temp file (RunImport reads from disk)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "find-skills.md")
	require.NoError(t, os.WriteFile(inputPath, []byte(sources[0].Content), 0644))

	provider := &testProvider{
		response: buildMockResponse([]string{"find-skills"}, ""),
	}

	result := RunImport(ImportOptions{
		Source:   inputPath,
		Provider: provider,
		MinScore: 0,
		OutputDir: dir,
	})

	require.True(t, result.Success, "import failed: %s", result.Error)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "find-skills", result.Skills[0].Skill.Skill)

	// Write the result and verify
	written, err := WriteImportResult(result, dir)
	require.NoError(t, err)
	assert.Len(t, written, 1)
}
