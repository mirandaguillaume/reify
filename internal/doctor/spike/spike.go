// Package spike implements the engineering spike for validating local LLM quality
// on agent file analysis. This is temporary code — it validates the approach
// before committing to full implementation.
package spike

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"context"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Models to test via OpenRouter (local-class quality = models that could run on Ollama)
var TestModels = []string{
	"google/gemma-3-12b-it",       // 12B — runs on Ollama, good for structured output
	"qwen/qwen3-8b",              // 8B — small, fast, strong for its size
	"meta-llama/llama-4-scout",    // Llama 4 Scout — latest Meta model
}

var CloudBaseline = "anthropic/claude-sonnet-4" // Cloud baseline for comparison

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

// Finding represents a single analysis finding from the LLM
type Finding struct {
	Category            string `yaml:"category"`
	Issue               string `yaml:"issue"`
	Confidence          string `yaml:"confidence"`
	CurrentState        string `yaml:"current_state"`
	SuggestedImprovement string `yaml:"suggested_improvement"`
}

// AnalysisResult holds the parsed LLM output
type AnalysisResult struct {
	Findings []Finding `yaml:"findings"`
}

// FileResult holds the results for a single file + model combination
type FileResult struct {
	File        string
	Model       string
	Parseable   bool
	FindingsN   int
	LatencyMs   int64
	RawOutput   string
	ParseError  string
	Findings    []Finding
}

// ModelSummary holds aggregate results for a model
type ModelSummary struct {
	Model          string
	FilesAnalyzed  int
	ParseableCount int
	ParseRate      float64
	TotalFindings  int
	AvgFindings    float64
	AvgLatencyMs   int64
	Results        []FileResult
}

// AnalysisPrompt builds the prompt for analyzing an agent file
func AnalysisPrompt(format, content string) string {
	return fmt.Sprintf(`Analyze this agent definition file for quality issues.

File format: %s
File content:
---
%s
---

Check for these categories:
1. GUARDRAILS: Are there explicit behavioral constraints (timeouts, output limits, prohibitions)?
2. SECURITY: Are there security declarations (filesystem access, network access, secrets)?
3. ORDERING: Are critical instructions positioned at the beginning (primacy bias)?
4. DECOMPOSITION: Is this a monolithic agent doing too many things?
5. CONTEXT: Does the agent reference codebase context or project-specific details?

For each issue found, respond ONLY in this YAML format (no markdown fences, no explanation, just YAML):
findings:
  - category: guardrails
    issue: "description of the issue"
    confidence: high
    current_state: "what the file has now"
    suggested_improvement: "what should change"`, format, content)
}

// yamlFenceRe matches markdown code fences wrapping YAML content.
var yamlFenceRe = regexp.MustCompile("(?s)```(?:ya?ml)?\\s*\\n(.*?)\\n```")

// ExtractYAML strips markdown code fences from LLM output
func ExtractYAML(output string) string {
	re := yamlFenceRe
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	// No fences — return trimmed output
	return strings.TrimSpace(output)
}

// ParseFindings parses YAML output into findings
func ParseFindings(yamlStr string) ([]Finding, error) {
	var result AnalysisResult
	if err := yaml.Unmarshal([]byte(yamlStr), &result); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	return result.Findings, nil
}

// CallOpenRouter sends a prompt to OpenRouter with a specific model
func CallOpenRouter(apiKey, model, prompt string) (string, time.Duration, error) {
	reqBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 2048,
		"temperature": 0.1,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", openRouterURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return "", elapsed, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", elapsed, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", elapsed, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", elapsed, fmt.Errorf("unmarshal: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", elapsed, fmt.Errorf("empty response")
	}

	return result.Choices[0].Message.Content, elapsed, nil
}

// DetectFormat guesses the format from the file path
func DetectFormat(path string) string {
	dir := filepath.Dir(path)
	if strings.Contains(dir, "claude") {
		return "claude-code"
	}
	if strings.Contains(dir, "copilot") || strings.Contains(dir, "github") {
		return "github-copilot"
	}
	return "unknown"
}

// LoadTestCorpus loads all .md files from the testdata directories
func LoadTestCorpus(testdataDir string) ([]string, error) {
	var files []string
	for _, subdir := range []string{"claude", "copilot"} {
		dir := filepath.Join(testdataDir, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Directory doesn't exist — skip
			}
			return nil, fmt.Errorf("reading %s: %w", dir, err) // Permission or other errors — fail
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .md files found in %s", testdataDir)
	}
	return files, nil
}

// AnalyzeFile runs a single file through a model and returns the result
func AnalyzeFile(apiKey, model, filePath string) FileResult {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return FileResult{File: filePath, Model: model, ParseError: err.Error()}
	}

	format := DetectFormat(filePath)
	prompt := AnalysisPrompt(format, string(content))

	output, elapsed, err := CallOpenRouter(apiKey, model, prompt)
	if err != nil {
		return FileResult{
			File: filePath, Model: model, LatencyMs: elapsed.Milliseconds(),
			ParseError: err.Error(),
		}
	}

	yamlStr := ExtractYAML(output)
	findings, err := ParseFindings(yamlStr)

	result := FileResult{
		File:      filepath.Base(filePath),
		Model:     model,
		LatencyMs: elapsed.Milliseconds(),
		RawOutput: output,
	}

	if err != nil {
		result.ParseError = err.Error()
		result.Parseable = false
	} else {
		result.Parseable = true
		result.FindingsN = len(findings)
		result.Findings = findings
	}

	return result
}

// Summarize computes aggregate stats for a model's results
func Summarize(model string, results []FileResult) ModelSummary {
	s := ModelSummary{Model: model, Results: results, FilesAnalyzed: len(results)}
	var totalLatency int64
	for _, r := range results {
		if r.Parseable {
			s.ParseableCount++
			s.TotalFindings += r.FindingsN
		}
		totalLatency += r.LatencyMs
	}
	if s.FilesAnalyzed > 0 {
		s.ParseRate = float64(s.ParseableCount) / float64(s.FilesAnalyzed) * 100
		s.AvgLatencyMs = totalLatency / int64(s.FilesAnalyzed)
	}
	if s.ParseableCount > 0 {
		s.AvgFindings = float64(s.TotalFindings) / float64(s.ParseableCount)
	}
	return s
}

// FormatReport generates a markdown report from model summaries
func FormatReport(summaries []ModelSummary, cloudSummary *ModelSummary) string {
	var b strings.Builder

	b.WriteString("# Engineering Spike: Local LLM Quality for Agent Analysis\n\n")
	b.WriteString(fmt.Sprintf("Date: %s\n\n", time.Now().Format("2006-01-02")))

	b.WriteString("## Summary\n\n")
	b.WriteString("| Model | Files | Parse Rate | Avg Findings | Avg Latency |\n")
	b.WriteString("|-------|-------|-----------|-------------|-------------|\n")

	for _, s := range summaries {
		b.WriteString(fmt.Sprintf("| %s | %d | %.0f%% | %.1f | %dms |\n",
			s.Model, s.FilesAnalyzed, s.ParseRate, s.AvgFindings, s.AvgLatencyMs))
	}
	if cloudSummary != nil {
		b.WriteString(fmt.Sprintf("| **%s** (baseline) | %d | %.0f%% | %.1f | %dms |\n",
			cloudSummary.Model, cloudSummary.FilesAnalyzed, cloudSummary.ParseRate,
			cloudSummary.AvgFindings, cloudSummary.AvgLatencyMs))
	}

	b.WriteString("\n## Decision Gate\n\n")
	if cloudSummary != nil && len(summaries) > 0 && cloudSummary.ParseRate > 0 {
		bestLocal := summaries[0]
		for _, s := range summaries[1:] {
			if s.ParseRate > bestLocal.ParseRate ||
				(s.ParseRate == bestLocal.ParseRate && s.AvgFindings > bestLocal.AvgFindings) {
				bestLocal = s
			}
		}
		ratio := bestLocal.ParseRate / cloudSummary.ParseRate * 100
		b.WriteString(fmt.Sprintf("Best local model: **%s** (%.0f%% parse rate)\n", bestLocal.Model, bestLocal.ParseRate))
		b.WriteString(fmt.Sprintf("Cloud baseline: **%s** (%.0f%% parse rate)\n", cloudSummary.Model, cloudSummary.ParseRate))
		b.WriteString(fmt.Sprintf("Local/Cloud ratio: **%.0f%%**\n\n", ratio))

		if ratio >= 50 {
			b.WriteString("**DECISION: PROCEED** with local-first architecture. Local models achieve ≥50% of cloud baseline.\n")
		} else {
			b.WriteString("**DECISION: PIVOT** to hybrid approach. Local models below 50% of cloud baseline.\n")
		}
	}

	// Per-file details
	b.WriteString("\n## Detailed Results\n\n")
	for _, s := range summaries {
		b.WriteString(fmt.Sprintf("### %s\n\n", s.Model))
		for _, r := range s.Results {
			status := "PASS"
			if !r.Parseable {
				status = "FAIL"
			}
			b.WriteString(fmt.Sprintf("- **%s** [%s] %d findings, %dms", r.File, status, r.FindingsN, r.LatencyMs))
			if r.ParseError != "" {
				b.WriteString(fmt.Sprintf(" — error: %s", r.ParseError))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
