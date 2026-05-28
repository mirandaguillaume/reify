package static

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/doctor/llmutil"
	"github.com/mirandaguillaume/reify/internal/doctor/parser"
	"github.com/stretchr/testify/assert"
)

type mockCheck struct {
	id       string
	tags     []string
	category string
	severity string
	findings []llmutil.Finding
}

func (m *mockCheck) ID() string                                    { return m.id }
func (m *mockCheck) Tags() []string                                { return m.tags }
func (m *mockCheck) Category() string                              { return m.category }
func (m *mockCheck) DefaultSeverity() string                       { return m.severity }
func (m *mockCheck) Run(_ *parser.AgentAnalysis) []llmutil.Finding { return m.findings }

func TestRegisterAndResolve(t *testing.T) {
	ResetChecks()
	defer ResetChecks()

	RegisterCheck(&mockCheck{id: "default-check", tags: []string{"default"}, findings: []llmutil.Finding{{Issue: "a"}}})
	RegisterCheck(&mockCheck{id: "thorough-check", tags: []string{"thorough"}, findings: []llmutil.Finding{{Issue: "b"}}})
	RegisterCheck(&mockCheck{id: "security-check", tags: []string{"security"}, findings: []llmutil.Finding{{Issue: "c"}}})
	RegisterCheck(&mockCheck{id: "default-security", tags: []string{"default", "security"}, findings: []llmutil.Finding{{Issue: "d"}}})

	assert.Len(t, AllChecks(), 4)

	// Default mode: only default-tagged
	defaultChecks := ResolveChecks("default")
	assert.Len(t, defaultChecks, 2) // default-check + default-security
	assert.Equal(t, "default-check", defaultChecks[0].ID())
	assert.Equal(t, "default-security", defaultChecks[1].ID())

	// Quick = same as default
	quickChecks := ResolveChecks("quick")
	assert.Len(t, quickChecks, 2)

	// Thorough: all checks
	thoroughChecks := ResolveChecks("thorough")
	assert.Len(t, thoroughChecks, 4)

	// Security: only security-tagged
	securityChecks := ResolveChecks("security")
	assert.Len(t, securityChecks, 2) // security-check + default-security
}

func TestRunChecks(t *testing.T) {
	ResetChecks()
	defer ResetChecks()

	RegisterCheck(&mockCheck{id: "a", tags: []string{"default"}, findings: []llmutil.Finding{{Category: "test", Issue: "finding-a"}}})
	RegisterCheck(&mockCheck{id: "b", tags: []string{"thorough"}, findings: []llmutil.Finding{{Category: "test", Issue: "finding-b"}}})

	// Default mode
	findings := RunChecks(&parser.AgentAnalysis{}, "default")
	assert.Len(t, findings, 1)
	assert.Equal(t, "finding-a", findings[0].Issue)

	// Thorough mode
	findings = RunChecks(&parser.AgentAnalysis{}, "thorough")
	assert.Len(t, findings, 2)
}

func TestEmptyRegistry(t *testing.T) {
	ResetChecks()
	defer ResetChecks()

	assert.Empty(t, AllChecks())
	assert.Empty(t, ResolveChecks("default"))
	assert.Empty(t, RunChecks(&parser.AgentAnalysis{}, "default"))
}

func TestCountWholeWord_Basic(t *testing.T) {
	// Whole word match — counts only complete-word occurrences
	assert.Equal(t, 1, countWholeWord("never give up", "never"))
	assert.Equal(t, 2, countWholeWord("do this and do that", "do"))
	assert.Equal(t, 0, countWholeWord("the cat sat", "dog"))
}

func TestCountWholeWord_NoSubstringMatches(t *testing.T) {
	// "do" must NOT match "doing", "indoor", "doable", "neverdo"
	assert.Equal(t, 0, countWholeWord("doing the work indoor with doable plans", "do"))
	assert.Equal(t, 0, countWholeWord("neverwhere", "never"))
	assert.Equal(t, 0, countWholeWord("endeavoring", "ever"))
}

func TestCountWholeWord_PunctuationBoundaries(t *testing.T) {
	// Trailing/leading punctuation should not prevent matches
	assert.Equal(t, 1, countWholeWord("never, give up", "never"))
	assert.Equal(t, 1, countWholeWord("do.", "do"))
	assert.Equal(t, 1, countWholeWord("(do)", "do"))
	assert.Equal(t, 1, countWholeWord("[do]", "do"))
	assert.Equal(t, 1, countWholeWord("'do'", "do"))
	assert.Equal(t, 1, countWholeWord("\"do\"", "do"))
	assert.Equal(t, 1, countWholeWord("`do`", "do"))
	// Multiple punctuation forms in same text
	assert.Equal(t, 3, countWholeWord("do. do, do!", "do"))
}

func TestCountWholeWord_CaseSensitive(t *testing.T) {
	// Helper is case-sensitive — caller normalizes
	assert.Equal(t, 1, countWholeWord("Never give up. never give up.", "never"))
	assert.Equal(t, 1, countWholeWord("Never give up. never give up.", "Never"))
}

func TestCountWholeWord_EmptyInputs(t *testing.T) {
	assert.Equal(t, 0, countWholeWord("", "word"))
	assert.Equal(t, 0, countWholeWord("text", ""))
	assert.Equal(t, 0, countWholeWord("", ""))
}

func TestCountWholeWord_MultiWordPhraseRejected(t *testing.T) {
	// Multi-word phrases are NOT supported by countWholeWord — caller uses strings.Count
	// Returns 0 to enforce the contract clearly
	assert.Equal(t, 0, countWholeWord("in addition to that", "in addition"))
}

func TestCountWholeWords_Slice(t *testing.T) {
	// Convenience: sum across multiple targets
	text := "do this and never do that"
	assert.Equal(t, 3, countWholeWords(text, []string{"do", "never"}))
	// Empty targets → 0
	assert.Equal(t, 0, countWholeWords(text, []string{}))
	// Targets with no matches → 0
	assert.Equal(t, 0, countWholeWords(text, []string{"xyz", "abc"}))
}

func TestRunChecks_PopulatesSeverity(t *testing.T) {
	ResetChecks()
	defer ResetChecks()

	RegisterCheck(&mockCheck{
		id:       "sev-check",
		tags:     []string{"default"},
		severity: "high",
		findings: []llmutil.Finding{{Category: "test", Issue: "test finding"}},
	})

	findings := RunChecks(&parser.AgentAnalysis{}, "default")
	assert.Len(t, findings, 1)
	assert.Equal(t, "high", findings[0].Severity, "RunChecks should populate Severity from DefaultSeverity()")
}

func TestRunChecks_PreservesExistingSeverity(t *testing.T) {
	ResetChecks()
	defer ResetChecks()

	RegisterCheck(&mockCheck{
		id:       "preset-check",
		tags:     []string{"default"},
		severity: "low",
		findings: []llmutil.Finding{{Category: "test", Issue: "already set", Severity: "high"}},
	})

	findings := RunChecks(&parser.AgentAnalysis{}, "default")
	assert.Len(t, findings, 1)
	assert.Equal(t, "high", findings[0].Severity, "should preserve existing Severity if already set")
}

func TestIsCodeFenceLine(t *testing.T) {
	assert.True(t, IsCodeFenceLine("```"))
	assert.True(t, IsCodeFenceLine("```go"))
	assert.True(t, IsCodeFenceLine("~~~"))
	assert.True(t, IsCodeFenceLine("~~~yaml"))
	assert.False(t, IsCodeFenceLine("``"))
	assert.False(t, IsCodeFenceLine("~~"))
	assert.False(t, IsCodeFenceLine("some text"))
	assert.False(t, IsCodeFenceLine(""))
}
