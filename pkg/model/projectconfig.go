package model

import "encoding/json"

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
//
// Two transports are modelled: local stdio servers (Command/Args/Env) and
// remote HTTP/SSE servers (URL/Headers); Type names the transport and is
// empty for stdio. Modelling the remote case is what stops a real .mcp.json
// (which routinely carries HTTP servers like Figma or Google Drive) from
// being silently corrupted into an empty-command stdio entry on a port.
type MCPServer struct {
	Type string // "stdio" (default), "http", or "sse"

	// stdio transport
	Command string
	Args    []string
	Env     map[string]string

	// http / sse transport
	URL     string
	Headers map[string]string

	// Extra preserves any other keys present in the source declaration
	// (e.g. cwd, envFile, oauth, sandboxEnabled, tool-specific fields) so a
	// same-schema port round-trips losslessly instead of silently dropping
	// fields reify doesn't explicitly model. Cross-schema renderers (the
	// Copilot surfaces) only carry the fields they know.
	Extra map[string]json.RawMessage `json:"-"`
}

// IsRemote reports whether this is a remote (HTTP/SSE) server rather than a
// local stdio command — i.e. it carries a URL or an explicit http/sse type.
func (s MCPServer) IsRemote() bool {
	return s.URL != "" || s.Type == "http" || s.Type == "sse"
}

// mcpKnownKeys are the fields MCPServer models explicitly; everything else in
// a server declaration is preserved verbatim in Extra.
var mcpKnownKeys = map[string]bool{
	"type": true, "command": true, "args": true, "env": true, "url": true, "headers": true,
}

// UnmarshalJSON reads the standard mcpServers schema, routing modelled keys to
// their fields and stashing every other key in Extra (lossless passthrough).
func (s *MCPServer) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type known struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	var k known
	if err := json.Unmarshal(data, &k); err != nil {
		return err
	}
	s.Type, s.Command, s.Args, s.Env, s.URL, s.Headers = k.Type, k.Command, k.Args, k.Env, k.URL, k.Headers
	for key, v := range raw {
		if !mcpKnownKeys[key] {
			if s.Extra == nil {
				s.Extra = map[string]json.RawMessage{}
			}
			s.Extra[key] = v
		}
	}
	return nil
}

// MarshalJSON writes the modelled fields (omitting empties) merged with the
// preserved Extra keys, reproducing the source declaration.
func (s MCPServer) MarshalJSON() ([]byte, error) {
	out := map[string]json.RawMessage{}
	put := func(key string, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		out[key] = b
		return nil
	}
	if s.Type != "" {
		if err := put("type", s.Type); err != nil {
			return nil, err
		}
	}
	if s.Command != "" {
		if err := put("command", s.Command); err != nil {
			return nil, err
		}
	}
	if len(s.Args) > 0 {
		if err := put("args", s.Args); err != nil {
			return nil, err
		}
	}
	if len(s.Env) > 0 {
		if err := put("env", s.Env); err != nil {
			return nil, err
		}
	}
	if s.URL != "" {
		if err := put("url", s.URL); err != nil {
			return nil, err
		}
	}
	if len(s.Headers) > 0 {
		if err := put("headers", s.Headers); err != nil {
			return nil, err
		}
	}
	for key, v := range s.Extra {
		out[key] = v
	}
	return json.Marshal(out)
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
