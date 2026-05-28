package static

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&stalenessCheck{})
}

const stalenessThreshold = 90 * 24 * time.Hour // 90 days

type stalenessCheck struct{}

func (s *stalenessCheck) ID() string              { return "git-staleness" }
func (s *stalenessCheck) Tags() []string          { return []string{"git-aware"} }
func (s *stalenessCheck) Category() string        { return "version_drift" }
func (s *stalenessCheck) DefaultSeverity() string { return "low" }

func (s *stalenessCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil {
		return nil
	}

	// Need the file path — stored in Frontmatter by convention (set by caller)
	filePath, ok := analysis.Frontmatter["_file_path"]
	if !ok {
		return nil
	}
	fp, ok := filePath.(string)
	if !ok || fp == "" {
		return nil
	}

	// Check if in a git repo
	if !isGitRepo() {
		return nil
	}

	lastModified := gitLastModified(fp)
	if lastModified.IsZero() {
		return nil
	}

	age := time.Since(lastModified)
	if age < stalenessThreshold {
		return nil
	}

	days := int(age.Hours() / 24)
	return []llmutil.Finding{{
		Category:             "version_drift",
		Issue:                fmt.Sprintf("Agent file not modified in %d days", days),
		Confidence:           "low",
		CitationID:           "version_drift",
		CurrentState:         fmt.Sprintf("Last git modification: %s (%d days ago)", lastModified.Format("2006-01-02"), days),
		SuggestedImprovement: "Review this agent file for stale content, outdated references, and alignment with current codebase",
	}}
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func gitLastModified(filePath string) time.Time {
	cmd := exec.Command("git", "log", "-1", "--format=%ct", "--", filePath)
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}
