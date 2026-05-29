# Calibration findings — `reify-calibrate explore` (in progress)

> Live log of the open-coding experiments run while building the
> classifier calibration battery. Updates as experiments land. The point
> of `explore` is to test whether the 5-facet Reify taxonomy is the
> right axis for classifying agent-context instructions — by letting
> language models invent their own labels and inspecting what emerges.

## 0. Setup

Corpus: 200 items, stratified-sampled (40 per Reify facet) from a
dogfooding run of `reify classify` over 7 OSS repos (claude-code,
next.js, supabase, cline, continue, ruff, vscode). See `rubric.md` for
the labelling rubric.

Each item has `text`, `section`, `source_file`, `source_repo`, and an
initial `llm_labels` from Haiku 4.5 (the production classifier).
Subsequent runs add other fields:

- `judge_labels` — Opus 4.8 labelling under the rubric (closed
  vocabulary = 5 facets), produced by `reify-calibrate judge`.
- `emergent_labels` — free-form labels produced by `reify-calibrate
  explore tag` (open vocabulary, no taxonomy hint).

Question: when you ask several models to label the same items without
constraining the vocabulary, does an intent taxonomy emerge?

## 1. Experiments

### 1.1 Run 1 — open coding, Sonnet 4.6 (no axis)

Prompt asked for "1-3 snake_case tags describing what concern this
item targets". No mention of intent vs topic.

Result: 491 tags emitted, **477 unique** (97% singletons). Top tag had
only 2 occurrences. Vocabulary entirely **topical**:
`sql_injection_prevention`, `null_safety`, `race_condition_detection`,
`logging_format`, `module_resolution`.

By keyword heuristic, only **36.5% of vocabulary maps** to any of the 5
Reify facets. The other 63.5% were domain-specific topics.

Lesson: with no axis specified, the model spontaneously classifies by
**topic** (what the instruction is about), not by **intent** (what the
author is doing).

### 1.2 Run 2 — intent-focused, Sonnet 4.6 (with biased examples)

Prompt added wrong-axis examples (topics to avoid) AND right-axis
examples ("illustrative axes" like `soft_recommendation`,
`hard_requirement`, **`domain_fact`**, `fallback_directive`, with
"do not reuse verbatim").

Result: 480 tags, top tag `domain_fact` with **9 occurrences**.
Multiple `*_prohibition` variants. Coverage of 5 facets jumped to
58.8%.

**Apparent success — but it was prompt contamination.** The "winning"
tag `domain_fact` was literally one of my illustrative examples, and
the LLM echoed it back despite the disclaimer.

Lesson: NEVER provide positive examples on the axis you want to
discover. Wrong-axis anti-examples are fine (they anchor what you
*don't* want). Right-axis examples seed the answer.

### 1.3 Run 3 — intent-focused, Sonnet 4.6 (pure)

Prompt rewritten: same wrong-axis anti-examples, plus a
same-topic-different-intent contrast ("Use parameterized queries" vs
"The project uses PostgreSQL 16"). NO positive intent examples.

Result: 406 tags, **350 unique** (-27% vs run 2). Top tag is
`risk_flagging` (5 occurrences). `domain_fact` count drops to **0** —
direct evidence the prior run was contaminated. Coverage of 5 facets:
~63.5% (slightly higher than run 2, with broader keyword patterns).

Top-15 tags are entirely intent-class:
`risk_flagging`, `context_provision`, `contextual_grounding`,
`domain_scoping`, `procedural_directive`, `boundary_definition`,
`diagnostic_guidance`, `pattern_flagging`, `procedural_guidance`,
`scope_exclusion`, `scope_restriction`, `background_framing`,
`concrete_example`, `context_grounding`, `context_setting`.

Lesson: the LLM **can** stay on the intent axis once the contrast is
made explicit by a same-topic example. But the emergent vocabulary is
diffuse — 89% singletons — meaning each item gets its own narrow
intent label. No single dominant "domain_fact"-style concept emerges.

### 1.4 Run 4 — intent-focused, Haiku 4.5 (pure, same prompt as run 3)

Same prompt as run 3. Different model.

Result: 315 tags, **302 unique**, 1.57 tags/item average. Top tags:
`constraint_acknowledgment` (3), `constraint_assertion` (3),
`capability_constraint` (2), `capability_declaration` (2),
`prescribe_naming_convention` (2), `prescriptive_style` (2),
`risk_escalation` (2), `threat_awareness` (2).

**Cross-model comparison Sonnet ↔ Haiku:**

| Metric | Sonnet | Haiku |
|---|---:|---:|
| Total tags | 406 | 315 |
| Unique tags | 350 | 302 |
| Mean tags/item | 2.03 | 1.57 |
| Vocabulary Jaccard | — | **0.014** |
| Per-item mean Jaccard | — | **0.008** |
| Items with 0 shared tag | — | **98%** |

**Style divergence is striking.** Both stay on the intent axis but
choose almost disjoint vocabularies:

- Sonnet prefers *nominal* intent labels: `quality_*`, `scope_*`,
  `domain_*`, `procedural_*`, `contextual_*`, `pattern_*`,
  `structural_*`, `diagnostic_*`, `context_*`, `output_*`
- Haiku prefers *verbal* intent labels: `prescribe_*`, `enforce_*`,
  `constraint_*`, `establish_*`, `capability_*`, `error_*`,
  `prescriptive_*`, `restrict_*`, `prohibit_*`

Lesson: **the same prompt produces non-overlapping vocabularies on
different models.** Two strong models on the same data with the same
intent-axis instruction share 1.4% of vocabulary. The natural-language
emergence we observe is heavily model-dependent — the underlying
intent structure might be the same, but its surface expression isn't.

### 1.5 Run 5 — intent-focused, Opus 4.8 (pure, pending)

In flight at the time of writing. Background task `bxkwoh9l6`,
`/tmp/cal-opus.jsonl`.

Expected: if Opus also produces a disjoint vocabulary, the three-way
overlap will be near-zero. If Opus's vocabulary partially overlaps
both Sonnet's and Haiku's, it suggests Opus is a synthesis point.

Open question to resolve once Opus is in: is the cluster structure
(round 2 below) consistent across models, even though surface
vocabulary isn't?

### 1.6 Run 6 — cluster the union (planned)

Subcommand `reify-calibrate explore cluster --input <each-run.jsonl>`
sends the **union** of all annotators' vocabularies to a referee LLM
(Opus 4.8 by default) and asks it to group the labels into thematic
intent clusters. The referee picks the cluster count (3-10), not us.

Goal: test the "does order emerge from chaos?" hypothesis. If the
Opus-Sonnet-Haiku union of ~700-900 disjoint tags resolves into a
small number of coherent thematic clusters that absorb vocabulary
from all three sources, the underlying intent structure is real
even when the surface vocabulary isn't shared.

Failure modes to watch for:
- The referee produces one cluster per source → annotators were
  speaking truly different ontologies.
- The referee picks 20+ tiny clusters → no natural reduction.
- A small handful of clusters absorb only one source each →
  hidden source biases dominate.

## 2. Cross-cutting findings

### 2.1 Topical ⊥ Intent

Without an explicit axis instruction, LLMs default to topical
classification (what the instruction is *about*), not intent
classification (what the *author is doing*). The Reify 5-facet
taxonomy lives on the intent axis. So:

- The 5 facets are NOT what models converge to spontaneously when
  asked for free-form labels.
- They ARE on the right axis when models are forced to consider intent
  vs topic, but the emergent vocabulary is still diffuse and
  model-dependent.

### 2.2 Coherence ≠ Validation

Two strong models sharing 1.4% of vocabulary without taxonomy
imposition vs ~48% set-Jaccard with the 5-facet taxonomy imposed
(measured separately via `judge_labels` vs `llm_labels` agreement)
suggests:

- The 5 facets produce ~46pp of coherence between models — they work
  as a Schelling point.
- But coherence is not validation. Two models can agree on the wrong
  taxonomy. Gold human labels remain the only stable anchor for
  measuring correctness.

(The user explicitly noted: this is fine — the point of `explore` is
not to maximise coherence but to see if order emerges. We're still
in the "exploration of the topology" phase, not the "calibration of
production" phase.)

### 2.3 Prompt contamination is real and easy to miss

Run 2's "domain_fact" example became the top tag with 9 occurrences.
A reader of the result without access to the prompt would conclude
that `domain_fact` is a robust emergent concept. It isn't — it's an
echo.

Methodological hygiene rule for open coding: positive examples on the
target axis are forbidden. Use only anti-examples to anchor the axis
distinction.

## 3. Next experiments

- **Round 2 (cluster) on Sonnet + Haiku + Opus union** — the
  emergence test.
- **Self-consistency** — same model, same prompt, multiple seeds.
  Tells us whether per-item variance is dominated by model identity
  or temperature noise.
- **Few-shot in `reify classify`** — once round 2 reveals stable
  clusters, use those examples (or the rubric's own examples) as
  few-shot anchors in the production classifier prompt. Hypothesis:
  cross-model agreement under the imposed 5-facet taxonomy climbs
  from ~48% to 70%+.
- **Gold labelling** — the irreducible anchor. ~50-100 items labelled
  by hand under rubric v1.1 multi-label, scored against every
  classifier path with `reify-calibrate score`.

## 4. Files

- `/tmp/calibration-corpus.jsonl` — original 200-item sample (only
  `llm_labels` from Haiku via `reify classify`)
- `/tmp/calibration-corpus-judged.jsonl` — adds `judge_labels` from
  Opus 4.8 under the rubric (closed vocabulary = 5 facets)
- `/tmp/calibration-corpus-explored.jsonl` — adds `emergent_labels`
  from Sonnet 4.6 (open vocabulary, pure intent prompt)
- `/tmp/cal-haiku.jsonl` — `emergent_labels` from Haiku 4.5 (same
  prompt as `explored`)
- `/tmp/cal-opus.jsonl` — `emergent_labels` from Opus 4.8 (in flight)
- `/tmp/calibration-judge-multilabel.log`, `/tmp/cal-haiku-run.log`,
  `/tmp/cal-opus-run.log` — execution logs

All of `/tmp/cal*.jsonl` are ephemeral artefacts. The reproducible
inputs are this `findings.md`, `rubric.md`, the `reify-calibrate`
binary, and the dogfooded `classify.log` files used by `sample`.
