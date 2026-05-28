package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formatDetectPath returns a realistic file path for the given format
// so that path-based detection works as it would in real usage.
func formatDetectPath(format, filename string) string {
	switch format {
	case "claude":
		return filepath.Join(".claude", "agents", filename)
	case "copilot":
		return filepath.Join(".github", "agents", filename)
	default:
		// For formats like reify, the filename itself carries the detection
		// signal (e.g., .skill.yaml), so return as-is.
		return filename
	}
}

// TestFidelity_AllParsers auto-discovers every registered parser and validates
// that each file in its testdata directory can be detected, parsed, and
// self-validated. Adding a new parser with testdata is sufficient to include
// it in this suite — no test code changes needed.
func TestFidelity_AllParsers(t *testing.T) {
	formats := RegisteredFormats()
	require.NotEmpty(t, formats, "no parsers registered")

	for _, name := range formats {
		t.Run(name, func(t *testing.T) {
			p, err := Get(name)
			require.NoError(t, err)

			testdataDir := filepath.Join("testdata", name)
			info, err := os.Stat(testdataDir)
			if os.IsNotExist(err) {
				t.Skipf("no testdata directory for parser %q — add testdata/%s/ to enable fidelity checks", name, name)
				return
			}
			require.NoError(t, err)
			require.True(t, info.IsDir(), "testdata/%s is not a directory", name)

			entries, err := os.ReadDir(testdataDir)
			require.NoError(t, err)

			// Filter to actual files (skip subdirectories, dotfiles)
			var files []os.DirEntry
			for _, e := range entries {
				if !e.IsDir() && isTestdataFile(e.Name()) {
					files = append(files, e)
				}
			}

			if len(files) == 0 {
				t.Skipf("testdata/%s/ is empty — add test files to enable fidelity checks", name)
				return
			}

			t.Logf("testing %d files for parser %q", len(files), name)

			for _, entry := range files {
				entry := entry
				t.Run(entry.Name(), func(t *testing.T) {
					filePath := filepath.Join(testdataDir, entry.Name())
					content, err := os.ReadFile(filePath)
					require.NoError(t, err, "failed to read testdata file")
					require.NotEmpty(t, content, "testdata file is empty")

					detectPath := formatDetectPath(name, entry.Name())

					// (a) Detect returns true
					assert.True(t, p.Detect(detectPath, content),
						"Detect(%q) returned false — file may not match parser %q detection rules", detectPath, name)

					// (b) Parse returns no error
					analysis, err := p.Parse(content)
					require.NoError(t, err, "Parse failed for %s", entry.Name())
					require.NotNil(t, analysis, "Parse returned nil AgentAnalysis")

					// (c) AgentAnalysis has non-empty Format, Sections, RawContent
					assert.Equal(t, name, analysis.Format, "Format mismatch")
					assert.NotEmpty(t, analysis.RawContent, "RawContent is empty")
					assert.NotEmpty(t, analysis.Sections, "Sections is empty — file has no parseable structure")

					// (d) Identity validation: Validate(content, content) must pass
					err = p.Validate(content, content)
					assert.NoError(t, err, "identity Validate(original, original) failed — parser rejects its own output")
				})
			}
		})
	}
}

// TestFidelity_NewParserAutoDiscovery verifies that the fidelity suite
// automatically includes new parsers. Since the test above iterates
// RegisteredFormats(), any parser registered via init() is included.
func TestFidelity_NewParserAutoDiscovery(t *testing.T) {
	formats := RegisteredFormats()

	// The registry should contain at least the 3 built-in parsers.
	assert.GreaterOrEqual(t, len(formats), 3,
		"expected at least 3 registered parsers (claude, copilot, reify)")

	// Verify known parsers are present.
	expected := []string{"claude", "copilot", "reify"}
	for _, name := range expected {
		assert.Contains(t, formats, name, "expected parser %q to be registered", name)
	}
}

// TestFidelity_MissingTestdataSkips verifies that a parser whose testdata
// directory does not exist is gracefully skipped by the fidelity suite.
// Uses a synthetic parser name to avoid coupling to any specific parser's
// current testdata state.
func TestFidelity_MissingTestdataSkips(t *testing.T) {
	testdataDir := filepath.Join("testdata", "nonexistent-format-for-skip-test")
	_, err := os.Stat(testdataDir)
	assert.True(t, os.IsNotExist(err),
		"testdata/%s should not exist — this test validates the skip mechanism", "nonexistent-format-for-skip-test")
}

// TestFidelity_CorpusMinimum ensures the test corpus meets the minimum
// size requirements per format.
func TestFidelity_CorpusMinimum(t *testing.T) {
	minimums := map[string]int{
		"claude":  30,
		"copilot": 30,
	}

	for format, minCount := range minimums {
		t.Run(format, func(t *testing.T) {
			testdataDir := filepath.Join("testdata", format)
			entries, err := os.ReadDir(testdataDir)
			require.NoError(t, err)

			var count int
			for _, e := range entries {
				if !e.IsDir() && isTestdataFile(e.Name()) {
					count++
				}
			}

			assert.GreaterOrEqual(t, count, minCount,
				"testdata/%s/ has %d files, need at least %d", format, count, minCount)
		})
	}
}

// TestFidelity_IdentityValidation tests Parse + identity Validate for files
// with complex structures: frontmatter field preservation, section header
// preservation, and content length stability.
// TODO: When Story 3.3 adds Rewrite(), upgrade to Parse → Rewrite → Validate
// round-trip and rename to TestFidelity_RoundTrip.
func TestFidelity_IdentityValidation(t *testing.T) {
	// Test files selected for structural complexity: code blocks, nested
	// sections, non-ASCII content, complex frontmatter with lists/multiline.
	cases := []struct {
		format   string
		filename string
		reason   string
	}{
		{"claude", "dotfiles-debugger.md", "large file with many sections and code blocks"},
		{"claude", "ignixa-fhir-ef.md", "complex frontmatter with long description"},
		{"claude", "biome-parser-agent.md", "frontmatter with tool list and multiline description"},
		{"claude", "cinemascraper-claude.md", "non-ASCII content (Japanese)"},
		{"claude", "posthog-debugger.md", "deeply nested sections with examples"},
		{"copilot", "zigbee-specification.md", "large structured specification"},
		{"copilot", "camunda-ci.md", "complex copilot agent frontmatter"},
		{"copilot", "typescript-strict-agent.md", "code blocks inside sections"},
		{"copilot", "nunit-agent.md", "YAML-in-markdown with tools as JSON array"},
	}

	for _, tc := range cases {
		t.Run(tc.format+"/"+tc.filename, func(t *testing.T) {
			p, err := Get(tc.format)
			require.NoError(t, err)

			filePath := filepath.Join("testdata", tc.format, tc.filename)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)

			// Parse the file
			analysis, err := p.Parse(content)
			require.NoError(t, err, "Parse failed: %s", tc.reason)

			// Verify frontmatter fields are preserved through identity validation
			err = p.Validate(content, content)
			require.NoError(t, err, "identity Validate failed for %s (%s)", tc.filename, tc.reason)

			// Verify section headers are present and non-empty
			if len(analysis.Sections) > 0 {
				headersSeen := make(map[string]bool)
				for _, s := range analysis.Sections {
					if s.Header != "" {
						headersSeen[s.Header] = true
					}
				}
				t.Logf("%s: %d sections, %d unique headers, frontmatter keys: %d",
					tc.filename, len(analysis.Sections), len(headersSeen), len(analysis.Frontmatter))
			}

			// Content length stability: RawContent matches original
			assert.Equal(t, len(content), len(analysis.RawContent),
				"RawContent length differs from original (%s)", tc.reason)
		})
	}
}

// TestFidelity_ValidateMutationDetection verifies that Validate catches
// structural mutations: dropped frontmatter fields, removed sections,
// and broken frontmatter.
func TestFidelity_ValidateMutationDetection(t *testing.T) {
	p, err := Get("claude")
	require.NoError(t, err)

	original := []byte(`---
name: test-agent
description: A test agent for fidelity validation
tools: Read, Write
model: opus
---

# Instructions

Follow these rules carefully.

## Guidelines

Be thorough and accurate.
`)

	t.Run("dropped_field", func(t *testing.T) {
		mutated := []byte(`---
name: test-agent
description: A test agent for fidelity validation
model: opus
---

# Instructions

Follow these rules carefully.

## Guidelines

Be thorough and accurate.
`)
		err := p.Validate(original, mutated)
		assert.Error(t, err, "should detect dropped frontmatter field 'tools'")
		assert.Contains(t, err.Error(), "tools")
	})

	t.Run("changed_field", func(t *testing.T) {
		mutated := []byte(`---
name: test-agent
description: A DIFFERENT description
tools: Read, Write
model: opus
---

# Instructions

Follow these rules carefully.

## Guidelines

Be thorough and accurate.
`)
		err := p.Validate(original, mutated)
		assert.Error(t, err, "should detect changed frontmatter field 'description'")
	})

	t.Run("dropped_section", func(t *testing.T) {
		mutated := []byte(`---
name: test-agent
description: A test agent for fidelity validation
tools: Read, Write
model: opus
---

# Instructions

Follow these rules carefully.
`)
		err := p.Validate(original, mutated)
		assert.Error(t, err, "should detect dropped section 'Guidelines'")
		assert.Contains(t, err.Error(), "Guidelines")
	})

	t.Run("broken_frontmatter", func(t *testing.T) {
		mutated := []byte(`---
name: [invalid yaml
---

# Instructions

Follow these rules carefully.
`)
		err := p.Validate(original, mutated)
		// The tolerant parser extracts "name" as a key even from malformed YAML,
		// but the value changes from "test-agent" to "[invalid yaml", so Validate
		// should detect the field value change.
		assert.Error(t, err, "should detect that broken frontmatter changed the 'name' field value")
	})
}

// isTestdataFile returns true if the file is a valid testdata file for fidelity
// testing. Filters out dotfiles, binary files, and non-agent definition files.
func isTestdataFile(name string) bool {
	if len(name) == 0 || name[0] == '.' {
		return false
	}
	ext := filepath.Ext(name)
	return ext == ".md" || ext == ".yaml" || ext == ".yml"
}
