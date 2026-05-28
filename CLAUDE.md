# Reify — context for Claude Code

Reify compiles agent context files across coding harnesses (Claude Code, GitHub Copilot, Cursor) from a single Reify YAML source of truth. It also analyses existing context files: structural completeness (`doctor`), facet classification (`classify`), and compliance risk per harness (`check`).

## Tech stack

- Go 1.22+
- CLI: Cobra
- YAML: gopkg.in/yaml.v3
- Testing: testify
- LLM providers: Ollama (local), Anthropic, OpenRouter

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

## Dev commands

```sh
go test ./...                # run all tests
go build ./cmd/reify         # compile binary
go vet ./...                 # static analysis
```

## Architecture

```
cmd/reify/                          # CLI entry point
pkg/
  model/                            # SkillBehavior, AgentComposition, validation
  spec/                             # TargetGenerator interface + registry
  dag/                              # DAG engine (auto-wiring, layers, router, retry)
  analysis/                         # formal property analysis
  schema/, sandbox/, qualitygate/   # supporting infrastructure
internal/
  cmd/                              # Cobra command handlers
  classifier/                       # 5-facet classifier (static + LLM)
  checker/                          # compliance risk estimator
  doctor/                           # forgent doctor — LLM-powered structural analysis
  importer/                         # agent file → skill spec decomposition
  builder/                          # build orchestration
  generator/
    claude/                         # Claude Code generator
    copilot/                        # GitHub Copilot generator
    cursor/                         # Cursor generator
    reify/                          # standalone Go runtime generator
  analyzer/                         # dependency check, loop detector, scoring
  linter/                           # lint rules
  llm/                              # LLM provider interface (Ollama, Anthropic, OpenRouter)
  scanner/, enricher/               # codebase scanning + skill enrichment
  yaml/                             # YAML loader
templates/                          # skill/agent YAML templates
skills/, agents/                    # dogfooded doctor skills + agents
```

## Conventions

- All new code in English (comments, identifiers, errors).
- Skills must have exactly one output in `produces`.
- Each `strategy.step` describes one action — no `and`/`then` conjunctions.
- Maximum 5 steps per skill; otherwise split into multiple skills connected by dataflow.
- Cross-target tests live with the generator (`internal/generator/<target>/*_test.go`).
- New targets implement `pkg/spec.Generator` and self-register via `init()` in `internal/builder/builder.go`.
