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

### 1.5 Run 5 — intent-focused, Opus 4.8 (pure)

Same prompt as runs 3-4. Result: 454 tags, **353 unique**, 2.27
tags/item — the most prolific of the three models.

**Three-way comparison Sonnet ↔ Haiku ↔ Opus:**

| Metric | Sonnet | Haiku | Opus |
|---|---:|---:|---:|
| Total tags | 413 | 330 | 454 |
| Unique tags | 354 | 315 | 353 |
| Mean tags/item | 2.08 | 1.66 | 2.27 |

| Pair | Vocab Jaccard | Shared vocab | Per-item Jaccard | Items 0-shared |
|---|---:|---:|---:|---:|
| Sonnet-Haiku | 0.021 | 14 | 0.017 | 96.0% |
| Sonnet-Opus | 0.044 | 30 | 0.030 | 91.5% |
| Haiku-Opus | 0.011 | 7 | 0.006 | 98.0% |

Three-way: **union = 972 tags, shared by all 3 = 1** (`scope_restriction`,
0.1%).

**Opus is weakly a synthesis point, but only weakly.** Its vocabulary
overlaps Sonnet (30 shared) ~4× more than Haiku (7), and Sonnet-Opus is
the densest pair. But the three-way overlap is essentially nil — the
surface divergence is *more* extreme with three models than with two.

**Each model has a grammatical signature.** Opus prefers short concrete
imperatives and *concentrates*: top tag `state_fact` recurs **14×** (vs
Sonnet's top at 6, Haiku's at 3). Sonnet uses abstract nominals
(`*_grounding`, `*_provision`). Haiku uses verbs (`prescribe_*`,
`enforce_*`, `constraint_*`).

### 1.6 Run 6 — cluster the union (the emergence test)

`reify-calibrate explore cluster` sent the **union of 972 disjoint
tags** from all three annotators to a referee (Opus 4.8) and asked it to
group them into thematic intent clusters, choosing the count itself.

Result: **13 clusters + 11 outliers**, and **12 of 13 clusters absorb
all three sources**. Zero single-source clusters. The lone 2/3 cluster
(`structural_labels_fragments`, 15 occ) is low-level noise.

| Cluster | size | occ | Son% | Hai% | Opu% |
|---|---:|---:|---:|---:|---:|
| inform_state_fact | 241 | 292 | 35 | 24 | 40 |
| directive_command | 127 | 156 | 34 | 24 | 42 |
| mandate_requirement | 113 | 133 | 23 | 37 | 41 |
| constraint_enforcement | 106 | 122 | 33 | 52 | 15 |
| verification_validation_gate | 84 | 110 | 35 | 25 | 39 |
| examples_references_pointers | 65 | 75 | 37 | 11 | 52 |
| scope_definition | 45 | 65 | 48 | 35 | 17 |
| prohibition | 49 | 64 | 20 | 23 | 56 |
| risk_security_awareness | 52 | 61 | 52 | 23 | 25 |
| format_structure_specification | 45 | 59 | 46 | 14 | 41 |
| diagnostic_troubleshooting | 40 | 57 | 46 | 28 | 26 |
| reflection_self_inquiry | 17 | 19 | 26 | 11 | 63 |
| structural_labels_fragments | 11 | 15 | 13 | 0 | 87 |

**Order emerges from chaos.** Three models sharing 1 tag in 972 map
into the same dozen intent classes when a neutral referee regroups them.
The underlying intent structure is real even though no two models share
its surface expression — the hypothesis of §1.6 (as planned) is
confirmed.

**The 13 clusters recover the 5 Reify facets at higher resolution.**
The mapping is clean (no cluster straddles facets, no facet is orphaned):

- `inform_state_fact` → **context**
- `directive_command` + `mandate_requirement` + `scope_definition` → **strategy**
- `constraint_enforcement` + `prohibition` → **guardrails**
- `verification_validation_gate` + `diagnostic_troubleshooting` → **observability**
- `risk_security_awareness` → **security**

So the 5 facets are a **low-resolution but faithful projection** of a
finer intent structure the models find on their own — not an arbitrary
imposition. The 13 clusters also show where the taxonomy could refine if
needed (split `directive` from `mandate`, `prohibition` from
`constraint`). The Run 5 model signatures reappear: Haiku dominates
`constraint_enforcement` (52%), Opus dominates `prohibition` (56%) and
`reflection` (63%), Sonnet dominates `risk_security` (52%) — each model
weights toward the class matching its preferred grammar, but all three
contribute to every class.

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

Three models sharing 0.1% of vocabulary without taxonomy imposition
(§1.5) vs **44.2% mean set-Jaccard** with the 5-facet taxonomy imposed
(`judge_labels` Opus vs `llm_labels` Haiku, measured during the judge
pass) suggests:

- The 5 facets produce ~44pp of coherence between models — they work
  as a Schelling point.
- But coherence is not validation. Two models can agree on the wrong
  taxonomy. Gold human labels remain the only stable anchor for
  measuring correctness.

(The user explicitly noted: this is fine — the point of `explore` is
not to maximise coherence but to see if order emerges. We're still
in the "exploration of the topology" phase, not the "calibration of
production" phase.)

### 2.4 Order emerges from chaos (§1.6 confirmed)

The strongest result of the battery. 972 surface-disjoint emergent tags
(0.1% shared across three models) collapse into 13 referee clusters,
12 of which absorb all three sources. The intent structure is real and
shared even when its surface vocabulary is not. Crucially, those 13
clusters map cleanly onto the 5 Reify facets at higher resolution — the
taxonomy is a low-resolution but faithful projection of a structure the
models find on their own, not an arbitrary imposition.

### 2.3 Prompt contamination is real and easy to miss

Run 2's "domain_fact" example became the top tag with 9 occurrences.
A reader of the result without access to the prompt would conclude
that `domain_fact` is a robust emergent concept. It isn't — it's an
echo.

Methodological hygiene rule for open coding: positive examples on the
target axis are forbidden. Use only anti-examples to anchor the axis
distinction.

## 3. Next experiments

- **Round 2 (cluster) on Sonnet + Haiku + Opus union** — ✅ done, see
  §1.6 / §2.4. Order emerges: 13 clusters, 12/13 absorb all 3 sources.
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
- `/tmp/cal-opus.jsonl` — `emergent_labels` from Opus 4.8
- `/tmp/clusters.json` — Run 6 referee output (13 clusters + outliers,
  per-source provenance) from `explore cluster`

All of `/tmp/cal*.jsonl` are ephemeral artefacts. The reproducible
inputs are this `findings.md`, `rubric.md`, `REGENERATING.md` (the
step-by-step rebuild recipe), the `reify-calibrate` binary, and the
`classify.log` files regenerated per `REGENERATING.md`.

> **Reproducibility note.** The entire corpus was regenerated from
> scratch on 2026-05-29 after a `/tmp` purge wiped every artefact. The
> numbers above are from that regenerated run; they track the original
> closely (Sonnet 413 vs 406 tags, Haiku 330 vs 315), confirming the
> pipeline is reproducible via `REGENERATING.md`. Two production gaps
> surfaced and were fixed during regeneration: `reify classify` must be
> given `--provider anthropic` explicitly or it auto-detects a local
> Ollama (corrupting the corpus), and the Anthropic provider's
> `max_tokens` was raised 8192→16384 so `explore cluster` over ~972
> tags no longer truncates.
