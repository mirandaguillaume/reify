# Reify Facet Labelling Rubric v1

This document is the **canonical reference** for assigning one of the five
Reify facets to an extracted instruction. It is used to label gold-standard
items for the classifier calibration battery (`reify calibrate`).

Pre-registered: the definitions and tie-breaking rules below are fixed before
any gold labelling begins. Changes after labelling has started require a
version bump (v2) and re-labelling of affected items.

---

## 1. The five facets

Every instruction belongs to exactly one facet. The label is the **dominant
intent** of the instruction — what the author is trying to make the agent do
or avoid. Not what the surface keywords suggest.

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
- "Never use `panic()`." → `guardrails`
- "Log every authentication attempt." → `observability`
- "The project uses bcrypt for password hashing." → `context`

---

## 2. Tie-breaking rules

When an instruction plausibly fits two facets, apply these in order:

1. **Security dominates.** If breaking the rule causes a security event,
   label `security` regardless of surface form.
2. **Guardrails over strategy.** A negatively-framed rule whose consequence
   is not a security event is `guardrails`, not `strategy`, even if it
   describes a workflow.
3. **Observability over context.** "Log X with field Y" is `observability`
   even if Y is described in `context`-like terms.
4. **Strategy over context.** When in doubt between `context` and `strategy`,
   ask "is this telling the agent *what to do* or *what is true*?". If both,
   pick `strategy`.
5. **No "other" / "general"** category. Force a choice. If the rubric
   genuinely cannot accommodate the instruction, log it in the rubric's
   `unresolved` section (see §4) and label provisionally with the closest
   facet, marking `notes` in the corpus.

## 3. Labelling protocol

1. Read the `text` field. Ignore the LLM label in `llm_label` for the first
   pass.
2. Read the `section` field as soft context: which heading the instruction
   appeared under in the source file.
3. Apply §1 definitions. If clear → assign. If unclear → apply §2 tie-breakers.
4. Optional: glance at `source_file` and `source_repo` for further context
   (a sentence about Python conventions is more likely `strategy` if the
   source is a Python-focused agent file).
5. Fill `gold_label`. Add a one-sentence `notes` if the choice required a
   tie-breaker or the item is unusual.
6. Calibrate yourself periodically: re-label the first 5 items after every
   20 to check for drift in your own application of the rubric.

## 4. Unresolved cases (append as encountered)

Items that don't cleanly fit any facet, kept here as evidence for the next
rubric version. Each entry: text + which facets it straddles + provisional
label + reason.

_(empty in v1)_

---

## 5. Versioning

- **v1** (2026-05-28): initial rubric. 5 facets, 5 tie-breakers, no
  unresolved cases yet.
- Bumping to v2 invalidates previously-labelled items if a v1→v2 facet
  definition shift could change their label. Mark items as `v1` in the
  corpus to support partial re-labelling.
