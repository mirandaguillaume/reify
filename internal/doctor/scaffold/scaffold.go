// Package scaffold generates AGENTS.md index + specialized files from a monolithic agent file.
// The scaffold is deterministic — no LLM needed. Templates are populated with codebase context.
package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/mirandaguillaume/reify/internal/scanner"
)

// SpecializedFile defines a target file in the scaffold.
type SpecializedFile struct {
	Name        string // e.g., "identity.md"
	Title       string // e.g., "Identity"
	Description string // e.g., "persona, goals, decision authority"
	Categories  []string // matching sectionMapping categories from presence.go
	Keywords    []string // keywords to match original sections
}

// DefaultFiles defines the 6 specialized files Doctor scaffolds.
var DefaultFiles = []SpecializedFile{
	{
		Name: "identity.md", Title: "Identity",
		Description: "persona, goals, decision authority, memory strategy, workflow triggers",
		Categories:  []string{"identity", "goals", "decision_authority", "memory_management", "workflow_triggers"},
		Keywords:    []string{"identity", "persona", "role", "you are", "your role", "goal", "objective", "purpose", "decision", "authority", "autonomy", "escalat", "memory", "session", "trigger", "invoke", "when to use"},
	},
	{
		Name: "security.md", Title: "Security",
		Description: "filesystem access, network permissions, secrets handling",
		Categories:  []string{"security", "prompt_injection"},
		Keywords:    []string{"security", "permission", "access", "filesystem", "network", "sandbox", "secret", "injection"},
	},
	{
		Name: "testing.md", Title: "Testing",
		Description: "test commands, conventions, coverage, examples",
		Categories:  []string{"testing", "examples", "build_commands"},
		Keywords:    []string{"test", "testing", "spec", "assert", "example", "sample", "demo", "build", "run", "commands", "scripts"},
	},
	{
		Name: "architecture.md", Title: "Architecture",
		Description: "layers, patterns, data flow, dependencies",
		Categories:  []string{"architecture_hints", "context", "dependency_declaration"},
		Keywords:    []string{"architecture", "structure", "design", "pattern", "layer", "input", "output", "consumes", "produces", "dependencies", "i/o"},
	},
	{
		Name: "guardrails.md", Title: "Guardrails",
		Description: "timeouts, output limits, behavioral constraints",
		Categories:  []string{"guardrails", "constraints", "output_constraints", "output_format"},
		Keywords:    []string{"guardrail", "constraint", "limit", "timeout", "boundar", "restriction", "output", "format", "prohibition", "never", "forbidden"},
	},
	{
		Name: "error-handling.md", Title: "Error Handling",
		Description: "fallback behavior, recovery steps, edge cases",
		Categories:  []string{"error_handling", "idempotency"},
		Keywords:    []string{"error", "failure", "fallback", "recovery", "exception", "retry", "idempoten"},
	},
}

// ScaffoldResult holds the output of a scaffold operation.
type ScaffoldResult struct {
	IndexContent    []byte            // AGENTS.md content
	Files           map[string][]byte // path -> content (e.g., ".agents/security.md" -> bytes)
	MigratedCount   int               // number of original sections migrated
	TemplatedCount  int               // number of sections generated from templates
}

// Scaffold generates an AGENTS.md index + specialized files from an original agent file.
// If ctx is nil, templates are generated without codebase enrichment.
func Scaffold(analysis *parser.AgentAnalysis, ctx *scanner.CodebaseContext) (*ScaffoldResult, error) {
	if analysis == nil {
		return nil, fmt.Errorf("nil analysis")
	}

	result := &ScaffoldResult{
		Files: make(map[string][]byte),
	}

	// Migrate original sections to specialized files
	migrations := migrateSections(analysis)

	// Generate each specialized file
	for _, sf := range DefaultFiles {
		content := generateFile(sf, migrations[sf.Name], ctx)
		result.Files[filepath.Join(".agents", sf.Name)] = []byte(content)
		if _, ok := migrations[sf.Name]; ok {
			result.MigratedCount++
		} else {
			result.TemplatedCount++
		}
	}

	// Generate AGENTS.md index
	result.IndexContent = []byte(generateIndex(analysis, ctx))

	return result, nil
}

// migrateSections routes original sections to specialized files based on keyword matching.
func migrateSections(analysis *parser.AgentAnalysis) map[string][]string {
	migrations := make(map[string][]string) // filename -> list of section contents

	for _, section := range analysis.Sections {
		header := strings.ToLower(section.Header)
		content := section.Content

		matched := false
		for _, sf := range DefaultFiles {
			for _, kw := range sf.Keywords {
				if strings.Contains(header, kw) {
					key := fmt.Sprintf("## %s\n\n%s", section.Header, strings.TrimSpace(content))
					migrations[sf.Name] = append(migrations[sf.Name], key)
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		// Unmatched sections go to identity.md (catch-all for project-specific content)
		if !matched && strings.TrimSpace(content) != "" {
			key := fmt.Sprintf("## %s\n\n%s", section.Header, strings.TrimSpace(content))
			migrations["identity.md"] = append(migrations["identity.md"], key)
		}
	}

	return migrations
}

// generateFile creates a specialized file with migrated content + template for gaps.
func generateFile(sf SpecializedFile, migrated []string, ctx *scanner.CodebaseContext) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", sf.Title))

	if len(migrated) > 0 {
		// Use migrated content
		for _, section := range migrated {
			b.WriteString(section)
			b.WriteString("\n\n")
		}
	}

	// Add template content for this file type
	template := getTemplate(sf, ctx)
	if template != "" {
		if len(migrated) > 0 {
			b.WriteString("---\n\n")
			b.WriteString("<!-- TODO: Review and customize the template below -->\n\n")
		}
		b.WriteString(template)
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

// generateIndex creates the AGENTS.md index file.
func generateIndex(analysis *parser.AgentAnalysis, ctx *scanner.CodebaseContext) string {
	var b strings.Builder

	b.WriteString("# Agent Configuration\n\n")
	b.WriteString("> Generated by `reify doctor --fix` — customize for your project.\n\n")

	// Quick reference from codebase context
	b.WriteString("## Quick Reference\n\n")
	if ctx != nil && len(ctx.Stack.Languages) > 0 {
		b.WriteString(fmt.Sprintf("- **Language:** %s\n", ctx.Stack.Languages[0].Name))
	}
	if ctx != nil {
		for _, cmd := range ctx.Commands {
			name := cmd.Name
			if len(name) > 0 {
				name = strings.ToUpper(name[:1]) + name[1:]
			}
			b.WriteString(fmt.Sprintf("- **%s:** `%s`\n", name, cmd.Command))
		}
	}
	if ctx == nil || (len(ctx.Stack.Languages) == 0 && len(ctx.Commands) == 0) {
		b.WriteString("- **Language:** <!-- TODO: add your primary language -->\n")
		b.WriteString("- **Test:** <!-- TODO: add your test command -->\n")
		b.WriteString("- **Build:** <!-- TODO: add your build command -->\n")
	}
	b.WriteString("\n")

	// File table
	b.WriteString("## Specialized Guides\n\n")
	b.WriteString("| File | Covers |\n")
	b.WriteString("|------|--------|\n")
	for _, sf := range DefaultFiles {
		b.WriteString(fmt.Sprintf("| [%s](.agents/%s) | %s |\n", sf.Title, sf.Name, sf.Description))
	}
	b.WriteString("\n")

	return b.String()
}

// getTemplate returns template content for a specialized file, enriched with codebase context.
func getTemplate(sf SpecializedFile, ctx *scanner.CodebaseContext) string {
	switch sf.Name {
	case "identity.md":
		return `## Persona

<!-- TODO: Define who this agent is -->

You are a development assistant for this project.

## Goals

<!-- TODO: Define the primary objectives -->

- Deliver correct, maintainable code
- Follow project conventions consistently

## Decision Authority

- **Autonomous:** Code changes within existing patterns, test fixes, lint fixes
- **Requires approval:** New dependencies, API changes, database migrations
- **Escalate:** Security-sensitive changes, breaking changes

## Memory Strategy

- **Session:** Current task context only
- **Persistent:** Project conventions loaded from this file
<!-- TODO: Define what should persist across sessions -->

## When to Invoke

- Use for development tasks within this project
- Not for infrastructure, deployment, or external system changes
<!-- TODO: Define specific trigger conditions -->
`
	case "security.md":
		return `## Filesystem Access

- Read/write: <!-- TODO: specify allowed directories -->
- Read-only: <!-- TODO: specify read-only directories -->
- Forbidden: .env, credentials/, secrets/

## Network Access

- Allowed: package registries
- Forbidden: external APIs without explicit approval

## Secrets

- Never hardcode API keys, tokens, or passwords
- Use environment variables for configuration
- Never log secrets or include them in error messages
`
	case "testing.md":
		tmpl := "## Commands\n\n"
		if ctx != nil {
			for _, cmd := range ctx.Commands {
				if strings.Contains(cmd.Name, "test") {
					tmpl += fmt.Sprintf("```bash\n%s\n```\n\n", cmd.Command)
				}
			}
		}
		if !strings.Contains(tmpl, "```") {
			tmpl += "```bash\n# TODO: add your test command\n```\n\n"
		}
		tmpl += `## Conventions

- Tests live next to source files
- Every PR must have tests for new functionality
- Mock external dependencies, never hit real APIs in tests

## Build Commands

` + "```bash\n# TODO: add your build command\n```\n\n"

		tmpl += `## Examples

<!-- TODO: Add 1-2 concrete examples of expected behavior -->

Example test pattern:
` + "```\n// TODO: add a representative test example\n```\n"
		return tmpl
	case "architecture.md":
		tmpl := "## Project Structure\n\n"
		if ctx != nil && len(ctx.Structure) > 0 {
			tmpl += "```\n"
			for _, dir := range ctx.Structure {
				if dir.Path == "." || dir.Path == "" {
					continue
				}
				// Show top-level dirs only
				if !strings.Contains(dir.Path, string(filepath.Separator)) {
					tmpl += fmt.Sprintf("%s/\n", dir.Path)
				}
			}
			tmpl += "```\n\n"
		} else {
			tmpl += "<!-- TODO: describe your project structure -->\n\n"
		}
		tmpl += `## Patterns

<!-- TODO: describe key architectural patterns -->

## Dependencies

- **Input:** <!-- TODO: what data/context this agent consumes -->
- **Output:** <!-- TODO: what this agent produces -->
`
		return tmpl
	case "guardrails.md":
		return `## Output Limits

- Max 500 lines per file change
- Max 10 files changed per PR

## Behavioral Constraints

- Never modify configuration files without explicit approval
- Always run tests before suggesting changes are complete
- Never skip linting

## Timeouts

<!-- TODO: define appropriate timeouts for your project -->
`
	case "error-handling.md":
		return `## On Test Failure

1. Read the full error message and stack trace
2. Identify the failing assertion
3. Fix the root cause, not the symptom

## On Build Failure

1. Check for type errors first
2. Fix dependency issues if needed
3. Rebuild and verify

## On Lint Failure

1. Run auto-fix if available
2. Manually fix remaining issues
3. Never disable lint rules without justification
`
	default:
		return ""
	}
}
