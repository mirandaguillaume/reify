package importer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectSourceType(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		input    string
		expected SourceType
	}{
		{"vercel prefix", "vercel:my-project", SourceVercel},
		{"existing directory", tmpDir, SourceLocalDir},
		{"file path", "/some/path/agent.md", SourceLocalFile},
		{"relative file", "agent.md", SourceLocalFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectSourceType(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected Framework
	}{
		{"claude path", "/project/.claude/agents/coder.md", FrameworkClaude},
		{"copilot path", "/project/.github/agents/coder.agent.md", FrameworkCopilot},
		{"unknown path", "/project/docs/agent.md", FrameworkUnknown},
		{"nested claude", "/a/b/.claude/skills/test/SKILL.md", FrameworkClaude},
		{"nested copilot", "/a/b/.github/skills/test/SKILL.md", FrameworkCopilot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFramework(tt.path)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveLocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "agent.md")
	content := "# My Agent\nDoes things."
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

	sources, err := ResolveSources(filePath)

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "agent.md", sources[0].Name)
	assert.Equal(t, filePath, sources[0].Path)
	assert.Equal(t, content, sources[0].Content)
}

func TestResolveLocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 2 .md files and 1 .txt file.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.md"), []byte("alpha"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.md"), []byte("beta"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("gamma"), 0644))

	sources, err := ResolveSources(tmpDir)

	require.NoError(t, err)
	assert.Len(t, sources, 2)

	names := make([]string, len(sources))
	for i, s := range sources {
		names[i] = s.Name
	}
	assert.Contains(t, names, "a.md")
	assert.Contains(t, names, "b.md")
}

func TestResolveVercel_FetchesFromGitHub(t *testing.T) {
	skillContent := "---\nname: test-skill\ndescription: A test skill\n---\n# Test Skill\nDoes things."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect path: /{owner}/{repo}/main/skills/{name}/SKILL.md
		expected := "/test-org/test-repo/main/skills/my-skill/SKILL.md"
		if r.URL.Path == expected {
			fmt.Fprint(w, skillContent)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Use fetchSkillFromRepo with a URL that points to our mock server
	// We need to temporarily override the raw GitHub URL base
	sources, err := fetchSkillFromURL(
		server.URL+"/test-org/test-repo/main/skills/my-skill/SKILL.md",
		"my-skill",
	)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "my-skill/SKILL.md", sources[0].Name)
	assert.Contains(t, sources[0].Content, "Test Skill")
	assert.Equal(t, FrameworkUnknown, sources[0].Framework)
}

func TestResolveVercel_SkillNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := fetchSkillFromURL(server.URL+"/org/repo/main/skills/nope/SKILL.md", "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveVercel_ExplicitRepo(t *testing.T) {
	skillContent := "---\nname: custom-skill\ndescription: Custom\n---\n# Custom"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, skillContent)
	}))
	defer server.Close()

	sources, err := fetchSkillFromURL(server.URL+"/custom/repo/main/skills/custom-skill/SKILL.md", "custom-skill")
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Contains(t, sources[0].Content, "Custom")
}

func TestResolveLocalFile_NotFound(t *testing.T) {
	_, err := ResolveSources("/nonexistent/path/agent.md")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading source file")
}
