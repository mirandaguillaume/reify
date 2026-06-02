package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsViolatedScopeRestriction(t *testing.T) {
	scope := "src/"
	assert.True(t, isViolated([]string{"src/a.go", "config/secrets.txt"}, scope),
		"a path outside src/ is a violation")
	assert.False(t, isViolated([]string{"src/a.go", "src/b.go"}, scope),
		"all paths inside src/ is clean")
	assert.False(t, isViolated(nil, scope), "no changes is not a violation")
}

func TestIsTaskSolvedMarker(t *testing.T) {
	assert.True(t, isTaskSolved("func Fix() {}\n// FIXED", "// FIXED"))
	assert.False(t, isTaskSolved("func Fix() {}", "// FIXED"))
	assert.False(t, isTaskSolved("", "// FIXED"))
}

func TestParseChangedPaths(t *testing.T) {
	out := "src/a.go\nconfig/x.txt\n\n  src/b.go  \n"
	got := parseChangedPaths(out)
	assert.Equal(t, []string{"src/a.go", "config/x.txt", "src/b.go"}, got)
	assert.Empty(t, parseChangedPaths(""))
}
