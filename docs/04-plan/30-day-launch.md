# Context CI — 30-Day Open-Source Launch Plan

**Start:** 17 November 2026 · **Launch:** 16 December 2026 · **Day 30:** 16 December 2026  
**Squad:** 4 engineers + 1 DevRel + 0.5 platform + 0.3 design + 0.2 security  
**Note:** the 4 engineers are the Context CI squad; DevRel, Platform, Design and Security are fractional shared-squad allocations, not additional headcount.  
**Budget:** ≤$100/month

---

## 1. Goal

Launch a credible open-source Context CI project that:

- Lets a team snapshot prompts, tools, retrieval config and model settings.
- Diffs context changes and runs regression tests in CI.
- Catches real regressions before release.
- Demonstrates engineering quality and builds trust for the company.

**Launch success:**
- 10 repositories using the GitHub Action.
- 3 design partners.
- 2 real regressions caught.
- 50+ GitHub stars (directional, not the goal).
- No critical security issues.
- 5-minute quickstart works.

**Non-goals:** prompt authoring, hosted inference, generic eval platform, paid billing, enterprise SSO.

---

## 2. Team and ownership

| Person | Role | Owns |
|---|---|---|
| Engineer 1 | CLI/Manifest | `contextci init`, `manifest` (the snapshot), `diff`, `check`, `classes`; schema and config — see the note below |
| Engineer 2 | Runner/Diff | Semantic diff, golden-set runner, assertions |
| Engineer 3 | CI/Action | GitHub Action, PR annotations, examples |
| Engineer 4 | Attribution/Docs | Failure attribution, docs, tests, release |
| DevRel | Launch/community | README, landing, demo, launch, support |
| Platform | 0.5 | Schema/SDK alignment, module manifest |
| Design | 0.3 | Logo, landing, diagrams |
| Security | 0.2 | Threat model, secrets, release review |

---

## 3. Scope for v0.1

### In scope

- Manifest: prompts, tool schemas, retrieval config, model/params, policies.
- Capture from repo files + `.context-ci.yaml`.
- Semantic diff across versions.
- Golden-set runner (JSONL/CSV) with assertions.
- GitHub Action: PR check, annotation, pass/fail.
- Local CLI; no hosted service required.
- Attribution: rank changed elements likely causing failures.
- Docs: quickstart, concepts, CI examples, security.

### Out of scope

- Hosted dashboard/history (paid later).
- LLM-judge at scale (optional, customer keys).
- Prompt editor.
- Multi-repo monorepo support beyond basic globs.

---

**Verb names.** This plan was written before the CLI existed and used `init` and
`snapshot`. The implemented surface is `init`, `manifest`, `diff`, `check` and `classes`, documented
in [cli-reference.md](../03-product/cli-reference.md). `init` — writing a starter config — landed on
15 November 2026, so week 2's "CLI skeleton: `init`, `snapshot`, `diff`" is already half done. Where
this document says "snapshot", read `manifest`.

## 4. Week-by-week plan

### Week 1 (17–23 Nov 2026) — Foundation

| Day | Task | Owner | Acceptance |
|---|---|---|---|
| 1 | Kickoff, scope, repo, license, issue templates | DevRel/Engineer 1 | Public repo skeleton |
| 1 | Define manifest schema v0 | Engineer 1 + Platform | JSON Schema committed |
| 2 | CLI skeleton: `init`, `snapshot`, `diff` | Engineer 1 | Runs locally |
| 2 | Example repo (support agent) | Engineer 3 | Runnable |
| 3 | Repo capture: prompts/tools/retrieval/model | Engineer 1 | 3 examples |
| 3 | Diff engine v0 (structural) | Engineer 2 | Detects changes |
| 4 | Golden-set format + loader | Engineer 2 | JSONL/CSV |
| 4 | Docs skeleton + README | DevRel | Quickstart draft |
| 5 | Security threat model v0 | Security | Reviewed |
| 5 | Weekly demo | All | Working CLI |

**Deliverable:** `contextci manifest` and `contextci diff` work on the example repo.

### Week 2 (24–30 Nov 2026) — Runner and CI

| Day | Task | Owner | Acceptance |
|---|---|---|---|
| 1 | Assertion runner (pass/fail, cost, latency) | Engineer 2 | 10 assertions |
| 2 | GitHub Action: install, run, annotate | Engineer 3 | PR check works |
| 3 | PR annotation format + summary | Engineer 3 | Clear cause report |
| 4 | Golden-set examples + fixtures | Engineer 2 | 3 datasets |
| 4 | Test suite: unit + integration | Engineer 4 | CI green |
| 5 | Design partner #1 install | DevRel | Installed |
| 5 | Weekly demo + feedback | All | Prioritized fixes |

**Deliverable:** PR check runs on a real repo; design partner #1 live.

### Week 3 (1–7 Dec 2026) — Attribution and quality

| Day | Task | Owner | Acceptance |
|---|---|---|---|
| 1 | Attribution v0: map changed elements to failures | Engineer 4 | Ranked causes |
| 2 | Config for model/params/retrieval | Engineer 1 | Works |
| 3 | Performance: 100 cases <2 min | Engineer 2 | Benchmark |
| 3 | Security review: secrets, permissions, telemetry | Security | Signed off |
| 4 | Docs: CI examples, troubleshooting | DevRel | Complete |
| 4 | Design partners #2–3 | DevRel | Installed |
| 5 | Catch/verify 2 real regressions | Team | Documented |
| 5 | Release candidate v0.1.0-rc1 | Engineer 4 | Tagged |

**Deliverable:** 2 regressions caught; RC ready.

### Week 4 (8–16 Dec 2026) — Launch

| Day | Task | Owner | Acceptance |
|---|---|---|---|
| 1 | Launch checklist review | DevRel | All items checked |
| 1 | Demo video/GIF | Design/DevRel | Published |
| 2 | Landing page + docs site | Design/DevRel | Live |
| 2 | Security policy, CONTRIBUTING, CODE_OF_CONDUCT | Engineer 4 | Published |
| 3 | Launch post + HN/Reddit/MCP directories | DevRel | Submitted |
| 3 | Community support rotation | DevRel/Engineers | Covered |
| 4 | **Launch v0.1.0** | All | Public |
| 5 | Monitor issues, fix P0/P1 | Team | <24h response |
| 30 | Retro + metrics + roadmap | All | Report |

**Deliverable:** public launch, 10 repos, 3 design partners, 2 regressions caught.

---

## 5. Quality bar

- Apache-2.0 license; no enterprise bait-and-switch.
- README with 5-minute quickstart and architecture diagram.
- Docs: concepts, CLI reference, GitHub Action, examples, security.
- Tests: unit, integration, golden events; CI green.
- Security: threat model, no secrets in logs, least-privilege Action, signed release, SBOM.
- Performance: 100 cases <2 minutes locally.
- False-positive rate <10% on examples.
- Responsive issues: <24h first response during launch week.
- Changelog and semantic versioning.

---

## 6. Launch checklist

- [ ] Public repo, license, README, docs.
- [ ] **The moment the repo is public:** run `go install github.com/evidary/context-ci/cmd/context-ci@<tag>`
      logged out, then the GitLab and CircleCI snippets in [spec §7](../03-product/spec.md). Until the
      repository is public an outsider cannot install at all — measured 13 Nov 2026 — so acceptance tests
      11.1 and 11.3 are blocked on this one action, not on more code.
- [ ] Publish both capture SDKs (`context-ci-capture` on PyPI, `@evidary/context-ci` on npm). Both build
      and install cleanly as of 13 Nov 2026 — wheel into a venv, tarball into an empty project — so this
      is a publishing step, not a code one.
- [ ] Demo video/GIF and example repo.
- [ ] GitHub Action published to Marketplace (or documented).
- [ ] Release tagged, signed, SBOM.
- [ ] Security policy and vulnerability disclosure.
- [ ] Issue/PR templates; roadmap discussion.
- [ ] Landing page and waitlist.
- [ ] HN/Reddit/MCP directory posts.
- [ ] Design-partner quotes (approved).
- [ ] Support rotation and status page.
- [ ] Metrics dashboard: installs, PR checks, regressions, stars.

---

## 7. Metrics

| Metric | Target |
|---|---:|
| Repositories using Action | 10 |
| Design partners | 3 |
| Regressions caught | 2 |
| PR checks run | 100 |
| Time to first check | <10 min |
| False positives | <10% |
| GitHub stars | 50+ (directional) |
| Critical security issues | 0 |
| Docs quickstart success | 5/5 testers |

---

## 8. Risks and mitigations

| Risk | Trigger | Mitigation |
|---|---|---|
| Launch quality too low | Missing docs/tests | Delay 1 week; quality bar is binding |
| No adoption | <10 repos | Improve quickstart; post in more communities; design partners |
| False positives | >10% | Tighten assertions; add exclusions |
| Security issue | Any | Stop release; fix; disclose |
| Scope creep | Requests for dashboard/hosting | Keep v0.1 CLI/CI; paid history later |
| Support overload | >20 issues/week | Docs-first; triage rotation; FAQ |

---

## 9. Post-launch 30 days

- Weekly patch releases; triage P0/P1 within 24h.
- 10 design-partner calls; prioritize top 3 requests.
- Publish a “context regression” benchmark.
- Add 2 integrations per week (e.g., Cursor, Codex, LangGraph).
- Decide paid tier: ≥10 repos, 3 design partners, 2 regressions, <10% false positives.
- If not met: keep as OSS side project; do not build paid tier yet.

---

## 10. Integration with the platform

- Standalone mode: local auth, local storage, no platform dependency.
- Integrated mode: module manifest, shared event schema, optional hosted history/SSO.
- Contract tests: manifest schema, tenant isolation, entitlements, events.
- GA gate: ≥70% shared code/schema; no separate billing/auth.
- Link to `acp-core:docs/05-contracts/shared-core-contracts.md` and `acp-core:docs/04-plan/integration-gates.md`.

---

## 11. Definition of done

- Public Apache-2.0 repo with docs, tests, CI and signed release.
- CLI + GitHub Action capture/diff prompts, tools, retrieval and model config.
- Regression runner and attribution report.
- 10 repos, 3 design partners, 2 real regressions caught.
- No critical security issues.
- Launch retro and paid-GA decision recorded.
