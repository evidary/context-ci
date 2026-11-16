# CLI reference

**Version:** 1.0.0 · **Date:** 16 November 2026 · **Owner:** Context CI squad

The four verbs in [the specification](spec.md) §1. All four are implemented, and this page is the
contract for them. The rest of this page also covers the verbs that serve them — `init`,
`classes`, `module-manifest`, `serve` and `version` — none of which changes what a gate decides.

---

## 1. Install and first run

```bash
context-ci init                                     # writes a commented .context-ci.yaml, refusing to overwrite one
context-ci manifest --out context-manifest.json     # the baseline: commit it
context-ci check --baseline context-manifest.json   # the baseline against the working tree, right now
```

That is the whole quickstart. No account, no network call, no secret. `init` is optional: a
configuration written by hand, as in §4, is read exactly the same way. [`examples/minimal`](../../examples/minimal)
is a project in this shape, with its manifest committed, and `tools/smoke_quickstart.sh` runs these
commands end to end in CI.

Three things a first run meets, each reported as one line that says what to do:

| Situation | What the tool says | Exit |
|---|---|---:|
| `init` found no prompt file | names `prompts/support.md` as the path it wrote and the file to create | 0 |
| `manifest` or `check`, and a configured source file is missing | `capture.sources.<kind> names <path>, which does not exist under <root>: create that file, or point capture.sources.<kind> at the file that holds it` | 3 |
| `manifest` or `check` with no commit to record — not a git repository, no commits yet, or no git | says which of the three it is, and `run this inside a git repository that has at least one commit, or pass --commit <id>` | 2 |

The missing file is reported first, before git is asked for anything.

**The manifest is declared, not observed.** `manifest` hashes the files named under `capture.sources`
and copies `model`, `retrieval` and `policy` from the configuration. It does not inspect a running
application, so the model id, the parameters, the retrieval settings and the policy version in a
manifest are what the configuration asserts. `--capture` replaces one field — the rendered prompt
hash — with the hash of text an SDK recorded at request-assembly time; see [`manifest`](#manifest).

In a pull request:

```yaml
- uses: actions/checkout@v4
  with: { fetch-depth: 0 }      # the baseline is read from the base branch
- uses: actions/setup-go@v5
  with: { go-version: "1.27" }
- name: Baseline from the base branch
  env:
    BASE_REF: ${{ github.base_ref }}
  run: git show "origin/${BASE_REF}:context-manifest.json" > "${RUNNER_TEMP}/context-manifest.base.json"
- uses: evidary/context-ci@v0.8.0
  with:
    baseline: ${{ runner.temp }}/context-manifest.base.json
    format: github      # annotations on the pull request
```

`action.yml` is at the repository root, so the action is `evidary/context-ci@<ref>`; there is no
`action/` subdirectory. The action builds the binary from its own checkout, so what runs is the
code the ref names and no release asset has to exist. It uses the Go toolchain on the runner and
installs the version its own `go.mod` names when the runner has none; the `setup-go` step above
pins the version instead of leaving it to the runner image.

The action passes no head manifest to `check` unless the `head` input is set, so the head is built
from the pull request's checkout on every run. `check` without a baseline fails rather than passing
on an empty comparison, and a baseline that is also named as the head is refused.

## 2. The verbs

| Verb | Command | What it does |
|---|---|---|
| Scaffold | `context-ci init` | Writes a commented `.context-ci.yaml` for the current checkout |
| Capture | `context-ci manifest` | Writes the manifest for the current checkout — to stdout, or to `--out` |
| Compare | `context-ci diff <base> <head>` | Semantic diff between two manifests |
| Gate | `context-ci check` | Diffs a baseline against the working tree and exits non-zero above the threshold |
| Explain | `context-ci classes` | Lists every diff class and its default severity |
| Attribute | `context-ci attribute` | Lists the changed context classes a regression is consistent with, when the regressed cases declare what they depend on |
| Discover | `context-ci module-manifest` | The platform's `module-manifest.v1.json`. Deliberately **not** `manifest`, which already means the tool manifest above |
| Probe | `context-ci serve` | `GET /healthz` and `GET /readyz`, so the manifest's `health_path` is real |
| Version | `context-ci version` | Prints the tool version, including the module version the toolchain stamped into a `go install`ed binary |

`attribute` was the last of the four specification verbs to land, because it needed the eval
results format settled first; `init` came after it. It still refuses to guess: when no regressed
case declares `depends_on` it reports a correlation and says so. See [`attribute`](#attribute).

**Flags may be written before or after a verb's arguments.** `context-ci diff a.json b.json --format
sarif` and `context-ci diff --format sarif a.json b.json` are the same command, and a bare `--` ends
the flags for a file whose name starts with a dash. `context-ci <verb> -h` prints that verb's flags
on stdout and exits 0.

### `init`

```bash
context-ci init [--root .] [--config path] [--force]
```

Writes a commented `.context-ci.yaml` for the checkout, then says on stderr what still has to be
changed before the file is true. It exists because the alternative is copying the example in §4 by
hand, and a configuration typo is a gate that looks on while being off.

| Flag | Default | Behaviour |
|---|---|---|
| `--root` | `.` | The checkout the file describes; the same directory `manifest`, `check` and `attribute` read the file from |
| `--config` | `.context-ci.yaml` under `--root` | Where to write it. A directory that does not exist is an error, not something to create |
| `--force` | off | Replace a file that already exists. Without it `init` refuses and changes nothing |

What it writes, and what it refuses to invent:

- **A source is named only when the checkout has the file.** Each kind is probed at a short fixed
  list, and a kind with no file is written as a comment rather than as a path nobody created.
  The lists are these, in preference order:

  | Source | Probed |
  |---|---|
  | `prompt` (required) | `prompts/support.md`, `prompts/system.md`, `prompts/system-prompt.md`, `prompts/prompt.md`, `prompt.md`, `PROMPT.md` |
  | `tools` | `config/tools.yaml`, `config/tools.yml`, `tools.yaml`, `tools.yml` |
  | `retrieval` | `config/retrieval.yaml`, `config/retrieval.yml`, `retrieval.yaml`, `retrieval.yml` |
  | `guardrails` | `config/guardrails.yaml`, `config/guardrails.yml`, `guardrails.yaml`, `guardrails.yml` |
  | `rendered` | `build/rendered.txt`, `build/rendered.md` |

  The prompt is the exception, because the configuration cannot omit it: `prompts/support.md` is
  written when nothing is found, and both the file and stderr say it does not exist yet.
- **`model.provider` and `model.id` are placeholders** — `openai` and `gpt-x` — and the file and
  stderr both say so. Nothing in a repository says which model a team calls, so a guess presented
  as a fact would gate on a model nobody uses. `retrieval` and `policy` are commented out entirely
  for the same reason: no file this tool may read holds their values.
- **The file is parsed before it is written**, by the same strict loader `check` uses, so `init`
  cannot produce a configuration the next command rejects. `cmd/context-ci/init_test.go` runs
  `manifest` and `check` against the generated file rather than trusting that claim.
- **The gate is written explicitly** as `gate.fail_on: fail`, which is also the default, so the
  protective setting is visible in the file a team reviews.

It stats a handful of candidate paths and writes one file. It makes no network call, needs no
account and no secret, and does not run `git`.

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

`--commit` defaults to git's `HEAD`. If git cannot answer, the command fails with exit 2 rather
than inventing a value: a manifest with no commit cannot be reproduced, which makes it useless as
evidence. The message says which case it is — the directory is not a git repository, the
repository has no commits yet, or git is not installed — and that `--commit <id>` supplies the
value by hand. Every configured source is checked first, so a missing prompt file is reported
before the commit is.

What the manifest holds is what the configuration declares. The hashes are computed from the files
named under `capture.sources`; `model.provider`, `model.id`, `model.params`, `retrieval.*` and
`policy.version` are copied from `.context-ci.yaml` unchanged. Nothing here contacts a provider or
reads a request, so a manifest cannot show that the application uses the model it names.

### `diff`

```bash
context-ci diff <base.json> <head.json> [--format text|github|json|sarif|junit] [--config path]
```

Needs no configuration file. Comparing two manifests should never require setup, so
severity overrides are optional here. The flags work in either position.

Both files are read strictly. A manifest is rejected with exit 3 when its digest does not match
its content, when it holds a field the schema does not define — an unknown field is outside the
digest, so it would be content nothing verified — or when a restricted value in
[spec.md](spec.md) §4 is outside what is allowed: a `model.provider` that is not `openai`,
`anthropic`, `google` or `other`, a sampling value that is not a number, a negative `top_k`,
`max_tokens` or `description_tokens`, or a hash that is not 64 lowercase hex characters. `check`
and `attribute` read manifests the same way.

### `check`

```bash
context-ci check --baseline <base.json> [--root .] [--commit <sha>] [--head <head.json>] \
                 [--config .context-ci.yaml] [--format text|github|json|sarif|junit]
```

`--baseline` is required. Without a baseline there is nothing to compare, and a
gate that silently passes because it had nothing to do is the most dangerous
possible default.

**The head is built from the working tree.** With no `--head`, `check` builds a manifest from the
checkout at `--root` — the same one `manifest` would write, in memory — and compares the baseline
with that. A line on stderr says so and gives its digest; stdout is only the report. The commit it
records is git's `HEAD`, or `--commit`, exactly as for `manifest`.

| Flag | Default | Behaviour |
|---|---|---|
| `--baseline` | none, required | The manifest to compare against: a committed file, or one read from the base branch |
| `--head` | built from the working tree | Read this manifest instead of building one. For a head produced elsewhere, such as an instrumented manifest written with `manifest --capture` |
| `--commit` | git `HEAD` | The commit recorded in a head `check` builds. Combining it with `--head` is a usage error, because that manifest already records one |
| `--root` | `.` | The checkout the head is built from, and where `.context-ci.yaml` is found |

**One file as both sides is a usage error.** `check --baseline m.json --head m.json` exits 2,
whichever way the two paths are spelled: a file compared with itself can only report that nothing
changed. `check` used to default `--head` to `context-manifest.json`, which is also the file the
quickstart named as the baseline, so the documented gate printed "no change" and exited 0 after
any edit.

**What fails the build.** `check` exits 1 for a change at or above `gate.fail_on`, which defaults
to `fail`. A prompt edit is `warn` by default, so with the default gate it is reported and the exit
code is 0; set `gate.fail_on: warn`, or `severity: {prompt.template_hash: fail}`, to make it block.
When the configuration names no `rendered` source the rendered hash is the template hash, and the
edit is reported once, as `prompt.template_hash`.

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
| 0 | Nothing changed, or nothing at or above the threshold. Also `-h`: asking for help is not an error |
| 1 | **The gate failed**: at least one change at or above `gate.fail_on` |
| 2 | Usage or configuration error, including `check` given one file as both baseline and head, and no commit to record |
| 3 | A manifest could not be read, or is invalid. `module-manifest` also uses it for a build with no released identity, and `serve` for a server that failed after it started: neither is a usage error, and both mean the document the tool was asked for could not be produced |

A configuration error is 2, never 1, so a pipeline can tell "the code is fine but
the tool is misconfigured" from "the context regressed". That distinction decides
whether a team looks at their prompt or at their YAML.

Every way `init` fails is 2 — refusing to replace a file without `--force`, a destination it cannot
write, a flag it does not know — because nothing was compared and nothing regressed.

---

## 4. Configuration

`.context-ci.yaml`, found at `--root` unless `--config` says otherwise. `context-ci init` writes a
commented file in this shape; the values it writes for `model` are placeholders, and the `retrieval`
and `policy` blocks it writes are commented out until someone knows the real values.

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
- **`model.provider` is required and is one of `openai`, `anthropic`, `google`, `other`.** The
  set is the one [spec.md](spec.md) §4 lists; `OpenAI` and `openai` would otherwise be two
  providers and a `model.provider` change between them.
- **`retrieval.top_k` and `model.params.max_tokens` cannot be negative**, and a sampling value
  must be a number: `temperature` and `top_p` a decimal, `seed` an integer.
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

- **No observation of the running application.** Every value is a hash of a file in the
  repository or a value copied from `.context-ci.yaml`. The one exception is `prompt.rendered_hash`
  in a manifest written with `--capture`, which is the hash of text an SDK recorded. A manifest
  therefore says what a team declared, and `capture.path` says which of the two it is.
- **No prompt text.** The manifest schema carries an optional `content` field for a copy a
  team opted into, and it is checked against its hash on load, but no verb writes one: the
  prompt reaches the manifest as a hash and nothing else.
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
| `sarif` | A SARIF 2.1.0 report: one rule per changed class and one result per change. The results carry no `locations` — see below |
| `junit` | A test report, for dashboards that only read XML |

`github` escapes `%`, `\r` and `\n` in every message, and additionally `:` and `,` in the
title property, so a string from a manifest cannot open a second workflow command. The two
sets differ on purpose: GitHub unescapes only the first three in a message, so escaping a
colon there would show the escape to the reader.

`diff` and `check` write one annotation per changed class and carry `severity` as the level.
`attribute` has no file to point at, so it writes the findings the report can name — see
[`attribute --format github`](#attribute---format-github) for what each level means there.

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

**A SARIF result has no location.** A manifest records hashes and declared values, not the file
and line that produced them, so a result carries a rule, a level, a message and the before and
after values, and no `locations`. A viewer that places findings on a file has nowhere to put them;
this page used to say each change "becomes a result on the pull request's file", and it does not.
For something visible on the pull request use `--format github`, whose annotations are attached to
the run rather than to a line.

**A diff with no change is a complete report.** `rules` and `results` are empty arrays, and the
JUnit suite is an empty `<testsuite tests="0" ...>`. Until 16 November 2026 the SARIF for an
unchanged context carried `"rules": null, "results": null`, which the schema rejects.

Both formats were validated against their published schemas on 13 November 2026: SARIF 2.1.0
against the OASIS `sarif-schema-2.1.0.json`, and JUnit against the `junit-10.xsd` the Jenkins xunit
plugin ships, with a diff containing one `fail`, one `warn` and one `ignore` entry. That last part
matters: `errors` is a required attribute on `testsuite` in that schema, so it is written as `0` —
a change is never a report that failed to be produced. That validation did not include a diff with
no entries, which is how the `null` above went unnoticed. On 16 November 2026 the SARIF for an
unchanged context and for a two-change diff were validated against the OASIS
`sarif-schema-2.1.0.json` with Python `jsonschema`: both pass, and the unchanged report with `null`
in place of its two arrays fails on exactly `rules` and `results`. JUnit was not re-validated
against the XSD; its empty suite is pinned by a test.

---

### `attribute`

```bash
context-ci attribute --baseline base.json --head context-manifest.json \
                     --baseline-results base-eval.json --results head-eval.json \
                     [--format text|github|json]
```

Two manifests and two eval runs. It reports what moved, and — when the regressed cases say what
they depend on — which changed context classes that movement is consistent with. **It lists
candidate causes. It does not establish a cause.** A `depends_on` declaration is the suite
author's statement about a case, not a measurement, and two eval runs cannot separate two changes
made together.

```json
{"version": 1, "suite": "support-quality", "cases": [
  {"id": "q1", "score": 0.92, "depends_on": ["prompt.template_hash"]},
  {"id": "q3", "score": 0.90, "depends_on": ["model.id"]}
]}
```

A results file is read strictly, and refused with exit 3 when it would be misread: a `version`
other than `1`, an unknown field, a case with no `id` or a repeated one, a `score` outside 0..1,
or a `depends_on` entry that is not a context class — `context-ci classes` prints the list. A
misspelled class could never match a change, so it would silently remove the case from every
attribution. Only `score` and `passed` are compared; `cost_usd` is accepted and not used, so
nothing here attributes cost.

**The distinction the verb exists to keep is attribution versus correlation.**

| The regressed cases declare `depends_on` | What the report says |
|---|---|
| At least one does | `attribution`: a regressed case that depends on one of the changed classes makes that class a candidate, with a share over all regressed cases |
| None does | `correlation`: the scores moved and the configuration moved, and nothing ties them together |

The mode is decided by the cases that regressed. A declaration on a case that did not regress is
not evidence about the ones that did.

```
support-quality: 3 compared, 3 regressed, 0 improved, 0 unchanged, 0 incomparable
3 of 3 cases regressed and every regressed case declares its dependencies; 3 of them declare a
dependency on a class that changed, so the classes below are candidate causes consistent with those
declarations, not proven causes.

changed classes: model.id, prompt.template_hash
2 context classes changed together, so their effects are confounded: this report is not causal, and
it cannot say which of those changes, if any, caused a movement in the scores.

  prompt.template_hash     3 of 3 regressed cases (100%)  the prompt template changed
  model.id                 0 of 3 regressed cases (0%)  the model changed
```

(Each sentence is one line in the output; it is wrapped here.) The second paragraph is requirement
C-08: it is printed whenever more than one class changed, in every mode, and the JSON report
carries the same fact as `"confounded": true`.

The same numbers with the declarations removed:

```
3 of 3 cases regressed, but no regressed case declares which context classes it depends on, so this
is a correlation: the scores moved and the configuration moved, and nothing here ties them together
beyond that.
```

Every sentence is computed from the counts it states. The other things the first line can say:

| Situation | Sentence |
|---|---|
| Some regressed cases declare and some do not | `2 of the 3 regressed cases declare dependencies` in place of `every regressed case declares its dependencies` |
| The regressed cases declare, but not a class that changed | `none of them declares a dependency on a class that changed, so no changed class is a candidate cause on this evidence` |
| Cases regressed and no class changed | `no context class changed between the two manifests, so nothing in the context is a candidate cause` |
| Nothing regressed and something improved | `No case regressed by more than 0.001 (2 improved), so there is no regression to attribute.` |
| Nothing moved | `No case moved by more than 0.001, so there is nothing to attribute.` |

Three rules worth knowing:

- **A movement smaller than 0.001 is not a movement.** Model scores do not repeat exactly, and a
  report that calls a rounding wobble a regression trains its reader to ignore it.
- **A case in one run and not the other is incomparable, not unchanged.** A vanished case is a
  result, and dropping it would quietly shrink the denominator.
- **A class that changed with no evidence against it is printed with zero**, not omitted. Silence
  would read as innocence.

One thing that surprises: with no `rendered` source in the configuration, a prompt edit is the
class `prompt.template_hash`, and `prompt.rendered_hash` is reported as unchanged. A case that
should follow prompt edits in that setup declares `prompt.template_hash`.

`attribute` never exits non-zero for a regression: gating is `check`'s job, and a verb that
reports and fails at once cannot be used to explain a build that already failed.

#### `attribute --format github`

The same report as workflow annotations, so a regression appears on the pull request instead of
only in the job log:

```
::notice title=context-ci attribute::support-quality: attribution: 2 compared, 1 regressed, 0 improved, 1 unchanged, 0 incomparable
::error title=context-ci attribute model.id::1 of 1 regressed cases (100%25) declare it: the model changed
::warning title=context-ci attribute case q1::regressed by more than 0.001
```

When more than one class changed, the summary notice ends with the confounding statement — `... 0
incomparable. 2 context classes changed together, so their effects are confounded: ...` — so the
first annotation a reader sees qualifies the ones below it.

| Annotation | Level | Why that level |
|---|---|---|
| The summary: suite, mode and the five counts | `notice` | It is the qualifier, not a finding, and it must be readable before the rest |
| A class that a regressed case declares a dependency on | `error` | A declaration ties a regression to this change: a candidate cause, not a proven one |
| A regressed case | `warning` | The score moved; the case name is what a reader needs to go and look |
| A changed class that nothing points at | `notice` | It changed. Printing it with zero beats silence that reads as innocence |

The level states how strong the claim is, not whether the build failed: `attribute` exits 0
whatever it finds, so an `error` annotation here never fails a job. `100%25` is how `100%` is
written on the wire; GitHub unescapes it back to `100%` when it renders the message.

**No annotation carries `file=` or `line=`.** An attribution report names eval cases and context
classes, and neither records where it lives. An invented location would send a reader to an
unrelated line, which is worse than no location at all — so the annotation says what it knows and
nothing else.

In a workflow, run the verb as its own step rather than capturing its output, because annotations
are read from the step's log:

```yaml
- uses: evidary/context-ci@v0.8.0
  with:
    command: attribute
    baseline: context-manifest.base.json
    head: context-manifest.json
    baseline-results: evals/base.json
    results: evals/head.json
    format: github
```

GitHub shows at most 10 warning and 10 error annotations per step (and 20 notices) on the pull
request; anything beyond that stays in the log. The summary line always carries the counts, so a
truncated list cannot make a report look empty.

## 8. Not yet implemented

| Gap | Why it is a gap | Planned |
|---|---|---|
| CI-provider verification | Every snippet's commands are exercised locally; none has run on a hosted runner, and no outsider can install at all while the repository is private. That includes this repository's own `.github/workflows/ci.yml`, whose Linux, macOS and Windows jobs have not yet run. Measured and dated in [spec.md](spec.md) §7, with a launch-day item in the [launch plan](../04-plan/30-day-launch.md) | Launch day |
| Observed capture of anything but the prompt | The manifest's model, parameters, retrieval settings and policy version are declared in `.context-ci.yaml`. No code reads them from a running application, so a change made outside the repository is not seen | Not scheduled |
| Cost attribution | `attribute` compares `score` and `passed`. An eval case's `cost_usd` is accepted and unused, and no cost event is emitted | Not scheduled |
| Prompts from a registry | **Undecided, so not built.** [spec.md](spec.md) §12 question 2 asks whether registries are a source at v0.1 or only files, and the repository records no answer. What a source would need is listed below, and requirement C-02 forbids the network call it would make | After v0.1 |

### Why prompts from a registry are still deferred

This row used to say "Spec §12 question 2" and nothing else, which named the open question and not
the reason. The reason is that the specification does not define the feature, and cannot without
changing a requirement it states twice.

- **The definition is an open question.** [spec.md](spec.md) §12 question 2 is "Do we support prompt
  registries (Langfuse, LangSmith) as a source at v0.1, or only files?" It has no recorded answer in
  any document here. §2 states the intent — "Registries are the thing we read from, not the thing we
  become" — and §3 defines the asserted capture path as files in the repository. Neither says what a
  registry source *is*, so implementing one would mean inventing the contract this page exists to
  record.
- **Six things would have to be specified, and none are.** The configuration key, given that the
  validator accepts exactly `prompt`, `rendered`, `tools`, `retrieval` and `guardrails` and rejects
  every other key; which registry, and which of its API versions; how one version is selected (label,
  tag, numeric version, or "latest"); whether the template, the rendered text, or both are fetched;
  where credentials come from, given that C-10 requires the tool to work with no account and no
  network; and what happens when the registry is unreachable during a check.
- **Requirement C-02 forbids the network call it would make.** [spec.md](spec.md) §9 C-02 is "no
  outbound network call in any mode", and §7 repeats "Network: None". A registry source fetches a
  prompt over HTTP by definition, so shipping one is an amendment to the requirement, which is a
  specification decision and not an implementation detail.
- **The digest story is unspecified.** A fetched prompt has no file whose bytes the digest covers.
  Whether the manifest records the registry's version id, the hash of the fetched text, or both,
  decides whether a baseline committed to the repository can still be verified and diffed with no
  network — and it must be, because `check` reads that baseline offline.

So no registry code exists and no configuration key accepts one. Before this row can be removed, §12
question 2 needs an answer, and the answer needs to settle each point above, in particular how a
fetched prompt is pinned so `diff` and `check` stay offline. Adding `init` did not make that decision,
and `init` writes only file sources for exactly this reason.
