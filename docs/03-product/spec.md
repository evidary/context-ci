# Context CI — product specification

**Version:** 1.1.0 · **Date:** 16 November 2026 · **Owner:** Context CI squad
**Target:** v0.1 public on **16 December 2026** · Paid tiers after the February 2027 gate

---

## 1. Scope

Context CI's product surface is four capabilities. The CLI also carries a few supporting verbs — `init`,
`classes`, `module-manifest`, `serve` and `version` — and none of them changes what a gate decides, so none is
a fifth capability. Anything that is not one of the four below is out of scope.

| Verb | Command | What it does |
|---|---|---|
| **Gate** | `context-ci check --base-ref <ref>` | Compares the working tree with the commit where the branch left `<ref>`, reports what changed and where, and exits non-zero when a change is at or above the configured threshold and nobody acknowledged it |
| **Capture** | `context-ci manifest` | Produces a manifest for the current checkout |
| **Compare** | `context-ci diff <base> <head>` | Semantic diff between two manifest files |
| **Attribute** (experimental) | `context-ci attribute --baseline <base> --baseline-results <file> --results <file>` | Lists the diff entries a quality delta is consistent with |

Implementation status, so this table is not mistaken for a description of more than the binary does.
The remaining gaps are in [cli-reference.md](cli-reference.md) §8.

- **Capture is declared.** A manifest holds hashes of files in the repository and the model,
  parameters, retrieval settings and policy version written in `.context-ci.yaml`. No code observes
  a running application, so a manifest records what a repository says, not what a request contained.
  The SDK path (§3, *Instrumented*) observes one thing, the rendered prompt text, and has only been
  exercised against fixtures.
- **Nothing has to be committed.** `check --base-ref` builds the baseline from git, with the
  configuration that commit had, and the head from the working tree. A manifest file is still
  accepted as a baseline (`--baseline`) for comparing two points that are not two commits of one
  repository. Until version 0.9 a committed `context-manifest.json` was the only baseline, and it
  conflicted on every pair of branches that regenerated it.
- **A configuration file is optional.** With none, the coding-agent files a repository holds
  (`AGENTS.md`, `CLAUDE.md`, rule and skill directories, MCP and hook configuration) are compared.
  §8 says what a configuration adds.
- **Attribute is experimental, reads quality only, and names candidates.** It compares `score` and
  `passed` between two eval runs. A cost, latency or safety delta is not read, and its output is
  "consistent with", per §6 rule 3, never a cause. It is useful only when eval cases declare what
  they depend on.

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
| **Instrumented** | The team will add a hook | A small SDK call at the point where the request is assembled (`context_ci.capture(...)` in Python, `capture(...)` in TypeScript), which serialises the assembled request |
| **Asserted** | The context is assembled from config files | Declarative `.context-ci.yaml` lists the sources; the tool resolves and hashes them |
| **Observed** | Neither is possible yet | Read a request fixture or a recorded sample, and state clearly in the manifest that this is a sample, not the live path |

Every manifest records **which path produced it**, because a sample-based manifest is weaker evidence and must never be presented as equivalent.

What each path can vouch for, at this version:

| Field | Asserted | Instrumented (`manifest --capture`) |
|---|---|---|
| `prompt.template_hash`, tool hashes, `files[]` | Hash of the file the configuration names or the built-in patterns find | The same, except that the SDK may supply the template |
| `retrieval.filters_hash`, `policy.guardrails_hash` | Hash of the values in the file the configuration names: a YAML or JSON file is parsed first, so a comment, key order or formatting is not a change | The same |
| `prompt.rendered_hash` | Hash of the `rendered` file if one is named, otherwise the template hash | Hash of the text the SDK recorded at request assembly |
| `model.*`, `retrieval.index_id`, `embedding_model`, `chunker`, `top_k`, `policy.version` | Copied from `.context-ci.yaml` | Copied from `.context-ci.yaml` |

The last row is the limit: those values are declared by the team in both paths. Nothing in this
tool reads them from a provider or from a request, so a change made outside the repository — an
alias a provider moved, a prompt edited in a vendor dashboard — does not appear in a manifest until
someone updates a file. The *Observed* path is defined above and is not implemented.

## 4. The manifest

A manifest, schema version `2`:

```json
{
  "manifest_version": 2,
  "created_at": "2026-12-01T10:00:00Z",
  "capture": {
    "path": "instrumented",
    "sdk_version": "0.1.0"
  },
  "commit": "9c1f2ab",
  "files": [
    {
      "group": "agent.instructions",
      "hash": "1111111111111111111111111111111111111111111111111111111111111111",
      "path": "AGENTS.md"
    },
    {
      "group": "prompts",
      "hash": "2222222222222222222222222222222222222222222222222222222222222222",
      "path": "prompts/tone.md"
    }
  ],
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
  "digest": "297ad4bda25dac03c7c062eb9572c30daed8ce226f8e9be74a1611c5dabe5771"
}
```

Every value above is a legal value, and `context-ci diff` accepts this document. It is
deliberately concrete: an earlier revision showed `"path": "instrumented|asserted|observed"`
as a way of listing the alternatives, which is not a manifest any tool accepts, so a team
following the specification produced a file the CLI rejected with exit 3.

The example is in the **normalised form**, which is the form the digest covers. Two
rules follow from that, and they are not symmetric:

| Kind of field | In a manifest |
|---|---|
| Always emitted | `model.alias_of`, `prompt.rendered_sample_hash`, `retrieval.filters_hash` and `policy.guardrails_hash` are written as `null` when they have no value, because they are part of the document as the tool emits it |
| Omitted when empty | `model` and `prompt` as a whole, when the repository declares none; `files`; `hidden_characters` on a prompt, tool or file when it is zero; `capture.sdk_version`, `capture.reason`, `model.params.temperature`, `model.params.max_tokens`, `model.params.seed`, `model.params.top_p` |

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
| `retrieval.top_k`, `model.params.max_tokens`, `tools[].description_tokens`, `hidden_characters` | zero or more; a negative count is rejected |
| `files[].group` | `agent.instructions`, `agent.config`, or a name the configuration gives a group of files |
| `files[].path` | relative to the project root, with forward slashes; a path appears once |

These are enforced, not advisory: a manifest outside them is rejected with exit 3 by every verb that
reads one. A version 1 manifest, written by 0.8 or earlier, is rejected with a message that says to
build it again: its digest covered the commit, and its retrieval and guardrail hashes were of bytes. So is a manifest with a field this schema does not define. The schema has no extension
point, because the digest covers the document as it is read into the manifest structure — a field
the reader does not know would be dropped before the digest is recomputed, and the file would
verify while carrying content nothing checked.

Rules:

1. **Hashes, not content, by default.** A manifest holds no prompt text. The schema carries an optional `content` field for a copy a team opted into, and it is verified against its hash on load, but no verb writes one: only the hash is emitted.
2. **Deterministic, and about the context only.** The same files and configuration produce the same digest on any commit, at any time, on any operating system. `created_at` and `commit` are recorded and excluded from `digest`: they say when and where a manifest was built, and two branches that hold the same context have the same digest. Until version 2 the commit was inside the digest, so every commit had a different one.
3. **The rendered prompt is hashed, not stored.** Where a rendered sample is captured it is hashed separately and labelled as a sample.
4. **No secrets.** Before a tool definition is hashed, a string stored under a credential-named key, a literal value (`default`, `example`, `const`, `enum`) of a credential-named property, and any string that matches a recognised token shape are replaced with `<redacted>`. The parameter itself stays in the hash: a tool that gains a `token` parameter has changed. A random secret with no recognisable prefix in an unremarkable field is still missed; this is a safety net, not a guarantee.
5. **Everything that steers the model is hashed.** A field is inside the digest, or it is a plaintext copy that is verified against its hash on load. A count is never the record: `description_tokens` estimates, `description_hash` decides. See [ADR-0007](../07-decisions/adr-0007-unit-of-integrity.md).
6. **Text nobody can see is counted.** `hidden_characters` is the number of invisible or direction-changing characters in a prompt, a tool description or a file: the Unicode tag block, bidirectional overrides and isolates, zero-width space and word joiner, invisible operators, variation selector supplements and blank Hangul fillers. Zero-width joiners, text-direction marks and emoji variation selectors are not counted, because ordinary text in several languages needs them.

## 5. The diff

A text diff of two manifests is useless — JSON key order alone would produce noise. The diff is semantic, organised by what actually changes behaviour:

| Diff class | Example | Default severity |
|---|---|---|
| `model.id`, `model.provider`, `model.alias_of` | `gpt-x` → `gpt-y`; an alias now resolves elsewhere | Fail |
| `model.params` | temperature 0 → 0.7 | Warn |
| `prompt.template_hash`, `prompt.variables` | an edit to the prompt file | Warn |
| `prompt.rendered_hash` | any rendered change | Warn |
| `tools` | a tool added or removed, or a definition changed | Fail |
| `tools.description` | the text the model reads about a tool changed | Fail, with a cost projection when it grew and the gate has prices |
| `tools.description_tokens` | reserved for manifests that carry no description hash | Warn |
| `retrieval.embedding_model` | re-embed | Fail |
| `retrieval.index_id`, `retrieval.chunker`, `retrieval.top_k` | 5 → 20 | Warn |
| `retrieval.filters_hash` | a value in the retrieval file changed | Fail |
| `policy.version`, `policy.guardrails_hash` | guardrail removed | Fail |
| `agent.instructions` | `AGENTS.md`, `CLAUDE.md`, a rule or a skill changed | Warn |
| `agent.config` | an MCP server, a hook or agent permissions changed | Fail |
| `files.<group>` | a file in a group the configuration names was added, removed or changed | Warn |
| `hidden_characters` | a new or changed prompt, tool description or file contains invisible characters | Fail |
| `capture.path` | asserted → instrumented | Warn |

`context-ci classes` prints the list the binary uses.

Severity is configurable, because a team has its own view of what should stop a merge. The default
exists so that doing nothing is safe.

**A change that fails the gate is merged by acknowledging it**, not by weakening the configuration:
`check --acknowledge model.id` (or `all`) reports the change as acknowledged and does not fail on
it. In a pull request the acknowledgement is whatever the team's workflow maps to that flag, such as
a label a reviewer applies. `all` does not cover `hidden_characters`, which has to be named.

**Where and what.** Every entry about a file carries the file's path and the first changed line, and
`check --base-ref` prints the changed lines themselves, from `git diff`, for prompt, tool,
instruction and group files. The manifest still holds only hashes: the text comes from the
repository at the time of the check. `--no-text` leaves it out.

## 6. Attribution

Attribution is the differentiator and the hardest part. Given a manifest diff and a delta in a measured outcome (eval score, cost, latency, safety flag), report **which entries are most consistent with the delta**, and show the evidence.

Rules:

1. **Never a single number without the measurement behind it.** "Prompt hash changed, eval score −4.0 on 120 cases, 87 of the failures cluster on the two cases whose variables changed" is attribution. "Prompt change caused −4.0" is not.
2. **Say when the evidence is weak.** If two entries changed together, report both and say they are confounded.
3. **No causal claims from correlation alone.** The output is "consistent with", never "caused by".
4. **Attribution requires eval results the customer supplies.** We do not generate them.

## 7. CI integration

GitHub Actions, through the wrapper in [`action.yml`](../../action.yml), which is at the repository
root and is therefore named `sigilary/context-ci@<ref>`:

```yaml
- uses: actions/checkout@v4
  with: { fetch-depth: 0 }      # the baseline is built from the base branch, so it has to be there
- uses: sigilary/context-ci@v0.9.0
  with:
    acknowledge: ${{ contains(github.event.pull_request.labels.*.name, 'context-change-approved') && 'all' || '' }}
```

On a pull request the action compares with the base branch without being told to. It writes
annotations on the changed lines and a summary, with the changed text, on the run's page. Named with
a release tag it downloads that release's binary and checks it against the release's `SHA256SUMS`;
named with a branch or a commit, or when no binary can be downloaded, it builds from its own
checkout and installs Go if the runner has none.

GitLab CI and CircleCI call the binary directly. There is nothing in the action a pipeline script
cannot do.

```yaml
# .gitlab-ci.yml
context-ci:
  image: golang:1.27-bookworm
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  variables:
    GOBIN: $CI_PROJECT_DIR/bin
    GIT_DEPTH: 0                # the merge base has to be in the clone
  script:
    - go install github.com/sigilary/context-ci/cmd/context-ci@v0.9.0
    # The refspec is explicit because a CI clone is usually single-branch, and `git fetch origin
    # main` then leaves no origin/main behind, only FETCH_HEAD.
    - git fetch origin "+refs/heads/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME:refs/remotes/origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME"
    # Redirected rather than --out, so the exit code that fails the job is check's own.
    - bin/context-ci check --base-ref "origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME" --format sarif > context-ci.sarif
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
      - image: cimg/go:1.27
    steps:
      - checkout
      - run:
          name: Install context-ci
          command: go install github.com/sigilary/context-ci/cmd/context-ci@v0.9.0
      - run:
          name: Gate
          command: |
            git fetch origin "+refs/heads/main:refs/remotes/origin/main"
            context-ci check --base-ref origin/main
workflows:
  pull-request:
    jobs:
      - context-ci
```

### What has actually been run

As of **16 November 2026**, for version 0.9:

| Where | What has been exercised |
|---|---|
| The binary | `go test ./...` on Windows, including end-to-end tests that create git repositories and run `check --base-ref` against them: an unchanged branch, an edit, a base branch that moved on, two branches merged, a base with no configuration, a project in a subdirectory. `tools/smoke_quickstart.sh` runs the README's steps against a built binary under Git Bash. |
| GitHub Actions | `.github/workflows/ci.yml` ran green on GitHub-hosted runners on 16 November 2026: tests on Linux, macOS and Windows, the smoke script, and `action.yml` on all three. The action built its binary from the checkout with no Go toolchain set up by the workflow, passed on an unedited checkout, failed on an edited prompt with a warning annotation on `examples/minimal/prompts/support.md` line 5, and passed with that class acknowledged, the annotation becoming a notice. **Not exercised:** the default base branch on a `pull_request` event, since the CI passes `base-ref` itself, and nobody has looked at the rendered job summary. The download step was extracted and run on Windows and Linux against published releases: a named release installs a binary matching `SHA256SUMS`, a branch, a commit or a missing release falls back to a build, and a mismatched binary stops the step. It has not run inside a workflow, because that needs a release of this version. |
| GitLab CI, CircleCI | The commands above were run on 16 November 2026 in the images the snippets name, `golang:1.27-bookworm` and `cimg/go:1.27`, against a single-branch clone of a fixture whose base branch had moved on after the branch was cut: `go install` from the public module (at a commit, since the tag did not exist yet), the explicit fetch, `check --base-ref` → **exit 1** naming the branch's two changes and not the base branch's, and a SARIF file written before the exit. **Not on a GitLab or CircleCI runner**: `rules:`, `GIT_DEPTH`, `artifacts:reports:sarif` and CircleCI's checkout depth are unexercised. |
| Public repositories | The defaults, with no configuration, over the last 100 commits of three public repositories that hold agent files. Reported: nothing, 3 changes and 24 changes, every one a real edit to an instruction, skill or settings file; no hidden-character reports; under five seconds on 4,400 files. It also found a defect, fixed the same day: on Windows a symbolic link was reported as an added file. Three repositories is a first look, not a false-positive rate. |

An earlier version of the GitLab snippet wrote `git fetch --depth 1 origin main`, and in a
single-branch CI clone that leaves no `origin/main` behind. Running it found that; reading it had
not. The same will be true of everything in the table above that says it has not been run.

The first run on hosted runners found what local runs on Windows could not: a test that had
failed on Linux and macOS since 0.8.0, because the two report a different error for a path below a
regular file. The tool was right and the test was wrong, and it had kept the 0.8.0 release from
publishing.

Acceptance test 11.1 (an outside engineer installs from the README) is possible now that the
repository is public, and nobody has been asked yet.

| Requirement | Target |
|---|---|
| Runtime | <30 s on a 2-core runner for a 50-tool manifest; not measured |
| Network | None by the binary. The action downloads a release binary when a release is named |
| Secrets | None required for the open-source path |
| Outputs | GitHub annotations on the changed line, a Markdown job summary, JUnit XML, SARIF, plain text |
| Failure message | Names the changed entry and the configured threshold |

## 8. Configuration

`.context-ci.yaml` is optional. Without it the built-in coding-agent files are compared and nothing
else. With it:

```yaml
version: 1
capture:
  path: asserted
  sources:                       # each is optional
    prompt: prompts/support.md
    tools: config/tools.yaml     # this tool's own shape, a provider's, or a saved MCP tools/list
    retrieval: config/retrieval.yaml
  files:                         # any other files that steer the model, in groups you name
    prompts: ["prompts/**/*.md"]
    examples: few_shot/*.jsonl
  agent_files: true              # the default: AGENTS.md, CLAUDE.md, rules, skills, MCP and hook configuration
model:                           # optional; provider and id go together
  provider: openai
  id: gpt-x
severity:
  model.id: fail
  retrieval.top_k: warn
  files.examples: fail
gate:
  fail_on: fail
  calls_per_day: 20000
  input_usd_per_mtok: 3
```

The gate compares severities and never dollars: `calls_per_day` and `input_usd_per_mtok` only
turn the growth of a tool description into a projection printed beside the entry, and a cost figure
never fails a build by itself. The token count behind it is an estimate from the number of words.

The configuration is part of what is compared: `check --base-ref` builds the baseline with the
configuration the base commit had. Adding a file group therefore reports its files as added, once.

## 9. Requirements

| ID | Requirement | Verification |
|---|---|---|
| C-01 | The same inputs produce the same digest, whatever the commit and the time | `TestDeterministicAcrossRuns` (100 runs on one machine), `TestDigestIgnoresCommit` and `TestGitTreeReadsOnlyWhatTheCommitHolds`. Run on Linux, macOS and Windows in CI |
| C-02 | The binary makes no outbound network call | **No test.** By inspection: the only network code is `serve`, which listens |
| C-03 | A 50-tool manifest is produced in under 10 s | **Not measured** |
| C-04 | Diff classifies every class `context-ci classes` lists | `TestEveryClassIsReachable` |
| C-05 | `check` exits non-zero on a configured `fail` class and zero on `warn` | `go test ./cmd/...` and `tools/smoke_quickstart.sh` |
| C-06 | Failure output names the changed entry, its file and line, and the threshold | `TestCheckAgainstABaseRef`, `TestCheckWritesASummaryAndAnnotations` |
| C-07 | No recognised credential value affects a hash | `TestCredentialValuesDoNotAffectTheHash`, `TestCredentialSpellings` |
| C-08 | Attribution states confounding when two or more classes changed | `TestTwoChangesTogetherAreStatedAsConfounded`, and `TestAttributeStatesConfoundingAndNeverACause` at the CLI |
| C-09 | Runs on Linux, macOS and Windows runners | CI matrix in `.github/workflows/ci.yml`, green on hosted runners on 16 November 2026. The three runners reported the same digest for `examples/minimal` |
| C-10 | The tool works with no account and no network | Manual test by an outsider; nobody has been asked yet |
| C-11 | A comment or a reordering in a retrieval or guardrails file is not a change | `TestCheckAgainstABaseRef`, `TestStructuredSourcesIgnoreCommentsAndOrder` |
| C-12 | Two branches that change different context files merge without conflict, and a pull request is not blamed for an earlier one | `TestTwoBranchesThatChangeContextMergeCleanly`, `TestADocsOnlyChangeIsNotBlamedForAnEarlierOne`, `TestTheBaselineIsWhereTheBranchLeftTheBase` |
| C-13 | A fail-class change can be accepted for one run without editing the configuration, and stays in the report | `TestAcknowledgeIsTheApprovalPath` |

## 10. Pricing

**Planned, not available.** Only the open-source row exists today. No hosted service has been built, and the
prices below are a plan that has not been tested with a customer.

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
5. The repository builds, tests and runs with no closed-source component present.

## 12. Open questions

| # | Question | Owner | Needed by |
|---|---|---|---|
| 1 | Manifest schema published as JSON Schema under what stability promise? | Squad | Week 2 |
| 2 | Do we support prompt registries (Langfuse, LangSmith) as a source at v0.1, or only files? | Squad | Week 3 |
| 3 | SARIF or JUnit as the primary machine-readable output? | Squad | Week 4 |
| 4 | Does the Team tier include CI minutes, or only history? | GTM | Before the February gate |

**Question 3 is answered: both, and neither is primary.** `--format sarif` is a SARIF 2.1.0 report
for a consumer that reads one, with a file and line on every result that is about a file; `--format junit` is for a dashboard that only reads XML, and the action still defaults to `github`
because an annotation on the pull request is cheaper to read than any report. See
[cli-reference.md](cli-reference.md) §7 for how each severity is translated, and why a `warn` is
not dressed up as a failure.
