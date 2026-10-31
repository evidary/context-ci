# Context CI — product specification

**Version:** 1.0.0 · **Date:** 29 October 2026 · **Owner:** Context CI squad
**Target:** v0.1 public on **16 December 2026** · Paid tiers after the February 2027 gate

---

## 1. Scope

Context CI has exactly four verbs. Anything that is not one of them is out of scope.

| Verb | Command | What it does |
|---|---|---|
| **Capture** | `context-ci manifest` | Produces `context-manifest.json` for the current checkout |
| **Compare** | `context-ci diff <base> <head>` | Semantic diff between two manifests |
| **Attribute** | `context-ci attribute --results <file>` | Correlates a quality, cost or safety delta with diff entries |
| **Gate** | `context-ci check` | Exits non-zero when the diff violates configured thresholds |

Implementation status, so this table is not mistaken for a description of the binary:
Capture, Compare and Gate are implemented and tested; **Attribute is not**. The gap
and its reason are in [cli-reference.md](cli-reference.md) §8.

## 2. Non-goals

| Not building | Why |
|---|---|
| An eval framework | promptfoo, DeepEval and Ragas exist and are good; we consume their output |
| A prompt registry or authoring UI | Registries are the thing we read from, not the thing we become |
| A model proxy, gateway or router | Puts us in the request path and on the cost line |
| A trace store or APM product | Different buyer and a data-protection burden with no Phase 1 value |
| A hosted control plane for the free tier | The core check must work with no account |
| Automatic prompt optimisation | Optimising against the eval set is how you get an eval-set-shaped product |

## 3. How capture works

The manifest must be produced **without asking the developer to restructure their application**. Three capture paths, in order of preference:

| Path | When it applies | Mechanism |
|---|---|---|
| **Instrumented** | The team will add a hook | A small SDK call at the point where the request is assembled (`ctxci.capture(...)`), which serialises the assembled request |
| **Asserted** | The context is assembled from config files | Declarative `context-ci.yaml` lists the sources; the tool resolves and hashes them |
| **Observed** | Neither is possible yet | Read a request fixture or a recorded sample, and state clearly in the manifest that this is a sample, not the live path |

Every manifest records **which path produced it**, because a sample-based manifest is weaker evidence and must never be presented as equivalent.

## 4. The manifest

`context-manifest.json`, schema version `1`:

```json
{
  "manifest_version": 1,
  "created_at": "2026-12-01T10:00:00Z",
  "commit": "git-sha",
  "capture": {
    "path": "instrumented|asserted|observed",
    "sdk_version": "0.1.0"
  },
  "model": {
    "provider": "openai|anthropic|google|other",
    "id": "string",
    "alias_of": "string|null",
    "params": {"temperature": 0, "max_tokens": 0, "seed": null}
  },
  "prompt": {
    "template_hash": "sha256",
    "rendered_hash": "sha256",
    "rendered_sample_hash": "sha256|null",
    "variables": {"name": "type"}
  },
  "tools": [
    {"name": "string", "schema_hash": "sha256", "description_tokens": 0}
  ],
  "retrieval": {
    "index_id": "string",
    "embedding_model": "string",
    "embedding_model_version": "string",
    "chunker": "string",
    "top_k": 0,
    "filters_hash": "sha256|null"
  },
  "policy": {"version": "string", "guardrails_hash": "sha256|null"},
  "digest": "sha256 over the canonicalised document above"
}
```

Rules:

1. **Hashes, not content, by default.** A manifest is safe to commit. Content capture is opt-in and off by default.
2. **Deterministic.** The same inputs produce a byte-identical manifest, or the tool is broken. The only exception is `created_at`, which is excluded from `digest`.
3. **The rendered prompt is hashed, not stored**, unless the team opts in. Where a sample is captured it is hashed separately and labelled.
4. **No secrets.** Tool definitions are hashed after stripping any credential-shaped field.

## 5. The diff

A text diff of two manifests is useless — JSON key order alone would produce noise. The diff is semantic, organised by what actually changes behaviour:

| Diff class | Example | Default severity |
|---|---|---|
| `model.id` changed | `gpt-x` → `gpt-y` | Fail |
| `model.alias_of` changed | alias now resolves elsewhere | Fail |
| `model.params` changed | temperature 0 → 0.7 | Warn |
| `prompt.rendered_hash` changed | any rendered change | Warn, with a text diff if content was captured |
| `tools[]` added/removed | a tool gained or lost | Fail |
| `tools[].description_tokens` delta | +900 tokens | Warn with a cost projection |
| `retrieval.embedding_model` changed | re-embed | Fail |
| `retrieval.top_k` changed | 5 → 20 | Warn |
| `retrieval.filters_hash` changed | scope silently widened | Fail |
| `policy.version` changed | guardrail removed | Fail |

Severity is configurable, because a team that intentionally changes a model does not want a permanent failure. The default exists so that doing nothing is safe.

## 6. Attribution

Attribution is the differentiator and the hardest part. Given a manifest diff and a delta in a measured outcome (eval score, cost, latency, safety flag), report **which entries are most consistent with the delta**, and show the evidence.

Rules:

1. **Never a single number without the measurement behind it.** "Prompt hash changed, eval score −4.0 on 120 cases, 87 of the failures cluster on the two cases whose variables changed" is attribution. "Prompt change caused −4.0" is not.
2. **Say when the evidence is weak.** If two entries changed together, report both and say they are confounded.
3. **No causal claims from correlation alone.** The output is "consistent with", never "caused by".
4. **Attribution requires eval results the customer supplies.** We do not generate them.

## 7. CI integration

```yaml
- uses: context-ci/action@v0
  with:
    mode: check                # manifest | diff | attribute | check
    baseline: origin/main
    fail-on: fail              # fail | warn | never
    config: .context-ci.yaml
```

| Requirement | Target |
|---|---|
| Runtime | <30 s on a 2-core runner for a 50-tool manifest |
| Network | None. No outbound call is made in any mode |
| Secrets | None required for the open-source path |
| Outputs | GitHub annotations, JUnit XML, SARIF, plain text |
| Failure message | Names the changed entry and the configured threshold |

## 8. Configuration

`.context-ci.yaml`:

```yaml
version: 1
capture:
  path: asserted
  sources:
    prompt: prompts/support.md
    tools: config/tools.yaml
    retrieval: config/retrieval.yaml
severity:
  model.id: fail
  retrieval.top_k: warn
gate:
  fail_on: fail
  max_cost_delta_usd_per_day: 50
```

## 9. Requirements

| ID | Requirement | Verification |
|---|---|---|
| C-01 | The same inputs produce a byte-identical manifest, excluding `created_at` | Determinism test across 100 runs and 3 operating systems |
| C-02 | No outbound network call in any mode | Network-namespace test that blocks all egress |
| C-03 | A 50-tool manifest is produced in under 10 s | Benchmark |
| C-04 | Diff correctly classifies all ten diff classes with a fixture set | Fixture suite, published |
| C-05 | `check` exits non-zero on a configured `fail` class and zero on `warn` | CI test |
| C-06 | Failure output names the changed entry and the threshold | Snapshot test |
| C-07 | No credential-shaped string appears in a manifest | Secret corpus test |
| C-08 | Attribution states confounding when two or more entries changed | Test with a two-entry fixture |
| C-09 | Runs on Linux, macOS and Windows runners | CI matrix |
| C-10 | The tool works with no account and no network | Manual test by an outsider |

## 10. Pricing

| Plan | Price | Includes |
|---|---|---|
| Open source | Free | CLI, GitHub Action, local manifest, diff, attribution, gate |
| Team | $250/month | Hosted history, 3 repositories, 30-day retention, PR checks |
| Business | $1,000/month | 10 repositories, SSO, 90-day retention, attribution dashboard |
| Enterprise | Custom | Unlimited, self-host, audit export, SLA |

**The paid boundary is deliberately not a feature the adopter needs.** A single developer never needs cross-repository history or retention guarantees; a platform team always does.

## 11. Acceptance tests for v0.1

1. An outside engineer installs from the public README and gets a manifest in under five minutes.
2. A seeded regression is correctly attributed in a live demo.
3. The action passes on the three major CI providers (GitHub Actions, GitLab CI, CircleCI) or the gap is documented with a date.
4. `check` blocks a real pull request with a message a non-expert understands.
5. The repository has no reference to any closed-source component.

## 12. Open questions

| # | Question | Owner | Needed by |
|---|---|---|---|
| 1 | Manifest schema published as JSON Schema under what stability promise? | Squad | Week 2 |
| 2 | Do we support prompt registries (Langfuse, LangSmith) as a source at v0.1, or only files? | Squad | Week 3 |
| 3 | SARIF or JUnit as the primary machine-readable output? | Squad | Week 4 |
| 4 | Does the Team tier include CI minutes, or only history? | GTM | Before the February gate |
