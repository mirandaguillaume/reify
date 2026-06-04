# Validating ported config against the real world

## The problem this solves

reify's unit tests assert that each target emits the config its author *coded*
— e.g. "copilot-vscode writes a `servers` key with `type:stdio`". But the code
writes that for the same reason the test asserts it: the author's reading of a
doc. **Both come from one source, so the test is circular** — it proves
internal consistency, not that the output is valid or that the real tool loads
it. This is the same trap the reify-eval bench avoids for obedience claims.

Three levels of confidence, only the first of which unit tests reach:

| Level | Proves | Source |
|---|---|---|
| Self-consistency (unit tests) | reify emits what its author intended | reify itself |
| **Schema conformance** | output is valid per an external schema | published JSON Schema |
| **Harness acceptance** | the real tool actually loads it | the tool's own CLI |

This document records the level-2 and level-3 checks reify actually runs.

## What is validated (and the result)

Run with `make validate` (or `go test -tags validation ./...`). The tests skip
gracefully when a validator or harness CLI is absent, so normal CI stays green
and dependency-free.

| Target / artifact | Level reached | Mechanism | Result |
|---|---|---|---|
| claude `settings.json` (hooks) | schema conformance | vendored official **SchemaStore "Claude Code Settings"** schema, checked with `jsonschema`/`check-jsonschema` | ✅ conforms |
| claude `.mcp.json` | **harness acceptance** | fed to the real `claude mcp get`; it recognizes the server and attributes it to the `.mcp.json` reify wrote | ✅ accepted |

Evidence (real `claude` output during validation):

```
$ claude mcp get git
git:
  Scope: Project config (shared via .mcp.json)
```

## Real-case run (not a synthetic fixture)

Beyond the gated tests (which use a toy config), the pipeline was exercised on
**real production config**: a developer's actual `~/.claude/settings.json`
(2 hooks) plus a real project `.mcp.json` (2 servers: `grepai`, `serena`).

| Step | Result |
|---|---|
| `reify check --input` on the real source | parsed clean; claude ✓ full, cursor/copilot ⚠ degrade (2 hooks) |
| emitted `settings.json` vs official schema | `jsonschema` exit 0 — valid |
| emitted `.mcp.json` vs real `claude mcp list` | both servers recognized (claude even flagged `serena` as multiply-scoped — proof it actually loaded and reasoned over the file) |
| `reify build --target cursor --input` | degraded correctly (`mcp.json` kept, hooks → `rules/ported-hooks.mdc` + warning) |

The temp project was deleted after the run — it held a copy of the real
`.mcp.json` whose MCP `env` may carry secrets. Real-case validation must not
leave secret copies behind.

## Honest gaps

These are **not** validated beyond self-consistency, and the reason is recorded
so it isn't mistaken for coverage:

- **VS Code / Cursor / Copilot MCP schemas don't exist as fetchable artifacts.**
  VS Code's `mcp.json` schema is internal (`vscode://`, not downloadable), and
  the MCP project has an *open* issue to define a standard config schema
  (modelcontextprotocol#292) — so there is no external schema to validate
  `copilot-vscode` / `cursor` / `copilot-cli` output against today.
- **Those tools are not installed here**, so harness-acceptance round-trips
  aren't possible for them in this environment. The validation tests will pick
  them up automatically if/when a CLI for them appears (extend
  `validation_test.go` per target).
- **Hooks and native skills degraded to prose are unverifiable by construction**
  — they are explicitly "not enforced" (that is the warning reify emits), so
  there is nothing for a schema or harness to accept.

## How to extend

Add a `//go:build validation` test next to a target's generator that:

1. emits the config to a temp dir (honoring `../` paths, as the builder does);
2. validates it against a vendored official schema (level 2) and/or feeds it to
   the real harness CLI (level 3), skipping when the tool is absent.

Vendor schemas under the target package's `testdata/`. Prefer a published
schema (SchemaStore, vendor repo) over a hand-written one — a hand-written
schema reintroduces the circularity this whole effort removes.
