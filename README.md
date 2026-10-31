# Context CI

**Catch the prompt, tool or retrieval change that broke your AI feature — before it merges.**

Context CI records the exact context an AI feature ran with, diffs it semantically between commits, and attributes quality, safety and cost regressions to the specific change that caused them. It runs in your CI, on your keys, with no account required.

**Status:** CLI working — `manifest`, `diff`, `check` implemented and tested. 30-day build starts **17 November 2026**; open-source launch **16 December 2026**. See [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md).
**Licence:** Apache-2.0 · **Part of:** one company, five repositories — this is one of four standalone modules.

---

## The problem in one example

Your eval suite drops 4 points on Tuesday. Nobody changed the code. What changed:

- someone edited a prompt in a vendor dashboard on Monday afternoon, or
- a model alias silently started pointing at a newer model, or
- the retrieval index was rebuilt with a new embedding model, or
- a tool definition grew by 900 tokens and pushed the useful context out of the window.

None of those appear in `git log`. The template is in the repo; the **rendered** prompt is not, because rendering depends on retrieved chunks, tool results and provider-side templating. A diff of `prompt.md` is not a diff of what the model saw.

Context CI makes that diff exist.

## What it does

| Step | Output |
|---|---|
| **Manifest** | A `context-manifest.json` per commit: rendered prompt hash, tool definitions, retrieval config, model id and parameters, policy version |
| **Diff** | A semantic diff between two manifests — not a text diff. It knows a `top_k` change from a loop rewrite |
| **Attribute** | Which manifest entry correlates with the behaviour, cost or safety change, with the measurement shown |
| **Gate** | A CI check that fails a pull request when a context change exceeds thresholds you set |

## Quick start (planned for v0.1)

```yaml
# .github/workflows/context.yml
- uses: context-ci/action@v0
  with:
    manifest: context.manifest.json
    baseline: main
    fail-on: regression
```

Runs in ~20 seconds, needs no account, and works with any model provider because it never calls one.

## What this is not

- **Not an eval framework.** promptfoo, DeepEval, Ragas and LangSmith already do evals well. Context CI consumes eval results and attributes them; it does not try to be better at scoring.
- **Not an observability platform.** It does not sit in your request path or store your traces.
- **Not a prompt registry.** It reads from wherever your prompts already live, including registries, and records what was actually used.
- **Not a model proxy.** It never sees your traffic and never holds your keys.

## Why it is a separate repository

This is one of four standalone module repositories. It is Apache-2.0, useful with no platform account, and separately releasable. The platform company is a sibling repo (`acp-core`); the integration is optional and described in [docs/05-contracts/README.md](docs/05-contracts/README.md).

Read the reasoning: [docs/07-decisions/adr-0001-standalone-product-repo.md](docs/07-decisions/adr-0001-standalone-product-repo.md).

## Read this first

| Question | Document |
|---|---|
| Where is everything? | [docs/00-index.md](docs/00-index.md) |
| Why does this product deserve to exist? | [docs/01-thesis.md](docs/01-thesis.md) |
| Who are the 45 competitors and what do they miss? | [docs/02-market/market-and-competitors.md](docs/02-market/market-and-competitors.md) |
| What exactly are we building? | [docs/03-product/spec.md](docs/03-product/spec.md) |
| What is the 30-day plan? | [docs/04-plan/30-day-launch.md](docs/04-plan/30-day-launch.md) |
| What is the 15-week plan? | [docs/04-plan/module-plan.md](docs/04-plan/module-plan.md) |
| How does it talk to the platform? | [docs/05-contracts/README.md](docs/05-contracts/README.md) |
| Who does the work? | [docs/06-operations/squad-and-metrics.md](docs/06-operations/squad-and-metrics.md) |

## The honest risk

The most likely failure is not technical. It is that this becomes a well-loved open-source tool with a paid tier nobody buys, because the people who install it on a Saturday are not the people who approve a budget.

The mitigation is in the plan, not in the pitch: the paid boundary is **history and attribution across repositories**, which a single developer never needs and a platform team always does. If five absorption events in the market this quarter teach anything (Humanloop into Anthropic, Launchable into CloudBees, W&B into CoreWeave, Keywords AI into Respan, Gitar into Sonar), it is that a standalone devtool with no team-level buyer gets absorbed.

Section 8 of the market research is the full analysis.

## Contributing

External contributions are welcome once v0.1 is public. Until then, this repository is pre-release. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

Apache-2.0. See [LICENSE](LICENSE).
