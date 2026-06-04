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
original targets were claude, copilot, cursor, reify. This note added more
and deepened what "a target" means — beyond prose instructions to the
runtime config a real setup carries.

> **Implemented set (see §4 for the full matrix):** claude, cursor,
> copilot, **copilot-vscode / copilot-cli / copilot-jetbrains** (Copilot is
> not one target — each surface reads MCP differently, §2c), agents, reify.
> windsurf + aider are specced below but not yet built.

Verified output formats for the prose targets added (from each tool's docs):

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

### 2c. MCP config — path-map for some, schema-map for others
The brainstorm assumed MCP was purely portable — one shared `mcpServers`
schema, only the path differing. **Implementation proved that wrong.**
Verifying each surface against current vendor docs (June 2026) showed the
*schema* differs too, so MCP is a **path-map for some targets and a
schema-map for others**. The implemented per-surface reality:

| Target / surface | MCP file (project) | Root key | Per-server shape | Fidelity |
|---|---|---|---|---|
| claude | `.mcp.json` (root) | `mcpServers` | `command/args/env` | full, verbatim |
| cursor | `.cursor/mcp.json` | `mcpServers` | `command/args/env` | full, path-map |
| copilot-vscode | `.vscode/mcp.json` | **`servers`** | **`type:"stdio"`** + command/args/env | full, **schema-map** |
| copilot-cli | `~/.copilot/mcp-config.json` | `mcpServers` | **`type:"local"`** + `tools:["*"]` | artefact + warn (**user-level**, out of project) |
| copilot-jetbrains | UI-managed (Copilot icon → settings) | — | — | snippet + warn (**no project file**) |
| agents | none standard | `mcpServers` (reference) | command/args/env | reference + warn |
| windsurf / aider | none standard (not yet implemented) | — | — | warn / skip |

Key correction: **a naive converter that copies the `mcpServers` blob
verbatim produces invalid config for VS Code** (which needs `servers` +
`type:stdio`). reify schema-maps per surface and warns where a surface has
no writable project file. The MCP *server* (binary/infra) stays out of
scope — only the **declaration** is compiled.

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

New concepts the model represents so import → build can carry them, grouped
under one `model.ProjectConfig` (distinct from per-skill `SkillBehavior`):

- **Hook**: `{Event, Matcher, Command string}` — the flattened form of one
  `.claude/settings.json` hook entry (event → matcher → command).
- **MCPServer**: `{Command string, Args []string, Env map}` — keyed by name
  in `ProjectConfig.MCPServers map[string]MCPServer`. JSON tags match the
  shared `.mcp.json` schema so it unmarshals directly. (HTTP/remote servers
  — `url`/`headers` — are not modelled yet; only stdio command servers.)
- **NativeSkill**: `{Name, Description, Body, DisableModelInvocation}` —
  parsed from `.claude/skills/<name>/SKILL.md`.
- **Skills (abstract)**: `SkillBehavior` already exists — the 5-facet source
  of truth, separate from the concrete harness config above.

These are additive to the model; treated as a `pkg/` contract change per the
stability note.

## 4. Compilation matrix (implemented)

Fidelity per pillar: ✅ native/full · ⚠️ degraded-with-warning · ✗ not yet built.
The config pillars (MCP / hooks / native skills) are emitted by each target's
`spec.ConfigGenerator`; `build --input <project>` drives the port.

| Target | instructions | MCP | hooks | native skills |
|---|---|---|---|---|
| claude | `CLAUDE.md` | ✅ `.mcp.json` | ✅ `settings.json` | ✅ `SKILL.md` |
| cursor | `.cursorrules` | ✅ `.cursor/mcp.json` | ⚠️ prose `.mdc` | ⚠️ rule `.mdc` |
| copilot-vscode | `.github/copilot-instructions.md` | ✅ `.vscode/mcp.json` (schema-map) | ⚠️ prose | ⚠️ prose |
| copilot-cli | (shared copilot instr.) | ⚠️ portable artefact (user-level) | ⚠️ prose | ⚠️ prose |
| copilot-jetbrains | (shared copilot instr.) | ⚠️ snippet (UI-managed) | ⚠️ prose | ⚠️ prose |
| copilot (generic) | `.github/copilot-instructions.md` | — (use a surface) | — | — |
| agents | `AGENTS.md` | ⚠️ reference snippet | ⚠️ prose | ⚠️ prose |
| windsurf | `.windsurfrules` | ✗ | ✗ | ✗ |
| aider | `CONVENTIONS.md` + `.aider.conf.yml` loader | ⚠️ reference snippet | ⚠️ prose | ⚠️ prose |

aider needed a new axis: a **loading artefact** (`.aider.conf.yml` with
`read: CONVENTIONS.md`) that aider must have to load the conventions file at
all. It's emitted via `spec.LoadingGenerator` (always, independent of
`--input`) and verified end-to-end against the real `aider` CLI.

The three Copilot surfaces are distinct targets (not one) because they read
MCP from different files with different schemas (§2c); they embed the base
copilot generator to share instruction/skill output. Shared port-config
renderers live in `internal/generator/portconfig.go`.

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

## 8. Sequencing — status

1. ✅ `agents` target (instructions + skills) — done; later gained
   observability/security rendering and a degrading `ConfigGenerator`.
2. ✅ `pkg/model`: `ProjectConfig` with `Hook{Event,Matcher,Command}`,
   `MCPServer{Command,Args,Env}` (map-keyed by name), `NativeSkill`.
3. ✅ `import`: `ImportProjectConfig` parses `settings.json` hooks,
   `.mcp.json`, and `.claude/skills/*/SKILL.md`.
4. ✅ MCP per target — and it was a **schema-map, not just a path-map**
   (§2c): claude/cursor `mcpServers`, copilot-vscode `servers`+stdio,
   copilot-cli `type:local`+tools.
5. ✅ Hook/skill degradation with build-time warnings, via
   `spec.ConfigGenerator` + `build --input`. ⚠️ `check`-time warnings (warn
   *before* a build) still TODO.
6. ✅ aider target — built, with the new `spec.LoadingGenerator` axis for its
   `.aider.conf.yml` loader; verified against the real aider CLI. ✗ windsurf
   still not built (GUI, no headless validation path).

Delivered targets: claude, cursor, copilot, copilot-vscode, copilot-cli,
copilot-jetbrains, agents, aider, reify. The port runs end-to-end via
`reify build --target <T> --input <project>`.

> **Deviation from §7 (out of scope).** copilot-cli's real MCP file is
> user-level (`~/.copilot/`). We hold the "no user-level writes" rule —
> reify emits a *project-local portable artifact* and warns the user to copy
> it. The rule constrains where we write, not what we acknowledge exists.
