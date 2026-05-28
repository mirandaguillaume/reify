# Reify

**Write your AI agent context once. Compile it to any coding harness.**

Reify reads agent context files (`CLAUDE.md`, `copilot-instructions.md`, `.cursorrules`...) and compiles them to other formats — or generates them from scratch using a single YAML source of truth. It also analyses what you have: where the gaps are, and which instructions are likely to be ignored.

The name comes from *reify* — "to make concrete what is abstract." That's what the pipeline does: it turns informal markdown intent into structured, portable, measurable specifications.

---

## Status

Pre-1.0. The CLI works end-to-end on real OSS projects (VS Code, MCP servers, LiteLLM, Anthropic Cookbook). The runtime evaluation framework that measures empirical compliance is intentionally left out of this release.

## Install

```sh
go install github.com/mirandaguillaume/reify/cmd/reify@latest
```

Or build from source:

```sh
git clone https://github.com/mirandaguillaume/reify
cd reify
go build ./cmd/reify
```

## Quick start

```sh
# Initialise a project
reify init

# Take an existing context file and decompose it into reusable skills
export ANTHROPIC_REIFY_API_KEY=sk-ant-...
reify import CLAUDE.md

# Or work the other way: write skills in YAML, compile to any target
reify build --target claude    # → .claude/skills/ + .claude/CLAUDE.md
reify build --target copilot   # → .github/skills/ + copilot-instructions.md
reify build --target cursor    # → .cursor/rules/*.mdc + .cursorrules

# Analyse what you have
reify classify CLAUDE.md       # → which facet does each line belong to?
reify doctor CLAUDE.md         # → what's structurally missing?
reify check CLAUDE.md          # → which instructions are likely ignored, per harness?
```

## What it does

### `reify classify <file>`

Maps each instruction in an agent file to one of five facets — context, strategy, guardrails, observability, security. Static by default, or LLM-powered (`--llm`) for semantic accuracy on ambiguous lines.

### `reify import <file>`

Reads a `CLAUDE.md`, `copilot-instructions.md`, or any agent file. Decomposes it into reusable skill specs with explicit `consumes`/`produces` contracts, then optionally composes them into an agent. Output is structured YAML that survives format changes.

### `reify build --target <name>`

Compiles your YAML skills and agents into a target framework's native format. Currently supported targets: `claude`, `copilot`, `cursor`, `reify` (standalone Go runtime).

Each target gets:
- skill files in the right place with the right format
- agent files with the right orchestration syntax
- a root index file (`CLAUDE.md`, `copilot-instructions.md`, `.cursorrules`) listing what's available

### `reify doctor <file>`

LLM-powered structural analysis. Checks 16 elements based on published research on agent context files. Tells you what's missing — `architecture_hints`, `decision_authority`, `error_handling`, etc.

### `reify check <file>`

Estimates compliance risk per harness, based on documented findings:

- **Negative framing** — instructions like *"never use X"* tend to be followed less reliably than *"always use Y"*. Source: IFEval (Zhou et al., 2023).
- **Middle position** — content in the middle of a file is less reliably followed than content at the start or end. Source: Liu et al. 2023 "Lost in the Middle"; Veseli et al. 2025.
- **Semantic vs. verifiable** — constraints you can check statically (file extensions, tabs vs spaces, copyright headers) are more reliably enforced than those that require judgement.
- **Harness-specific weaknesses** — observed differences between Claude Code, Copilot, and Cursor on the same kind of instruction. Explicitly labelled as community observation, not peer-reviewed.

Output is qualitative risk levels with cited factors. No invented percentages.

## Reify YAML format

A skill is a YAML file declaring its inputs, outputs, and behaviour across five facets:

```yaml
skill: code-review
version: "0.1.0"
context:
  consumes: [pull_request]
  produces: [review_report]
  memory: short-term
strategy:
  tools: [read_file, grep]
  approach: Read the diff and surface correctness bugs
  steps:
    - read the diff
    - identify changed functions
    - check for null-handling and concurrency bugs
guardrails:
  - never approve PRs that drop tests
  - timeout: 120s
observability:
  trace_level: minimal
  metrics: [issues_found]
security:
  filesystem: read-only
  network: none
  secrets: []
negotiation:
  file_conflicts: yield
  priority: 0
```

An agent composes skills with an orchestration strategy:

```yaml
agent: pr-reviewer
skills: [code-review, security-audit, test-coverage]
orchestration: parallel-then-merge
description: Reviews pull requests across correctness, security, and coverage
```

## Targets

| Target  | Output                                              | Status |
|---------|-----------------------------------------------------|--------|
| claude  | `.claude/skills/<name>/SKILL.md` + `.claude/CLAUDE.md` + `.claude/agents/<name>.md` | stable |
| copilot | `.github/skills/<name>/SKILL.md` + `.github/copilot-instructions.md` + `.github/agents/<name>.agent.md` | stable |
| cursor  | `.cursor/rules/<name>.mdc` + `.cursorrules`         | beta |
| reify   | standalone Go binary (`main.go` + `go.mod`)         | experimental |

Adding a target means implementing the `pkg/spec.Generator` interface — see `internal/generator/cursor/` as a reference.

## LLM providers

Reify uses an LLM for `import`, `classify --llm`, `doctor`, and `check`. Auto-detected in order:

1. `--provider` flag explicit
2. Ollama (if `http://localhost:11434` responds)
3. `ANTHROPIC_REIFY_API_KEY` / `ANTHROPIC_API_KEY` → Anthropic
4. `OPENROUTER_REIFY_API_KEY` / `OPENROUTER_API_KEY` → OpenRouter

Default model when using Anthropic is Claude Haiku — small, fast, sufficient for classification and structural analysis.

## Limitations

- The risk model in `reify check` is heuristic. The factors are documented, but the *interaction* between them isn't peer-reviewed. Treat the qualitative risk levels as informed hypotheses, not measurements.
- Tool name mapping during cross-target builds is best-effort. Some harness-specific features (Claude Code hooks, Copilot agent mode tools) don't have direct equivalents and are signalled rather than translated.
- `cursor` target's `globs` field is inferred from skill name and tool list. Verify before relying on conditional rule loading.

## License

Apache License 2.0. See [`LICENSE`](./LICENSE).

## References

- Liu et al., 2023. *Lost in the Middle: How Language Models Use Long Contexts.* [arXiv:2307.03172](https://arxiv.org/abs/2307.03172)
- Veseli et al., 2025. *Positional Biases Shift as Inputs Approach Context Window Limits.* [arXiv:2508.07479](https://arxiv.org/abs/2508.07479)
- Zhou et al., 2023. *Instruction-Following Evaluation for Large Language Models (IFEval).* [arXiv:2311.07911](https://arxiv.org/abs/2311.07911)
- Liu et al., 2026. *Dive into Claude Code: The Design Space of Today's and Future AI Agent Systems.* [arXiv:2604.14228](https://arxiv.org/abs/2604.14228)
