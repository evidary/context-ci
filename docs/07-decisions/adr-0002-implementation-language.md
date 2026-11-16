# ADR-0002 — Go for the CLI, with capture SDKs in Python and TypeScript

**Status:** accepted · **Date:** 31 October 2026 · **Supersedes:** none

## Context

The product specification lists one CLI with four verbs — `manifest`, `diff`,
`attribute`, `check` — that must run in a customer's CI on every pull request.
Nowhere in this repository was the implementation language recorded, and the first
line of code needs it.

Constraints that actually bear on the choice:

1. **Byte-identical output on Linux, macOS and Windows.** The manifest is a hash
   input. If two developers on different operating systems get different bytes for
   the same repository, the product is broken, and the whole diff is noise.
2. **No network, no account, no secrets** for the open-source path (§9 of the
   specification).
3. **Under 30 seconds on a 2-core runner** for a 50-tool manifest.
4. **Distribution inside CI must not need a toolchain install.** A CI image that
   needs a Python venv and a resolver adds minutes and a failure mode.
5. Five repositories, one company, no capital. The team is strongest in Go and
   already runs a Go 1.27 toolchain for `acp-core`.
6. The capture path needs an SDK call (`context_ci.capture(...)` in Python) inside the customer's application, and
   those applications are Python or TypeScript.

## Decision

**Go for the CLI.** Python and TypeScript for the capture SDKs, as separate
libraries that write the capture file the CLI turns into a manifest.

## Why

- A static binary removes constraint 4 entirely, and cold start is milliseconds
  rather than a resolver hop.
- Determinism is the load-bearing property, and Go makes it one test with three
  operating systems in CI rather than three runtime versions to pin.
- The manifest is a digest computation over files. That is the same shape of work
  as `acp-core`'s integrity core, so the team's existing knowledge transfers even
  though the code is not shared.
- Constraint 6 is a real argument for Python, and it is satisfied by the SDKs.
  Users who never touch Go still never touch Go.

## Rejected

| Option | Why not |
|---|---|
| **Python for everything** | Satisfies constraint 6 for the whole product, but makes the CLI a packaging problem: a venv or a bundler on every CI image, and three interpreters to pin for determinism. The SDK still has to be Python regardless, so Python-everywhere buys nothing the split does not. |
| **TypeScript / Node** | Same distribution problem with an extra runtime, and the tooling for byte-level hashing and canonical JSON is weaker. |
| **Rust** | Best determinism and binary story of all. Rejected on team velocity alone: nobody on the team ships Rust weekly, and the 16 December date is not negotiable. |
| **A shell script plus `jq`** | Cannot do semantic diff or attribution, and `jq`'s number handling differs from canonical JSON. |

## Consequences

- `go.mod` pins a Go version; the release workflow builds for three operating
  systems and publishes SHA-256 checksums beside the binaries. Nothing is signed yet.
- The manifest is built in **one implementation**, Go. The SDKs in Python and
  TypeScript write the capture file that `manifest --capture` reads; they do not
  compute the manifest or its digest, so there is no second implementation to keep
  in step. The cross-implementation check this decision did plan exists only for the
  digest, in `tools/interop/reference_digest.py`, which recomputes the pinned value
  independently of the Go code.
- The CLI may take third-party Go dependencies. `acp-core`'s zero-dependency rule
  exists because a verifier must run on a hostile host with no supply chain; a CI
  tool has no such constraint, and hand-rolling a YAML parser would be worse than
  depending on one.
- The CLI must never import `acp-core` code. Integration is over the vendored
  contracts in `docs/05-contracts/vendor/`, as ADR-0001 requires.

## Reversibility

Expensive after the first public release: the manifest digest and therefore every
stored manifest is tied to the canonicalisation, and the distribution story is
tied to the binary. **Cheap until then** — before the first public release there is no user, no stored
manifest and no install path to break, so the language could still change for the
cost of the CLI code alone.
