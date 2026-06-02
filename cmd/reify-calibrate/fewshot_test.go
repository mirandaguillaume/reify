package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJudgeFewShotHeaderContainsBaseRubric(t *testing.T) {
	fs := judgeFewShotHeader()
	// The base rubric facet definitions must survive intact.
	assert.Contains(t, fs, "Reify facet rubric")
	assert.Contains(t, fs, "context: background knowledge")
	assert.Contains(t, fs, "strategy: how to approach a task")
}

func TestJudgeFewShotHeaderAddsContrastiveExamples(t *testing.T) {
	fs := judgeFewShotHeader()
	// The strategy<->context anchors must be present.
	assert.Contains(t, fs, "Use camelCase for variables.\" -> strategy")
	assert.Contains(t, fs, "The project uses Cobra for the CLI.\" -> context")
	// The same-topic contrast pair (the discriminating signal).
	assert.Contains(t, fs, "parameterized queries")
	assert.Contains(t, fs, "PostgreSQL 16")
}

func TestJudgeFewShotHeaderSingleTrailingSeparator(t *testing.T) {
	fs := judgeFewShotHeader()
	// Per-item content follows "---\n\n"; there must be exactly one such
	// trailing separator so the examples sit inside the preamble, not after.
	assert.True(t, strings.HasSuffix(fs, "---\n\n"), "must end with the item separator")
	assert.Equal(t, 1, strings.Count(fs, "---\n\n"), "exactly one separator")
}

func TestJudgeFewShotHeaderLongerThanBase(t *testing.T) {
	assert.Greater(t, len(judgeFewShotHeader()), len(judgePromptHeader()))
}
