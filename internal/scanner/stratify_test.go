package scanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStratumKey(t *testing.T) {
	assert.Equal(t, "apps", stratumKey("apps/big-app/pages"))
	assert.Equal(t, "src", stratumKey("src/Controller"))
	assert.Equal(t, ".", stratumKey("."))
}

func TestIdentifyStrata_MonorepoLayout(t *testing.T) {
	dirs := []string{
		"apps/bo/pages", "apps/bo/hooks", "apps/fo/pages",
		"libs/shared/src", "libs/design/src",
		"tests/e2e/auth", "tests/e2e/users",
		"scripts",
	}
	sc := map[string]int{
		"apps/bo/pages": 20, "apps/bo/hooks": 10, "apps/fo/pages": 15,
		"libs/shared/src": 5, "libs/design/src": 3,
		"tests/e2e/auth": 4, "tests/e2e/users": 3,
		"scripts": 2,
	}

	strata := identifyStrata(dirs, sc)

	// Should have 4 strata: apps, libs, tests, scripts.
	require.Len(t, strata, 4)
	// Sorted by source count: apps (45) > libs (8) > tests (7) > scripts (2)
	assert.Equal(t, "apps", strata[0].Prefix)
	assert.Equal(t, 45, strata[0].SourceCount)
	assert.False(t, strata[0].IsTest)

	// tests stratum should be marked as test.
	var testStratum *Stratum
	for i := range strata {
		if strata[i].Prefix == "tests" {
			testStratum = &strata[i]
		}
	}
	require.NotNil(t, testStratum)
	assert.True(t, testStratum.IsTest)
}

func TestIdentifyStrata_FlatProject(t *testing.T) {
	dirs := []string{"cmd", "pkg/model", "internal/cmd"}
	sc := map[string]int{"cmd": 1, "pkg/model": 3, "internal/cmd": 2}

	strata := identifyStrata(dirs, sc)
	require.Len(t, strata, 3)
	// pkg has most source files.
	assert.Equal(t, "pkg", strata[0].Prefix)
}

func TestIdentifyStrata_RootFiles(t *testing.T) {
	dirs := []string{".", "src/main"}
	sc := map[string]int{".": 2, "src/main": 5}

	strata := identifyStrata(dirs, sc)
	require.Len(t, strata, 2)

	// "." is its own stratum.
	found := false
	for _, s := range strata {
		if s.Prefix == "." {
			found = true
			assert.Equal(t, 2, s.SourceCount)
		}
	}
	assert.True(t, found, "root stratum should exist")
}

func TestAllocateBudget_Proportional(t *testing.T) {
	strata := []Stratum{
		{Prefix: "apps", SourceCount: 4000},
		{Prefix: "libs", SourceCount: 800},
		{Prefix: "scripts", SourceCount: 200},
		{Prefix: "tests", SourceCount: 100, IsTest: true},
	}
	allocateBudget(strata, 80, 0.2)

	// Source strata share 64 entries, test stratum gets 16.
	total := 0
	for _, s := range strata {
		total += s.Budget
		assert.GreaterOrEqual(t, s.Budget, 1, "minimum 1 for %s", s.Prefix)
	}
	assert.Equal(t, 80, total, "total should match budget")

	// apps should get the lion's share among source strata.
	assert.Greater(t, strata[0].Budget, strata[1].Budget, "apps > libs")
	assert.Greater(t, strata[1].Budget, strata[2].Budget, "libs > scripts")
}

func TestAllocateBudget_MinimumOne(t *testing.T) {
	strata := []Stratum{
		{Prefix: "apps", SourceCount: 10000},
		{Prefix: "docs", SourceCount: 1},
	}
	allocateBudget(strata, 80, 0.0)

	assert.GreaterOrEqual(t, strata[1].Budget, 1, "tiny stratum gets at least 1")
}

func TestAllocateBudget_TestRatio(t *testing.T) {
	strata := []Stratum{
		{Prefix: "src", SourceCount: 500},
		{Prefix: "tests", SourceCount: 200, IsTest: true},
	}
	allocateBudget(strata, 80, 0.2)

	// Source pool = 64, test pool = 16.
	assert.InDelta(t, 64, strata[0].Budget, 2, "source gets ~80%% of budget")
	assert.InDelta(t, 16, strata[1].Budget, 2, "tests get ~20%% of budget")
	assert.Equal(t, 80, strata[0].Budget+strata[1].Budget, "total = budget")
}

func TestAllocateBudget_Empty(t *testing.T) {
	var strata []Stratum
	allocateBudget(strata, 80, 0.2)
	// No panic.
}
