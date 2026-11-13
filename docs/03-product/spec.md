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
Capture, Compare, Gate and Attribute are implemented and tested. The remaining gaps are in
[cli-reference.md](cli-reference.md) §8.

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
  "capture": {
    "path": "instrumented",
    "sdk_version": "0.1.0"
  },
  "commit": "9c1f2ab",
  "model": {
    "alias_of": null,
    "id": "gpt-x",
    "params": {
      "max_tokens": 1024,
      "temperature": "0"
    },
    "provider": "openai"
  },
  "policy": {
    "guardrails_hash": null,
    "version": "2026.11"
  },
  "prompt": {
    "rendered_hash": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "rendered_sample_hash": null,
    "template_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "variables": {
      "tone": "string"
    }
  },
  "retrieval": {
    "chunker": "recursive-800",
    "embedding_model": "embed-3",
    "embedding_model_version": "2026-05",
    "filters_hash": null,
    "index_id": "kb-prod",
    "top_k": 5
  },
  "tools": [
    {
      "description_hash": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
      "description_tokens": 12,
      "name": "refund.create",
      "schema_hash": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    }
  ],
  "digest": "9ac41aab288437780e7438a9f7061a59e235835848ce88143a3239544b06d1b1"
}
```

Every value above is a legal value, and this document passes `context-ci check`. It is
deliberately concrete: an earlier revision showed `"path": "instrumented|asserted|observed"`
as a way of listing the alternatives, which is not a manifest any tool accepts, so a team
following the specification produced a file the CLI rejected with exit 3.

The example is in the **normalised form**, which is the form the digest covers. Two
rules follow from that, and they are not symmetric:

| Kind of field | In a manifest |
|---|---|
| Always emitted | `model.alias_of`, `prompt.rendered_sample_hash`, `retrieval.filters_hash` and `policy.guardrails_hash` are written as `null` when they have no value, because they are part of the document as the tool emits it |
| Omitted when empty | `capture.sdk_version`, `capture.reason`, `model.params.temperature`, `model.params.max_tokens`, `model.params.seed`, `model.params.top_p` |

The digest is computed over the document **after it has been read into the manifest
structure**, so writing `"seed": null` and omitting `seed` produce the same digest, while
writing `"filters_hash": null` and omitting `filters_hash` do not. A team implementing from
this specification should emit the normalised form, so that the bytes on disk are the bytes
that were hashed. `internal/manifest/spec_example_test.go` fails if this example ever stops
reproducing its own digest.

The values that are restricted, and what is allowed:

| Field | Allowed |
|---|---|
| `capture.path` | `instrumented`, `asserted`, `observed` |
| `model.provider` | `openai`, `anthropic`, `google`, `other` |
| `model.params.temperature`, `top_p`, `seed` | decimal strings, so a value never depends on float formatting |
| `prompt.variables` | variable name to declared type; values are never recorded |
| hash fields | 64 lowercase hex characters |
```

Rules:

1. **Hashes, not content, by default.** A manifest is safe to commit. Content capture is opt-in and off by default.
2. **Deterministic.** The same inputs produce a byte-identical manifest, or the tool is broken. The only exception is `created_at`, which is excluded from `digest`.
3. **The rendered prompt is hashed, not stored**, unless the team opts in. Where a sample is captured it is hashed separately and labelled.
4. **No secrets.** Tool definitions are hashed after stripping any credential-shaped field, and any string that matches a recognised token shape is replaced with `<redacted>` whatever its field is called. A random secret with no recognisable prefix is still missed; this is a safety net, not a guarantee.
5. **Everything that steers the model is hashed.** A field is inside the digest, or it is a plaintext copy that is verified against its hash on load. A count is never the record: `description_tokens` estimates, `description_hash` decides. See [ADR-0007](../07-decisions/adr-0007-unit-of-integrity.md).

## 5. The diff

A text diff of two manifests is useless — JSON key order alone would produce noise. The diff is semantic, organised by what actually changes behaviour:

| Diff class | Example | Default severity |
|---|---|---|
| `model.id` changed | `gpt-x` → `gpt-y` | Fail |
| `model.alias_of` changed | alias now resolves elsewhere | Fail |
| `model.params` changed | temperature 0 → 0.7 | Warn |
| `prompt.rendered_hash` changed | any rendered change | Warn, with a text diff if content was captured |
| `tools[]` added/removed | a tool gained or lost | Fail |
| `tools[].description_hash` changed | an edit that keeps the length | Fail |
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

GitHub Actions, through the wrapper in [`action.yml`](../../action.yml):

```yaml
- uses: evidary/context-ci/action@v0.4.1
  with:
    command: check              # manifest | check
    baseline: context-manifest.json
    format: github              # text | github | json | sarif | junit
    config: .context-ci.yaml
```

GitLab CI and CircleCI call the binary directly. The action is a shell wrapper around
`go install`; there is nothing in it a pipeline script cannot do, and a wrapper that exists for one
provider is not worth copying three times.

```yaml
# .gitlab-ci.yml
context-ci:
  image: golang:1.23-bookworm
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  variables:
    GOBIN: $CI_PROJECT_DIR/bin
  script:
    - go install github.com/evidary/context-ci/cmd/context-ci@v0.4.1
    # The baseline is the manifest committed on the target branch, not a re-render of it: a
    # baseline regenerated at compare time would agree with whatever it was generated from.
    # The refspec is explicit because a CI clone is usually single-branch, and `git fetch origin
    # main` then leaves no origin/main behind — only FETCH_HEAD.
    - git fetch --depth 1 origin "+refs/heads/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME:refs/remotes/origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME"
    - git show "origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME:context-manifest.json" > baseline.json
    - bin/context-ci manifest --commit "$CI_COMMIT_SHA" --out head.json
    # Redirected rather than --out, so the exit code that fails the job is check's own.
    - bin/context-ci check --baseline baseline.json --head head.json --format sarif > context-ci.sarif
  artifacts:
    when: always
    reports:
      sarif: context-ci.sarif
```

```yaml
# .circleci/config.yml
version: 2.1
jobs:
  context-ci:
    docker:
      - image: cimg/go:1.23
    steps:
      - checkout
      - run:
          name: Install context-ci
          command: go install github.com/evidary/context-ci/cmd/context-ci@v0.4.1
      - run:
          name: Baseline from the base branch
          command: |
            # Explicit refspec, for the same reason as the GitLab snippet: a single-branch clone
            # has no origin/main to read from.
            git fetch --depth 1 origin "+refs/heads/main:refs/remotes/origin/main"
            git show "origin/main:context-manifest.json" > baseline.json
      - run:
          name: Gate
          command: |
            context-ci manifest --commit "$CIRCLE_SHA1" --out head.json
            context-ci check --baseline baseline.json --head head.json
workflows:
  pull-request:
    jobs:
      - context-ci
```

### What has actually been run

Acceptance test 11.3 allows either three providers or a documented gap with a date. As of
**13 November 2026**:

| Provider | Snippet | What has been exercised |
|---|---|---|
| GitHub Actions | `action.yml` | The binary, the `github` format and `check`'s exit codes run locally, and the action's own install command is proven from an empty module cache. **No run on a GitHub-hosted runner**, because the Actions API returns 404 for this repository. |
| GitLab CI | above | All five commands, verbatim from this page, in Git Bash against a single-branch shallow clone of a fixture project with a committed baseline manifest: install, fetch, baseline from the target branch, head manifest, then `check --format sarif` → **exit 1** with a schema-valid SARIF report already written, which is what `artifacts: when: always` needs. **Not on a GitLab runner**: the `golang:1.23-bookworm` image, `rules:` and `artifacts:reports:sarif` are unexercised. |
| CircleCI | above | The three commands, verbatim, the same way: **exit 1**, and the readable output names `retrieval.top_k` with its before and after. **Not on a CircleCI runner**: the `cimg/go` image and its checkout depth are unexercised. |

Running them was worth more than reading them. The first version of the GitLab snippet wrote
`git fetch --depth 1 origin main`, and in a single-branch CI clone that leaves no `origin/main`
behind — only `FETCH_HEAD` — so `git show origin/main:context-manifest.json` failed. Both snippets now
fetch with an explicit refspec. That class of bug is exactly what a table saying "unverified" would
have shipped.

What is left is the runner, not the script: an image, a trigger and a report consumer. Each closes
the first time a stranger runs one and says whether it worked.

**One thing blocks all of them, and it is not a defect.** This repository is private until the
16 December launch, so no outsider can install the tool at all — measured on 13 November 2026:
an anonymous `go install ...@v0.4.1` fails at `git ls-remote` with *could not read Username*, and
`api.github.com/repos/evidary/context-ci` answers 404. Acceptance test 11.1 (an outside engineer
installs from the README) cannot be attempted until that changes, and it is the first thing to
re-run on launch day.

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

**Question 3 is answered: both, and neither is primary.** `--format sarif` lands in code scanning,
`--format junit` in a dashboard that only reads XML, and the action still defaults to `github`
because an annotation on the changed line is cheaper to read than any report. See
[cli-reference.md](cli-reference.md) §7 for how each severity is translated, and why a `warn` is
not dressed up as a failure.
