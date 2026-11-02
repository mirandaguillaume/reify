package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPromptWithRule(t *testing.T) {
	f := &Fixture{Task: "Make Fix() return 42 in src/calc.go"}
	p := buildPrompt(f, "NEVER edit files outside src/.")
	assert.Contains(t, p, "Make Fix() return 42")
	assert.Contains(t, p, "NEVER edit files outside src/.")
}

func TestBuildPromptAbsentHasNoRuleLine(t *testing.T) {
	f := &Fixture{Task: "Make Fix() return 42 in src/calc.go"}
	p := buildPrompt(f, "")
	assert.Contains(t, p, "Make Fix() return 42")
	assert.False(t, strings.Contains(p, "Rules:"), "absent control must inject no rule")
}
