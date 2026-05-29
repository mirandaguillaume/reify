package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIORoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.jsonl")

	items := []calibrateItem{
		{
			ID:             "ctx-0001",
			Text:           "Go 1.24 project",
			Section:        "Stack",
			SourceFile:     "CLAUDE.md",
			SourceRepo:     "reify",
			LLMLabels:      []string{"context"},
			GoldLabels:     []string{"context", "strategy"},
			EmergentLabels: []string{"tech_stack"},
			Notes:          "first",
		},
		{
			ID:          "grd-0002",
			Text:        "Never commit secrets",
			Section:     "Rules",
			SourceFile:  "CLAUDE.md",
			SourceRepo:  "reify",
			LLMLabels:   []string{"guardrails"},
			GoldLabels:  []string{"guardrails", "security"},
			JudgeLabels: []string{"guardrails", "security"},
		},
		{
			ID:         "sec-0003",
			Text:       "Store keys in vault",
			SourceFile: "AGENTS.md",
			SourceRepo: "other",
			LLMLabels:  []string{"security"},
			GoldLabels: []string{"security"},
		},
	}

	require.NoError(t, writeJSONL(path, items))

	got, err := readJSONL(path)
	require.NoError(t, err)
	assert.Equal(t, items, got)

	// One JSON object per line: count non-trailing newlines.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := 0
	for _, b := range raw {
		if b == '\n' {
			lines++
		}
	}
	assert.Equal(t, len(items), lines, "expected one line per item")
}

func TestIOReadJSONLSkipsBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blanks.jsonl")

	content := "" +
		`{"id":"a","text":"one","llm_labels":["context"]}` + "\n" +
		"\n" +
		"   \n" +
		`{"id":"b","text":"two","llm_labels":["security"]}` + "\n" +
		"\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	got, err := readJSONL(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "b", got[1].ID)
	assert.Equal(t, []string{"context"}, got[0].LLMLabels)
	assert.Equal(t, []string{"security"}, got[1].LLMLabels)
}

func TestIOReadJSONLBadLineErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")

	content := "" +
		`{"id":"a","text":"ok"}` + "\n" +
		`{this is not valid json}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	got, err := readJSONL(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad JSONL line")
	assert.Nil(t, got)
}

func TestIOUnmarshalV1Singular(t *testing.T) {
	var it calibrateItem
	data := []byte(`{"id":"x","text":"t","llm_label":"context","gold_label":"security"}`)
	require.NoError(t, json.Unmarshal(data, &it))

	assert.Equal(t, "x", it.ID)
	assert.Equal(t, "t", it.Text)
	assert.Equal(t, []string{"context"}, it.LLMLabels)
	assert.Equal(t, []string{"security"}, it.GoldLabels)
	assert.Nil(t, it.JudgeLabels)
}

func TestIOUnmarshalV1SingularJudge(t *testing.T) {
	var it calibrateItem
	data := []byte(`{"id":"y","text":"t","judge_label":"observability"}`)
	require.NoError(t, json.Unmarshal(data, &it))

	assert.Equal(t, []string{"observability"}, it.JudgeLabels)
	assert.Nil(t, it.LLMLabels)
	assert.Nil(t, it.GoldLabels)
}

func TestIOUnmarshalV11Plural(t *testing.T) {
	var it calibrateItem
	data := []byte(`{"id":"z","text":"t","llm_labels":["context","security"],"gold_labels":["guardrails"]}`)
	require.NoError(t, json.Unmarshal(data, &it))

	assert.Equal(t, []string{"context", "security"}, it.LLMLabels)
	assert.Equal(t, []string{"guardrails"}, it.GoldLabels)
}

func TestIOUnmarshalPluralWinsOverSingular(t *testing.T) {
	// When both plural and singular are present, chooseLabels prefers plural.
	var it calibrateItem
	data := []byte(`{"id":"w","text":"t","llm_label":"context","llm_labels":["strategy","guardrails"]}`)
	require.NoError(t, json.Unmarshal(data, &it))

	assert.Equal(t, []string{"strategy", "guardrails"}, it.LLMLabels)
}

func TestIOInferRepo(t *testing.T) {
	sep := string(filepath.Separator)

	cases := []struct {
		name       string
		sourceRoot string
		logPath    string
		want       string
	}{
		{
			name:       "logs segment returns next component",
			sourceRoot: sep + "data",
			logPath:    filepath.Join(sep+"data", "logs", "supabase", "classify.log"),
			want:       "supabase",
		},
		{
			name:       "no logs segment returns first component",
			sourceRoot: sep + "data",
			logPath:    filepath.Join(sep+"data", "nextjs", "classify.log"),
			want:       "nextjs",
		},
		{
			name:       "rel error returns empty (relative root, absolute log)",
			sourceRoot: "rel" + sep + "root",
			logPath:    filepath.Join(sep+"abs", "nextjs", "classify.log"),
			want:       "",
		},
		{
			name:       "logs as last component falls back to first component",
			sourceRoot: sep + "data",
			logPath:    filepath.Join(sep+"data", "logs"),
			want:       "logs",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, inferRepo(tc.logPath, tc.sourceRoot))
		})
	}
}

func TestIOFindClassifyLogs(t *testing.T) {
	root := t.TempDir()

	a := filepath.Join(root, "repoA", "logs")
	b := filepath.Join(root, "repoB", "nested", "deep")
	require.NoError(t, os.MkdirAll(a, 0o755))
	require.NoError(t, os.MkdirAll(b, 0o755))

	logA := filepath.Join(a, "classify.log")
	logB := filepath.Join(b, "classify.log")
	decoy := filepath.Join(root, "repoA", "other.log")

	require.NoError(t, os.WriteFile(logA, []byte("[]"), 0o644))
	require.NoError(t, os.WriteFile(logB, []byte("[]"), 0o644))
	require.NoError(t, os.WriteFile(decoy, []byte("[]"), 0o644))

	got, err := findClassifyLogs(root)
	require.NoError(t, err)

	sort.Strings(got)
	want := []string{logA, logB}
	sort.Strings(want)
	assert.Equal(t, want, got)
}

func TestIOLoadClassifyLog(t *testing.T) {
	root := t.TempDir()
	logDir := filepath.Join(root, "logs", "myrepo")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logPath := filepath.Join(logDir, "classify.log")

	body := `[{"file":"CLAUDE.md","format":"claude","facets":{"guardrails":[{"text":"Never commit secrets","section":"Rules"}],"context":[{"text":"Go 1.24","section":"Stack"}]}}]`
	require.NoError(t, os.WriteFile(logPath, []byte(body), 0o644))

	items, err := loadClassifyLog(logPath, root)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// Facet map iteration order is nondeterministic: index by text.
	byText := map[string]calibrateItem{}
	for _, it := range items {
		byText[it.Text] = it
		assert.Equal(t, "myrepo", it.SourceRepo)
		assert.Equal(t, "CLAUDE.md", it.SourceFile)
		require.Len(t, it.LLMLabels, 1, "each classify.log item carries exactly one owning facet")
	}

	guard, ok := byText["Never commit secrets"]
	require.True(t, ok)
	assert.Equal(t, "Rules", guard.Section)
	assert.Equal(t, []string{"guardrails"}, guard.LLMLabels)

	ctx, ok := byText["Go 1.24"]
	require.True(t, ok)
	assert.Equal(t, "Stack", ctx.Section)
	assert.Equal(t, []string{"context"}, ctx.LLMLabels)
}

func TestIOLoadClassifyLogRejectsNonArray(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "classify.log")
	// A JSON object, not the expected array.
	require.NoError(t, os.WriteFile(logPath, []byte(`{"file":"CLAUDE.md"}`), 0o644))

	items, err := loadClassifyLog(logPath, root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a classify --json array")
	assert.Nil(t, items)
}

func TestIOLoadClassifyLogMissingFileErrors(t *testing.T) {
	root := t.TempDir()
	_, err := loadClassifyLog(filepath.Join(root, "nope.log"), root)
	require.Error(t, err)
}
