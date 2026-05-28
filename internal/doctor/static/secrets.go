package static

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
)

func init() {
	RegisterCheck(&secretsCheck{})
}

type secretPattern struct {
	name    string
	pattern *regexp.Regexp
}

// Ordered: specific patterns first, generic patterns last.
// The first match per line wins, so specific tokens must precede generic "token=" patterns.
var secretPatterns = []secretPattern{
	{"Anthropic API key", regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{90,}`)},
	{"OpenAI API key", regexp.MustCompile(`sk-proj-[a-zA-Z0-9]{20,}|sk-[a-zA-Z0-9]{48,}`)},
	{"AWS access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"GitHub PAT", regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`)},
	{"GitHub OAuth token", regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`)},
	{"GitHub App token", regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`)},
	{"GitLab PAT", regexp.MustCompile(`glpat-[a-zA-Z0-9\-_]{20,}`)},
	{"Slack bot token", regexp.MustCompile(`xoxb-[0-9]{10,}`)},
	{"Slack user token", regexp.MustCompile(`xoxp-[0-9]{10,}`)},
	{"Private key", regexp.MustCompile(`-----BEGIN (RSA |EC )?PRIVATE KEY-----`)},
	{"Stripe key", regexp.MustCompile(`(sk|pk)_live_[a-zA-Z0-9]{24,}`)},
	{"npm token", regexp.MustCompile(`npm_[a-zA-Z0-9]{36,}`)},
	{"SendGrid API key", regexp.MustCompile(`SG\.[a-zA-Z0-9_\-]{22,}\.[a-zA-Z0-9_\-]{22,}`)},
	{"Password assignment", regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`)},
	{"Secret assignment", regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`)},
	{"Token assignment", regexp.MustCompile(`(?i)\btoken\s*[:=]\s*\S+`)},
	{"API key assignment", regexp.MustCompile(`(?i)api_key\s*[:=]\s*\S+`)},
}

// placeholderIndicators are words near a match that suggest it's an example, not a real secret.
var placeholderIndicators = []string{"example", "placeholder", "your_", "xxx", "<your", "replace", "dummy", "fake", "sample"}

type secretsCheck struct{}

func (s *secretsCheck) ID() string              { return "secret-scanning" }
func (s *secretsCheck) Tags() []string          { return []string{"default", "security"} }
func (s *secretsCheck) Category() string        { return "security" }
func (s *secretsCheck) DefaultSeverity() string { return "high" }

func (s *secretsCheck) Run(analysis *parser.AgentAnalysis) []llmutil.Finding {
	if analysis == nil || len(analysis.RawContent) == 0 {
		return nil
	}

	lines := strings.Split(NormalizeContent(string(analysis.RawContent)), "\n")
	inCodeFence := false
	var findings []llmutil.Finding

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if IsCodeFenceLine(trimmed) {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}

		for _, sp := range secretPatterns {
			if sp.pattern.MatchString(line) {
				// Check if this looks like a placeholder/example
				if isPlaceholder(line) {
					continue
				}
				// Skip generic patterns (password/secret/token/api_key assignments)
				// when the line is about configuring secrets, not containing them.
				if strings.Contains(sp.name, "assignment") && isConfigContext(line) {
					continue
				}
				findings = append(findings, llmutil.Finding{
					Category:             "security",
					Issue:                fmt.Sprintf("Possible %s detected on line %d", sp.name, lineNum+1),
					Confidence:           "high",
					CitationID:           "security",
					CurrentState:         fmt.Sprintf("Line %d matches %s pattern", lineNum+1, sp.name),
					SuggestedImprovement: "Remove the secret and use environment variables or a secrets manager instead",
				})
				break // one finding per line
			}
		}
	}

	return findings
}

// configContextIndicators are words near a match that suggest the line is
// documentation about configuring a secret, not an actual secret value.
var configContextIndicators = []string{"configure", "set ", "env var", "variable", "environment"}

func isPlaceholder(line string) bool {
	lower := strings.ToLower(line)
	for _, indicator := range placeholderIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func isConfigContext(line string) bool {
	lower := strings.ToLower(line)
	for _, indicator := range configContextIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}
