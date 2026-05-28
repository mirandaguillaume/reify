package spike

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "yaml fenced",
			input: "```yaml\nfindings:\n  - category: guardrails\n```",
			want:  "findings:\n  - category: guardrails",
		},
		{
			name:  "yml fenced",
			input: "```yml\nfindings:\n  - category: security\n```",
			want:  "findings:\n  - category: security",
		},
		{
			name:  "plain fenced",
			input: "```\nfindings:\n  - category: ordering\n```",
			want:  "findings:\n  - category: ordering",
		},
		{
			name:  "no fences",
			input: "findings:\n  - category: context",
			want:  "findings:\n  - category: context",
		},
		{
			name:  "text before fence",
			input: "Here are the findings:\n```yaml\nfindings:\n  - category: guardrails\n```\nEnd.",
			want:  "findings:\n  - category: guardrails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractYAML(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFindings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantN   int
		wantErr bool
	}{
		{
			name: "valid findings",
			input: `findings:
  - category: guardrails
    issue: "No timeout specified"
    confidence: high
    current_state: "No guardrails section"
    suggested_improvement: "Add timeout and output limits"
  - category: security
    issue: "No security declarations"
    confidence: moderate
    current_state: "No security section"
    suggested_improvement: "Add filesystem and network declarations"`,
			wantN: 2,
		},
		{
			name:    "empty findings",
			input:   "findings: []",
			wantN:   0,
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			input:   "not: [valid: yaml: {{",
			wantN:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := ParseFindings(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, findings, tt.wantN)
			}
		})
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"testdata/claude/zen-qa.md", "claude-code"},
		{"testdata/copilot/maui-sandbox-agent.md", "github-copilot"},
		{"random/file.md", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectFormat(tt.path))
		})
	}
}

func TestAnalysisPrompt(t *testing.T) {
	prompt := AnalysisPrompt("claude-code", "# My Agent\nDo stuff")
	assert.Contains(t, prompt, "claude-code")
	assert.Contains(t, prompt, "# My Agent")
	assert.Contains(t, prompt, "GUARDRAILS")
	assert.Contains(t, prompt, "SECURITY")
	assert.Contains(t, prompt, "ORDERING")
	assert.Contains(t, prompt, "findings:")
}

func TestLoadTestCorpus(t *testing.T) {
	testdataDir := "../parser/testdata"
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("testdata directory not found — run from project root")
	}
	files, err := LoadTestCorpus(testdataDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 10, "should have at least 10 test files")
}

// TestRunSpike is the main spike execution — requires OPENROUTER_API_KEY
func TestRunSpike(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping spike in short mode — requires API calls")
	}
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set — skipping spike execution")
	}

	testdataDir := "../parser/testdata"
	files, err := LoadTestCorpus(testdataDir)
	require.NoError(t, err)

	// Spike uses 5 files per model — sufficient for validation (ADR-8 accepted)
	maxFiles := 5
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	t.Logf("Testing %d files per model", len(files))

	var summaries []ModelSummary

	for _, model := range TestModels {
		t.Logf("Testing model: %s", model)
		var results []FileResult
		for _, f := range files {
			t.Logf("  Analyzing: %s", f)
			result := AnalyzeFile(apiKey, model, f)
			t.Logf("    Parseable: %v, Findings: %d, Latency: %dms",
				result.Parseable, result.FindingsN, result.LatencyMs)
			if result.ParseError != "" {
				t.Logf("    Error: %s", result.ParseError)
			}
			results = append(results, result)
		}
		summaries = append(summaries, Summarize(model, results))
	}

	// Cloud baseline
	t.Logf("Testing cloud baseline: %s", CloudBaseline)
	var cloudResults []FileResult
	for _, f := range files {
		t.Logf("  Analyzing: %s", f)
		result := AnalyzeFile(apiKey, CloudBaseline, f)
		t.Logf("    Parseable: %v, Findings: %d, Latency: %dms",
			result.Parseable, result.FindingsN, result.LatencyMs)
		cloudResults = append(cloudResults, result)
	}
	cloudSummary := Summarize(CloudBaseline, cloudResults)

	// Generate report
	report := FormatReport(summaries, &cloudSummary)
	t.Log("\n" + report)

	// Write report to file
	reportPath := "../../../docs/plans/2026-03-spike-llm-quality.md"
	err = os.WriteFile(reportPath, []byte(report), 0644)
	if err != nil {
		t.Logf("Warning: could not write report to %s: %v", reportPath, err)
	} else {
		t.Logf("Report written to: %s", reportPath)
	}

	// Log summary for quick review
	t.Log("\n=== SUMMARY ===")
	for _, s := range summaries {
		t.Logf("%-50s Parse: %.0f%%  Findings: %.1f  Latency: %dms",
			s.Model, s.ParseRate, s.AvgFindings, s.AvgLatencyMs)
	}
	t.Logf("%-50s Parse: %.0f%%  Findings: %.1f  Latency: %dms (BASELINE)",
		cloudSummary.Model, cloudSummary.ParseRate, cloudSummary.AvgFindings, cloudSummary.AvgLatencyMs)
}
