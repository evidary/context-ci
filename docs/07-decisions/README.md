# Decision register

**Rule:** every decision that is expensive to reverse is a numbered ADR in this folder. If it is not here, it is a preference and anyone may change it.

---

## Accepted

| ADR | Decision | Date | Status |
|---|---|---|---|
| [0001](adr-0001-standalone-product-repo.md) | Context CI is its own Apache-2.0 product repository with no dependency on the platform | 29 Oct 2026 | Accepted |
| [0002](adr-0002-implementation-language.md) | Go for the CLI; Python and TypeScript for the capture SDKs | 29 Oct 2026 | Accepted |

## Decided without an ADR

Cheap to reverse, so they are recorded here rather than given an ADR each.

| # | Decision | Answer | Date | Why |
|---|---|---|---|---|
| D-03 | Primary machine-readable output | Both, and neither is primary: `sarif` for code scanning, `junit` for XML-only dashboards, `github` still the action's default | 13 Nov 2026 | A team that already runs code scanning should not be asked to add a second report format. See [cli-reference](../03-product/cli-reference.md) §7 |
| D-05 | Namespace for the GitHub Action | `evidary/context-ci` — `action.yml` is at the repository root, so a workflow writes `uses: evidary/context-ci@<tag>` | 13 Nov 2026, corrected 16 Nov 2026 | The repository already lives under the `evidary` org; a second org is another namespace to own and another place to forget to update. The answer first recorded here was `evidary/context-ci/action`, which names a subdirectory that has never existed and so could not resolve; the documents were changed to match the file rather than the file moved, because a root action is the shorter name and needs no second copy of `action.yml` |

## Open decisions

| # | Decision needed | Options | Owner | Due |
|---|---|---|---|---|
| D-01 | Manifest schema stability promise | Semver with N-1 / frozen at 1 forever / no promise | Squad lead | Week 2 |
| D-02 | Registry sources at v0.1 | Files only / files + Langfuse / files + Langfuse + LangSmith | Squad lead | Week 3 |
| D-04 | Whether the Team tier includes CI minutes | History only / history + hosted runner | Founders | February gate |
| D-06 | Support model for the free tier | GitHub Discussions only / Discussions + monthly office hours | DevRel | Launch |

## How to write one

Copy the structure of [0001](adr-0001-standalone-product-repo.md): Context, Decision, Consequences, Alternatives rejected, Revisit trigger, References. A decision with no alternatives and no revisit trigger is a note, not an ADR.
