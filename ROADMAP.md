# Roadmap

**Build window:** 17 November 2026 – 27 February 2027 · **Public launch:** 16 December 2026
Full plan: [docs/04-plan/30-day-launch.md](docs/04-plan/30-day-launch.md) · [docs/04-plan/module-plan.md](docs/04-plan/module-plan.md)

---

## The 30 days to launch

| Week | Dates | Ship | Gate |
|---|---|---|---|
| 1 | 17–23 Nov 2026 | Repository public, manifest schema v1, CLI skeleton, determinism harness | Schema review |
| 2 | 24–30 Nov 2026 | Capture paths (asserted, instrumented), GitHub Action, first runner pass | Works on a real repo |
| 3 | 1–7 Dec 2026 | Semantic diff, severity model, attribution v0, output formats | Seeded regression attributed correctly |
| 4 | 8–16 Dec 2026 | Docs site, install path, launch post, v0.1.0 tag | **Public launch** |

## After launch

| When | What |
|---|---|
| 17 Dec 2026 – 15 Jan 2027 | Weeks 5–8: registry sources, more CI providers, attribution accuracy work, community triage |
| 16 Jan – 15 Feb 2027 | Weeks 9–12: hosted history (Team tier), SSO groundwork, design-partner conversations |
| 16–28 Feb 2027 | Weeks 13–15: paid tier decision gate, v0.2, first paying customers or a documented decision not to sell |

## Milestones

| # | Milestone | Target | Definition of done |
|---|---|---|---|
| M1 | Manifest schema v1 published | 21 Nov 2026 | JSON Schema in the release artefact, versioned |
| M2 | First real repository producing manifests | 30 Nov 2026 | Not a fixture; someone else's code |
| M3 | Seeded regression attributed | 7 Dec 2026 | Correct entry named, evidence shown |
| M4 | v0.1.0 public | 16 Dec 2026 | Tagged, signed, SBOM, docs site live |
| M5 | 100 installs / 40 active repos | 31 Dec 2026 | Measured, not claimed |
| M6 | Paid-tier decision | 28 Feb 2027 | Repositories with a platform-team contact ≥3 → sell; else stay free |

## What we are not doing in this window

| Not doing | Why |
|---|---|
| A hosted control plane before the February gate | Cost and data-protection burden before demand is proven |
| Prompt authoring or a registry UI | Different product; destroys the positioning |
| More than three capture paths | Each path is a support surface |
| Windows-first or IDE-first distribution | The adopter lives in CI |
