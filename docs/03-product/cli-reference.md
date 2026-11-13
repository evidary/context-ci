# CLI reference

**Version:** 1.0.0 · **Date:** 31 October 2026 · **Owner:** Context CI squad

The four verbs in [the specification](spec.md) §1. `attribute` is specified but not
yet implemented; the other three are, and this page is the contract for them.

---

## 1. Install and first run

```bash
context-ci manifest          # writes context-manifest.json for this checkout
```

That is the whole quickstart. No account, no network call, no secret.

In a pull request:

```yaml
- uses: actions/checkout@v4
  with: { fetch-depth: 0 }
- uses: actions/setup-go@v5
  with: { go-version: "1.27" }
- uses: evidary/context-ci/action@v0.4.1
  with:
    baseline: context-manifest.base.json
    format: github      # annotations on the diff
```

The action installs the binary with `go install` at its own tag, so what runs is the binary
built from the commit the action names. `check` without a baseline fails rather than passing on
an empty comparison.

## 2. The verbs

| Verb | Command | What it does |
|---|---|---|
| Capture | `context-ci manifest` | Writes `context-manifest.json` for the current checkout |
| Compare | `context-ci diff <base> <head>` | Semantic diff between two manifests |
| Gate | `context-ci check` | Diffs against a baseline and exits non-zero above the threshold |
| Explain | `context-ci classes` | Lists every diff class and its default severity |
| Attribute | `context-ci attribute` | Says which context class moved, when the cases declare what they depend on |

`attribute` was the last verb to land, because it needed the eval results format settled first.
It still refuses to guess: with no `depends_on` declarations it reports a correlation and says so.
See [`attribute`](#attribute).

### `manifest`

```bash
context-ci manifest [--config .context-ci.yaml] [--root .] [--out path] \
                    [--commit <sha>] [--sdk-version <ver>] [--capture <file>]
```

`--capture` reads a file written by an SDK at request-assembly time — see
[sdk/python](../../sdk/python/README.md) and [sdk/typescript](../../sdk/typescript/README.md) — and
produces an **instrumented** manifest: `capture.path` becomes `instrumented`, `capture.sdk_version`
records which SDK produced it, and `rendered_hash` becomes the digest of the text that was actually
sent rather than of a file in the repository. `rendered_sample_hash` is left null, because a live
capture is not a sample.

The capture supplies the rendered prompt and, when it has it, the template. Tools, retrieval and
policy still come from the configuration: they are configuration, and recording them twice would
create two sources of truth for the same fact. Variable *values* are never taken from a capture —
the manifest records types, because it is a file teams commit and share.

With no `--out`, the manifest goes to stdout and a one-line summary goes to stderr,
so `context-ci manifest > context-manifest.json` behaves.

`--commit` defaults to `git rev-parse HEAD`. If git cannot answer, the command
fails rather than inventing a value: a manifest with no commit cannot be
reproduced, which makes it useless as evidence.

### `diff`

```bash
context-ci diff <base.json> <head.json> [--format text|json] [--config path]
```

Needs no configuration file. Comparing two manifests should never require setup, so
severity overrides are optional here.

### `check`

```bash
context-ci check --baseline <base.json> [--head context-manifest.json] \
                 [--config .context-ci.yaml] [--format text|github|json]
```

`--baseline` is required. Without a baseline there is nothing to compare, and a
gate that silently passes because it had nothing to do is the most dangerous
possible default.

### `classes`

```bash
context-ci classes
```

Prints every class with its default severity. Use it when writing a
`severity:` override, because an unknown key is rejected rather than ignored.

---

## 3. Exit codes

Exit codes are part of the contract, so a pipeline can script them.

| Code | Meaning |
|---:|---|
| 0 | Nothing changed, or nothing at or above the threshold |
| 1 | **The gate failed**: at least one change at or above `gate.fail_on` |
| 2 | Usage or configuration error |
| 3 | A manifest could not be read, or is invalid |

A configuration error is 2, never 1, so a pipeline can tell "the code is fine but
the tool is misconfigured" from "the context regressed". That distinction decides
whether a team looks at their prompt or at their YAML.

---

## 4. Configuration

`.context-ci.yaml`, found at `--root` unless `--config` says otherwise.

```yaml
version: 1

capture:
  path: asserted            # asserted | instrumented | observed
  sources:                  # files whose content is hashed
    prompt: prompts/support.md
    tools: config/tools.yaml
    retrieval: config/retrieval.yaml
    guardrails: config/guardrails.yaml   # optional
    rendered: build/rendered.txt         # optional

model:
  provider: openai
  id: gpt-x
  alias_of: gpt-x-latest    # optional
  params:
    temperature: "0"        # keep it quoted: the literal text is hashed
    max_tokens: 1024
    seed: "42"              # optional
    top_p: "1"              # optional

retrieval:
  index_id: kb-prod
  embedding_model: embed-3
  embedding_model_version: "2026-05"
  chunker: recursive-800
  top_k: 5

policy:
  version: "2026.11"

severity:                   # override any class: fail | warn | ignore
  model.id: fail
  retrieval.top_k: warn

gate:
  fail_on: fail             # fail | warn | never  (default: fail)
  calls_per_day: 20000      # for the token-delta projection
  input_usd_per_mtok: 3     # omit to get tokens without a dollar figure
```

### Rules that are enforced, not documented

- **Unknown keys are rejected.** A misspelled `gat:` would otherwise leave a team
  believing a gate exists.
- **`capture.sources.prompt` is required.** A manifest with no prompt hash proves
  nothing.
- **Source paths must stay inside the repository.** `../secrets.env` is refused.
- **`capture.path: instrumented` and `observed` are refused by the CLI** with a
  message naming what produces them. Only the SDK and a recorded sample can, and
  pretending otherwise would put a weaker manifest behind a stronger label.
- **`severity.<class>` is validated against the real class list.**
- **Sampling values are read as literal text.** `temperature: 0` and `0.0` are the
  same setting but different bytes; the literal keeps the digest independent of the
  parser.

### Why `retrieval` maps to `filters_hash`

The manifest schema publishes `retrieval.filters_hash`, and the specification's
example config names the source `retrieval`. So the hash of the retrieval
configuration file lands in `filters_hash`. It is one file and one field, and the
change it signals — the scope of what can be retrieved moved — is the same.

---

## 5. Determinism

The same inputs produce a byte-identical manifest, excluding `created_at`. That is
requirement C-01, and three rules make it true:

1. **Line endings are normalised.** CRLF and LF hash identically, so a Windows
   checkout is equivalent to a Unix one. Without this, half a team's pull requests
   would report a prompt change on every commit.
2. **Keys sort by UTF-16 code unit**, not by byte, and floats are refused.
3. **Nothing derived from iteration order or the environment is recorded.**

`created_at` is recorded but excluded from the digest, because a timestamp is not a
property of the context.

---

## 6. What the manifest does not contain

- **No prompt text**, unless `Content` capture is enabled and recorded separately.
  Only its hash.
- **No credential-shaped field from a tool schema.** `api_key`, `token`,
  `secret`, `password`, `authorization`, `private_key` and similar are removed
  before hashing, so a default value in a schema cannot leak into a file that gets
  committed.
- **No token counts claimed as exact.** `description_tokens` is a whitespace-word
  estimate and is documented as one; a cost projection built on a guess is labelled
  as a projection.

---

## 7. Output formats

| Format | For |
|---|---|
| `text` (default) | A human reading a log |
| `github` | Workflow commands, so the entry appears on the pull request |
| `json` | Scripting and the next tool |
| `sarif` | Code scanning: each change becomes a result on the pull request's file |
| `junit` | A test report, for dashboards that only read XML |

`github` escapes `%`, `\r`, `\n`, `:` and `,` in every value, so a string from a
manifest cannot open a second workflow command.

### `sarif` and `junit`, and why severity is not translated

| Severity | SARIF `level` | JUnit |
|---|---|---|
| `fail` | `error` | `<failure>` |
| `warn` | `warning` | passes, with the warning in `<system-out>` |
| `ignore` | `note` | `<skipped>` |

JUnit has no warning level. The two dishonest options are to call a warning a failure, which
teaches a team to ignore red, and to drop it, which hides the change. So a `warn` entry passes its
test case and carries its message in `<system-out>`; only `fail` becomes `<failure>`. A team that
wants warnings to break the build sets `severity:` in the configuration, where that decision
belongs.

SARIF output is deterministic: the same diff renders byte-identically twice, so it can be committed
or diffed. Every result names a rule that the run declares, because a SARIF consumer drops a result
whose rule is missing.

Both formats were validated against their published schemas on 13 November 2026: SARIF 2.1.0
against the OASIS `sarif-schema-2.1.0.json`, and JUnit against the `junit-10.xsd` the Jenkins xunit
plugin ships, with a diff containing one `fail`, one `warn` and one `ignore` entry. That last part
matters: `errors` is a required attribute on `testsuite` in that schema, so it is written as `0` —
a change is never a report that failed to be produced.

---

### `attribute`

```bash
context-ci attribute --baseline base.json --head context-manifest.json \
                     --baseline-results base-eval.json --results head-eval.json \
                     [--format text|json]
```

Two manifests and two eval runs. It reports what moved, and — when the cases say what they depend
on — which context class the movement points at.

```json
{"version": 1, "suite": "support-quality", "cases": [
  {"id": "q1", "score": 0.92, "depends_on": ["prompt.rendered_hash"]},
  {"id": "q3", "score": 0.90, "depends_on": ["model.id"]}
]}
```

**The distinction the verb exists to keep is attribution versus correlation.**

| The cases declare `depends_on` | What the report says |
|---|---|
| Yes | `attribution`: a regressed case that depends on one of the changed classes implicates it, with a share over all regressed cases |
| No | `correlation`: the scores moved and the configuration moved, and nothing ties them together |

```
prompt.rendered_hash      3 of 3 regressed cases (100%)  the rendered prompt changed
```

The same numbers with the declarations removed:

```
3 of 4 cases regressed, but no case declares which context classes it depends on, so this is a
correlation: the scores moved and the configuration moved, and nothing here ties them together.
```

Three rules worth knowing:

- **A movement smaller than 0.001 is not a movement.** Model scores do not repeat exactly, and a
  report that calls a rounding wobble a regression trains its reader to ignore it.
- **A case in one run and not the other is incomparable, not unchanged.** A vanished case is a
  result, and dropping it would quietly shrink the denominator.
- **A class that changed with no evidence against it is printed with zero**, not omitted. Silence
  would read as innocence.

`attribute` never exits non-zero for a regression: gating is `check`'s job, and a verb that
reports and fails at once cannot be used to explain a build that already failed.

## 8. Not yet implemented

| Gap | Why it is a gap | Planned |
|---|---|---|
| CI-provider verification | Every snippet's commands are exercised locally, verbatim; none has run on a hosted runner, and no outsider can install at all while the repository is private. Measured and dated in [spec.md](spec.md) §7, with a launch-day item in the [launch plan](../04-plan/30-day-launch.md) | Launch day |
| Prompt registries as a source | Spec §12 question 2 | After v0.1 |
