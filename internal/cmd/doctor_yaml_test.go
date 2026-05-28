package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorSkills_Lint asserts that all skills in skills/doctor/ pass lint with zero errors.
func TestDoctorSkills_Lint(t *testing.T) {
	result := LintDirectory("../../skills/doctor")

	require.Equal(t, 4, result.TotalFiles, "expected 4 doctor skill files")
	assert.Equal(t, 0, result.Errors, "doctor skills must have zero lint errors")
}

// TestDoctorAgent_Score asserts that the doctor agent scores at least 80/100.
func TestDoctorAgent_Score(t *testing.T) {
	report := RunScore("../../skills/doctor", "../../agents")

	var doctorScore int
	found := false
	for _, a := range report.Agents {
		if a.Agent == "doctor" {
			doctorScore = a.Total
			found = true
			break
		}
	}

	require.True(t, found, "doctor agent must be present in score report")
	assert.GreaterOrEqual(t, doctorScore, 95, "doctor agent score must be >= 95, got %d", doctorScore)
}
