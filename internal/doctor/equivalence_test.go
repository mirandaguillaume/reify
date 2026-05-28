package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mirandaguillaume/reify/internal/doctor/analyzer"
	doctorctx "github.com/mirandaguillaume/reify/internal/doctor/context"
	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/doctor/registry"
	"github.com/mirandaguillaume/reify/internal/llm"
)

// deterministicProvider returns a fixed YAML response for any prompt.
type deterministicProvider struct{}

func (p *deterministicProvider) Complete(prompt string) (string, error) {
	return `findings:
- category: "tool-integration"
  issue: "Agent lacks explicit error handling guidance"
  confidence: "moderate"
  current_state: "No error handling section present"
  suggested_improvement: "Add error handling guidelines"
- category: "context-management"
  issue: "Agent does not define context boundaries"
  confidence: "low"
  current_state: "No context management section"
  suggested_improvement: "Define explicit context boundaries"
`, nil
}

var _ llm.Provider = (*deterministicProvider)(nil)

// directPath runs the analysis pipeline using direct Go function calls
// (the pre-DAG approach) and returns the combined findings.
func directPath(t *testing.T, filePath string, content []byte, provider llm.Provider, reg *registry.Registry) []llmutil.Finding {
	t.Helper()

	p, err := parser.DetectFormat(filePath, content)
	require.NoError(t, err, "direct: detect format for %s", filePath)

	analysis, err := p.Parse(content)
	require.NoError(t, err, "direct: parse %s", filePath)

	// LLM analysis (nil provider → skip)
	var llmFindings []llmutil.Finding
	if provider != nil {
		var aErr error
		llmFindings, aErr = analyzeWithProvider(analysis, provider, reg)
		require.NoError(t, aErr, "direct: analyze %s", filePath)
	}

	// Context enrichment
	ctxFindings, _ := doctorctx.Enrich(analysis, "")

	var all []llmutil.Finding
	all = append(all, llmFindings...)
	all = append(all, ctxFindings...)
	return all
}

// dagPath runs the analysis via the DAG engine and returns the combined findings.
func dagPath(t *testing.T, filePath string, content []byte, provider llm.Provider, reg *registry.Registry) []llmutil.Finding {
	t.Helper()

	d, err := BuildDAG(false)
	require.NoError(t, err, "dag: build for %s", filePath)

	input := DoctorInput{
		FilePath:    filePath,
		Content:     content,
		Provider:    provider,
		Registry:    reg,
		ProjectRoot: "",
		Debug:       false,
	}

	result, err := RunDAG(context.Background(), d, input, 1)
	require.NoError(t, err, "dag: run for %s", filePath)

	return result.AllFindings
}

// analyzeWithProvider wraps the LLM analysis call.
func analyzeWithProvider(analysis *parser.AgentAnalysis, provider llm.Provider, reg *registry.Registry) ([]llmutil.Finding, error) {
	return analyzer.Analyze(analysis, provider, reg)
}

// TestEquivalence_DirectVsDAG verifies that direct Go calls and DAG-based
// execution produce matching recommendations on >= 10 agent files.
func TestEquivalence_DirectVsDAG(t *testing.T) {
	testdataDir := filepath.Join("parser", "testdata")
	if _, err := os.Stat(testdataDir); err != nil {
		t.Skipf("testdata not available: %v", err)
	}

	reg, err := registry.Load(".")
	require.NoError(t, err)

	provider := &deterministicProvider{}

	// Collect test files from claude and copilot testdata
	var testFiles []struct {
		path    string
		simPath string // simulated path for parser detection
	}

	claudeDir := filepath.Join(testdataDir, "claude")
	if entries, err := os.ReadDir(claudeDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
				continue
			}
			testFiles = append(testFiles, struct {
				path    string
				simPath string
			}{
				path:    filepath.Join(claudeDir, e.Name()),
				simPath: filepath.Join(".claude", "agents", e.Name()),
			})
		}
	}

	copilotDir := filepath.Join(testdataDir, "copilot")
	if entries, err := os.ReadDir(copilotDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
				continue
			}
			testFiles = append(testFiles, struct {
				path    string
				simPath string
			}{
				path:    filepath.Join(copilotDir, e.Name()),
				simPath: filepath.Join(".github", "agents", e.Name()),
			})
		}
	}

	require.GreaterOrEqual(t, len(testFiles), 10,
		"equivalence test requires >= 10 testdata agent files, found %d", len(testFiles))

	passed := 0
	for _, tf := range testFiles {
		t.Run(filepath.Base(tf.path), func(t *testing.T) {
			content, err := os.ReadFile(tf.path)
			require.NoError(t, err)

			direct := directPath(t, tf.simPath, content, provider, reg)
			dagResult := dagPath(t, tf.simPath, content, provider, reg)

			// Compare finding counts
			assert.Equal(t, len(direct), len(dagResult),
				"finding count mismatch: direct=%d, dag=%d", len(direct), len(dagResult))

			// Compare categories
			directCats := findingCategories(direct)
			dagCats := findingCategories(dagResult)
			assert.Equal(t, directCats, dagCats, "category mismatch")

			// Compare confidence levels
			directConf := findingConfidences(direct)
			dagConf := findingConfidences(dagResult)
			assert.Equal(t, directConf, dagConf, "confidence mismatch")

			passed++
		})
	}

	t.Logf("Equivalence verified on %d agent files", passed)
}

func findingCategories(findings []llmutil.Finding) map[string]int {
	cats := make(map[string]int)
	for _, f := range findings {
		cats[f.Category]++
	}
	return cats
}

func findingConfidences(findings []llmutil.Finding) map[string]int {
	confs := make(map[string]int)
	for _, f := range findings {
		confs[f.Confidence]++
	}
	return confs
}
