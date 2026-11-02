# Context CI — document index

**This file routes every question to exactly one document in this repository.** Context CI is a standalone product; nothing needed to build, sell or support it lives outside this repository except the vendored contracts, which are copied in.

**Repo:** `context-ci` · **Version:** 0.1.0-dev (pre-release; the CLI is implemented, the build starts 17 Nov 2026) · **Last updated:** 29 October 2026

---

## 1. Route by question

| If you need to know… | Read |
|---|---|
| What this product is, in one page | [../README.md](../README.md) |
| Why it deserves to exist, and what must be true | [01-thesis.md](01-thesis.md) |
| Who we compete with, what they charge, what they miss | [02-market/market-and-competitors.md](02-market/market-and-competitors.md) |
| Who buys it and who blocks it | [02-market/market-and-competitors.md](02-market/market-and-competitors.md) §4 |
| What exactly we build, in scope and out | [03-product/spec.md](03-product/spec.md) |
| The manifest format | [03-product/spec.md](03-product/spec.md) §4 |
| Every flag, exit code and configuration key | [03-product/cli-reference.md](03-product/cli-reference.md) |
| What happens in the first 30 days | [04-plan/30-day-launch.md](04-plan/30-day-launch.md) |
| What happens in weeks 5–15 | [04-plan/module-plan.md](04-plan/module-plan.md) |
| When we decide to charge money | [04-plan/module-plan.md](04-plan/module-plan.md) §10 and §14 |
| Which platform contracts we consume and at what version | [05-contracts/README.md](05-contracts/README.md) |
| Who does the work, and how we measure it | [06-operations/squad-and-metrics.md](06-operations/squad-and-metrics.md) |
| Why this is a separate repository | [07-decisions/adr-0001-standalone-product-repo.md](07-decisions/adr-0001-standalone-product-repo.md) |
| Which language the CLI is written in, and why the SDKs are not | [07-decisions/adr-0002-implementation-language.md](07-decisions/adr-0002-implementation-language.md) |
| What is decided and what is still open | [07-decisions/README.md](07-decisions/README.md) |
| Milestones and dates | [../ROADMAP.md](../ROADMAP.md) |
| How to contribute | [../CONTRIBUTING.md](../CONTRIBUTING.md) |

---

## 2. Full document map

| File | What it holds | State |
|---|---|---|
| [01-thesis.md](01-thesis.md) | The bet, the evidence, the buyer, the wedge, the four things that must be true, and the honest failure case | Current |
| [02-market/market-and-competitors.md](02-market/market-and-competitors.md) | 45 competitor dossiers in five segments, 61 sources, pricing landscape, open-source conversion analysis, distribution, structural gap, ten failure modes | **Dated 29 Oct 2026** |
| [03-product/spec.md](03-product/spec.md) | Scope, non-goals, the manifest schema, CLI surface, CI integration, requirements, acceptance tests, pricing | Current |
| [03-product/cli-reference.md](03-product/cli-reference.md) | The implemented verbs, every flag, the exit-code contract, the full configuration schema and the rules that are enforced rather than documented | Current |
| [04-plan/30-day-launch.md](04-plan/30-day-launch.md) | Week-by-week to a public v0.1: foundation, runner and CI, attribution, launch | Current |
| [04-plan/module-plan.md](04-plan/module-plan.md) | The full 15-week plan: scope, architecture, workstreams, sprints, pricing, metrics, risks, gates | Current, reframed for standalone |
| [05-contracts/README.md](05-contracts/README.md) | What we consume from `acp-core`, what we publish, and the vendoring rule | Current |
| [05-contracts/vendor/acp-core@1.0.1/VENDORED.md](05-contracts/vendor/acp-core@1.0.1/VENDORED.md) | Pinned contract copies with SHA-256 digests | Pinned 1.0.0 |
| [06-operations/squad-and-metrics.md](06-operations/squad-and-metrics.md) | Squad charter, rituals, decision rights, the metrics that matter, and the release process | Current |
| [07-decisions/README.md](07-decisions/README.md) | Decision register | Current |
| [07-decisions/adr-0001-standalone-product-repo.md](07-decisions/adr-0001-standalone-product-repo.md) | Why this is its own repository and its own licence, and what it may never depend on | Current |
| [07-decisions/adr-0002-implementation-language.md](07-decisions/adr-0002-implementation-language.md) | Go for the CLI, Python and TypeScript for the capture SDKs, and why the split buys more than one language everywhere | Current |
| [07-decisions/adr-0007-unit-of-integrity.md](07-decisions/adr-0007-unit-of-integrity.md) | What the digest covers: everything that steers the model is hashed; a count is never the record; a plaintext copy must verify against its hash | Current |

---

## 3. What is deliberately not in this repository

| Not here | Where it is | Why |
|---|---|---|
| Platform strategy, pricing, finance | `acp-core:docs/01-strategy/` | Not needed to build this product |
| The platform's contracts in full | vendored copies in [05-contracts/vendor/](05-contracts/vendor/acp-core@1.0.1/VENDORED.md) | We pin what we consume; we do not carry the whole contract set |
| Other modules' plans | their own repositories | Separate products, separate buyers |
| Cross-group research archive | not in any repo | Portfolio context, not build context |

---

## 4. Index integrity

Every markdown file under `docs/` must be referenced from this file. `tools/check_docs.py` fails CI if it is not, and also fails on a broken internal link, a malformed table or an encoding fault. Run it before every pull request:

```
python tools/check_docs.py
```
