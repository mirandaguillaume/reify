# Adding a New Format Parser

This guide explains how to add a new format parser to Forgent's doctor analysis pipeline.

## Overview

Parsers implement the `FormatParser` interface and register via `init()`. The fidelity test suite automatically discovers and validates new parsers.

## Steps

### 1. Implement the FormatParser Interface

Create a new file `<format>.go` in this directory:

```go
package parser

import (
    "path/filepath"
    "strings"
)

type cursorParser struct{}

func init() {
    Register("cursor", func() FormatParser { return &cursorParser{} })
}

func (p *cursorParser) Format() string { return "cursor" }

func (p *cursorParser) Detect(path string, content []byte) bool {
    // Path-based detection
    base := filepath.Base(path)
    if base == ".cursorrules" || strings.HasSuffix(base, ".mdc") {
        return true
    }
    // Content-based detection (optional, for ambiguous paths)
    // ...
    return false
}

func (p *cursorParser) Parse(content []byte) (*AgentAnalysis, error) {
    // Extract frontmatter, sections, tools from the content.
    // Return an AgentAnalysis with non-empty Format, Sections, RawContent.
    return &AgentAnalysis{
        Format:      "cursor",
        Frontmatter: make(map[string]interface{}),
        Sections:    parseSections(content), // reuse shared helpers
        RawContent:  content,
    }, nil
}

func (p *cursorParser) Validate(original, rewritten []byte) error {
    // Verify that rewritten preserves original structure.
    // At minimum: frontmatter fields preserved, section headers preserved.
    c := &claudeParser{}
    return c.Validate(original, rewritten) // reuse if format is similar
}
```

### 2. Add Test Data

Create a `testdata/<format>/` directory with at least 5 real files:

```
internal/doctor/parser/testdata/cursor/
    awesome-cursor.mdc
    example-project.mdc
    react-cursor.mdc
    python-cursor.mdc
    go-cursor.mdc
```

**Requirements:**
- Files must be real-world examples (collected from public repositories)
- Each file must be detectable by your parser's `Detect()` method
- Each file must parse without error
- Keep files under 50KB each
- Update `testdata/README.md` with source attribution

### 3. Write Unit Tests

Create `<format>_test.go` with tests for your parser:

```go
package parser

import "testing"

func TestCursorParser_Detect(t *testing.T) {
    // Test path-based and content-based detection
}

func TestCursorParser_Parse(t *testing.T) {
    // Test parsing of representative files
}
```

### 4. Verify with the Fidelity Suite

The fidelity test suite automatically includes your parser:

```bash
go test ./internal/doctor/parser/ -run TestFidelity -v
```

This validates that every file in `testdata/<format>/` can be:
- Detected by your parser
- Parsed without error
- Self-validated (identity `Validate(content, content)` passes)

No changes to the fidelity test code are needed.

### 5. Run the Full Test Suite

```bash
go test ./internal/doctor/parser/ -count=1 -race
```

## Architecture Notes

- **Registry pattern:** Parsers register via `init()` in `registry.go`. Registration order determines detection priority in `DetectFormat()`.
- **Shared helpers:** `extractFrontmatter()`, `parseSections()`, `extractTools()` in `claude.go` work for any markdown-with-frontmatter format.
- **Validation strategy:** `Validate()` checks structural preservation (frontmatter fields and section headers), not content equivalence.
