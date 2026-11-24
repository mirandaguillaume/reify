package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildFillerApproxSize(t *testing.T) {
	// ~4 chars/token; 200 tokens => ~800 chars, allow generous slack.
	f := buildFiller(200)
	assert.Greater(t, len(f), 600)
	assert.Empty(t, buildFiller(0))
}

func TestBuildPromptLITMPositions(t *testing.T) {
	f := &Fixture{Task: "do the thing"}
	rule := "NEVER add a dependency."
	head := buildPromptLITM(f, rule, 400, 0.0)
	mid := buildPromptLITM(f, rule, 400, 0.5)
	tail := buildPromptLITM(f, rule, 400, 1.0)

	for _, p := range []string{head, mid, tail} {
		assert.Contains(t, p, rule, "rule present at every position")
		assert.Contains(t, p, "do the thing", "task always present")
		// task comes after the rule's context block, near the end
		assert.Greater(t, strings.Index(p, "Task:"), strings.Index(p, "guidelines"))
	}

	// Head: rule appears before most filler. Tail: rule appears after most.
	// Compare the rule's relative offset across conditions.
	relHead := float64(strings.Index(head, rule)) / float64(len(head))
	relTail := float64(strings.Index(tail, rule)) / float64(len(tail))
	assert.Less(t, relHead, relTail, "rule sits earlier at head than at tail")
}

func TestBuildPromptLITMAbsentKeepsFiller(t *testing.T) {
	f := &Fixture{Task: "do the thing"}
	absent := buildPromptLITM(f, "", 400, 0.5)
	// no rule, but filler + task still present so context length matches
	assert.NotContains(t, absent, "## Rules")
	assert.Contains(t, absent, "do the thing")
	assert.Contains(t, absent, "Project context")
}

func TestLitmPositionName(t *testing.T) {
	assert.Equal(t, "head", litmPositionName(0.0))
	assert.Equal(t, "tail", litmPositionName(1.0))
	assert.Equal(t, "mid-50%", litmPositionName(0.5))
}
