# Copy vs. Compile

A reproducible demonstration of what separates reify from a format converter.

Both reify and tools like [`rule-porter`](https://github.com/nedcodes-ok/rule-porter)
turn a `CLAUDE.md` into an `AGENTS.md`. The output file has the same name and
the same format. The difference is what happens to the *content* in between.

- **rule-porter copies.** It re-wraps the prose in the target's frontmatter and
  preserves the original order, verbatim.
- **reify compiles.** It classifies every instruction into a facet, then
  re-emits it in an order that is proven to change how well a model obeys —
  guardrails first (primacy), security last (recency for least-privilege).

Same input, same output format, categorically different artifact: a photocopier
vs. an optimizing compiler.

## The input

[`input/CLAUDE.md`](input/CLAUDE.md) is a deliberately mis-ordered project file.
The prohibitions a model most needs to respect — *never log the Stripe secret
(PCI)*, *never skip the idempotency check (double-charge)* — sit at the very
**bottom**, under verbose strategy and buried credentials. This is the worst
case for primacy/recency: the load-bearing rules are in the position a model
attends to least.

## The two outputs

| | [`via-rule-porter/AGENTS.md`](via-rule-porter/AGENTS.md) | [`via-reify/AGENTS.md`](via-reify/AGENTS.md) |
|---|---|---|
| Guardrails (`Never …`) | **last** (kept where the input had them) | **first** (`### Rules`) |
| Security / secrets | middle, as prose, mixed in | **last**, as a distinct `### Security` section |
| Observability | dropped into prose, unlabeled | own `### Observability` section |
| Ordering logic | none — verbatim | facet classification + proven-efficacy order |
| Provenance line | mislabels source as `.cursor/rules/` | states it was compiled |

rule-porter's own run summary calls the file "4 rules converted" — it never
inspects *what* the rules are. reify's output leads with the five `Never …`
guardrails, then steps, then observability, and closes with the security
envelope.

## Why the reorder is allowed (not vandalism)

Reordering someone's instructions is only legitimate if you can prove the order
matters. It does, and reify measures it: the reify-eval bench shows that
formulation and position change obedience, that a guardrail stated first is
honored more than the same guardrail buried last, and that this holds across
Haiku, Sonnet, and Opus. That measured result is the *license* to move
guardrails to the top — without it, reordering would just be guessing.

This is the half a pure converter cannot reach: it has no taxonomy to classify
by, and no evidence to reorder by.

## Reproduce it

The reify side is deterministic and needs no API key — `build` never calls an
LLM:

```sh
# rule-porter (verbatim copy)
cd input && npx -y rule-porter --from claude-md --to agents-md \
  --out ../via-rule-porter/AGENTS.md

# reify (classify + reorder)
reify build --target agents \
  --skills via-reify/src/skills \
  --agents via-reify/src/agents \
  --output via-reify/build
# → writes via-reify/AGENTS.md
```

### About the committed `via-reify/src/`

The catalogued source in [`via-reify/src/skills/`](via-reify/src/skills/) is the
facet classification reify produces from the flat input. It is committed so the
demo rebuilds deterministically (LLM output is non-deterministic and unfit for a
golden file). To regenerate the classification live from the flat file:

```sh
reify import input/CLAUDE.md --provider anthropic -o via-reify/src --yes
```

That step is what no format converter does: `import` runs the same five-facet
classifier (`internal/classifier`) that grounds `classify`, `check`, and
`doctor`. The cataloging, not the file rename, is the moat.
