package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

func TestSecrets_DetectsOpenAIProjectKey(t *testing.T) {
	// sk-proj- prefix + 20+ chars (new project-scoped keys)
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Use this key: sk-proj-abcdefghijklmnopqrstuvwx"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "OpenAI")
	assert.Equal(t, "high", findings[0].Confidence)
}

func TestSecrets_DetectsOpenAILegacyKey(t *testing.T) {
	// sk- prefix + 48+ chars (legacy format)
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Use this key: sk-ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwx"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "OpenAI")
}

func TestSecrets_IgnoresShortSkPrefix(t *testing.T) {
	// Short sk- prefix should NOT match (e.g., "sk-learn" references)
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Use sk-learn for machine learning"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "short sk- prefix should not be detected as OpenAI key")
}

func TestSecrets_DetectsAWSKey(t *testing.T) {
	// Use a realistic-looking key (not containing "example" which triggers placeholder detection)
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("AWS_ACCESS_KEY_ID=AKIAIOSFODNN7ABCDEFG"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "AWS")
}

func TestSecrets_DetectsGitHubPAT(t *testing.T) {
	// ghp_ + exactly 36 alphanumeric chars
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "GitHub PAT")
}

func TestSecrets_SkipsPlaceholders(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("Example: sk-your_api_key_here_replace_me_placeholder"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Empty(t, findings, "placeholder indicators should suppress finding")
}

func TestSecrets_DetectsPasswordAssignment(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("password = mysecretpassword123"),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "Password")
}

func TestSecrets_DetectsPrivateKey(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBA..."),
	}
	check := &secretsCheck{}
	findings := check.Run(analysis)
	assert.Len(t, findings, 1)
	assert.Contains(t, findings[0].Issue, "Private key")
}

func TestSecrets_NoSecrets(t *testing.T) {
	analysis := &parser.AgentAnalysis{
		RawContent: []byte("## Rules\nAlways run tests.\nNever skip the linter."),
	}
	check := &secretsCheck{}
	assert.Empty(t, check.Run(analysis))
}

func TestSecrets_SkipsConfigContext(t *testing.T) {
	// Lines about configuring secrets should not trigger generic pattern findings
	cases := []string{
		"Set the environment variable PASSWORD=your_password",
		"Configure your SECRET_KEY environment variable",
		"Set your API_KEY env var before running",
	}
	check := &secretsCheck{}
	for _, line := range cases {
		analysis := &parser.AgentAnalysis{RawContent: []byte(line)}
		findings := check.Run(analysis)
		assert.Empty(t, findings, "config context line should be suppressed: %s", line)
	}
}

func TestSecrets_NilAnalysis(t *testing.T) {
	check := &secretsCheck{}
	assert.Nil(t, check.Run(nil))
}
