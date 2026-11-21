# ADR-0001 — Context CI is its own product repository

**Status:** Accepted · **Date:** 29 October 2026 · **Deciders:** founding team

---

## Context

Context CI was originally planned as a module inside one platform repository, built by a squad that also contributed to the platform, released under the platform's security review, and gated on integration with a paid platform before it could sell.

That arrangement fails this product specifically, for reasons that do not apply to the other three modules:

1. **The audience is different.** The person who adopts Context CI is an individual engineer who installs a GitHub Action without asking anyone. The platform's buyer is a CISO with a questionnaire. A single repository forces one release note and one issue tracker onto both.
2. **The credibility is the product.** This is the one artefact in the portfolio that a stranger can read end to end. It must be Apache-2.0, buildable offline, and free of references to closed code — otherwise it reads as open-core theatre and stops being credible.
3. **The launch window is 30 days.** The plan calls for a public v0.1 on 16 December 2026, five weeks after program start. Anything that puts a platform security review and a contracts gate in front of that date makes the date impossible.
4. **The failure mode is different.** A platform module that fails is a feature nobody uses. A standalone devtool that fails is absorbed. The response to absorption risk is reach and readability, both of which argue for a separate, self-contained, public repository.

## Decision

1. **Context CI lives in its own repository** with its own versioning, changelog, CODEOWNERS, CI and release cadence.
2. **Licence is Apache-2.0.** The whole tool. Not open-core with a closed folder — the free tool is the complete check.
3. **No dependency on `ledger`.** The platform integration is optional and one-directional: the tool emits a manifest digest that a platform may reference. Nothing is required.
4. **The paid boundary is hosted history and cross-repository attribution**, which a single developer does not need. It is not a feature withheld from the free tool.
5. **The product must build and pass its full test suite with no platform present.** CI runs with no platform present, so a dependency on it would fail the build.
6. This repository keeps a **pinned copy** of the two platform contracts it may optionally use, with checksums, so it can be built and released without network access to the platform.

## Consequences

**Positive**

- The 30-day launch is achievable because nothing external gates it.
- The licence boundary is unambiguous: everything here is Apache-2.0.
- A contributor can read the entire product in an afternoon, which is what makes the credibility claim true.
- A fork is possible, which is a feature for a tool whose job is to be trusted.

**Negative**

- Separate issue tracking, separate CI, separate release notes. Roughly 2 engineer-days per month of overhead.
- The squad is smaller than it would be inside a larger team, so the product must stay narrow. That is enforced by the four-verb scope in the specification.
- Duplicated effort with the platform on auth, tenancy and billing **when** hosted tiers arrive. Accepted: those arrive after the February gate, not before.
- Cross-repository reading is harder for a platform developer. Mitigated by the vendored contracts and by [../05-contracts/README.md](../05-contracts/README.md).

## Alternatives rejected

| Alternative | Why rejected |
|---|---|
| Module inside the platform repo, open-sourced later | The launch window closes; the OSS tree would reference closed paths |
| Open-core with a closed premium folder in the same repo | Makes the "read the whole thing" claim false, which is the entire asset |
| Separate repo but GPL | Kills enterprise adoption; the value here is reach, not copyleft |
| Separate repo but requiring a platform account for attribution | Removes the reason a stranger installs it |

## Revisit trigger

1. Fewer than 3 repositories with a named platform-team contact by the February 2027 gate: cancel the paid tier and keep the tool free.
2. A hosted tier requires tenancy, billing or SSO: at that point either reuse the platform's services under a service agreement, or accept a small amount of duplicated code deliberately.
3. Absorption: if the project is acquired or merged into a vendor, this ADR is void.

## References

- [../05-contracts/README.md](../05-contracts/README.md)
- The platform repository's own decision record on keeping separate repositories (not public)
