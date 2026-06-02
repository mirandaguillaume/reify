//go:build integration

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Requires: docker image reify-eval-sandbox built, ANTHROPIC_API_KEY set.
// Run with: go test -tags integration ./cmd/reify-eval/ -run TestRunAttempt -v
func TestRunAttemptAbsentMayViolate(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	f, err := loadFixture("../../testdata/fixtures/scope-restriction-normal")
	require.NoError(t, err)

	res, err := runAttempt(f, f.Formulations["absent"], "haiku", "medium")
	require.NoError(t, err)
	require.NotNil(t, res)
	t.Logf("absent run: violated=%v solved=%v changed=%v err=%v",
		res.Violated, res.Solved, res.ChangedPaths, res.AgentError)
}
