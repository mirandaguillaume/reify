package importer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SourceType indicates where an import source comes from.
type SourceType int

const (
	SourceLocalFile SourceType = iota
	SourceLocalDir
	SourceVercel
)

// Framework identifies the agent framework a source belongs to.
type Framework int

const (
	FrameworkUnknown Framework = iota
	FrameworkClaude
	FrameworkCopilot
)

// String returns the lowercase name of the framework for display and prompt use.
func (f Framework) String() string {
	switch f {
	case FrameworkClaude:
		return "claude"
	case FrameworkCopilot:
		return "copilot"
	default:
		return "unknown"
	}
}

// Source represents a resolved import source with its content and framework.
type Source struct {
	Name      string
	Path      string
	Content   string
	Framework Framework
}

// DetectSourceType determines the type of import source from an input string.
// A "vercel:" prefix indicates a Vercel source; an existing directory indicates
// a local directory; everything else is treated as a local file.
func DetectSourceType(input string) SourceType {
	if strings.HasPrefix(input, "vercel:") {
		return SourceVercel
	}
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		return SourceLocalDir
	}
	return SourceLocalFile
}

// DetectFramework guesses the agent framework from a file path.
func DetectFramework(path string) Framework {
	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, ".claude/") {
		return FrameworkClaude
	}
	if strings.Contains(normalized, ".github/") {
		return FrameworkCopilot
	}
	return FrameworkUnknown
}

// ResolveSources resolves the input string into a list of Source values.
// For a local file it reads the file content; for a directory it globs *.md
// files; for Vercel it returns an error indicating the feature is not yet
// implemented.
func ResolveSources(input string) ([]Source, error) {
	st := DetectSourceType(input)
	switch st {
	case SourceVercel:
		return resolveVercel(input)
	case SourceLocalDir:
		return resolveDir(input)
	default:
		return resolveFile(input)
	}
}

func resolveFile(path string) ([]Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading source file: %w", err)
	}
	return []Source{
		{
			Name:      filepath.Base(path),
			Path:      path,
			Content:   string(data),
			Framework: DetectFramework(path),
		},
	}, nil
}

func resolveDir(dir string) ([]Source, error) {
	pattern := filepath.Join(dir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing directory: %w", err)
	}

	var sources []Source
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", m, err)
		}
		sources = append(sources, Source{
			Name:      filepath.Base(m),
			Path:      m,
			Content:   string(data),
			Framework: DetectFramework(m),
		})
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no .md files found in %s", dir)
	}
	return sources, nil
}

// defaultVercelRepos lists the GitHub repos to search for Vercel skills.
// Format: "owner/repo"
var defaultVercelRepos = []string{
	"vercel-labs/skills",
	"vercel/ai",
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// resolveVercel fetches a skill's SKILL.md from GitHub repos.
// Input format: "vercel:skill-name" or "vercel:owner/repo:skill-name"
func resolveVercel(input string) ([]Source, error) {
	ref := strings.TrimPrefix(input, "vercel:")

	// Check if user specified a repo: "vercel:owner/repo:skill-name"
	if parts := strings.SplitN(ref, ":", 2); len(parts) == 2 {
		repo := parts[0]
		skillName := parts[1]
		return fetchSkillFromRepo(repo, skillName)
	}

	// Search default repos for the skill name
	skillName := ref
	for _, repo := range defaultVercelRepos {
		sources, err := fetchSkillFromRepo(repo, skillName)
		if err == nil {
			return sources, nil
		}
	}

	return nil, fmt.Errorf("skill %q not found in default repos (%s)", skillName, strings.Join(defaultVercelRepos, ", "))
}

func fetchSkillFromRepo(repo, skillName string) ([]Source, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/skills/%s/SKILL.md", repo, skillName)
	return fetchSkillFromURL(url, skillName)
}

func fetchSkillFromURL(url, skillName string) ([]Source, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill %q not found (HTTP %d)", skillName, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return []Source{{
		Name:      skillName + "/SKILL.md",
		Path:      url,
		Content:   string(body),
		Framework: FrameworkUnknown,
	}}, nil
}
