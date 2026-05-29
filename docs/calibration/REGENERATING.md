# Regenerating the calibration corpus

> The calibration pipeline consumes **ephemeral** artefacts: `classify.log`
> files produced by `reify classify`, and the `*.jsonl` corpora derived from
> them. None of these are committed (they are large, LLM-derived, and
> reproducible). This file is the recipe to rebuild them from scratch when
> `/tmp` is purged or you move machines.
>
> The reproducible inputs are: this file, `rubric.md`, `findings.md`, the
> `reify-calibrate` binary, and the public list of source repos below.

## Why nothing here is committed

`reify classify` runs an LLM over each agent file, so its output is neither
free to regenerate (API cost) nor byte-stable across model versions. We treat
the *recipe* as the source of truth, not the artefacts. The sampler is seeded
(`--seed 42`, stable item IDs) so the **selection** of items is deterministic
even though the **labels** are not.

## Prerequisites

```sh
# Build both binaries
go build ./cmd/reify
go build ./cmd/reify-calibrate

# An LLM provider for `reify classify` and the calibrate LLM subcommands
export ANTHROPIC_API_KEY=...        # or OPENROUTER_API_KEY / a local Ollama
```

## Step 1 — collect the source repos

The corpus is stratified-sampled from `reify classify` runs over the agent
files of seven OSS repos. Clone them under a single tree:

```sh
mkdir -p /tmp/cal-src && cd /tmp/cal-src
for r in \
  anthropics/claude-code \
  vercel/next.js \
  supabase/supabase \
  cline/cline \
  continuedev/continue \
  astral-sh/ruff \
  microsoft/vscode ; do
  git clone --depth 1 "https://github.com/$r" "$(basename "$r")"
done
```

> The exact list and any commit pinning belong in `findings.md` §0. If a repo
> has restructured its agent files since the original run, the per-facet pool
> sizes will drift — note it in findings rather than silently re-baselining.

## Step 2 — produce `classify.log` per repo

`reify classify --json` writes a per-file JSON array to **stdout**. The
sampler walks a tree looking for files named exactly `classify.log`, and
`inferRepo` reads the repo name from a `logs/<repo>/` path segment. So lay the
logs out as `logs/<repo>/classify.log`:

```sh
cd /tmp/cal-src
mkdir -p logs
for d in */ ; do
  repo="${d%/}"
  [ "$repo" = "logs" ] && continue
  mkdir -p "logs/$repo"
  reify classify "$repo" --json > "logs/$repo/classify.log"
done
```

`classify.log` is a JSON array of `{file, format, facets: {<facet>: [{text,
section}]}}` — exactly what `reify-calibrate sample` parses.

## Step 3 — stratified sample (deterministic)

```sh
reify-calibrate sample \
  --source /tmp/cal-src \
  --n 40 \
  --seed 42 \
  --output /tmp/calibration-corpus.jsonl
```

40 items per facet × 5 facets = 200 items. The `id` field is stable for a
given seed, so a partially gold-labelled corpus can be re-sampled and merged
by `id` without losing existing labels.

## Step 4 — the three labelling passes

Each pass writes a new label column onto a copy of the corpus. Keep them as
separate files so cross-annotator comparison (`explore cluster`, `score`)
stays possible.

```sh
# Closed-vocabulary judge (Opus under the 5-facet rubric) -> judge_labels
reify-calibrate judge \
  -i /tmp/calibration-corpus.jsonl \
  -o /tmp/calibration-corpus-judged.jsonl \
  --provider anthropic --model claude-opus-4-8

# Open-coding (free-form intent tags, NO taxonomy hint) -> emergent_labels
# Run once per model you want to compare. NOTE: the prompt deliberately
# contains NO positive intent-axis examples — see findings.md Run 2 and the
# contamination regression test in prompt_test.go. Do not "helpfully" add some.
reify-calibrate explore tag \
  -i /tmp/calibration-corpus.jsonl \
  -o /tmp/calibration-corpus-explored.jsonl \
  --provider anthropic --model claude-sonnet-4-6

reify-calibrate explore tag \
  -i /tmp/calibration-corpus.jsonl \
  -o /tmp/cal-haiku.jsonl \
  --provider anthropic --model claude-haiku-4-5

reify-calibrate explore tag \
  -i /tmp/calibration-corpus.jsonl \
  -o /tmp/cal-opus.jsonl \
  --provider anthropic --model claude-opus-4-8
```

## Step 5 — Round 2: cluster the emergent vocabulary

Feed the union of every annotator's `emergent_labels` to a referee LLM and let
it pick the cluster count (3–10):

```sh
reify-calibrate explore cluster \
  -i /tmp/calibration-corpus-explored.jsonl \
  -i /tmp/cal-haiku.jsonl \
  -i /tmp/cal-opus.jsonl \
  -o /tmp/clusters.json \
  --provider anthropic --model claude-opus-4-8
```

The summary reports, per cluster, how much of each source's vocabulary it
absorbs — the "does order emerge from chaos?" diagnostic of `findings.md` §1.6.

## Step 6 — score against gold

Gold labels are the only stable anchor. Once a human has filled `gold_labels`
on a slice of the corpus:

```sh
reify-calibrate score -i /tmp/calibration-corpus-judged.jsonl
```

Reports precision/recall/F1, Cohen's κ, confusion, and taxonomy diagnostics
(label cardinality, singleton rate, facet co-occurrence/PMI) for every
classifier path that has labels present.

## Output files (all ephemeral, all under `/tmp`)

| File | Produced by | Adds |
|---|---|---|
| `calibration-corpus.jsonl` | `sample` | `llm_labels` (Haiku via `reify classify`) |
| `calibration-corpus-judged.jsonl` | `judge` | `judge_labels` (Opus, closed vocab) |
| `calibration-corpus-explored.jsonl` | `explore tag` | `emergent_labels` (Sonnet, open vocab) |
| `cal-haiku.jsonl` | `explore tag` | `emergent_labels` (Haiku) |
| `cal-opus.jsonl` | `explore tag` | `emergent_labels` (Opus) |
| `clusters.json` | `explore cluster` | referee cluster assignment |

If you need any of these to survive a reboot, copy them out of `/tmp` — the
pipeline intentionally does not persist them.
