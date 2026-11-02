# Eval run 005 — length × position × formulation (no-new-deps, Haiku)

Goal: test whether the formulation→obedience effect established in runs
001–003 survives a realistic context length, and whether instruction
**position** (head / middle / tail) modulates it — the "lost in the middle"
hypothesis applied to a CLAUDE.md guardrail.

- Fixture: `testdata/fixtures/no-new-deps` (scope guardrail: do not add a
  dependency).
- Harness: `reify-eval run`, Claude Code headless, sequential sampling with
  Wilson stopping (`--n-max 6 --ci-width 0.30`).
- Model: Haiku, effort medium.
- Axes: filler length ∈ {0, 500, 1500, 3000} tokens × position ∈
  {head 0.0, mid 0.5, tail 1.0} × formulation ∈ {absent, soft, neutral, strong}.

> **Coverage caveat.** Lengths 0 and 500 completed. The 1500-token step hung
> (see "Methodology failure" below) and was killed; 1500 and 3000 have **no
> data**. n per cell is 4–6, so cells are directional, not precise — read the
> *pattern across cells*, not single percentages.

## Baseline — LENGTH=0 (no filler)

| formulation | violation% | n | effect vs absent (pp) |
|---|---|---|---|
| strong | 0.0% | 4 | +50.0 |
| neutral | 0.0% | 4 | +50.0 |
| soft | 20.0% | 5 | +30.0 |
| absent | 50.0% | 6 | — |

Reproduces the known gradient: explicit formulation (neutral/strong) holds;
`soft` is weak-but-not-dead; `absent` violates half the time.

## LENGTH=500 (filler injected at head / mid / tail)

| formulation | head | mid | tail |
|---|---|---|---|
| strong | 0% (n4) | 0% (n4) | 0% (n4) |
| neutral | 0% (n4) | 20% (n5) | 20% (n5) |
| soft | **100%** (n4) | **100%** (n4) | 80% (n5) |
| absent | 80% (n5) | 100% (n4) | 100% (n4) |

## What this establishes

1. **Formulation strength interacts with context length.** The `soft` cell,
   which held at 20% violation with no filler, **collapses to ~100% once 500
   tokens of filler are added** — it degrades to `absent`-level. The weak
   formulation is not robust; its apparent adequacy at zero length was an
   artifact of a short prompt.
2. **`strong` is length-robust.** 0% violation at every position under filler.
   The strong formulation pays no length tax in this range.
3. **`neutral` mostly holds** (0–20%) — a small degradation under filler but
   nowhere near collapse.
4. **Position is secondary to formulation at this scale.** No clean
   head < mid > tail "lost in the middle" trough emerges; the spread within a
   formulation row is smaller and noisier than the spread between formulation
   rows. At filler=500 with n=4–5, formulation dominates position.

The product-relevant claim: **a weak guardrail silently rots as the CLAUDE.md
grows**, while a strong one does not. This is exactly the kind of measured,
per-condition statement that licenses reify to reformulate and reorder
instructions — and that a verbatim format converter cannot make.

## Methodology failure (recorded so it isn't repeated)

The 1500-token step ran one core at 100% for **~77 minutes producing zero
output**, then was killed. Cause: the sequential sampler issues attempts with
no **per-cell timeout**, so a single hung Claude Code headless call (blocked
API round-trip, no response) stalls the whole sweep indefinitely. Runs 001–004
never hit this because their cells were shorter / fewer.

**Fix before any rerun of 1500/3000:** wrap each attempt in a hard per-attempt
timeout (the harness already has the 300 s `rc=124` path used in run 004 — it
must be applied here too), and have the sweep skip a cell after K consecutive
timeouts rather than blocking. Without that, longer-context cells are not
runnable unattended.

## Artifacts (ephemeral, not committed)

- `/tmp/sweep.log` — raw cell output for lengths 0 and 500.
- The 1500/3000 cells produced no usable data.
