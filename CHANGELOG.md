# Changelog

All notable changes to Context CI. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

**Renamed to Sigilary.** The company changed name on 21 November 2026; the earlier name belongs to
someone else. This document uses the new name throughout, including in entries written before the
change. What a user has to change:

- the action is `sigilary/context-ci@<tag>`, and the Go module is `github.com/sigilary/context-ci`;
- the TypeScript SDK is `@sigilary/context-ci`. It was not published under the old name.

The platform repository this tool can optionally report to is now called `ledger`, and its contract set
was republished as 1.2.1 with every schema `$id` moved under that name. The two contracts vendored here
are re-vendored at 1.2.1; they differ from the earlier copies in their `$id` and nothing else.

## [0.9.0] — 2026-11-16

**The check no longer needs anything committed, and it says what changed.** A review of 0.8 against
how the tool would be used on a real team found that the committed manifest it depended on could
not survive two people: every pair of branches that regenerated it conflicted, a change merged
without regenerating it was blamed on the next pull request, and a comment in a retrieval file
failed the gate. This release replaces that workflow. Nothing here has run on a hosted runner.

**Breaking:** the manifest is `manifest_version: 2` and version 1 files are rejected; digests
differ from 0.8 for the same inputs; the retrieval and guardrails hashes are of parsed values;
tool schema hashes changed; a missing `.context-ci.yaml` is no longer an error;
`examples/minimal/context-manifest.json` is gone; the action's `baseline` input is no longer
required and its `version` input now prefers a release binary.

### Added

- **`check --base-ref <ref>`.** The baseline is built from the merge base of `<ref>` and `HEAD`,
  read through git with the configuration that commit had, and compared with the working tree.
  No manifest is committed. `--baseline <file>` still works, and exactly one of the two is
  required.
- **Where and what.** Each entry about a file carries its path and first changed line; `check`
  prints the changed lines from `git diff` for prompt, tool, instruction and group files, at most
  60 per file; `--no-text` leaves them out. GitHub annotations carry `file=` and `line=`, SARIF
  results carry `locations`, and `--summary <file>` appends a Markdown report.
- **`check --acknowledge <classes|all>`.** An accepted change is reported as acknowledged and does
  not fail the gate. `all` does not cover `hidden_characters`.
- **File groups.** `capture.files` maps a name to glob patterns; each group is a class
  `files.<group>`, and each file is reported by path.
- **Agent files, with no configuration.** `AGENTS.md`, `CLAUDE.md`, rule and skill directories are
  `agent.instructions` (warn); MCP, hook and settings files are `agent.config` (fail). A
  repository with no `.context-ci.yaml` is checked with these defaults.
- **`hidden_characters`** (fail): a new or changed prompt, tool description or file that contains
  invisible or direction-changing Unicode characters, with the code point and line of the first.
- **Tool definitions in common shapes**: chat-completions function tools, the flattened form,
  `input_schema`, function declarations, and a saved MCP `tools/list` response, in YAML or JSON.
  Written from the documented formats; not checked against vendor exports.
- **The action** takes `base-ref` (defaulting to the pull request's base branch), `acknowledge`,
  `summary` and `text`; downloads the release binary and checks it against `SHA256SUMS` when it is
  named with a release tag, building from source otherwise; and writes the job summary.

### Changed

- **The digest covers the context only.** `commit` joins `created_at` outside it, so the same
  files have the same digest on every commit. `model` and `prompt` are optional in a manifest and
  in the configuration.
- **Retrieval and guardrails files are parsed before they are hashed.** A comment, key order or
  formatting is not a change. A file that is not YAML or JSON is hashed as text.
- **Every field of a tool definition is hashed**, not only `schema`; the description is still
  hashed alone. A changed definition says "the definition of tool X changed".
- **Credential values are replaced, not removed with their keys.** A tool that gains a `token`
  parameter has changed; before, the whole key was dropped and the change was invisible.
- **`init` leaves the prompt line commented out when it finds no prompt**, so the file it writes
  works as written.
- **The cost projection is attached to `tools.description`** when a description grows, where it
  can be reached; it was on a class that current manifests never emit. It is labelled as an
  estimate from a word count.
- **`attribute` is marked experimental**, counts changed classes instead of entries when it says
  changes are confounded, and refuses one file as both manifests.
- The README no longer leads with changes this tool cannot see, and states what it does not catch.

### Removed

- `examples/minimal/context-manifest.json`, and the instruction to commit and regenerate a
  manifest.

### Fixed

- **A test that failed on Linux and macOS since 0.8.0.** `TestInitReportsAnUnwritableDestination`
  required "does not exist" for a path below a regular file, which is what Windows reports; the
  others report "not a directory". `init` was correct. The failure kept the 0.8.0 release from
  publishing, so 0.8.0 has a tag and no binaries.

- **A symbolic link was reported as an added file on Windows.** Git checks a link out as a
  small text file where the file system has no links, so the checkout listed it and the commit
  did not. Found by running against a public repository that links one skills directory to
  another: 14 of its 17 reported changes were this. Links are now left out of both sides, on
  every operating system.

### Verified, and not

- `ci.yml` is green on GitHub-hosted Linux, macOS and Windows runners, including `action.yml` on
  all three with annotations on the changed line. The three report the same digest for the example.
- The action's download step was extracted from `action.yml` and run on Windows and on Linux
  against published releases: a release named by `version` or by the action's ref installs a
  binary that matches `SHA256SUMS`; a branch, a commit, a tag with no assets and a tag that does
  not exist fall back to building; a binary that does not match, or has no entry, stops the step.
  It has not run inside a workflow, because no release of this version exists yet.
- The GitLab CI and CircleCI commands were run in `golang:1.27-bookworm` and `cimg/go:1.27`
  against a single-branch clone whose base branch had moved on: install from the public module,
  the explicit fetch, then `check --base-ref` reporting the branch's own two changes and exit 1,
  with a SARIF file written. Not on those providers' runners.
- SARIF with locations validates against the OASIS 2.1.0 schema.
- The defaults were run over the last 100 commits of three public repositories that hold agent
  files: 0, 3 and 24 changes reported, each a real edit to an instruction, skill or settings
  file, no hidden-character reports, and under five seconds on a 4,400-file repository. Three
  repositories is a first look, not a false-positive rate.
- Not exercised: the action's default base branch on a real pull request, and the rendered job
  summary.


## [0.8.0] — 2026-11-16

**The gate could not fail, and now it can.** An audit on 16 November 2026 ran the built binary
against the README and found that the documented quickstart compared a manifest with itself. This
entry is that defect and the ten found beside it. None of it is in the `v0.7.1` tag, which should not
be used: this release carries the fixes, and the documents pin it. `tools/check_docs.py` now fails
when a pin and `VERSION` disagree.

**Breaking:** `check` builds its head from the working tree unless `--head` names one, and refuses one
file as both sides; a manifest or configuration that used to be accepted loosely may now be refused.

### Fixed

- **`check` compared the baseline with itself.** The README set `baseline: context-manifest.json`,
  `--head` defaulted to the same file, and the action wrote a head only when none existed — so an
  edited prompt printed "no change" and exited 0. `check` now builds its head from the working tree
  unless `--head` names one, takes `--commit` for that head the way `manifest` does, and refuses one
  file as both sides with exit 2, however the two paths are spelled. The action passes no head to
  `check` and its `head` input defaults to empty. `TestCheckGatesAnEditMadeAfterTheBaseline` writes a
  baseline, edits the prompt and requires exit 1.
- **`uses: sigilary/context-ci/action@v0.6.0` could not resolve.** `action.yml` is at the repository
  root and there has never been an `action/` directory. The documents now say
  `sigilary/context-ci@v0.8.0`, decision D-05 records the correction, and the README's workflow is a
  whole one: a checkout with history, a Go toolchain, the baseline read from the base branch, then
  the action. The action installs Go itself when the runner has none.
- **A flag after a positional was not parsed.** `context-ci diff a.json b.json --format sarif` — the
  order the CLI reference documents — failed with "need a base and a head manifest", because the
  flag package stops at the first positional. Every verb now accepts its flags in either position.
- **SARIF for an unchanged context was not SARIF.** `rules` and `results` were `null`, which the
  schema rejects, on the one report a green build always writes. They are empty arrays. The CLI
  reference claimed each change "becomes a result on the pull request's file"; a result has no
  location, and the page now says so.
- **`attribute` printed sentences that were not true.** "Every one of them declared dependencies"
  appeared when no regressed case declared any, because the mode was decided by whether *any* case
  declared one; "no case moved" appeared when cases had improved. Every sentence is now computed
  from the counts it states, the mode is decided by the cases that regressed, and the wording is
  "candidate causes consistent with those declarations, not proven causes" — the specification's §6
  rule 3. When more than one class changed the report says they are confounded (requirement C-08),
  in the text, in the summary annotation and as `"confounded": true`.
- **`attribute` accepted results it would misread**: `"version": 99`, and a `depends_on` naming a
  class that does not exist, which could never match a change. Both are refused with exit 3.
- **A manifest could carry content nothing verified.** An unknown field was dropped before the
  digest was recomputed, so a file with an injected key reported "no change" and exit 0. Manifests
  are now decoded strictly, and the restricted values in the specification's §4 are enforced:
  `model.provider` is one of four, sampling values are numbers, and `top_k`, `max_tokens` and
  `description_tokens` are not negative. The configuration refuses the same values, so
  `model.provider` is now required there.
- **`-h` on a verb exited 2.** Asking for help is not a usage error: it prints the verb's flags on
  stdout and exits 0.
- **A first run failed three times without saying what to do.** In an empty directory `manifest`
  reported git's own "not a git repository", then "unknown revision", then a path error from the
  operating system. A missing source is now reported first, by its configuration key; a missing
  commit says whether there is no repository, no commit or no git, and that `--commit` supplies one.
- **The module manifest advertised `cost.attribute`.** Nothing attributes cost: `attribute` reads
  `score` and `passed`, and `cost_usd` is accepted and unused. The entitlement is removed, and the
  comments there that named `review-router serve` name this binary.
- **This file had the `[0.7.1]` entry five times**, once above each older release. One remains.

### Changed

- **The documents say the manifest is declared.** The README described a record of the context a
  feature "ran with". At this version a manifest is hashes of files in the repository and the model,
  parameters, retrieval settings and policy version written in `.context-ci.yaml`; only the rendered
  prompt can be observed, through an SDK capture. README, specification and CLI reference now say
  that, and say what it means: a change made outside the repository is not seen.
- The README and the document index said 0.7.0 while `VERSION` said 0.7.1.
- The specification's GitLab and CircleCI snippets pinned `v0.4.2` and a Go 1.23 image for a module
  whose `go.mod` requires 1.27. They pin this release and Go 1.27, and the page says these two edits
  have not been re-run.
- A stray code fence in the specification's §4 rendered everything from "Rules" to §7 as code.

### Added

- **CI builds and tests the code.** `.github/workflows/ci.yml` ran two documentation checks and
  nothing else. It now runs `gofmt`, `go vet`, `go build` and `go test` on Linux, Windows and macOS
  with the Go version `go.mod` names, runs the README quickstart end to end
  (`tools/smoke_quickstart.sh`: a baseline, an edited prompt, exit 1), and runs `action.yml` from
  the checkout against the example project.
- **`examples/minimal`** — one prompt, one configuration and the manifest they produce, which the
  README's quick start runs. A test fails when the committed manifest stops matching the
  files beside it.

### Removed

- The market research, the two plans, the operations page and the thesis. They were internal planning
  documents, dated before the tool was built, and no longer described it. The specification, the CLI
  reference and the roadmap are the documents that describe the tool.

### Not done

None of the workflow has run on a GitHub-hosted runner: not the new CI jobs, not the action's
`setup-go` fallback, not the README's workflow. Everything a shell can run locally was run locally
on this date — the tests, the quickstart script, and the action's own steps extracted from
`action.yml`. The gap stays recorded in [docs/03-product/spec.md](docs/03-product/spec.md) §7.

## [0.7.1] — 2026-11-16

**Documentation accuracy pass.** Every "planned", "not implemented" and "does not exist" claim
was checked against the tree; every verb, flag and exit code in the docs against the
dispatcher; every number re-derived or de-quantified; and every code comment read for the
behaviour it describes. Code defects the audit found were fixed rather than documented.

## [0.7.0] — 2026-11-15

### Added

- **`context-ci init`** — writes a commented `.context-ci.yaml` for the current checkout. Both
  `docs/04-plan/30-day-launch.md` (since removed) and
  `docs/04-plan/module-plan.md` (since removed) had it in scope while a team wrote the
  file by hand, and a hand-copied configuration is one typo away from a gate that looks on while
  being off. The file it writes is accepted by `check` without an edit, which
  `cmd/context-ci/init_test.go` proves by running `manifest` and then `check` against it.
- **`init` discovers the sources the checkout has.** It probes a short fixed list per source kind —
  `prompts/support.md` and five related prompt paths, `config/tools.yaml` and its variants, and the
  same pattern for retrieval, guardrails and a rendered prompt — and writes a kind it did not find as
  a comment instead of as a path nobody created. The prompt is the exception, because the validator
  requires it: the documented default is written and both the file and stderr say it does not exist
  yet.
- **`init` refuses to replace an existing file without `--force`**, and `--root` and `--config` say
  where to write. Every flag is optional; the default location is the one every other verb reads.
- **The generated file is parsed by the same strict loader `check` uses before it is written**, so
  `init` cannot produce a configuration the next command rejects. `model.provider` and `model.id` are
  written as placeholders and labelled as such, and `retrieval`/`policy` stay commented until their
  values are known: a value invented by a scaffolder would be recorded as configuration the team never
  chose.
- Tests cover the failure paths as well as the happy one: an existing malformed file is left untouched,
  `--force` replaces it with a valid one, a destination that cannot be written exits 2, a directory
  named like a source is not treated as a source, and an empty checkout still produces a file that
  `check` accepts once the prompt path it named has been created — which is the only edit it asks
  for.

### Changed

- `--help` lists `init`, and `TestUsageNamesEveryDispatchedVerb` — which derives the verbs from the
  dispatcher — passes without a hand-kept list.
- [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) documents `init`, its three
  flags, its exit codes and the exact probe list. §8's "prompt registries" row now says why the
  feature is still deferred instead of pointing at an open question: the specification does not define
  it, and requirement C-02 forbids the network call it would make.
- `docs/04-plan/30-day-launch.md` (since removed) and
  `docs/04-plan/module-plan.md` (since removed) no longer say `init` is unimplemented.

---

## [0.6.1] — 2026-11-15

**`--help` now names every verb it accepts.** `attribute`, `module-manifest`, `serve` and `version` were all
dispatched, and all documented in the CLI reference, while the help screen listed four verbs. The only way to
learn they existed was the changelog or the source, and `attribute` — the verb this product's whole argument
rests on — had been invisible there since it shipped in 0.6.0.

### Fixed

- The usage text lists `attribute`, `module-manifest`, `serve` and `version`.
- A test now derives the dispatched verbs from the dispatcher and fails when the usage text does not name one,
  so the help screen cannot go stale by hand again. Verified by deleting a line and watching it fail.

---

## [0.6.0] — 2026-11-15

### Added

- **`attribute --format github`.** The attribution report as workflow annotations, one line per finding, so a
  regression appears on the pull request instead of only in a job log. The level states how strong the claim is:
  `error` for a class that a regressed case declares a dependency on, `warning` for a regressed case, and
  `notice` for a changed class that nothing points at — printed with zero rather than dropped, because silence
  reads as innocence. No annotation carries `file=` or `line=`: an eval case and a context class have no source
  location, and an invented one would send a reader to an unrelated line. The default output, every existing flag
  and every exit code are unchanged; `attribute` still exits 0 for a regression.
- **The action runs `attribute`.** `action.yml` takes `baseline-results` and `results` and passes `format`
  through, so the same composite step that gates a pull request can explain what moved it.

### Changed

- **The action builds the binary from its own checkout** instead of `go install`ing a module version that only
  exists once a tag has been pushed. No release asset is assumed to exist: `go build ./cmd/context-ci` against the
  action's own tree needs no upload, no tag and no download, and what runs is the code the ref names. `version`
  still pins a released module version when it is set.
- Every action input now reaches its script through an environment variable rather than a `${{ }}` substitution
  inside the shell, and `manifest` honours `root` and `config` the way `check` and `attribute` already did.

### Note

The action's steps were extracted from `action.yml` and run under Git Bash against a fixture on this date: the
build step, `check --format github` (exit 1 on a fail-severity change) and `attribute --format github` (exit 0,
three annotations). No GitHub-hosted runner ran it — the Actions API answers 404 for this private repository, as
recorded in [docs/03-product/spec.md](docs/03-product/spec.md) §7.

## [0.5.0] — 2026-11-14

### Added

- **`module-manifest`** — emits `module-manifest.v1.json`. Deliberately **not** named `manifest`: that verb
  already writes this product's own `context-manifest.json`, and two documents behind one name is a trap for
  whoever scripts against it.
- **`serve`** — `GET /healthz` and `GET /readyz`. The contract requires a `health_path`, and this product had
  no endpoint. Readiness answers `stateless: true`: a repository that fails a gate is this product working,
  not this product being unhealthy.

### Changed

- The package is `internal/modulemanifest`, since `internal/manifest` here already means the context manifest.

---

## [0.4.2] — 2026-11-13

### Fixed

- **The npm package would have shipped with no code in it.** `dist/` is gitignored, and nothing built
  it at pack time, so `npm pack` from a clean checkout produced a tarball containing only
  `package.json` and the README — a package whose every import fails. `prepack` now builds. Proved by
  packing from a clean tree, installing the tarball into an empty project, importing it, type-checking
  a TypeScript consumer with `tsc --strict`, and feeding the capture it wrote to the installed CLI,
  which produced an instrumented manifest.
- **The Python SDK was not installable.** There was no `pyproject.toml`, so `pip install` had nothing
  to build and the distribution name in the changelog did not exist. It now builds a wheel
  (`context-ci-capture`), proved by installing it into a clean venv with `--no-index` and capturing
  through the installed module. Four tests keep the metadata honest: the declared version matches
  `__version__`, there are no runtime dependencies, and the README says how to install.

### Note

Every `release.yml` step except Publish was run locally on this date and passed, including the five
cross-target builds and `sha256sum -c` over all five artefacts. Publish needs the GitHub API, which
answers 404 for a private repository. See
`docs/06-operations/squad-and-metrics.md` (since removed) §4.

## [0.4.1] — 2026-11-13

### Fixed

- **An installed binary reported `dev`.** `go install github.com/sigilary/context-ci/cmd/context-ci@v0.4.0`
  — the command in the action, the README and both CI snippets — builds without the release
  workflow's ldflags, so `context-ci version` printed `dev` and SARIF's `tool.driver.version` was
  `dev` for every user who installed it the documented way. The version now falls back to the module
  version the Go toolchain records in the binary, which for that command is the tag. A build inside
  a checkout reports the nearest tag (`0.4.1`, or `0.4.1+dirty` with local changes); only a build
  with no VCS to ask reports `dev`.
- The release workflow's own verification step compared the binary's output against
  `context-ci v$(cat VERSION)` while the build injected `VERSION`, so the check could never pass.
  It now compares like with like, and v0.4.1 is the first release where that step can succeed.
- **The GitLab snippet did not work.** It fetched with `git fetch --depth 1 origin main`, and in a
  single-branch CI clone that leaves no `origin/main` behind — only `FETCH_HEAD` — so the next line,
  `git show origin/main:context-manifest.json`, failed. Both snippets now fetch an explicit refspec.
  Found by running the snippets verbatim rather than reading them.

### Note

Measured, not assumed: `go install ...@v0.4.0` needs credentials to this repository, because it is
private until launch. Anonymous installs fail at `git ls-remote` with *could not read Username*. That
is a visibility decision, not a code defect, and it is recorded with a date in
[docs/03-product/spec.md](docs/03-product/spec.md) §7.

## [0.4.0] — 2026-11-13

### Added

- **SARIF and JUnit output.** `--format sarif` writes SARIF 2.1.0 and `--format junit` writes a
  JUnit report, for `diff` and `check` alike. Severity is not translated dishonestly: `fail` is
  `error` in SARIF and `<failure>` in JUnit, `ignore` is `note` and `<skipped>`, and `warn` stays a
  warning — JUnit has no warning level, so a warning passes its case and carries its message in
  `<system-out>` rather than being dressed up as a failure.
- Both formats were validated against their published schemas on the day of release — SARIF 2.1.0
  against the OASIS schema, JUnit against the Jenkins `junit-10.xsd` — using a diff containing one
  `fail`, one `warn` and one `ignore` entry.
- **The TypeScript capture SDK** (`sdk/typescript`, `@sigilary/context-ci`): no runtime
  dependencies, the same five-field capture format as the Python SDK, and `captureMessages()` for
  callers holding a message list rather than one string. A test runs `context-ci manifest --capture`
  against a real capture and pins `rendered_hash` to the SHA-256 of the exact text the application
  sent.
- CI snippets for GitLab CI and CircleCI, with the SARIF report wired to GitLab's
  `artifacts:reports:sarif`.

### Fixed

- A released binary reported `dev` as its version: the release workflow built without
  `-ldflags -X main.version`, so `context-ci version` was useless in a bug report and SARIF's
  `tool.driver.version` was meaningless to a consumer that dedupes results by it. The workflow now
  injects `VERSION` and fails if the binary it just built does not report it.
- `errors` is a required attribute on JUnit's `testsuite` in the schema most CI tools validate
  against, so it is now written as `0` instead of being omitted.
- The CLI reference no longer calls `attribute` unimplemented, and the README's quick start now
  matches `action.yml`'s actual inputs.

### Not done

None of the CI snippets has run on a hosted runner. The gap is recorded with a date in
[docs/03-product/spec.md](docs/03-product/spec.md) §7.

## [0.3.0] — 2026-11-13

### Added

- **Instrumented capture, end to end.** `sdk/python` (`context-ci-capture`, no dependencies) writes
  a capture file at request-assembly time; `context-ci manifest --capture <file>` turns it into a
  manifest with `capture.path: instrumented`, the SDK's version recorded, and `rendered_hash` the
  digest of the text actually sent. `rendered_sample_hash` stays null, because a live capture is
  not a sample.
- The capture format is five fields and no more: `capture_version`, `sdk`, `sdk_version`,
  `rendered`, and optional `template`. Tools, retrieval and policy still come from the
  configuration — recording them twice would create two sources of truth for one fact.
- **Variable values are deliberately not captured.** The manifest records types, as it does for
  asserted capture, because a manifest is a file teams commit and share; the rendered text already
  contains the values, and copying them in would turn a hash into a leak.
- Loading refuses unknown fields, an empty rendered prompt, a missing SDK version and a prompt over
  4 MiB, in both languages. A capture that would produce a valid-looking manifest with an empty
  prompt hash is the failure mode this feature exists to avoid.
- `capture_messages()` joins a chat-style message list one documented way, so callers holding
  messages do not each invent their own join.

### Note

The TypeScript SDK is not written. §8 of the CLI reference says so rather than leaving the row
looking complete.

## [0.2.0] — 2026-11-13

### Added

- **`attribute`: the fourth verb, and the differentiator.** Two manifests and two eval runs, and
  it reports which context class a movement in the scores points at — when the cases declare
  `depends_on`. Without those declarations it reports a correlation and says in the same
  sentence that nothing ties the movement to the configuration. A tool that printed the second
  as if it were the first would be inventing the answer the product exists to give.
- A movement smaller than 0.001 is not a movement: model scores do not repeat exactly, and a
  report that calls a rounding wobble a regression trains its reader to ignore it.
- A case present in one run and absent from the other is **incomparable, not unchanged**, so a
  vanished case is counted rather than shrinking the denominator quietly.
- A changed class with no evidence against it is printed with a zero share instead of being
  omitted; silence would read as innocence.
- `attribute` never exits non-zero for a regression. Gating is `check`'s job, and a verb that
  reports and fails at once cannot be used to explain a build that already failed.

## [0.1.0] — 2026-11-13

### Added

- **`action.yml`**: a composite GitHub Action, so `uses: sigilary/context-ci/action@v0.1.0` runs the
  binary built from the same tag as the action. It installs with `go install` rather than
  downloading an asset, so there is nothing between the tag and the binary a user can check
  against. `check` without a baseline fails: a gate that passes because it had nothing to compare
  is the most dangerous default available.
- **`.github/workflows/release.yml`**: on a `v*` tag it asserts the tag matches `VERSION`, builds,
  tests, runs the documentation and contract gates, builds five static targets, writes
  `SHA256SUMS`, checks the checksums describe what will be attached, and publishes.
- A `manifest` written by the action is not committed: it is an artefact of the run, and the
  action says which path it used in its output.

### Note

`0.1.0-dev` never existed as a release; this heading replaces it rather than pretending the dev
version shipped.

## [0.1.0-dev] — 2026-10-31

The first working CLI. Three of the four specified verbs are implemented, tested
and exercised end to end against a real git checkout.

### Added

- **`context-ci manifest`** writes `context-manifest.json` for a checkout, with a
  digest over a canonical form that excludes `created_at`. `--commit` defaults to
  `git rev-parse HEAD` and the command fails rather than inventing a value, because
  a manifest with no commit cannot be reproduced.
- **`context-ci diff`** classifies changes into the ten behavioural classes the
  specification names, plus seven extras, each with the severity the specification
  assigns. Needs no configuration file: comparing two manifests should never
  require setup.
- **`context-ci check`** gates on the result and exits 1 only for a change at or
  above `gate.fail_on`. `--baseline` is required, because a gate that silently
  passes when it had nothing to compare is the most dangerous default available.
- **`context-ci classes`** lists every class and its default severity, so an
  override can be written without guessing.
- `internal/manifest` — the manifest model, strict validation, and RFC 8785-style
  canonicalisation with UTF-16 key ordering and no floats.
- `internal/capture` — builds a manifest from a checkout: hashes sources with CRLF
  normalised to LF, refuses paths outside the repository, strips credential-shaped
  fields from tool schemas, and records why when no rendered sample was supplied.
- `internal/config` — `.context-ci.yaml`, parsed with unknown fields rejected and
  sampling values kept as their literal text so the digest never depends on how a
  parser represents a number.
- [ADR-0002](docs/07-decisions/adr-0002-implementation-language.md) — Go for the
  CLI, Python and TypeScript for the capture SDKs. No language had been decided
  anywhere in the repository.
- [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) — the
  implemented surface, the exit-code contract and the full configuration schema.

### Decisions worth recording

- **`prompt.template_hash` defaults to warn, not fail.** Editing a prompt is the
  most common deliberate change in this audience. A default that failed every one
  of them would be switched off within a week, and then nothing would be gated.
  A test locks this.
- **`capture.sources` uses `retrieval`, not an invented key.** An early
  implementation drifted from the specification's example configuration; the
  mismatch was caught by a test and the code was moved to match the document.
- **`description_tokens` is a whitespace-word estimate**, documented as an estimate.
  A cost projection with no configured price reports tokens and no dollars, because
  an invented price is worse than an absent figure.

### Not yet implemented

`attribute`, SARIF and JUnit output, the GitHub Action wrapper, prompt registries as
a source, and the two capture SDKs. Listed in
[docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) §8.

---

## [1.0.0-docs] — 2026-10-29

**Repository created.** No product code yet. This entry records the repository bootstrap so the first real release has a baseline.

### Added

- `docs/01-thesis.md` (since removed) — the bet, the evidence, the buyer, the wedge, the four things that must be true, and the failure case.
- `docs/02-market/market-and-competitors.md` (since removed) — 45 competitor dossiers in five segments, 61 sources, prices observed 29 October 2026.
- [docs/03-product/spec.md](docs/03-product/spec.md) — four verbs, manifest schema v1, semantic diff classes, attribution rules, requirements, acceptance tests, pricing.
- `docs/04-plan/30-day-launch.md` (since removed) and `docs/04-plan/module-plan.md` (since removed).
- [docs/05-contracts/](docs/05-contracts/README.md) — vendored `ledger@1.0.0` contracts with SHA-256 digests, and the rule that this product works without the platform.
- [docs/07-decisions/adr-0001-standalone-product-repo.md](docs/07-decisions/adr-0001-standalone-product-repo.md).
- `tools/check_docs.py` and `tools/check_vendor.py`, both enforced in CI.

### Changed

- Plan documents moved here from the platform repository and reframed as a standalone product rather than a platform module. Platform integration is now an optional channel, not a gate on release.

### Notes

- Licence: Apache-2.0, whole repository.
- Nothing in this repository requires an account, a key or network access.
