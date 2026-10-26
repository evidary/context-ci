# Decision register

**Rule:** every decision that is expensive to reverse is a numbered ADR in this folder. If it is not here, it is a preference and anyone may change it.

---

## Accepted

| ADR | Decision | Date | Status |
|---|---|---|---|
| [0001](adr-0001-standalone-product-repo.md) | Context CI is its own Apache-2.0 product repository with no dependency on the platform | 29 Oct 2026 | Accepted |

## Open decisions

| # | Decision needed | Options | Owner | Due |
|---|---|---|---|---|
| D-01 | Manifest schema stability promise | Semver with N-1 / frozen at 1 forever / no promise | Squad lead | Week 2 |
| D-02 | Registry sources at v0.1 | Files only / files + Langfuse / files + Langfuse + LangSmith | Squad lead | Week 3 |
| D-03 | Primary machine-readable output | SARIF / JUnit / both | CI engineer | Week 4 |
| D-04 | Whether the Team tier includes CI minutes | History only / history + hosted runner | Founders | February gate |
| D-05 | Namespace for the GitHub Action | `context-ci/action` / org-scoped | Founders | Before week 4 |
| D-06 | Support model for the free tier | GitHub Discussions only / Discussions + monthly office hours | DevRel | Launch |

## How to write one

Copy the structure of [0001](adr-0001-standalone-product-repo.md): Context, Decision, Consequences, Alternatives rejected, Revisit trigger, References. A decision with no alternatives and no revisit trigger is a note, not an ADR.
