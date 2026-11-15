# Changelog

All notable changes to Context CI. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/).

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

## [Unreleased]

Nothing yet.

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
[docs/06-operations/squad-and-metrics.md](docs/06-operations/squad-and-metrics.md) §4.

## [0.4.1] — 2026-11-13

### Fixed

- **An installed binary reported `dev`.** `go install github.com/evidary/context-ci/cmd/context-ci@v0.4.0`
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
- **The TypeScript capture SDK** (`sdk/typescript`, `@evidary/context-ci`): no runtime
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

- **`action.yml`**: a composite GitHub Action, so `uses: evidary/context-ci/action@v0.1.0` runs the
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

- [docs/01-thesis.md](docs/01-thesis.md) — the bet, the evidence, the buyer, the wedge, the four things that must be true, and the failure case.
- [docs/02-market/market-and-competitors.md](docs/02-market/market-and-competitors.md) — 45 competitor dossiers in five segments, 61 sources, prices observed 29 October 2026.
- [docs/03-product/spec.md](docs/03-product/spec.md) — four verbs, manifest schema v1, semantic diff classes, attribution rules, requirements, acceptance tests, pricing.
- [docs/04-plan/30-day-launch.md](docs/04-plan/30-day-launch.md) and [docs/04-plan/module-plan.md](docs/04-plan/module-plan.md).
- [docs/05-contracts/](docs/05-contracts/README.md) — vendored `acp-core@1.0.0` contracts with SHA-256 digests, and the rule that this product works without the platform.
- [docs/07-decisions/adr-0001-standalone-product-repo.md](docs/07-decisions/adr-0001-standalone-product-repo.md).
- `tools/check_docs.py` and `tools/check_vendor.py`, both enforced in CI.

### Changed

- Plan documents moved here from the platform repository and reframed as a standalone product rather than a platform module. Platform integration is now an optional channel, not a gate on release.

### Notes

- Licence: Apache-2.0, whole repository.
- Nothing in this repository requires an account, a key or network access.
