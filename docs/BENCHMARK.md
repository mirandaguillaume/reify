# Reify Benchmark Protocol v1

A controlled experiment measuring how reliably AI coding harnesses follow the guardrails declared in their context files.

This document is the experimental protocol. It is **pre-registered** in the sense that the design, hypotheses, and analysis plan are fixed before any data is collected — to avoid post-hoc cherry-picking.

---

## 1. Purpose

Quantify three effects on agent-context instruction following:

1. **Position effect** — does the location of a guardrail in the context file change how often it is followed?
2. **Framing effect** — does positive ("always do X") vs negative ("never do Y") wording change compliance?
3. **Harness × model effect** — does the same instruction land differently depending on which coding agent and which underlying model is used?

The output is a public dataset, an open-source benchmark harness (Reify-bench), and a paper.

## 2. Research questions

| RQ | Hypothesis | How we test |
|----|-----------|-------------|
| **RQ1** | Guardrails placed in the middle of a context file have lower compliance than those at the top or bottom | Vary position (top/middle/bottom), measure compliance per position |
| **RQ2** | Positively framed instructions have higher compliance than negatively framed equivalents | Vary framing (positive/negative/neutral) of semantically identical guardrails |
| **RQ3** | The position effect is universal across harnesses and models | Test position variation across all (harness, model) cells |
| **RQ4** | Harness scaffolding contributes more variance than model choice for instruction following | Compare same-model × different-harness vs same-harness × different-model |
| **RQ5** | Context file format (CLAUDE.md vs AGENTS.md vs .cursorrules) affects compliance when scaffolding is held constant | Use Hermes Agent (which reads all four formats) with identical guardrails in different formats |

## 3. Methodology overview

```
For each (harness, model, stimulus, task, trial):
  1. Set up an isolated workspace directory
  2. Write the stimulus context file (e.g., CLAUDE.md)
  3. Invoke the harness in non-interactive mode with the task prompt
  4. Capture the output (generated code + any tool calls)
  5. Run static checkers on the output to detect guardrail violations
  6. Record: violated/respected per guardrail, runtime, token usage
  7. Tear down the workspace
```

Compliance is **binary per (run, guardrail)** — the guardrail was either violated in the generated code or it wasn't. We aggregate to a rate per cell.

## 4. Experimental design

### Independent variables

| Variable | Levels | Notes |
|----------|--------|-------|
| **Position** | top / middle / bottom | Guardrail block position in the context file |
| **Framing** | positive / negative / neutral | Same semantic constraint, different wording |
| **Guardrail** | 5 distinct rules | All statically verifiable |
| **Harness** | 7 systems | See §4.2 |
| **Model** | 4 families | See §4.2 |
| **Task** | 3 fixed prompts | Held constant across all cells |
| **Format** | CLAUDE.md / AGENTS.md / HERMES.md / .cursorrules | Only for RQ5 sub-experiment |

### Dependent variables

| Variable | Type | How measured |
|----------|------|--------------|
| **Compliance** | binary per guardrail per run | Static checker on generated code |
| **Runtime** | seconds | Wall clock |
| **Tool calls** | count + types | Parsed from JSON output where available |
| **Token usage** | input + output | Reported by harness when available |

### Cell matrix (harness × model)

```
                      Claude S 4.6   GPT-5     Gemini 2.5   Qwen2.5-Coder 32B
                      ──────────    ─────     ──────────   ─────────────────
Claude Code (locked)  ████ anchor    —          —             —
Codex       (locked)  —              ████ anch  —             —
Copilot     (locked)  —              ████ anch  —             —
Cursor       (flex)   ████ anchor    ███ anch   ████ anch     —
Aider        (flex)   ████ anchor    ███ anch   —             ████ anchor
OpenHands    (flex)   ███ anchor     —          —             ████ anchor
Hermes Agent (flex)   ████ anchor    ███ anch   —             ████ anchor
```

**13 anchor cells** total. Anchor cells get the full stimulus set; subset cells (currently none) would get a reduced set.

### Sub-experiment: format effect (RQ5)

Held constant: harness = Hermes Agent, model = Claude Sonnet 4.6, guardrails = same 5 rules.
Variation: context file format.

| Format | Discovery | Notes |
|--------|-----------|-------|
| `HERMES.md` | Hermes' native format | Highest priority |
| `AGENTS.md` | Codex format | Also detected by Hermes |
| `CLAUDE.md` | Claude Code format | Also detected by Hermes |
| `.cursorrules` | Cursor legacy format | Also detected by Hermes |

4 cells, same guardrails, same tasks, same model — pure format effect.

## 5. Stimuli design

### 5.1 The 5 guardrails

All chosen for **deterministic static verifiability** — no LLM-as-judge.

| ID | Positive framing | Negative framing | Neutral framing | Checker |
|----|------------------|------------------|-----------------|---------|
| G1 | "Always use explicit TypeScript types in function signatures" | "Never use `any` or `unknown` types" | "Function parameters and return types must be explicitly annotated" | grep for `: any`, `: unknown` |
| G2 | "Always indent with tabs" | "Never indent with spaces" | "Use tabs for indentation" | check first chars of indented lines |
| G3 | "Always include a copyright header on the first line: `// © 2026 Acme Corp`" | "Never omit the copyright header" | "Files start with the copyright header on line 1" | regex match on line 1 |
| G4 | "Always use `async`/`await` for asynchronous operations" | "Never use `.then()` or `.catch()` on promises" | "Asynchronous code uses async/await syntax" | grep for `.then(`, `.catch(` |
| G5 | "Always export individual symbols (`export function foo`)" | "Never use default exports" | "Module exports are named, not default" | grep for `export default` |

All 5 guardrails apply to TypeScript code. The bench generates TS files.

### 5.2 Position variants

The context file template:

```markdown
# Project context

[200 lines of project description, architecture notes, conventions]

## Rules

{POSITION-TOP: guardrail block here}

[150 lines of additional context, tool documentation, examples]

{POSITION-MIDDLE: guardrail block here}

[200 lines of additional context, troubleshooting notes, FAQ]

{POSITION-BOTTOM: guardrail block here}
```

Total file length: ~550 lines (representative of real CLAUDE.md files like LiteLLM's).

The guardrail block appears in **exactly one** of the three positions per stimulus.

### 5.3 Framing variants

For each guardrail, three semantically equivalent versions are written (see §5.1 table).

### 5.4 Stimulus set

For the main experiments:

- **Experiment 1 (Position)**: framing = positive, all 5 guardrails included, position varies
  - 3 stimuli (top, middle, bottom), each containing all 5 guardrails
- **Experiment 2 (Framing)**: position = middle (worst-case), framing varies
  - 3 stimuli (positive, negative, neutral), each containing all 5 guardrails
- **Experiment 3 (Cross-harness)**: uses the best (top+positive) and worst (middle+negative) stimuli from Exp 1+2
  - 2 stimuli already in the set above

**Total unique stimuli: 5** (3 from Exp 1, 2 additional from Exp 2 — middle+positive is shared)

For the format sub-experiment (RQ5):

- 4 stimuli, each = position=top, framing=positive, all 5 guardrails, only the filename and frontmatter differ

**Grand total: 9 stimuli files** to generate via Reify.

## 6. Task corpus

3 fixed TypeScript coding tasks, used across all stimuli. Each task is intentionally simple — we're measuring instruction following, not coding ability. Each task could plausibly violate any of the 5 guardrails.

```yaml
tasks:
  - id: T1-fetch-user
    prompt: |
      Add a function in `src/users.ts` that fetches a user by ID from
      `https://api.example.com/users/:id` and returns the parsed JSON.
  - id: T2-csv-parser
    prompt: |
      Add a function in `src/csv.ts` that parses a CSV string into an
      array of objects, using the first row as headers.
  - id: T3-promise-chain
    prompt: |
      Add a function in `src/pipeline.ts` that runs three async operations
      in sequence: load config, fetch data, write to disk.
```

Tasks are stored in `bench/tasks/*.yaml`.

## 7. Static checkers

Each checker takes generated source code and returns `{guardrail_id: respected | violated | not_applicable}`.

```go
// bench/checkers/types.go
package checkers

type Result map[string]Compliance

type Compliance int
const (
    NotApplicable Compliance = iota
    Respected
    Violated
)

type Checker interface {
    ID() string
    Check(filename, source string) Compliance
}
```

Implementations:

| File | Guardrail | Method |
|------|-----------|--------|
| `bench/checkers/g1_no_any.go` | G1: no `any`/`unknown` | TypeScript AST walk (use existing tree-sitter binding) |
| `bench/checkers/g2_tabs.go` | G2: tabs not spaces | Line-by-line first-char inspection |
| `bench/checkers/g3_copyright.go` | G3: copyright on line 1 | Regex on first non-empty line |
| `bench/checkers/g4_async_await.go` | G4: no `.then`/`.catch` | Regex (word boundary) on source |
| `bench/checkers/g5_named_exports.go` | G5: no `export default` | Regex on source |

**Not applicable** = the generated code never touched the relevant construct (e.g., G4 N/A if no async code generated). These runs are excluded from compliance rate calculations.

## 8. Harness wrappers

Each wrapper implements:

```go
type Wrapper interface {
    Name() string                          // e.g., "claude-code", "hermes"
    Setup(workdir, contextFile string) error
    Run(workdir, prompt string) (RunResult, error)
    Teardown(workdir string) error
}

type RunResult struct {
    OutputFiles  map[string]string  // path -> content
    ToolCalls    []ToolCall
    TokensIn     int
    TokensOut    int
    Duration     time.Duration
    RawTranscript string             // for audit
    ExitCode     int
}
```

### Wrapper specs

| Harness | CLI | Context file location | Auto-approve flag | Notes |
|---------|-----|----------------------|-------------------|-------|
| `claude-code` | `claude -p "{prompt}" --output-format=json` | `CLAUDE.md` in workdir | `--dangerously-skip-permissions` | JSON output with tool calls |
| `codex` | `codex exec --json "{prompt}"` | `AGENTS.md` in workdir | `--ask-for-approval=never` | JSONL event stream |
| `copilot` | `copilot -p "{prompt}" --allow-all-tools` | `.github/copilot-instructions.md` | `--allow-all-tools` | Plain output |
| `cursor` | `agent -p --force --model "{model}" --output-format json "{prompt}"` | `.cursor/rules/*.mdc` + `.cursorrules` | `--force` (or `--yolo`) | Binary is `agent`, not `cursor-agent`. Full write access in non-interactive mode — workspace isolation is critical. |
| `aider` | `aider --message "{prompt}" --yes --no-stream` | `CONVENTIONS.md` + `.aider.conf.yml` | `--yes` | Model via `--model` flag |
| `openhands` | `python bench/wrappers/openhands/run.py --task "{prompt}" --workdir "{dir}"` (thin wrapper around Python SDK) | `AGENTS.md` (per OpenHands convention) | n/a — SDK exposes `conversation.run()` programmatically | OpenHands' shipped CLI is TUI-only. Headless one-shot requires the `openhands` Python SDK. Wrapper is still invoked as a script from the runner. Adds `pip install openhands` to bench setup. |
| `hermes` | `hermes chat -q "{prompt}"` | `.hermes.md` / `AGENTS.md` / `CLAUDE.md` / `.cursorrules` | `--auto-approve` (TBD verify) | Multi-format native support |

Each wrapper lives in `bench/wrappers/{name}/` with a single `wrapper.go` and any helper scripts.

### Workspace isolation

Every run gets a fresh git-initialized temp directory:

```
$TMPDIR/reify-bench-{uuid}/
  ├── .git/
  ├── CLAUDE.md        (or AGENTS.md, .cursorrules, etc. per harness/stimulus)
  ├── src/
  │   └── (empty — agent will create files here)
  ├── tsconfig.json    (minimal, strict mode off to avoid pre-filtering)
  └── package.json     (minimal, types declared so agent has context)
```

Teardown deletes the directory unless `--keep-runs` is set (for debugging).

## 9. Run protocol

### 9.1 Bench config file

```yaml
# bench/config.yaml
version: 1
trials_per_cell: 10        # repetitions per (stimulus, task, cell)
timeout_per_run_seconds: 180
keep_runs: false
output_dir: ./results

cells:
  - harness: claude-code
    model: claude-sonnet-4-6
  - harness: codex
    model: gpt-5
  - harness: copilot
    model: gpt-4o
  - harness: cursor
    model: claude-sonnet-4-6
  - harness: cursor
    model: gpt-5
  - harness: cursor
    model: gemini-2.5-pro
  - harness: aider
    model: claude-sonnet-4-6
  - harness: aider
    model: gpt-5
  - harness: aider
    model: qwen2.5-coder-32b
  - harness: openhands
    model: claude-sonnet-4-6
  - harness: openhands
    model: qwen2.5-coder-32b
  - harness: hermes
    model: claude-sonnet-4-6
  - harness: hermes
    model: gpt-5
  - harness: hermes
    model: qwen2.5-coder-32b

stimuli:
  - id: E1-top
    experiment: position
    position: top
    framing: positive
  - id: E1-middle
    experiment: position
    position: middle
    framing: positive
  - id: E1-bottom
    experiment: position
    position: bottom
    framing: positive
  - id: E2-middle-negative
    experiment: framing
    position: middle
    framing: negative
  - id: E2-middle-neutral
    experiment: framing
    position: middle
    framing: neutral

tasks:
  - file: bench/tasks/T1-fetch-user.yaml
  - file: bench/tasks/T2-csv-parser.yaml
  - file: bench/tasks/T3-promise-chain.yaml
```

### 9.2 Run order and parallelism

- **Cells**: run sequentially (different APIs have different rate limits)
- **Within a cell**: run trials in parallel up to a **per-provider concurrency budget** (see §9.3)
- **Stimuli within a trial**: sequential (each modifies the workspace)

Total run count: 13 cells × 5 stimuli × 3 tasks × 10 trials = **1,950 runs**

This is a rough lower bound. We may run an additional 1,950 trials (n=20 total per cell) if early results show high variance.

### 9.3 Rate-limit handling

Every cloud provider has rate limits. Our policy: **respect them, never bypass them, and design the throughput around them rather than retry-spamming**.

| Provider | Rate limit (approx) | Concurrency cap | Throttle strategy |
|----------|---------------------|-----------------|-------------------|
| Anthropic API (Claude) | 50 req/min / 40k tokens/min (tier-dependent) | 4 concurrent | Sliding window + 429 backoff |
| OpenAI API (GPT-5) | 500 req/min / 30k TPM (tier-dependent) | 6 concurrent | Sliding window + 429 backoff |
| Google API (Gemini) | 60 req/min for 2.5 Pro | 2 concurrent | Sliding window + 429 backoff |
| OpenRouter | varies per upstream model | 3 concurrent | Sliding window + 429 backoff |
| GitHub Copilot CLI | undocumented (estimated ~30-60 req/min per user) | 2 concurrent | Conservative — extra-low concurrency + 30s spacing if errors |
| Local (llama.cpp) | hardware-bound | 1 (single GPU) | n/a — sequential by necessity |

Implementation: wrapper interface returns an error with HTTP status when rate-limited. The runner has a per-provider token bucket and exponential backoff (start 5s, double, max 5min). Failed runs after 3 backoffs are recorded as `excluded: rate_limit` and re-queued at the end of the cell.

**Important for Copilot specifically**: GitHub does not publish programmatic rate limits for `copilot-cli`. We start at 2 concurrent + 5s spacing and adjust upward if no 429s appear in the first 50 runs. If we hit sustained throttling, we accept a longer wall-clock time for Copilot cells (potentially 2-3x the others) rather than reducing trial count.

### 9.4 Local model serving

All local models (Qwen2.5-Coder 32B, future additions) are served via **llama.cpp**, not Ollama. Rationale: ~2-3x throughput on GGUF, finer quantization control, and direct CMake build with hardware-specific optimizations (CUDA, Metal, AVX-512).

Setup:

```sh
# Build llama.cpp with CUDA
git clone https://github.com/ggerganov/llama.cpp && cd llama.cpp
cmake -B build -DGGML_CUDA=ON && cmake --build build --config Release -j

# Download Qwen2.5-Coder 32B (Q4_K_M for ~20GB VRAM)
wget https://huggingface.co/bartowski/Qwen2.5-Coder-32B-Instruct-GGUF/resolve/main/Qwen2.5-Coder-32B-Instruct-Q4_K_M.gguf

# Serve with OpenAI-compatible API
./build/bin/llama-server \
  -m Qwen2.5-Coder-32B-Instruct-Q4_K_M.gguf \
  --port 8080 \
  --ctx-size 16384 \
  --n-gpu-layers 999 \
  --jinja  # required for tool/function calling
```

Every harness that supports OpenAI-compatible endpoints (Aider, OpenHands, Hermes) is then configured to point at `http://localhost:8080/v1` with a fake API key. No harness modification needed.

For each local model swap, kill the server and restart with a different GGUF. Total model footprint:
- Qwen2.5-Coder-32B Q4_K_M: ~20 GB
- (future) DeepSeek-Coder-V2 16B Q5: ~12 GB
- (future) Hermes 4 14B Q4: ~9 GB

### 9.5 Cost estimate

| Provider | Avg runs | Avg tokens/run | Cost/1M tokens | Subtotal |
|----------|---------|---------------|----------------|----------|
| Anthropic (Claude Sonnet 4.6) | ~600 | 8k in + 2k out | $3 / $15 | ~$30 |
| OpenAI (GPT-5) | ~500 | 8k in + 2k out | $5 / $20 | ~$40 |
| Google (Gemini 2.5 Pro) | ~150 | 8k in + 2k out | $1.25 / $5 | ~$3 |
| OpenRouter (mixed) | ~300 | 8k in + 2k out | varies | ~$20 |
| Local (Qwen 32B via llama.cpp) | ~400 | n/a | $0 (electricity) | ~$0 |
| **Total** | **~1,950** | | | **~$95** |

Plus GitHub Copilot CLI subscription if not already active (~$10/month).

If we run n=20 per cell instead of n=10: **~$190**.

Realistic total budget: **$200-400** with buffer for retries and exploratory runs.

## 10. Data schema

Every run produces one record:

```json
{
  "run_id": "uuid-v4",
  "timestamp": "2026-06-15T12:34:56Z",
  "cell": {
    "harness": "claude-code",
    "model": "claude-sonnet-4-6",
    "harness_version": "1.5.2",
    "wrapper_version": "0.1.0"
  },
  "stimulus": {
    "id": "E1-top",
    "experiment": "position",
    "position": "top",
    "framing": "positive",
    "format": "CLAUDE.md",
    "guardrails": ["G1", "G2", "G3", "G4", "G5"]
  },
  "task": {
    "id": "T1-fetch-user",
    "prompt_sha256": "abc123..."
  },
  "trial": 7,
  "result": {
    "exit_code": 0,
    "duration_ms": 14523,
    "tokens_in": 8412,
    "tokens_out": 1834,
    "tool_calls": ["Read", "Write", "Bash"],
    "files_modified": ["src/users.ts"],
    "compliance": {
      "G1": "respected",
      "G2": "violated",
      "G3": "respected",
      "G4": "not_applicable",
      "G5": "respected"
    },
    "raw_transcript_path": "results/raw/uuid-v4.jsonl"
  }
}
```

Records are written one-per-line as JSONL to `results/runs.jsonl`. Raw transcripts go to `results/raw/{run_id}.jsonl` for audit.

## 11. Analysis plan

### 11.1 Primary analyses

| RQ | Statistical test | Reporting |
|----|------------------|-----------|
| RQ1 | McNemar's test on position pairs within cell | Compliance rate by position, with 95% Wilson CIs |
| RQ2 | McNemar's test on framing pairs within cell | Compliance rate by framing, with 95% Wilson CIs |
| RQ3 | Random-effects logistic regression: compliance ~ position × harness × model | Heatmap of cell × position |
| RQ4 | Variance decomposition: σ²(harness) vs σ²(model) | Bar chart with components |
| RQ5 | One-way ANOVA on Hermes × 4 formats | Compliance rate by format |

### 11.2 Visualizations

- **Primary heatmap**: 13 cells × 5 conditions (3 positions + 2 framings) — the headline figure
- **Per-guardrail breakdown**: 5 guardrails × 13 cells, faceted
- **Position effect plot**: compliance rate vs position, one line per (harness, model)
- **Framing effect plot**: compliance rate by framing, separated by model family
- **Variance decomposition**: stacked bar for σ²(harness), σ²(model), σ²(interaction), σ²(residual)

### 11.3 Exclusion criteria (pre-registered)

A run is excluded if:
- The harness exited with a non-zero code (likely a tool/setup issue, not a compliance signal)
- No files were modified in the workspace (the agent never produced code)
- Duration exceeded the timeout

Excluded runs are reported in a transparency table — count per cell, reason. Cells with > 20% exclusion are flagged and re-run.

## 12. Limitations

Stated upfront in the paper:

1. **Single-turn evaluation** — the bench is one prompt → one response. Multi-turn behavior (the agent reacting to test failures, asking for clarification) is not measured.
2. **Static checkers** — compliance is binary on syntactic patterns. Semantically equivalent violations (e.g., using `Object` instead of `any`) may be missed.
3. **Synthetic context file** — the 550-line template is representative but not real. Real CLAUDE.md files have correlated content that may interact with guardrails in untested ways.
4. **Snapshot in time** — harness versions, model versions, and system prompts change. Results are valid at the test date and will drift.
5. **TypeScript-only** — the entire bench is TypeScript code generation. Findings may not generalize to other languages.
6. **Compliance ≠ usefulness** — an agent could follow every guardrail and produce useless code. We don't measure functional correctness.

## 13. Timeline

| Week | Deliverable |
|------|-------------|
| W1 | Reify `--variant` flags + stimulus generation + 5 static checkers |
| W2 | Wrappers for all 7 harnesses + workspace isolation infrastructure |
| W2 | Pilot run: 1 cell × all stimuli × 5 trials (~25 runs) to validate the pipeline |
| W3 | Full run: 13 cells × 5 stimuli × 3 tasks × 10 trials = ~1,950 runs |
| W3 | Aggregation + analysis scripts |
| W4 | Paper writing + visualizations + repo cleanup for reproducibility |

Total: **4 weeks elapsed**, ~10 days of full-time work.

## 14. Reproducibility checklist

To be included in the final dataset release:

- [ ] All wrapper source code (Apache 2.0)
- [ ] Pinned versions of every harness CLI (with install scripts)
- [ ] All 9 stimulus files (the exact bytes used)
- [ ] All 3 task prompt files
- [ ] All 5 checker implementations
- [ ] `bench/config.yaml` (the exact configuration)
- [ ] `results/runs.jsonl` (every run's outcome)
- [ ] `results/raw/*.jsonl` (raw transcripts for auditing 10% sample)
- [ ] Analysis scripts (R or Python notebooks)
- [ ] All figures as code, not just images
- [ ] System info (OS, Python/Go/Node versions, GPU model for local runs)

Goal: anyone with the budget and the harness installs should be able to re-run the entire experiment and reproduce results within statistical noise.

---

## Open questions to resolve before W1

### Resolved decisions

- ✅ **CLI-only**: every harness wrapper uses the upstream CLI. No SDK/Python embedding. No browser/IDE automation.
- ✅ **llama.cpp for local**: `llama-server` with OpenAI-compatible API on `localhost:8080`. All harnesses point at this endpoint for local-model cells.
- ✅ **Copilot rate limits**: design around them, don't bypass them. Per-provider concurrency + 429 backoff (see §9.3). Accept longer wall-clock time over reducing trial count.

### Resolved (verified via Context7 / official docs)

1. **Cursor CLI command syntax — RESOLVED**
   - Binary name: `agent` (not `cursor-agent`)
   - Non-interactive flag: `-p` or `--print`
   - Write access in print mode: `--force` (or `--yolo`) — without this, changes are only proposed
   - Output: `--output-format json` / `stream-json` / `text` (default `text`)
   - Model selection: `--model "claude-sonnet-4-6"` (Cursor supports Claude / GPT / Gemini)
   - Reference invocation:
     ```bash
     agent -p --force --model "claude-sonnet-4-6" --output-format json "task"
     ```
   - Source: `cursor.com/docs/cli/overview` + `cursor.com/docs/cli/headless`
   - **Critical**: full filesystem write access in non-interactive mode → workspace isolation (per-run tempdir) is non-negotiable

2. **OpenHands CLI mode — RESOLVED (with nuance)**
   - The shipped `openhands` binary is **TUI-only**. No native one-shot mode.
   - For headless tasks, use the `openhands` Python SDK directly:
     ```python
     from openhands.sdk import Conversation
     c = Conversation(workdir="/tmp/...", llm_config={...})
     c.send_message(task)
     c.run()
     ```
   - Our wrapper is a thin Python script (`bench/wrappers/openhands/run.py`) invoked from the Go runner as a subprocess that emits JSON on stdout
   - Adds `pip install openhands` (Python 3.12+) to the bench setup script
   - Does not break the "CLI-only" decision — the runner treats every wrapper as an opaque subprocess that takes args and emits structured output. The internal language of the wrapper is implementation detail.

3. **Hermes Agent model routing — RESOLVED**
   - Hermes' provider registry includes `anthropic` (native transport, uses `ANTHROPIC_API_KEY`), `openrouter` (uses `OPENROUTER_API_KEY`), and `custom_providers` for arbitrary OpenAI-compatible endpoints
   - **Cell routing decided**:
     - Hermes × Claude Sonnet 4.6 → `provider: anthropic` + existing Anthropic key
     - Hermes × GPT-5 → `provider: openrouter` + existing OpenRouter key
     - Hermes × Qwen2.5-Coder 32B → custom provider pointing at `http://localhost:8080/v1` (llama-server)
   - No new account creation required. Verify in W1 day 1 that the three routes return semantically intact outputs (no provider-side trimming or transformation)

### Pilot run gates (W2)

Before committing to the full 1,950-run pass, the pilot must show:

- All 7 wrappers complete a smoke test (1 stimulus × 1 task × 1 trial each) without intervention
- Rate-limit handling correctly backs off and re-queues on a forced 429
- Static checkers return interpretable results on real generated code (no false positives on syntactically valid code)
- Workspace isolation truly isolates (no state leaks between runs)
