# Harness targets — design note

> Status: design spec for new compilation targets in the reify compiler
> (the product — `reify build` / `import` / `check`), NOT the reify-eval
> research bench. Captures a brainstorm (2026-06-03). The perimeter grew
> across the conversation; this note freezes it so we build one target
> end-to-end before widening.
>
> Reference: existing generators in `internal/generator/{claude,copilot,
> cursor,reify}/`, the `pkg/spec.Generator` interface, and the `pkg/`
> public-boundary note in CLAUDE.md ("new targets are compile-time
> plugins").

## 0. Repositioning — what reify actually is

Building this chapter clarified the product. "Multi-harness compiler"
undersells it (sounds like a markdown templater). What reify is becoming:

> **A translator of AI tooling from one ecosystem to another.** Not just
> prose instructions — the whole working environment: automations
> (hooks), connected tools (MCP), packaged competencies (native skills).
> A transpiler of *agent universes*.

Crucially, fidelity is **per-pillar, and reify is honest about it**:

| Pillar | Translation fidelity |
|---|---|
| MCP | **Full** — shared `mcpServers` schema; copy to each target's path, no loss. |
| Native skills | **Partial** — same SKILL.md across Claude/Copilot; Cursor/Aider lack the concept → passthrough or degrade. |
| Hooks | **Lossy** — no standard; a hook (an *enforced guarantee*) becomes a prose *suggestion* the LLM may ignore. We translate the intent, not the guarantee. |

The product is not "works everywhere" — it's "here is what translates
faithfully, and here is exactly what degrades and how" (the `check`
warnings). That honesty is the differentiator.

## 1. Goal

Reify's wedge is multi-harness compilation (1 source → N harnesses). The
existing targets are claude, copilot, cursor, reify. This adds more, and
deepens what "a target" means — beyond prose instructions to the runtime
config a real setup carries.

Verified output formats (from each tool's docs):

| Target | Instructions file | Loaded how |
|---|---|---|
| `agents` (also serves Codex, Zed, Aider-recent) | `AGENTS.md` (root, nestable per dir) | **auto** — agent reads nearest in tree |
| `windsurf` | `.windsurfrules` (root, legacy but still read) | auto |
| `aider` | `CONVENTIONS.md` (root) | **NOT auto** — needs `read: CONVENTIONS.md` in `.aider.conf.yml` |

## 2. The four axes a target must handle

A target is not just "write a file". The brainstorm surfaced four things
that differ per harness:

### 2a. Instructions (prose) — already modelled
Skills + facet-ordered rules. Existing generators already produce this via
`GenerateInstructions` / `GenerateSkill`. New targets reuse the same
`model.SkillBehavior` → just a new renderer + path. **Cheapest axis.**

### 2b. Loading mechanism — input differs, not just output
Writing the file is not enough if the harness doesn't load it:
- `agents`/`windsurf`: file is auto-discovered → writing it suffices.
- `aider`: **must also write/merge `.aider.conf.yml`** with a
  `read: CONVENTIONS.md` entry, else the file is ignored. A target that
  needs a second artefact to be effective must emit it. ("Fidelity:
  file + loading mechanism" — the chosen support level.)

### 2c. MCP config — portable by schema, differs by path
Correction during brainstorm: MCP config **is** portable. All harnesses
share the `mcpServers` JSON schema
(`{"mcpServers":{"name":{"command","args","env"}}}`); only the file path
differs. So MCP is a **mechanical path-mapping**, not a translation
problem. Project-level paths (reify compiles a project, not a user home):

| Target | MCP file (project) |
|---|---|
| claude | `.mcp.json` (root) |
| cursor | `.cursor/mcp.json` |
| vscode/copilot | `.vscode/mcp.json` |
| windsurf | (user-level `~/.codeium/...`; no standard project file — warn) |
| agents/aider | no native MCP file — warn / skip |

The MCP *server* (the binary, the infra) is out of scope — that's machine
install, not compilation. Only the **declaration** is compiled.

### 2d. Hooks — the hard axis: degrade + warn
A hook (Claude Code `settings.json` PreToolUse/PostToolUse + shell
command) is *behaviour*, and most harnesses (cursor/windsurf/aider) have
**no hook system**. Compilation rule:
- Target WITH native hooks (claude) → preserve verbatim.
- Target WITHOUT hooks → **degrade the hook to a prose instruction**
  ("after editing a .go file, run gofmt") AND have `check` **warn** that
  the guarantee is lost (a harness-enforced hook becomes a model
  suggestion the LLM may ignore).

This degradation-with-warning is the core of the "faithful translator"
value: reify never silently drops a guarantee — it tells you where one
weakens crossing targets. (Directly ties to the moat note's
fidelity thesis.)

## 3. Model changes (`pkg/model`)

New concepts the model must represent so import → build can carry them:

- **Hook**: `{Event: pre|post, Tool: glob, Command: string}` — parsed
  from `.claude/settings.json`.
- **MCPServer**: `{Name, Command, Args[], Env{}}` — parsed from
  `.mcp.json`. A near-verbatim struct of the shared schema.
- **Skills**: already exist (`SkillBehavior`) — new targets get them
  free.

These are additive to the model; treat as a `pkg/` contract change
(breaking, update internal/ call sites in the same change), per the
stability note.

## 4. Compilation matrix (what build emits per target)

| | instructions | skills | MCP | hooks |
|---|---|---|---|---|
| claude | CLAUDE.md | .claude/skills/ | .mcp.json (verbatim) | settings.json (verbatim) |
| agents | AGENTS.md | inline/section | — (warn) | prose + warn |
| windsurf | .windsurfrules | inline/section | — (warn) | prose + warn |
| aider | CONVENTIONS.md + .aider.conf.yml | inline | — (warn) | prose + warn |
| cursor (exists) | .cursorrules | .cursor/rules/ | .cursor/mcp.json | prose + warn |

## 5. Import (harness → reify)

`reify import` must read a real `.claude/` setup: instructions (prose →
skills/facets, already done), **+ `settings.json` hooks → Hook model**,
**+ `.mcp.json` → MCPServer model**. This is the inverse direction; it
populates the model so build can re-emit to any target.

## 6. Build ONE target end-to-end first (anti-drift)

The perimeter is wide; do NOT build it all at once. First increment:
**`agents` target**, complete:
- `GenerateInstructions` → AGENTS.md (prose + skills as sections)
- registered via `spec.Register("agents", ...)` + blank-import in builder
- generator tests (frontmatter/section order/path parity with siblings)
- `check`: warn that hooks degrade to prose on this target

`agents` is chosen because it's auto-loaded (no loading-mechanism
complication) and the most universal (Codex/Zed/Aider-recent read it) —
maximum value, minimum special-casing. Hooks/MCP carry-through and the
other targets (windsurf, aider with its conf.yml) follow once the model
gains Hook/MCPServer.

## 7. Out of scope

- MCP servers themselves (install/infra), only their declaration.
- User-level config paths (reify compiles a project).
- The reify-eval research bench (separate branch/effort).
- Driving harnesses headlessly (that was eval-run-004, a different thing).

## 8. Sequencing

1. `agents` target (instructions + skills only) — prove the increment,
   merge. No model change needed (reuses SkillBehavior).
2. `pkg/model`: add Hook + MCPServer.
3. `import`: parse settings.json hooks + .mcp.json into the model.
4. MCP path-mapping in build (mechanical, per target).
5. Hook degradation + `check` warnings.
6. windsurf + aider targets (aider needs the `.aider.conf.yml` artefact).

Each step is independently shippable and gated on the previous.
