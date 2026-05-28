package cmd_test

import (
	"testing"

	"github.com/mirandaguillaume/reify/internal/cmd"
	"github.com/stretchr/testify/assert"
)

func TestFormatTimestamp(t *testing.T) {
	ts := cmd.FormatTimestamp()
	assert.Regexp(t, `^\d{2}:\d{2}:\d{2}$`, ts)
}

func TestIsRelevantFile(t *testing.T) {
	assert.True(t, cmd.IsRelevantFile("test.skill.yaml"))
	assert.True(t, cmd.IsRelevantFile("test.agent.yaml"))
	assert.False(t, cmd.IsRelevantFile("test.yaml"))
	assert.False(t, cmd.IsRelevantFile("test.go"))
	assert.False(t, cmd.IsRelevantFile(""))
}
