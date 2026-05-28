# ROADMAP — Dogfooding Reify on OSS projects + DX hardening

> Started 2026-05-28. Status: proposed, not started.

## Objective

1. **Phase 1 — Dogfooding demo**: run Reify (analysis mode: `doctor`, `classify`, `check`) on 5–8 well-known OSS projects to validate that the CLI handles real-world content and produce a visual summary.
2. **Phase 2 — Hardening**: turn the frictions surfaced by phase 1 into (a) bug fixes on parser/classifier/importer edge cases, (b) DX improvements (error messages, output formats, flag ergonomics, onboarding).

DX is handled **continuously** during phase 1 via a friction log (`/tmp/reify-dx-notes.md`), not as a separate refactor pass.

## Constraints & decisions

| Decision | Choice | Rationale |
|---|---|---|
| LLM provider | Anthropic | Analysis quality > cost; demo corpus is small |
| Storage | `/tmp/reify-dogfood-<date>/` | Ephemeral, no repo pollution |
| Repo list | curated (5–8), not GitHub crawl | Demo target, not statistical benchmark |
| Documents | English (per `CLAUDE.md` convention) | Consistent with code/comments rule |

## DX frictions already identified (before any run)

To address in phase 2 — without prejudging what phase 1 will surface in addition.

- **F1. Inconsistent output flags.** `classify --json`, `doctor --format`, `check` has neither. → standardize on `--format=text|json` across all analysis commands. (Angles: *output formats* + *ergonomics*.)
- **F2. Divergent error handling** when the LLM provider is missing: `classify` exits with a hint, `check` silently falls back to static, `doctor` has its own flow. → define a shared policy and extract a single code path. (Angle: *error messages*.)
- **F3. No consistent short flags.** Only `import` exposes `-p` for `--provider`. → decide: all or none. (Angle: *ergonomics*.)
- **F4. No auto-discovery of the instructions file.** `classify` and `check` require an explicit path; only `doctor` accepts a directory. → `reify classify` with no argument could search for `CLAUDE.md` / `.github/copilot-instructions.md` / `.cursorrules` in CWD. (Angles: *ergonomics* + *onboarding*.)
- **F5. Root `--help`** mentions `doctor agent.md` without explaining Reify's two modes (build = synthesis / doctor+classify+check+import = analysis). → rewrite `rootCmd.Long` to expose the duality. (Angle: *onboarding / discoverability*.)

## Phase 1 — Dogfooding demo

### Candidate targets (to finalize)

| Repo | Target file | Why |
|---|---|---|
| `anthropics/claude-code` | `CLAUDE.md` | "Official" reference — baseline for `doctor` |
| `vercel/next.js` | `.cursorrules` | Large monorepo, dense guardrails |
| `supabase/supabase` | `.cursorrules` | Dense mix of the 5 facets |
| `cline/cline` | `.clinerules/` | Non-standard IDE-agent format — `import` robustness test |
| `continuedev/continue` | `.continue/` | Same — yet another format |
| `astral-sh/ruff` | `CLAUDE.md` | Rust project with highly structured instructions |
| `microsoft/vscode` | `.github/copilot-instructions.md` | Native Copilot format |

### Deliverables

1. `/tmp/dogfood-oss.sh` (throwaway, **not committed** — generated in `/tmp`) which:
   - shallow-clones each repo into `/tmp/reify-dogfood-<date>/<repo>/`,
   - runs `reify doctor`, `reify classify --json`, `reify check` on the instructions file,
   - captures stdout/stderr/exit code per command,
   - aggregates into `/tmp/reify-dogfood-<date>/RESULTS.md` (a Markdown table).
2. `/tmp/reify-dx-notes.md` — friction log written live during the run. Categories: `error`, `output`, `ergonomy`, `onboarding`.
3. Final summary (~10 lines) pasted into the phase 2 PR as motivation.

### Success criteria

- Reify does not crash (Go panic) on any of the repos. A crash = blocking bug, log first.
- At least 4/7 produce a usable `classify` output (≥3 facets detected).
- The DX log holds at least 5 dated entries at end of session.

## Phase 2 — Hardening

### Per-friction workflow

```
friction logged  →  reproduce with minimal test  →  TDD: failing test  →  fix  →  green  →  commit
```

For parser/classifier/importer bugs uncovered on edge cases: **always a regression test before the fix**. This is the discipline from the `test-driven-development` skill.

### DX fix plan (to be completed from phase 1 notes)

- [ ] F1. Standardize `--format=text|json` across `classify`, `check`, `doctor`. Migration: keep `--json` on `classify` as a deprecated alias.
- [ ] F2. Extract a shared `requireOrFallbackProvider()` in `internal/cmd/provider.go`, use it across the 4 LLM-touching commands.
- [ ] F3. Decide short flags. Proposal: `-p` everywhere for `--provider`, `-m` for `--model`, `-f` for `--format`. Document in `CLAUDE.md`.
- [ ] F4. Implement auto-discovery of the instructions file in `classify`/`check` when no argument is passed. Priority: `CLAUDE.md` > `.github/copilot-instructions.md` > `.cursorrules`.
- [ ] F5. Rewrite `rootCmd.Long` to expose both modes (build / analyze) with one example per mode.
- [x] **F7 (done 2026-05-28).** `classify` and `check` now accept a file, an agent directory, or a repo root.
    - New package `internal/discovery/` exposes `Resolve(path)` and `DiscoverAgentFiles(root)`.
    - `internal/doctor/directory.go` removed; `doctor` imports from `internal/discovery/`.
    - JSON output for `classify` is now always an array `[{file, format, facets}, ...]` (breaking: single-file invocations produce an array of length one).
    - `--verbose` flag added to `classify` and `check` to switch directory-mode output from summary table to per-file detail.
    - Verified on the original failure cases: `anthropics/claude-code` now classifies 26 files (was 0); `continuedev/continue` classifies 24 (was 0).

### Out of scope (intentionally)

- No refactor of `internal/cmd/doctor.go` despite its 28k size. Too large for this ROADMAP, deserves a dedicated issue.
- No new target generator.
- No change to the YAML spec format.

## Follow-up

At the end of phase 1, update this ROADMAP with:
- the final list of frictions (F6+ added),
- concrete target status (which produced usable output, which crashed),
- revised estimate for phase 2.
