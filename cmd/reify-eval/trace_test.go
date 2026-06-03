package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseToolCalls(t *testing.T) {
	// One assistant event with a Bash tool_use, one with an Edit tool_use,
	// plus noise lines that must be skipped.
	stream := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"ok"},{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}
not json at all
{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"src/calc.go"}}]}}
{"type":"result","subtype":"success"}`
	calls := parseToolCalls(stream)
	assert.Len(t, calls, 2)
	assert.Equal(t, "Bash", calls[0].Name)
	assert.Equal(t, "go test ./...", calls[0].Command)
	assert.Equal(t, "", calls[0].FilePath)
	assert.Equal(t, "Edit", calls[1].Name)
	assert.Equal(t, "src/calc.go", calls[1].FilePath)
	assert.Equal(t, "", calls[1].Command)
}

func TestParseToolCallsEmpty(t *testing.T) {
	assert.Empty(t, parseToolCalls(""))
	assert.Empty(t, parseToolCalls("{\"type\":\"result\"}"))
}

func TestRanTestCommand(t *testing.T) {
	calls := []ToolCall{
		{Name: "Edit", FilePath: "src/calc.go"},
		{Name: "Bash", Command: "go build ./..."},
		{Name: "Bash", Command: "go test ./... -run TestX"},
	}
	assert.True(t, ranTestCommand(calls, "go test"))
	assert.False(t, ranTestCommand(calls, "pytest"))
	// no calls => false
	assert.False(t, ranTestCommand(nil, "go test"))
	// pattern matches anywhere in the command
	assert.True(t, ranTestCommand([]ToolCall{{Name: "Bash", Command: "cd x && go test"}}, "go test"))
	// only Bash commands count, not file paths that happen to contain the pattern
	assert.False(t, ranTestCommand([]ToolCall{{Name: "Edit", FilePath: "go test helper"}}, "go test"))
}
