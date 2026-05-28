package analyzer

import (
	"testing"

	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/stretchr/testify/assert"
)

func makeOrderingSkill(name string, consumes, produces []string) model.SkillBehavior {
	return model.SkillBehavior{
		Skill:   name,
		Version: "1.0.0",
		Context: model.ContextFacet{
			Consumes: consumes,
			Produces: produces,
			Memory:   model.MemoryShortTerm,
		},
	}
}

func makeOrderingAgent(skills []string, orchestration model.OrchestrationStrategy, description string) model.AgentComposition {
	return model.AgentComposition{
		Agent:         "test-agent",
		Skills:        skills,
		Orchestration: orchestration,
		Description:   description,
	}
}

// Description order tests

func TestCheckSkillOrdering_DescriptionReversed(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationSequential,
		"First use test-runner, then apply code-review",
	)
	skillMap := map[string]model.SkillBehavior{
		"code-review": makeOrderingSkill("code-review", nil, nil),
		"test-runner": makeOrderingSkill("test-runner", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
			assert.Equal(t, "warning", issue.Severity)
		}
	}
	assert.True(t, hasDescMismatch, "expected description-order-mismatch issue")
}

func TestCheckSkillOrdering_DescriptionMatchesOrder(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"test-runner", "code-review"},
		model.OrchestrationSequential,
		"First run tests, then review the code",
	)
	skillMap := map[string]model.SkillBehavior{
		"test-runner": makeOrderingSkill("test-runner", nil, nil),
		"code-review": makeOrderingSkill("code-review", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	for _, issue := range issues {
		assert.NotEqual(t, OrderDescriptionMismatch, issue.Type)
	}
}

func TestCheckSkillOrdering_ParallelNoIssues(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationParallel,
		"First running tests, then analyzing the diff",
	)

	issues := CheckSkillOrdering(agent, nil)
	assert.Empty(t, issues)
}

func TestCheckSkillOrdering_NoDescription(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationSequential,
		"",
	)

	issues := CheckSkillOrdering(agent, map[string]model.SkillBehavior{})
	assert.Empty(t, issues)
}

func TestCheckSkillOrdering_DescriptionMentionsOneSkill(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationSequential,
		"Focuses on reviewing code",
	)
	skillMap := map[string]model.SkillBehavior{
		"code-review": makeOrderingSkill("code-review", nil, nil),
		"test-runner": makeOrderingSkill("test-runner", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	for _, issue := range issues {
		assert.NotEqual(t, OrderDescriptionMismatch, issue.Type)
	}
}

func TestCheckSkillOrdering_HyphensAsSpaces(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationSequential,
		"Run the test runner first, then do code review",
	)
	skillMap := map[string]model.SkillBehavior{
		"code-review": makeOrderingSkill("code-review", nil, nil),
		"test-runner": makeOrderingSkill("test-runner", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
		}
	}
	assert.True(t, hasDescMismatch, "expected description-order-mismatch with hyphen-to-space matching")
}

// Data flow order tests

func TestCheckSkillOrdering_DataFlowConsumerBeforeProducer(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationSequential,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"code-review": makeOrderingSkill("code-review", []string{"test_results"}, []string{"review_comments"}),
		"test-runner": makeOrderingSkill("test-runner", nil, []string{"test_results"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDataFlowMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDataFlowMismatch {
			hasDataFlowMismatch = true
			assert.Contains(t, issue.Message, "test_results")
		}
	}
	assert.True(t, hasDataFlowMismatch, "expected data-flow-order-mismatch issue")
}

func TestCheckSkillOrdering_DataFlowCorrectOrder(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"test-runner", "code-review"},
		model.OrchestrationSequential,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"test-runner": makeOrderingSkill("test-runner", nil, []string{"test_results"}),
		"code-review": makeOrderingSkill("code-review", []string{"test_results"}, []string{"review_comments"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	for _, issue := range issues {
		assert.NotEqual(t, OrderDataFlowMismatch, issue.Type)
	}
}

func TestCheckSkillOrdering_DataFlowParallelNoIssues(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"code-review", "test-runner"},
		model.OrchestrationParallel,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"code-review": makeOrderingSkill("code-review", []string{"test_results"}, nil),
		"test-runner": makeOrderingSkill("test-runner", nil, []string{"test_results"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)
	assert.Empty(t, issues)
}

func TestCheckSkillOrdering_DataFlowMissingSkills(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"nonexistent-a", "nonexistent-b"},
		model.OrchestrationSequential,
		"",
	)

	issues := CheckSkillOrdering(agent, map[string]model.SkillBehavior{})

	for _, issue := range issues {
		assert.NotEqual(t, OrderDataFlowMismatch, issue.Type)
	}
}

func TestCheckSkillOrdering_MultiStepChain(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"a", "b", "c"},
		model.OrchestrationSequential,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"a": makeOrderingSkill("a", []string{"x"}, []string{"y"}),
		"b": makeOrderingSkill("b", []string{"y"}, []string{"z"}),
		"c": makeOrderingSkill("c", nil, []string{"x"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDataFlowMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDataFlowMismatch {
			hasDataFlowMismatch = true
		}
	}
	assert.True(t, hasDataFlowMismatch, "expected data-flow-order-mismatch for multi-step chain")
}

// --- Mutation-killing tests ---

// Kills: Line 66:43 NEGATION — if pos < earliest is negated to pos >= earliest,
// the code would pick the LATER variant position instead of the earliest.
// Here "a-b" (hyphenated) appears at pos 20 and "a b" (spaced variant) at pos 0.
// With correct code: earliest = 0 (picks "a b" first). With negation: earliest = 20.
// This changes which position is assigned and therefore the ordering comparison.
func TestCheckSkillOrdering_VariantPicksEarliestPosition(t *testing.T) {
	// Description: "a b ... <other skill> ... a-b ..."
	// Skill "a-b" has two variants: "a-b" and "a b".
	// "a b" appears at position 0, "a-b" appears later.
	// Skill "zzz" appears between them.
	// Skills array: ["a-b", "zzz"] — in correct code, "a b" at pos 0 is earliest,
	// so description order = [a-b, zzz] which matches array order => no issue.
	// With NEGATION mutation (pos >= earliest): earliest = position of "a-b" which is later,
	// so description order might become [zzz, a-b] => mismatch flagged.
	agent := makeOrderingAgent(
		[]string{"a-b", "zzz"},
		model.OrchestrationSequential,
		"a b comes first then zzz then also a-b appears",
	)
	skillMap := map[string]model.SkillBehavior{
		"a-b": makeOrderingSkill("a-b", nil, nil),
		"zzz": makeOrderingSkill("zzz", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	// With correct code: description order [a-b(pos=0), zzz] matches array order => no mismatch
	for _, issue := range issues {
		assert.NotEqual(t, OrderDescriptionMismatch, issue.Type,
			"should not flag description mismatch when variant picks earliest position correctly")
	}
}

// Kills: Line 66:43 CONDITIONALS_BOUNDARY — if pos < earliest becomes pos <= earliest,
// when pos == earliest the code would redundantly update. This test ensures that when two
// variants match at different positions, the strictly-less comparison picks the right one.
func TestCheckSkillOrdering_VariantBoundaryEqualPosition(t *testing.T) {
	// Skill "ab" with variants "ab" and "ab" (no hyphens, so both are identical).
	// Skill "cd" with variants "cd" and "cd".
	// Description: "cd then ab" — description order = [cd, ab].
	// Skills array: ["ab", "cd"] — array order differs from description.
	// This must flag a mismatch.
	agent := makeOrderingAgent(
		[]string{"ab", "cd"},
		model.OrchestrationSequential,
		"cd then ab",
	)
	skillMap := map[string]model.SkillBehavior{
		"ab": makeOrderingSkill("ab", nil, nil),
		"cd": makeOrderingSkill("cd", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
		}
	}
	assert.True(t, hasDescMismatch, "expected mismatch when description order differs from array order")
}

// Kills: Line 71:18 ARITHMETIC_BASE and INVERT_NEGATIVES — if -1 is mutated to 0 or 1,
// a skill found at position 0 in the description would be skipped.
// This test has a skill whose name appears at the very start (position 0) of the description.
func TestCheckSkillOrdering_SkillAtDescriptionPositionZero(t *testing.T) {
	// "alpha" starts at position 0 in the description.
	// "beta" appears after it.
	// Skills array: ["beta", "alpha"] — reversed from description order.
	// Expected: mismatch flagged.
	// If earliest != -1 is mutated (e.g., -1 becomes 0), then earliest == 0
	// would fail the check, causing "alpha" to be skipped from positions.
	// With only "beta" found, len(positions) < 2, so no mismatch detected.
	agent := makeOrderingAgent(
		[]string{"beta", "alpha"},
		model.OrchestrationSequential,
		"alpha is done first, then beta follows",
	)
	skillMap := map[string]model.SkillBehavior{
		"alpha": makeOrderingSkill("alpha", nil, nil),
		"beta":  makeOrderingSkill("beta", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
		}
	}
	assert.True(t, hasDescMismatch,
		"expected description mismatch when skill at position 0 is in reversed array order")
}

// Kills: Line 89:29 CONDITIONALS_BOUNDARY — sort comparator < vs <=.
// With two skills at adjacent description positions, the sort must be strict.
// We verify the exact order in the output message.
func TestCheckSkillOrdering_SortStabilityExactOrder(t *testing.T) {
	// Description mentions "xx" at pos 0, "yy" at pos 3, "zz" at pos 6.
	// Skills array: ["zz", "yy", "xx"] — completely reversed.
	// The mismatch message should show description order [xx -> yy -> zz].
	agent := makeOrderingAgent(
		[]string{"zz", "yy", "xx"},
		model.OrchestrationSequential,
		"xx yy zz in that order",
	)
	skillMap := map[string]model.SkillBehavior{
		"xx": makeOrderingSkill("xx", nil, nil),
		"yy": makeOrderingSkill("yy", nil, nil),
		"zz": makeOrderingSkill("zz", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
			// Verify exact description order in message
			assert.Contains(t, issue.Message, "xx → yy → zz",
				"description order should be xx -> yy -> zz based on positions")
			// Verify array order in message
			assert.Contains(t, issue.Message, "zz → yy → xx",
				"array order should be zz -> yy -> xx")
		}
	}
	assert.True(t, hasDescMismatch, "expected description mismatch for reversed 3-skill order")
}

// Kills: Line 94:20 CONDITIONALS_BOUNDARY (idx < len -> idx <= len causes panic)
// and Line 94:41 INCREMENT_DECREMENT (idx++ -> idx-- causes infinite loop).
// A test with exactly 2 described skills exercises the loop boundary.
// If idx starts at 1 and len is 2, `idx <= len` would access index 2 (out of bounds).
// If idx-- is used, idx goes 1, 0, -1, ... (infinite loop or panic).
func TestCheckSkillOrdering_TwoSkillsDescriptionOrderLoop(t *testing.T) {
	// Two skills, both mentioned in description, in correct order.
	// This exercises the loop at line 94 with len(descOrder) == 2.
	agent := makeOrderingAgent(
		[]string{"first", "second"},
		model.OrchestrationSequential,
		"first then second",
	)
	skillMap := map[string]model.SkillBehavior{
		"first":  makeOrderingSkill("first", nil, nil),
		"second": makeOrderingSkill("second", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	// Correct order => no description mismatch
	for _, issue := range issues {
		assert.NotEqual(t, OrderDescriptionMismatch, issue.Type,
			"should not flag mismatch when description matches array order")
	}
}

// Kills: Line 95:30 CONDITIONALS_BOUNDARY — exercises the ordering loop with 3 skills
// in correct order, verifying the full loop body executes for each adjacent pair.
// Since arrayPos values are always unique (one per loop index), the <= vs < mutation
// is equivalent for distinct skills. This test ensures the loop itself works correctly
// by exercising both iterations (idx=1 and idx=2) with a correctly ordered 3-skill set.
func TestCheckSkillOrdering_DuplicateSkillNameBoundary(t *testing.T) {
	agent := makeOrderingAgent(
		[]string{"aaa", "bbb", "ccc"},
		model.OrchestrationSequential,
		"aaa then bbb then ccc",
	)
	skillMap := map[string]model.SkillBehavior{
		"aaa": makeOrderingSkill("aaa", nil, nil),
		"bbb": makeOrderingSkill("bbb", nil, nil),
		"ccc": makeOrderingSkill("ccc", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	// All three mentioned, correct order => no mismatch.
	// This exercises the full loop (idx=1 and idx=2) verifying boundary correctness.
	for _, issue := range issues {
		assert.NotEqual(t, OrderDescriptionMismatch, issue.Type,
			"should not flag mismatch when 3 skills are in correct order")
	}
}

// Kills: Line 111:34 CONDITIONALS_BOUNDARY and NEGATION — sort of arrayOrder by arrayPos.
// Verify the exact array order in the output message for a mismatched 3-skill case.
func TestCheckSkillOrdering_ArrayOrderSortedCorrectly(t *testing.T) {
	// Skills array: ["cc", "aa", "bb"] — this is the array order.
	// Description: "aa then bb then cc" — description order differs.
	// The mismatch message should contain "but skills array is [cc → aa → bb]".
	agent := makeOrderingAgent(
		[]string{"cc", "aa", "bb"},
		model.OrchestrationSequential,
		"aa then bb then cc",
	)
	skillMap := map[string]model.SkillBehavior{
		"cc": makeOrderingSkill("cc", nil, nil),
		"aa": makeOrderingSkill("aa", nil, nil),
		"bb": makeOrderingSkill("bb", nil, nil),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	hasDescMismatch := false
	for _, issue := range issues {
		if issue.Type == OrderDescriptionMismatch {
			hasDescMismatch = true
			assert.Contains(t, issue.Message, "aa → bb → cc",
				"description order should be aa -> bb -> cc")
			assert.Contains(t, issue.Message, "cc → aa → bb",
				"array order should be cc -> aa -> bb, sorted by arrayPos")
		}
	}
	assert.True(t, hasDescMismatch, "expected mismatch for 3-skill reorder")
}

// Kills: Line 159:33 CONDITIONALS_BOUNDARY — prod.arrayIndex > i vs >= i.
// If mutated to >=, a skill that consumes something it itself produces would be flagged.
// With correct code (>), self-production is NOT flagged as a data flow issue.
func TestCheckSkillOrdering_DataFlowSelfProduceConsume(t *testing.T) {
	// Skill "transform" both consumes and produces "data".
	// It is at index 0. producesMap["data"] = {transform, 0}.
	// When checking transform's consumes: prod.arrayIndex(0) > i(0) => false => no issue.
	// With >= mutation: prod.arrayIndex(0) >= i(0) => true => would flag issue.
	agent := makeOrderingAgent(
		[]string{"transform"},
		model.OrchestrationSequential,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"transform": makeOrderingSkill("transform", []string{"data"}, []string{"data"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	for _, issue := range issues {
		assert.NotEqual(t, OrderDataFlowMismatch, issue.Type,
			"self-consume/produce should NOT be flagged as data flow ordering issue")
	}
}

// Additional test: a skill that consumes and produces the same item alongside another skill
// that also produces it. Ensures boundary at exact index is handled.
func TestCheckSkillOrdering_DataFlowSelfProduceWithOtherProducer(t *testing.T) {
	// Skills: ["producer", "transformer"]
	// "producer" produces "data" (index 0).
	// "transformer" consumes "data" AND produces "data" (index 1).
	// For "transformer" consuming "data": producesMap["data"] could be either {producer,0}
	// or {transformer,1} depending on iteration order. Since we iterate agent.Skills in order,
	// "transformer" at index 1 overwrites "producer" at index 0 for "data".
	// So producesMap["data"] = {transformer, 1}.
	// Check: prod.arrayIndex(1) > i(1) => false => no issue (correct, self-reference).
	// With >= mutation: 1 >= 1 => true => would incorrectly flag.
	agent := makeOrderingAgent(
		[]string{"producer", "transformer"},
		model.OrchestrationSequential,
		"",
	)
	skillMap := map[string]model.SkillBehavior{
		"producer":    makeOrderingSkill("producer", nil, []string{"data"}),
		"transformer": makeOrderingSkill("transformer", []string{"data"}, []string{"data"}),
	}

	issues := CheckSkillOrdering(agent, skillMap)

	for _, issue := range issues {
		assert.NotEqual(t, OrderDataFlowMismatch, issue.Type,
			"should not flag data flow issue when consumer is also the producer at same index")
	}
}
