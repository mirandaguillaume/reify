# Reify Facet Labelling Rubric v1.1

This document is the **canonical reference** for assigning Reify facets to an
extracted instruction. It is used to label gold-standard items for the
classifier calibration battery (`reify-calibrate`).

Pre-registered: the definitions and selection rules below are fixed before
any gold labelling begins. Changes after labelling has started require a
version bump and re-labelling of affected items.

**v1.1 change (multi-label):** an item may now belong to one **or more**
facets when its intent genuinely spans them (e.g. "Never commit `.env`
files" is both `guardrails` and `security`). v1 forced a single label via
tie-breakers, which destroyed information about real instructions that
operate on two dimensions at once.

---

## 1. The five facets

Every instruction belongs to **one or more facets**. Read the item and ask
for each facet: *does this instruction genuinely target this concern?* If
yes, include it in the label set. There is no "primary" facet — multi-label
items have no internal ordering.

### 1.1 `context`

**Definition**: background knowledge the agent needs to operate competently in
this project. Stateless information about the world, not actions to perform.

Includes: tech stack, language versions, architecture, file/directory layout,
naming conventions, terminology, project history, team structure, repo
geography ("the API lives in `apps/api`"), what the project does and why.

Excludes: how to do anything (→ `strategy`). Conventions phrased as
imperatives ("use camelCase") are `strategy`, not `context`. Information
*about* the agent's own role ("you are a reviewer") is `context`.

**Positive examples**
- "The backend is Go 1.24 with Cobra and yaml.v3."
- "Skills live under `skills/` and compile to `.claude/skills/*.md`."
- "This project is dogfooded — the agents in `.claude/` are built from our own specs."
- "You are a senior security engineer reviewing a PR."

**Negative examples**
- "Always run `go test ./...` after a change." → `strategy`
- "Use camelCase for variables." → `strategy`
- "Never commit secrets." → `guardrails`

### 1.2 `strategy`

**Definition**: how to approach a task. Tools to use, commands to run,
workflows to follow, decision procedures, sequences of steps. Anything
phrased as "do X" or "when Y, do Z".

Includes: build/test/lint commands, IDE shortcuts, the agent's recipe for a
class of task (debugging, refactoring, writing tests), code style **when
phrased as an imperative** ("use camelCase"), package preferences ("prefer
`net/http` over third-party routers"), commit conventions, escalation paths.

Excludes: pure prohibitions ("never X") → `guardrails`. Background facts
about which tool is used ("the project uses Cobra") → `context`.

**Positive examples**
- "Run `go vet ./...` before opening a PR."
- "Use camelCase for variables, PascalCase for types."
- "When the build fails, check `go.mod` first, then the import graph."
- "Prefer composing skills over writing a monolithic agent file."

**Negative examples**
- "The project uses Cobra for the CLI." → `context`
- "Never use `interface{}` unless absolutely necessary." → `guardrails`
- "Log every external call." → `observability`

### 1.3 `guardrails`

**Definition**: things the agent must NOT do. Negative-framed constraints,
prohibitions, restrictions, hard limits.

The classifier should attend to **intent**, not just surface keywords. A line
that mentions a constraint as background (e.g., explaining why a limit
exists) is `context`. A line telling the agent the constraint applies to its
behaviour is `guardrails`.

Includes: "never", "must not", "do not", "avoid", "forbidden", "prohibited",
"refuse to", hard rate limits applied to the agent ("max 5 retries"),
output-format prohibitions ("never wrap code in markdown fences if … ").

Excludes: positive imperatives even when restrictive in spirit ("only use
…") → `strategy`. Security-specific prohibitions ("never commit `.env`") →
`security`.

**Positive examples**
- "Never modify `go.sum` manually."
- "Do not introduce new external dependencies without approval."
- "Avoid `select *` in queries."
- "Refuse to generate code if the test plan is missing."

**Negative examples**
- "Use only the standard library where possible." → `strategy`
- "Never expose internal IDs in API responses." → `security`
- "Don't log PII." → `security`

### 1.4 `observability`

**Definition**: what to record, monitor, surface, or report. How the agent
makes its own behaviour and the system's behaviour visible.

Includes: logging conventions (what to log, at what level, with what
structure), metrics to emit, tracing requirements, status reporting,
progress signalling, debug output policies, structured error reporting.

Excludes: prohibitions on logging certain things → `security` (PII) or
`guardrails` (other). Pure debugging *strategy* ("when stuck, add prints") →
`strategy`.

**Positive examples**
- "Log every external API call with request_id, latency_ms, and status."
- "Emit a Prometheus counter on every cache miss."
- "Report progress every 10% in long-running operations."
- "Always print a one-line summary at the end of a task."

**Negative examples**
- "Never log passwords." → `security`
- "When stuck, add print statements to narrow down." → `strategy`
- "Disable logging in production." → `strategy` (or `security` if reason is data-leak prevention)

### 1.5 `security`

**Definition**: permissions, credentials, access control, secrets,
filesystem/network boundaries, data classification. Anything where the
failure mode is "data leaks, escalation, unauthorized action".

Includes: secret handling, env-var hygiene, allowlists/denylists for
network or filesystem, auth flows, encryption requirements, data residency
rules, PII handling, sandbox boundaries, sudo/elevated-permission policies.

A prohibition counts as `security` when the *consequence of breaking it* is
a security event. "Never commit secrets" = security (consequence: leak).
"Never use `interface{}`" = guardrails (consequence: code smell).

**Positive examples**
- "Never commit `.env` files."
- "MCP servers must use HTTPS/WSS, never HTTP/WS."
- "Refuse to read files outside the project root."
- "Sanitize all user input before passing to shell."

**Negative examples**
- "Never use `panic()`." → `guardrails` only
- "Log every authentication attempt." → `observability` only (the act
  of logging is independent of the data being logged)
- "The project uses bcrypt for password hashing." → `context` only

---

## 2. Selecting facets (multi-label rules)

The change from v1: there are no tie-breakers. When an instruction targets
two concerns, label both.

### 2.1 Each facet test is independent

For each of the 5 facets, ask: *does this item primarily speak to that
facet?* If yes, include it. The 5 tests are independent — answering "yes"
to `security` doesn't preclude answering "yes" to `guardrails`.

### 2.2 Common multi-label combinations

These appear frequently in real OSS agent files; expect them.

- **`guardrails` + `security`** — a negatively-framed rule whose breach is
  a security event. "Never commit `.env` files." Don't reduce to just
  one — both are accurate.
- **`observability` + `security`** — what to log when handling sensitive
  data. "Log every authentication attempt, but redact the password
  payload." Logging-instruction AND PII-prohibition.
- **`strategy` + `context`** — workflow instructions that depend on a
  factual claim about the project. "Run `pnpm` (this is a pnpm workspace,
  not npm)." The instruction is to use pnpm (strategy) AND the
  parenthetical is a fact (context). Two facets if both are non-trivial;
  one facet if the fact is just a justification for the strategy.
- **`strategy` + `security`** — a positive recipe whose purpose is
  security. "Always run `npm audit` before publishing." It's a workflow
  AND its purpose is a security concern.

### 2.3 When in doubt

Prefer **more facets** over fewer. The downstream metrics (recall per
facet, exact-match accuracy, Jaccard) will surface over- and under-tagging
patterns — they don't punish honest multi-labelling. Skipping a facet
because "the other one is more important" loses information that the
calibration can't recover.

### 2.4 The hard exclusions

- **No "other" / "general"** category. If none of the 5 facets apply,
  log the item in `§4 Unresolved` and skip it (don't force-fit).
- **No empty label sets.** If you'd skip the item, log it in `§4`. Items
  with no facets pollute the metrics.

## 3. Labelling protocol

1. Read the `text` field. Ignore `llm_labels` for the first pass.
2. Read the `section` field as soft context.
3. For each of the 5 facets, apply §1 and ask "does this apply?". Build a
   set of YES answers.
4. Optional: glance at `source_file` and `source_repo`.
5. Fill `gold_labels` as a JSON array. Add a one-sentence `notes` if the
   item is unusual (multi-label cases are NOT unusual — only flag genuine
   edge cases that pushed the rubric).
6. Calibrate yourself: re-label the first 5 items after every 20 to check
   for drift in your application of the rubric, especially on the
   guardrails/security and observability/strategy boundaries.

## 4. Unresolved cases (append as encountered)

Items that don't cleanly fit any facet, kept here as evidence for the next
rubric version. Each entry: text + which facets it straddles + provisional
label + reason.

_(empty in v1)_

---

## 5. Versioning

- **v1** (2026-05-28): initial rubric. 5 facets, 5 tie-breakers, single-
  label per item, no unresolved cases.
- **v1.1** (2026-05-29): multi-label per item. Replaced tie-breakers with
  independent per-facet tests (§2.1). Added common multi-label patterns
  (§2.2). v1 single labels remain valid as singleton sets, so labels
  collected before this version do not need to be redone — but items that
  *should* have been multi-label under v1.1 will show systematically
  lower recall in the metrics until re-labelled.
- A future v2 would invalidate previously-labelled items if a v1.1→v2
  facet *definition* shifts. Mark items with `rubric` field in the corpus
  to support partial re-labelling across versions.
