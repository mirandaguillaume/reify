package doctor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
)

var lineNumberRe = regexp.MustCompile(`line (\d+)`)

// RenderGitHubAnnotations outputs findings as GitHub Actions annotations.
func RenderGitHubAnnotations(findings []llmutil.Finding, filePath string) {
	for _, f := range findings {
		sev := f.Severity
		if sev == "" {
			sev = f.Confidence
		}
		level := "warning"
		if strings.ToLower(sev) == "high" {
			level = "error"
		}

		line := extractLineNumber(f.Issue)
		if line == 0 {
			line = extractLineNumber(f.CurrentState)
		}

		msg := f.Issue
		if f.SuggestedImprovement != "" {
			msg += " — " + f.SuggestedImprovement
		}

		escaped := escapeAnnotation(msg)
		if line > 0 {
			fmt.Printf("::%s file=%s,line=%d::%s\n", level, filePath, line, escaped)
		} else {
			fmt.Printf("::%s file=%s::%s\n", level, filePath, escaped)
		}
	}
}

// RenderGateAnnotation outputs the gate result as a GitHub Actions annotation.
func RenderGateAnnotation(result GateResult) {
	if result.Pass {
		fmt.Println("::notice::Doctor quality gate passed")
	} else {
		for _, f := range result.Failures {
			fmt.Printf("::error::Quality gate failed: %s\n", escapeAnnotation(f))
		}
	}
	for _, w := range result.Warnings {
		fmt.Printf("::warning::Quality gate warning: %s\n", escapeAnnotation(w))
	}
}

// escapeAnnotation escapes special characters for GitHub Actions workflow commands.
func escapeAnnotation(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, "::", " - ")
	return s
}

func extractLineNumber(text string) int {
	matches := lineNumberRe.FindStringSubmatch(text)
	if len(matches) >= 2 {
		n, _ := strconv.Atoi(matches[1])
		return n
	}
	return 0
}
