# Reify — Direction note: the compliance moat

> Status: strategic direction, not an implementation spec. Captures a
> brainstorm (2026-06-02). Scope is the *why* and the *shape*; each
> chantier below earns its own spec → plan → implementation cycle later.
>
> This note builds directly on the calibration work in
> `docs/calibration/findings.md` (Runs 1–8). Those runs were not a side
> quest — they were the prototype of the experimental method this
> direction productises.

## 1. Thesis

Reify rests on one measurable hypothesis:

> **The space of *intentions* an agent-context file expresses is small,
> convergent, and enumerable — even though the surface text is infinitely
> varied.**

Run 6 already showed this in miniature: 972 surface-disjoint emergent
tags (0.1% shared across three models) collapsed into 13 clusters, 12 of
which absorbed all three sources. This direction generalises that result
and turns it into a product.

From that hypothesis:

> **Reify is the agent-context compiler backed by an empirical, dated
> matrix of what actually *works* — per harness × model couple. It
> guarantees context effectiveness on two layers of knowledge:
> structural (deterministic, universal) and empirical (measured,
> perishable).**

The pitch is no longer "write once, generate for Claude/Copilot/Cursor"
(AGENTS.md commoditises that). It is: *reify knows — and proves — which
formulations your model actually obeys, and tells you where your context
is structurally weak.*

## 2. The central primitive: the canonical intention

Today's model is `facet → instruction`. This direction inserts a layer:

```
facet  (5, fixed)            e.g. security
  └─ canonical intention     e.g. secrets-protection   ← NEW primitive
       └─ instruction        e.g. "never commit .env files"  (varied text)
```

Properties of a canonical intention:

- **Finite and enumerable** — on the order of dozens covering ~95% of
  the corpus. The exact count and the saturation point are themselves a
  measurement (a publishable result: "the ~N intentions that cover 95%
  of real agent-context files").
- **Cross-text** — `"never commit secrets"`, `"don't push .env"`, `"keep
  credentials out of git"` all map to `secrets-protection`.
- **Granularity** — that of the Run 6 clusters, or one notch finer.
  Between the category (facet) and the text (instruction).
- **The indexing unit for everything else** — the curated library, the
  efficacy matrix, and the comparison hook are all keyed by intention.

This is a `pkg/model` contract change (a new type between `Facet` and the
instruction). Treat it as breaking, done cleanly — consistent with the
`pkg/` stability note added earlier.

### Discovery vs join — two distinct phases

- **Discovery (offline)** — cluster a large corpus to *establish* the
  catalogue. Bottom-up, until saturation (no new intention after N
  repos). The catalogue is discovered, not decreed.
- **Join (runtime)** — for each sentence in a user's AGENTS.md: embed it,
  find its nearest neighbour in the catalogue/moat, and surface that
  entry's stats. No re-discovery — just k-NN.

Embeddings are the **runtime join mechanism**, not the discovery
mechanism. They bridge "infinite text" to "finite intentions": the
catalogue entries are centroids in embedding space; a user sentence
belongs to the entry it is nearest to. A fixed/versioned reference
embedding model is required so vectors stay comparable over time
(implementation detail, decide later).

## 3. The moat: a table of canonical instructions with stats

The moat is a table. Each row is a canonical instruction carrying its
measured statistics:

| field | example |
|---|---|
| intention | `secrets-protection` (facet `security`) |
| canonical formulation | `"never commit secrets"` |
| reference embedding | `[0.12, -0.04, …]` |
| **frequency** | present in 87% of comparable repos |
| **optimal formulation** | `never X` > `avoid X` (+Y pts compliance) |
| **efficacy / couple** | 94% claude-code+opus-4-8 · 71% copilot+gpt (dated) |
| **structural profile** | expected force = strong; ideal position = head |

When a user points reify at their AGENTS.md, for each recognised rule it
returns the full panel: **frequency + optimal formulation + efficacy per
couple + structural diagnostic**. That per-rule report is the unit of
value — the acquisition hook incarnate.

### What "efficacy" decomposes into

A rule's measured effectiveness is `f(lexicon, position, structural
force, context length, facet)`:

- **Lexicon / shibboleths** — certain markers (`never`, `MUST`,
  `IMPORTANT:`) act as *obedience triggers a model was post-trained to
  honour*. They are **per-family** (a Claude shibboleth may be lukewarm
  on GPT). The matrix is, in part, a map of each family's obedience
  vocabulary. **Shibboleths are one dimension among several**, not the
  whole story — and their existence as discrete word-triggers (vs a
  continuous formulation gradient) is itself a hypothesis the eval
  harness must test by ablation, not assume.
- **Position** — lost-in-the-middle, primacy/recency. See §4.
- **Structural force, context length, facet** — the rest.

## 4. Two layers of guarantee

### 4a. Structural layer — deterministic, universal, ships first

Quasi-universal properties of transformer attention: lost-in-the-middle,
primacy/recency, context length, lexical force. These are **stable
cross-model** — they do not need per-couple measurement. Reify's README
already asserts the ordering principle ("guardrails first, security
last"); this layer makes it a first-class, verifiable capability.

Scope decision: **detect, do not auto-correct.** Reify *flags* ("your
critical rule sits in the lost-in-the-middle zone"; "this guardrail's
force is weak for its facet"); the human decides. Auto-reordering can
break meaning in subtle ways, and doing it without proof the new
placement is better would be premature (and hypocritical given the
fidelity pitch). Auto-correction waits until the empirical
position→efficacy curve justifies it.

This layer is **OSS, zero-LLM, shippable now**, and reuses Run 7's
finding that `security`/`guardrails` are structurally stable
(flip < 0.6%).

### 4b. Empirical layer — measured, perishable, the moat

The efficacy matrix (§3), indexed by harness × model couple. Refines on
top of the structural layer where structure alone does not decide.

**Perishability is handled by dating, not by heavy infra.** Every matrix
cell carries its model version and measurement date ("never X = 94% on
claude-code + opus-4-8, measured 2026-06"). Reify warns when a
recommendation leans on a stale cell. Re-measuring is an incremental
improvement, not an obligation.

**Freshness is also the theft protection.** A scraped matrix rots within
months (new model → stale cells); the scraper must re-scrape
indefinitely, which costs more than subscribing. The moat's treadmill is
its anti-theft.

## 5. The data flywheel — three tiers

- **Public (open-source seed)** — solves cold-start and is a credibility
  asset. Already half-produced: the 5537 classified instructions from
  the 7 OSS repos of the calibration. The curated library v1 is the
  recurring patterns from that corpus, hand-curated by facet.
- **Federated (opt-in, anonymised)** — never the raw text. Upload the
  **embedding vector + label + structural features**, not the sentence.
  The corpus learns the semantic geometry without seeing proprietary
  content. (An embedding is already a practically non-reversible
  representation — the privacy decision and the embedding backbone are
  the same solution.)
- **Personal** — a user's accumulated databank: retention value that
  grows with use.

Acquisition (public + comparison hook) and the flywheel are the same
engine: the "missing/weak rules" surfaced to a user come from corpus
patterns; using reify feeds the corpus.

## 6. Business model & moat boundary

**Open-core, hosted.** The open/hosted split *is* the moat boundary —
and it coincides exactly with the deterministic/empirical split of the
thesis. That coincidence is the sign of a clean architecture: one
cleavage explains the technical line, the value line, and the
monetisation line at once.

| Layer | Side | Why |
|---|---|---|
| Compiler (`build`, targets, `pkg/spec`) | **OSS** | acquisition, credibility, already public |
| Structural layer (`doctor`/`check`: lost-in-the-middle, order, force) | **OSS** | real value, no LLM, not the moat |
| Bare intention catalogue (centroids, no stats) | **OSS** | shared mental framework (like 12-factor); value is in the stats |
| **Efficacy matrix + frequencies + optimal formulation** | **Hosted** | the moat: costly to produce, perishable, sellable |
| Reference embeddings + runtime join | **Hosted** | the index that serves the moat |

The moat never ships with the binary. The client *asks the question*
(sends an embedding/hash); the service *holds the answer*. Symmetric to
the privacy decision: users don't upload their text, you don't download
your moat.

The recurring production cost of the matrix (thousands of rollouts per
couple, re-run per model release) implies **recurring revenue** — you
don't sell *the* matrix, you sell *access to the matrix kept fresh*.

### Premium tier: exact test on the user's own formulation

Beyond matching a rule to its nearest moat neighbour (reads the matrix),
a premium tier **runs the eval harness on the user's exact formulation,
on their exact couple** — returning the efficacy of *their* rule, not the
nearest known one.

- This *produces a new matrix cell* and keeps it: the paying customer
  funds the moat's growth while being served. Flywheel and monetisation
  become the same act.
- It is also the fallback for atypical rules that fall far from any
  centroid (low match confidence) — premium covers the free tier's
  coverage gap.

Scope: **any rule, trap generated on demand** (the ambitious option).
The trap generator is bootstrapped by the catalogue ("this rule ≈
intention X; here are trap templates for X; instantiate them").

**Non-negotiable guardrail:** every generated trap must pass a
*validity* check — does it actually induce the violation when the rule is
*absent*? A trap that breaks nothing without the rule yields a false ~98%
compliance for everything: noise sold as signal. An invalid trap's
measurement is never served. (This is the base-rate problem from the
brainstorm: without temptation, every model "obeys" and nothing
separates good formulations from bad.)

## 7. The two studies (productised experiments)

### Run 9 — the deterministic frontier (4 levels)

A "good practice" is verifiable at four levels of increasing semantic
difficulty:

1. **Presence** — a rule of type X exists (glorified grep; surely
   deterministic).
2. **Placement** — it sits in the right structural section (deterministic
   via parsing).
3. **Force** — it is phrased with the right strength (`never` not `try to
   avoid`) (semi-deterministic: a force lexicon).
4. **Facet correctness** — it really *is* that facet (Run 7 proved this
   is fuzzy on strategy/context).

The study does not assume "deterministic or not" — it **measures where
determinism breaks**, level by level. The result is a quantified
frontier: exactly how much of context evaluation needs no LLM. Same
methodological move as Run 7 (net/fuzzy boundary between facets), new
axis (deterministic/LLM boundary between verification levels).

Context: reify *had* a keyword static classifier and removed it ("subjective
prose cannot be classified reliably from surface lexical patterns"). So
the answer is likely "deterministic for levels 1–3, breaks at 4" — and
that boundary is precisely what no one has measured.

### Run 10 — formulation efficacy (the eval harness)

A controlled A/B harness with **intentionally trapped tasks**: the task
makes violating the rule *natural*, then we measure whether the rule
holds. Outcome is **code-verifiable** (the trap is semantic in design,
but "did the agent commit the `.env`?" is a mechanical check on the
diff). The trap creates the *pressure* that makes `never` vs `avoid` vs
absent separate — without it, base-rate ~100% obedience hides all signal.

Per harness × model couple, starting with **same-family couples** (the
model a harness is built for: Claude Code + Claude, Copilot + GPT…) —
cuts combinatorics and maximises real-world relevance.

## 8. Chantiers & sequencing (deliberately not linear)

Three workstreams, and the brainstorm surfaced that they are
**iterative, not sequential**:

- **A — Eval harness (de-risks the thesis).** Build the trapped-task
  harness + trap-validity check; run on one couple for 5–10 intentions.
  If efficacy is not measurable / does not vary by formulation, the whole
  financial value collapses — so testing this early is the biggest
  de-risk.
- **B — Intention catalogue (the foundation).** Scale Run 6: cluster a
  large corpus to the ~N canonical intentions + saturation. Everything
  indexes on it.
- **C — Structural layer OSS (value + early adoption).** Enrich
  `doctor`/`check` with the deterministic invariants. Ships fast, no
  infra, seeds the OSS channel while the moat is built.

**The A↔B coupling (raised in the brainstorm):** B does not merely feed
targets to A — it changes *what A measures*. The catalogue's granularity
defines what counts as "a different formulation" vs "a different
intention". So A cannot wait for the *full* catalogue, but needs a
*seed*: a hand-made mini-catalogue of 5–10 obvious intentions
(`secrets-protection`, `test-before-commit`…) is enough to run A and
prove "efficacy varies by formulation". Then B at scale refines the
boundaries, and A re-runs on the real catalogue. The path is
**A-seed → B → A-full**, iterative, not a one-time ordering.

Sequencing across A/B/C is intentionally left open here; decide it cold
after reviewing this note.

## 9. Open questions

- Reference embedding model: local/open fixed vs versioned space.
  (Possible pre-study: are clusters stable across embedders? If the
  catalogue emerges identically regardless of embedder — as Run 6's
  structure emerged regardless of model — the choice matters little.)
- Trap-generation quality at scale (the hard sub-problem of the premium
  tier).
- Whether shibboleths are discrete word-triggers or a continuous
  formulation gradient (Run 10 ablation must distinguish).
- Cross-family efficacy (currently blocked: OpenRouter credits
  exhausted; all calibration runs are Anthropic-only).
- Gold labelling (from `findings.md` §3) remains the correctness anchor
  and the prerequisite for porting any few-shot/formulation advice into
  the production prompt.
