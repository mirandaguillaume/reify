package model

// ProjectConfig holds harness-specific runtime configuration that lives at
// the PROJECT level, distinct from the per-skill instruction model
// (SkillBehavior). These are native harness features — hooks, MCP server
// declarations, and native skill files — that reify captures on import and
// either ports natively or degrades-with-warning on build, per target.
//
// This is deliberately separate from SkillBehavior: SkillBehavior is
// reify's abstract source of truth (the 5 facets); ProjectConfig is
// concrete harness config carried across targets.
type ProjectConfig struct {
	Hooks        []Hook               `json:"hooks,omitempty"`
	MCPServers   map[string]MCPServer `json:"mcp_servers,omitempty"`
	NativeSkills []NativeSkill        `json:"native_skills,omitempty"`
}

// IsEmpty reports whether the project carries no harness config at all.
func (p ProjectConfig) IsEmpty() bool {
	return len(p.Hooks) == 0 && len(p.MCPServers) == 0 && len(p.NativeSkills) == 0
}

// Hook is a behaviour trigger: a shell command run on a tool event. This
// is the flattened form of one Claude Code settings.json hook entry
// (event -> matcher -> command). Hooks are *behaviour*: portable as a
// guarantee only to harnesses that have a hook system; elsewhere they
// degrade to a prose instruction (and check warns).
type Hook struct {
	Event   string `json:"event"`   // e.g. "PreToolUse", "PostToolUse"
	Matcher string `json:"matcher"` // tool name / glob the hook fires on
	Command string `json:"command"` // shell command to run
}

// MCPServer is one entry of the standard mcpServers schema shared across
// harnesses (Claude Code, Cursor, VS Code...). Only the declaration is
// modelled — the server binary/infra is out of scope. The JSON tags match
// the cross-tool schema so it unmarshals straight from .mcp.json.
type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// NativeSkill is a harness-native skill file (e.g. Claude Code
// .claude/skills/<name>/SKILL.md): frontmatter + markdown body. Ported
// verbatim to harnesses that support native skills; degraded/warned
// elsewhere.
type NativeSkill struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	Body                   string `json:"body"`
	DisableModelInvocation bool   `json:"disable_model_invocation,omitempty"`
}
