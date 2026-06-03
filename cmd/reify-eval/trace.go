package main

import (
	"encoding/json"
	"strings"
)

// ToolCall is one tool invocation extracted from a Claude Code stream-json
// trace. Command is set for Bash calls; FilePath for file tools (Edit,
// Write, Read). Only the fields we need for trace-based checks are kept.
type ToolCall struct {
	Name     string
	Command  string
	FilePath string
}

// parseToolCalls extracts tool_use blocks from a stream-json (JSONL)
// trace. Non-JSON and non-assistant lines are skipped. The stream format:
// each line is an event object; assistant events carry message.content[]
// which may include {"type":"tool_use","name":...,"input":{...}}.
func parseToolCalls(stream string) []ToolCall {
	var out []ToolCall
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev struct {
			Message struct {
				Content []struct {
					Type  string `json:"type"`
					Name  string `json:"name"`
					Input struct {
						Command  string `json:"command"`
						FilePath string `json:"file_path"`
					} `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		for _, b := range ev.Message.Content {
			if b.Type == "tool_use" {
				out = append(out, ToolCall{
					Name:     b.Name,
					Command:  b.Input.Command,
					FilePath: b.Input.FilePath,
				})
			}
		}
	}
	return out
}

// ranTestCommand reports whether any Bash tool call's command contains the
// given pattern (e.g. "go test"). Only Bash calls are considered — a file
// path that happens to contain the pattern does not count. This is the
// deterministic, judge-free check for "process" intentions like
// test-before-commit.
func ranTestCommand(calls []ToolCall, pattern string) bool {
	for _, c := range calls {
		if c.Name == "Bash" && strings.Contains(c.Command, pattern) {
			return true
		}
	}
	return false
}
