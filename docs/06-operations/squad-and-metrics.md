# Squad, operating model and metrics

> **Planning document, kept for history.** It was written before the tool was built and records what the team intended and assumed at the time. Staffing, dates, targets, prices and features in it are plans: some have been overtaken, and several were never built. For what the tool does today read [spec.md](../03-product/spec.md) and [cli-reference.md](../03-product/cli-reference.md); where this page disagrees with them, they are right.

**Owner:** Context CI squad · **Version:** 1.0.0 · **Date:** 29 October 2026

---

## 1. Squad

Four engineers plus DevRel. This is the smallest squad in the program on purpose: the product must be small enough that a stranger can read all of it.

| Role | Count | Owns |
|---|---:|---|
| Squad lead / engineer | 1 | Architecture, manifest schema, release management, review |
| CLI and capture engineer | 1 | Manifest production, determinism, the three capture paths |
| Diff and attribution engineer | 1 | Semantic diff, severity model, attribution, eval-result ingest |
| CI and integration engineer | 1 | GitHub Action, other CI providers, outputs (SARIF/JUnit), install path |
| DevRel / community | 1 | README, docs site, launch, issues, discussions, comparison content |

**What this squad does not own:** the platform's contracts (Platform Core), billing (Platform Core), or any hosted service (deferred until the February gate).

## 2. Rituals

| Ritual | When | Duration | Output |
|---|---|---|---|
| Standup | Daily, async | 10 min | Blockers only |
| Sprint review | Every 2 weeks, Friday | 45 min | Demo of something that runs, not slides |
| Release | Every 2 weeks from week 4 | — | Tag, changelog, signed artefact |
| Launch readiness | Weekly from week 3 | 30 min | Checklist status from [04-plan/30-day-launch.md](../04-plan/30-day-launch.md) §6 |
| Community triage | Twice weekly | 30 min | Issues labelled and answered within 48 h |

## 3. Decision rights

| Decision | Who decides | Who must be consulted |
|---|---|---|
| Manifest schema change | Squad lead | Platform Core (if it affects the hash-reference integration) |
| Adding a CI provider | CI engineer | Squad lead |
| Launch date slip | Squad lead | Founders |
| Introducing a paid feature | Founders | GTM, squad lead |
| Re-vendoring contracts | Squad lead | None — it is this repo's decision |
| Adopting a dependency | Any engineer | Squad lead; must pass the licence check |

## 4. Release process

1. Changelog entry written before the tag.
2. `python tools/check_docs.py` and `python tools/check_vendor.py` pass.
3. Determinism test passes on three operating systems.
4. Artefacts signed; SBOM attached.
5. Tag `vX.Y.Z`; GitHub release with a short "what changed and why it matters" note.
6. Announcement only if the release changes behaviour a user would notice.

### What has actually been exercised

`release.yml` is tag-driven, and on 13 November 2026 every step of it except **Publish** was run
locally in Git Bash with `GITHUB_REF_NAME=v0.4.2`: the tag/VERSION match, build/vet/test, both
documentation gates, the five cross-target builds with `-X main.version`, the check that the built
binary reports that version, `SHA256SUMS`, and `sha256sum -c` over all five artefacts. The version
check was run against the host target (`windows_amd64`) because a `linux_amd64` binary cannot execute
here; the step is otherwise identical.

**Publish is unverified.** It calls the GitHub API with the workflow token, which is unreachable from
the development environment — the Actions API answers 404 for this repository, which is private until
launch. So the honest status is: the pipeline's logic is exercised, the handoff to GitHub is not.

## 5. Metrics

Two sets, because the open-source product and the business have different scoreboards. Confusing them is how devtools projects convince themselves they are winning.

### 5.1 Adoption (weekly)

| Metric | Definition | Why it matters | Target by 16 Dec 2026 |
|---|---|---|---|
| Installs | Unique repositories running the action at least once | The only honest adoption number | 100 |
| Active repos (28-day) | Repos running the action in the last 28 days | Retention, not curiosity | 40 |
| Manifest success rate | Runs producing a valid manifest ÷ total runs | Product quality | >95% |
| Time to first manifest | Median, from `uses:` to first success | The five-minute promise | <5 min |
| Attribution accepted | Attributions a user marked correct ÷ attributions shown | The differentiator's accuracy | >70% |

### 5.2 Business (monthly, from February 2027)

| Metric | Definition | Target |
|---|---|---|
| Team-tier conversations | Named person with budget engaged | 5 by end of February |
| Paid conversions | Repos on a paid plan ÷ active repos | >5% |
| Net revenue retention | Expansion minus churn | >100% |
| Support load per paid repo | Tickets per month per paid repo | <1 |

### 5.3 The metric that decides whether this product continues

> **Repositories whose *platform team* is a named contact.**

Not stars. Not installs. If a platform team has not adopted it, the paid boundary does not exist and the product should stay free. Measured at the February gate; below 3, the paid tier is cancelled and the tool becomes a pure distribution asset for the platform.

## 6. Risks this squad owns

| Risk | Early signal | Response |
|---|---|---|
| Manifest is hard to produce for real apps | Time to first manifest >15 min for outside users | Add capture path; consider a provider-specific helper |
| Attribution is wrong or vague | "Attribution accepted" <50% | Narrow the claim; report only high-confidence cases |
| Registries pull prompts out of repos | Users ask for registry sync | Integrate as a *source*, never as a store |
| A vendor ships context diff | Feature appears in LangSmith/Langfuse | Publish the manifest schema wider; compete on the gate, not the dashboard |
| Nobody pays | Repositories with a platform-team contact <3 | Cancel the paid tier; keep it free |
| Licence contamination | Any GPL or unclear-licence dependency | Licence check in CI, blocking |
