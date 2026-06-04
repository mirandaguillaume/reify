package claude

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirandaguillaume/reify/internal/generator"
	"github.com/mirandaguillaume/reify/pkg/model"
	"github.com/mirandaguillaume/reify/pkg/spec"
)

// GenerateConfig re-emits project-level harness config in Claude Code's
// native formats. Claude Code supports hooks, MCP, and native skills, so
// everything is preserved verbatim — no degradation, no warnings.
//
// Output dir is .claude, so: settings.json (hooks), ../.mcp.json (root),
// skills/<name>/SKILL.md (native skills).
func (g *claudeGenerator) GenerateConfig(cfg model.ProjectConfig) ([]spec.ConfigFile, []string) {
	if cfg.IsEmpty() {
		return nil, nil
	}
	var files []spec.ConfigFile

	if len(cfg.Hooks) > 0 {
		files = append(files, spec.ConfigFile{Path: "settings.json", Content: renderHooks(cfg.Hooks)})
	}
	if len(cfg.MCPServers) > 0 {
		files = append(files, spec.ConfigFile{Path: "../.mcp.json", Content: generator.RenderMCPServers(cfg.MCPServers)})
	}
	for _, s := range cfg.NativeSkills {
		files = append(files, spec.ConfigFile{
			Path:    "skills/" + s.Name + "/SKILL.md",
			Content: renderSkillMd(s),
		})
	}
	return files, nil
}

// renderHooks rebuilds Claude Code's nested settings.json hooks schema
// (event -> [{matcher, hooks:[{type:command, command}]}]) from the
// flattened model. Hooks with the same event+matcher are grouped under one
// matcher entry, preserving command order.
func renderHooks(hooks []model.Hook) string {
	type cmdEntry struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type matcherEntry struct {
		Matcher string     `json:"matcher"`
		Hooks   []cmdEntry `json:"hooks"`
	}

	// event -> matcher -> []commands, keeping first-seen matcher order.
	byEvent := map[string][]string{}            // event -> ordered matchers
	cmds := map[string]map[string][]string{}    // event -> matcher -> commands
	for _, h := range hooks {
		if cmds[h.Event] == nil {
			cmds[h.Event] = map[string][]string{}
		}
		if _, seen := cmds[h.Event][h.Matcher]; !seen {
			byEvent[h.Event] = append(byEvent[h.Event], h.Matcher)
		}
		cmds[h.Event][h.Matcher] = append(cmds[h.Event][h.Matcher], h.Command)
	}

	events := make([]string, 0, len(byEvent))
	for e := range byEvent {
		events = append(events, e)
	}
	sort.Strings(events)

	out := map[string][]matcherEntry{}
	for _, e := range events {
		for _, m := range byEvent[e] {
			var entries []cmdEntry
			for _, c := range cmds[e][m] {
				entries = append(entries, cmdEntry{Type: "command", Command: c})
			}
			out[e] = append(out[e], matcherEntry{Matcher: m, Hooks: entries})
		}
	}

	doc := map[string]any{"hooks": out}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b) + "\n"
}

// renderSkillMd writes a native SKILL.md: YAML frontmatter + body.
func renderSkillMd(s model.NativeSkill) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", s.Name)
	if s.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", s.Description)
	}
	if s.DisableModelInvocation {
		b.WriteString("disable-model-invocation: true\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(s.Body))
	b.WriteString("\n")
	return b.String()
}
