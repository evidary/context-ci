# Changelog

All notable changes to Context CI. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/).

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
