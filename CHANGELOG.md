# Changelog

All notable changes to Context CI. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

Nothing yet.

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
