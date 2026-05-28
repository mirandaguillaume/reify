# Reify — context for Claude Code

Reify compiles agent context files across coding harnesses (Claude Code, GitHub Copilot, Cursor) from a single Reify YAML source of truth. It also analyses existing context files: structural completeness (`doctor`), facet classification (`classify`), and compliance risk per harness (`check`).

## Tech stack

- Go 1.24+ (per `go.mod`)
- CLI: Cobra
- YAML: gopkg.in/yaml.v3
- Testing: testify (`assert` + `require`)
- LLM providers: Ollama (local), Anthropic, OpenRouter

## The 5 facets

Every instruction in a Reify spec belongs to exactly one facet. Generators emit sections in a facet-aware order (guardrails first, security last) because primacy/recency bias affects compliance.

| Facet | Maps to |
|---|---|
| `context` | background, project description, tech stack, architecture, conventions |
| `strategy` | tools, commands, workflows, how to approach a task |
| `guardrails` | prohibitions — must NOT do (never, don't, avoid) |
| `observability` | logging, metrics, monitoring, tracing, reporting |
| `security` | permissions, credentials, filesystem/network rules, secrets |

The canonical list lives in `internal/classifier/classifier.go` (`AllFacets`).

## Dataflow

`reify build` runs this pipeline in order — each step's output feeds the next:

```
YAML specs ──► yaml.Loader ──► linter ──► scanner (codebase index)
                                              │
                                              ▼
                                          enricher (attach index to skills)
                                              │
                                              ▼
                                          builder (orchestrate)
                                              │
                                              ▼
                                  spec.Generator (target-specific render)
                                              │
                                              ▼
                                  files in OutputDir
```

`doctor`, `classify`, `check`, `import` are independent pipelines that share the parser/classifier but skip the generator.

## Commands

```
reify init                            # initialise a Reify project
reify skill create <name>             # scaffold a new skill
reify classify <file>                 # map instructions to the 5 facets
reify import <source>                 # decompose an agent file into skills
reify doctor <file>                   # LLM-powered structural analysis
reify check <file>                    # compliance risk per harness
reify lint [path]                     # lint skill specs
reify score [path]                    # quality scoring
reify build --target claude           # compile to Claude Code
reify build --target copilot          # compile to GitHub Copilot
reify build --target cursor           # compile to Cursor
reify build --target reify            # compile to standalone Go binary
```

### Output paths per target

| Target | `DefaultOutputDir` | Instructions file |
|---|---|---|
| `claude` | `.claude` | `CLAUDE.md` |
| `copilot` | `.github` | `copilot-instructions.md` |
| `cursor` | `.cursor` | `../.cursorrules` (root) |
| `reify` | `.reify` | — |

## Dev commands

```sh
go test ./...                        # run all tests
go test -cover ./...                 # with coverage
go test ./internal/generator/...     # one package
go build ./cmd/reify                 # compile binary
go vet ./...                         # static analysis
```

Or via the Makefile (mirrors what CI runs):

```sh
make test         # all tests
make cover        # coverage + printed summary
make cover-html   # open HTML coverage report
make mutation     # mutation testing on pilot packages (requires gremlins)
make build        # compile to ./reify
```

## CI

- `.github/workflows/ci.yml` — runs on every push to `main` and PR. Verifies `go mod tidy`, runs `go vet`, executes the full test suite with `-race` and `-coverprofile`, uploads coverage to Codecov, then builds the CLI.
- `.github/workflows/mutation.yml` — runs nightly (cron `0 3 * * *`) and via manual `workflow_dispatch`. Executes [gremlins](https://github.com/go-gremlins/gremlins) on a pilot set of pure-logic packages (`pkg/dag`, `internal/classifier`, `internal/checker`). Results are uploaded as an artifact; the job never blocks PRs.

When extending the mutation pilot, **only add deterministic packages with high line coverage**. Mutation testing on LLM-touching code produces too many non-killable mutants to be useful.

## Environment variables

LLM-backed commands (`doctor`, `classify`, sometimes `import`) need a provider. Resolution order is per-provider:

| Var | Purpose |
|---|---|
| `REIFY_PROVIDER` | default provider (`anthropic`, `openrouter`, `ollama`) |
| `REIFY_MODEL` | default model name |
| `REIFY_API_KEY` | catch-all key, used when a provider-specific key is unset |
| `ANTHROPIC_API_KEY` / `ANTHROPIC_REIFY_API_KEY` | Anthropic provider |
| `OPENROUTER_API_KEY` / `OPENROUTER_REIFY_API_KEY` | OpenRouter provider |
| `REIFY_DEBUG` | verbose logging (LLM prompts/responses) |

Ollama uses its local HTTP endpoint and needs no API key.

## Repository layout

### `pkg/` — public, importable by external consumers

| Package | Purpose |
|---|---|
| `model/` | `SkillBehavior`, `AgentComposition`, validation |
| `spec/` | `Generator` interface + registry (extension point for new targets) |
| `dag/` | DAG engine (auto-wiring, layers, router, retry) |
| `analysis/` | formal property analysis |
| `schema/`, `sandbox/`, `qualitygate/`, `budget/` | supporting infrastructure |

> TODO (you): one sentence on *why* `pkg/` is public — what's the stability promise vs `internal/`? Embedding-as-library? Compile-time plugins? This shapes how aggressive we can be when refactoring `internal/`.

### `internal/` — implementation details

```
cmd/                            # Cobra command handlers
classifier/                     # 5-facet classifier (static + LLM)
checker/                        # compliance risk estimator (uses classifier)
doctor/                         # reify doctor — LLM-powered structural analysis
importer/                       # agent file → skill spec decomposition
builder/                        # build orchestration (entry point for `reify build`)
generator/
  claude/                       # Claude Code generator
  copilot/                      # GitHub Copilot generator
  cursor/                       # Cursor generator
  reify/                        # standalone Go runtime generator
analyzer/                       # dependency check, loop detector, scoring
linter/                         # lint rules
llm/                            # Provider interface + Ollama/Anthropic/OpenRouter impls
scanner/, enricher/             # codebase scanning + skill enrichment
config/                         # CLI config + env var resolution
fileops/                        # filesystem helpers
yaml/                           # YAML loader
```

### Top-level non-Go dirs

- `templates/` — embedded skill/agent YAML templates used by `reify init` / `skill create`.
- `skills/`, `agents/` — **dogfooded specs**. These are real Reify specs that compile into this repo's own `.claude/` so Claude Code can use them. Treat them as both production code *and* canonical examples.
- `docs/` — user-facing documentation.

## Adding a new target

1. Create `internal/generator/<target>/` with files mirroring `cursor/` (`<target>.go`, `instructions.go`, `skill.go`).
2. Implement `pkg/spec.Generator` (and optionally `SkillGenerator`, `InstructionsGenerator`, `Configurable`).
3. Self-register in `init()`:
   ```go
   func init() { spec.Register("<target>", func() spec.Generator { return &gen{} }) }
   ```
4. Add a blank-import in `internal/builder/builder.go` so the registration fires.
5. Add tests in the same package (see Testing).

## Adding a new LLM provider

1. Create `internal/llm/<provider>.go` implementing `llm.Provider` (`Complete(prompt) (string, error)`).
2. Optionally implement `AgenticProvider`, `StructuredProvider`, or `TokenAwareProvider` from `internal/llm/provider.go` for richer behavior.
3. Wire env-var resolution in `internal/cmd/provider.go`.

## Testing

- **Framework**: testify (`assert` for non-fatal, `require` for fatal preconditions).
- **Package suffix**: prefer `package <pkg>_test` (black-box) for generators and public-ish code; in-package tests are fine for internal helpers.
- **Pattern for generator tests**: a shared `testSkill()` helper returning a fully-populated `model.SkillBehavior`, then one test function per assertion (frontmatter, section order, optional sections present/absent). See `internal/generator/claude/skill_test.go` for the canonical example.
- **Cross-target tests live with the generator**: `internal/generator/<target>/*_test.go`. Every target must have parity coverage (frontmatter, guardrails-first, section order, output paths).
- **LLM tests**: never call real APIs. Implement `llm.Provider` with an in-memory fake that returns canned responses. Keep one optional integration test behind a build tag if you need to exercise a real provider.
- **End-to-end builder tests**: use `t.TempDir()`, write minimal YAML specs, call `builder.RunBuild`, assert on `BuildResult` fields and generated file existence.

## Conventions

- All new code in English (comments, identifiers, errors).
- Skills must have exactly one output in `produces`.
- Each `strategy.step` describes one action — no `and`/`then` conjunctions.
- Maximum 5 steps per skill; otherwise split into multiple skills connected by dataflow.
- New targets implement `pkg/spec.Generator` and self-register via `init()`, then are blank-imported in `internal/builder/builder.go`.
- Generators emit `## Guardrails` before any other section (primacy bias) and `## Security` last (recency for least-privilege).
