# Plan 06 — Context CI

**Opportunity:** O6 — Context CI
**Plan window:** 17 November 2026 – 27 February 2027; 30-day open-source launch 17 November – 16 December 2026
**GA target:** open-source v0.1 on 16 December 2026; paid Team tier decided at the February 2027 gate
**Owner:** Context CI squad (4 engineers + DevRel), in this repository

**Product context:** this is a **standalone Apache-2.0 product**, not a module of the platform. It must build, test and ship with `acp-core` absent. The platform integration in section 14 is an optional channel, not a release gate. Repository rationale: [../07-decisions/adr-0001-standalone-product-repo.md](../07-decisions/adr-0001-standalone-product-repo.md). Contracts: [../05-contracts/README.md](../05-contracts/README.md).

---

## 1. Objective

Build a CI tool that catches **context regressions** before they reach production:

> “We changed a prompt, tool schema, retrieval config or policy — did quality, cost or latency get worse?”

Context CI snapshots every element that shapes an agent's behaviour, runs regression tests against the customer's own traces/golden set, and attributes failures to specific context changes.

**Success:** 10 repositories using the GitHub Action, 3 design partners, and at least 2 customers who can name a regression caught before release.

---

## 2. Trigger and decision

The build is **approved**: 30 days to public v0.1, ending 16 December 2026. The trigger gates the **paid tier**, not whether the squad starts.

Proceed to a paid tier if at least **three** are true **by 15 February 2027** (the evidence cut-off). The decision itself is taken at the **28 February 2027** gate, two weeks later, so there is time to price and package before the answer lands.

- ≥3 repositories have a **named platform-team contact** — the buyer, not just the adopter.
- ≥5 teams ask for cross-repository history or retention beyond 30 days.
- ≥2 teams ask for SSO or an audit export before they will roll it out further.
- A team reports a production regression they attribute to a context change and asks how to prevent the next one.

**Decision:** launch as a free, complete, Apache-2.0 tool on 16 December 2026 **regardless**. Do not withhold any capability from the free tool to create a paid reason — the paid reason is history and cross-repository attribution, which a single developer does not need.

If fewer than three of the above hold, the tool stays free permanently and is treated as distribution for the platform, not as a product with a price. Record that decision in [../07-decisions/README.md](../07-decisions/README.md).

---

## 3. Scope

### In scope

- **Context manifest:** versioned snapshot of system prompts, tool definitions, retrieval configuration, model/version, temperature, policies and feature flags.
- **Capture:** CLI/GitHub Action from repo files (`prompts/`, `tools/`, config), SDK/OTel runtime capture, and manual upload.
- **Diff engine:** semantic diff across context versions; who changed what, when.
- **Regression runner:** run the customer's traces/golden set against old and new context; compare pass rate, cost, latency and groundedness.
- **Attribution:** map failing tests to changed context elements; rank likely causes.
- **CI integrations:** GitHub Actions, GitLab CI, CLI; PR annotations and status checks.
- **Dashboard:** context timeline, regression alerts, run comparison, export.
- **Open-source core:** Apache-2.0 CLI and GitHub Action.

### Out of scope

- Hosting models or paying for inference (customer keys only).
- A full observability platform (integrate LangSmith/Langfuse/Braintrust).
- Prompt authoring/IDE; we test changes, not write them.
- Agent runtime or harness.

---

## 4. ICP and personas

| Attribute | Value |
|---|---|
| Company | B2B SaaS/AI teams with 20–500 engineers |
| Trigger | Weekly prompt/tool/retrieval changes; production quality incidents |
| Buyer | Head of AI Platform/DevEx, VP Engineering |
| User | AI engineer, platform engineer, QA |
| Current tools | LangSmith, Braintrust, Langfuse, promptfoo, DeepEval, spreadsheets |
| Budget | $250–$2,000/month; developer tooling line |

**Personas:**
- **AI engineer:** “Tell me which prompt/tool change broke this workflow before I merge.”
- **DevEx/platform:** “Make context changes reviewable like code.”
- **QA:** “Give me a repeatable regression suite for agent behaviour.”

---

## 5. Competitive landscape

| Competitor | Type | Pricing (public) | Strength | Weakness vs Context CI |
|---|---|---|---|---|
| [promptfoo](https://www.promptfoo.dev/) | OSS/direct | Free OSS; enterprise | Strong CLI/CI, evals, red-teaming; developer adoption | Eval-centric; no context manifest/diff/attribution across prompt+tool+retrieval changes |
| [DeepEval](https://deepeval.com/) | OSS/direct | Free OSS; cloud paid | Unit-test framework for LLMs; CI-friendly | Test framework, not a context-versioning product |
| [LangSmith](https://www.langchain.com/langsmith) | Platform/adjacent | $39/seat/month + usage | Traces, evals, prompt hub, Insights | Vendor-centric; context diff/attribution is manual |
| [Braintrust](https://www.braintrust.dev) | Platform/adjacent | Free; $249/month + usage | Evals, Loop, prompt edits, PRs | Pre-production eval bias; no context manifest |
| [Langfuse](https://langfuse.com) | OSS/adjacent | $0/$29/$199/$2,499 | Cheap, self-hostable, prompt management | Static versioning; no regression attribution |
| [Arize Phoenix](https://arize.com) | OSS/adjacent | Free OSS; enterprise | Tracing/evals, OSS credibility | Observability-first; no CI context gate |
| [Ragas](https://docs.ragas.io/) | OSS/adjacent | Free | RAG evaluation metrics | RAG-only; no CI workflow/attribution |
| [Humanloop](https://humanloop.com/) | Adjacent | Sunsetting/team joined Anthropic | Prompt/eval pioneer | Category risk: standalone prompt tooling struggles |

**Differentiation:** context manifest + semantic diff + failure attribution + CI gate. Not another eval dashboard. The wedge is **“git blame for agent behaviour.”**

**Platform risk:** high — LangSmith/Braintrust can add context diffs; keep the CLI open-source and cross-vendor.

---

## 6. Architecture

```
Repo / SDK / OTel
   │
Context manifest builder
   ├── prompts
   ├── tools/MCP schemas
   ├── retrieval config
   ├── model/params
   └── policies
   │
Manifest store (Postgres + object storage)
   │
Diff engine (semantic + structural)
   │
Regression runner (CI/local; customer model keys)
   ├── golden traces
   ├── assertions
   └── LLM-judge (optional)
   │
Attribution engine
   │
GitHub Action / CLI / dashboard / PR annotations
```

**Requirements:**
- Manifest schema versioned; canonical JSON; stable hashes.
- Diff handles prompt text, tool schema, retrieval config, model/params, policy.
- Runner executes locally/in CI; no hosted inference by default.
- Results reproducible: seed, model version, temperature recorded.
- PR check returns pass/fail with a concise cause report.
- No prompt content required for FinOps; Context CI requires customer-owned content.

---

## 7. Workstreams and owners

| Workstream | Owner | Deliverable |
|---|---|---|
| Product/schema | CEO/Product | Manifest schema, PRD, success metrics |
| CLI/Action | Backend | `contextci` CLI, GitHub Action |
| Diff/attribution | Backend/ML | Semantic diff, failure ranking |
| Runner/evals | Backend | Golden-set runner, assertions, judge |
| Dashboard | Frontend | Timeline, compare, alerts |
| Docs/DevRel | DevRel | Quickstart, examples, launch |
| GTM | CEO | 3 design partners, pricing |

---

## 8. Timeline and milestones

| Milestone | Date | Deliverable | Gate |
|---|---|---|---|
| C0 Launch squad | 17 Nov 2026 | Charter, public repo, 3 design partners, PRD | Squad staffed |
| C1 Manifest + CLI alpha | 24 Nov 2026 | Capture/diff from repo; local run | 3 repos |
| C2 CI runner beta | 2 Dec 2026 | GitHub Action, PR annotations | 5 repos |
| C3 Attribution + dashboard | 9 Dec 2026 | Failure ranking, compare view | 2 regressions caught |
| C4 Open-source launch | 16 Dec 2026 | Public Apache-2.0 v0.1, docs, launch | 10 repos by 31 Jan |
| C5 Paid GA gate | 28 Feb 2027 | 3 design partners, 2 paid/LOIs | ≥70% reuse; G4–G5 |

---

## 9. Build sprint detail

The 30-day launch is detailed in [30-day-launch.md](30-day-launch.md); this section covers the full Wave 1 build.

### Weeks 1–4 (launch sprint, 17 Nov–16 Dec 2026)
- 10 interviews with AI platform/DevEx engineers.
- Competitive teardown (promptfoo, DeepEval, LangSmith, Braintrust).
- CLI: `manifest` (the snapshot), `diff`, `check`, `classes`; manifest store and hashing. `init` is not implemented and is in scope for the 30-day plan.
- GitHub Action: PR check, annotations; local runner with 10 assertions.
- Attribution: map changed elements to failing tests.
- Open-source release; docs; examples; launch post.

### Weeks 5–8 (17 Dec 2026–15 Jan 2027)
- Harden manifest coverage (prompts, tools, retrieval, policy).
- Golden-set import from JSONL/CSV; pass rate/cost/latency comparison.
- Design partners #1–3 live; catch/verify 2 real regressions.
- Grow to 10 repos; fix false positives.

### Weeks 9–12 (16 Jan–15 Feb 2027)
- Dashboard: timeline, compare, alerts.
- Paid-tier groundwork: history, SSO, attribution reports.
- 3 design partners, 2 paid/LOIs.
- Integration with FinOps event pipeline.

### Weeks 13–15 (16–28 Feb 2027)
- Pricing: free OSS; Team $250/mo; Business $1,000/mo.
- GA-gate review; packaging as a FinOps add-on.

---

## 10. Pricing and GTM

| Plan | Price | Includes |
|---|---|---|
| Open source | Free | CLI, GitHub Action, local manifest/diff |
| Team | $250/month | Hosted history, 3 repos, 30-day retention, PR checks |
| Business | $1,000/month | 10 repos, SSO, 90-day retention, attribution dashboard |
| Enterprise | Custom | Unlimited, self-host, audit, SLA |

**GTM:** open-source launch, GitHub Marketplace, promptfoo/DeepEval comparison content, “context regression” benchmark, design partners. No paid ads.

**Positioning:** *“Git blame for agent behaviour — catch the prompt/tool/retrieval change that broke quality before it ships.”*

---

## 11. Team and budget

| Role | Allocation |
|---|---|
| Squad lead / CLI | 1.0 |
| Manifest / runner | 1.0 |
| CI / GitHub Action | 1.0 |
| Attribution / frontend / docs | 1.0 |
| DevRel | 0.3 |
| Platform | 0.2 |
| Design | 0.1 |

**Total:** 4 dedicated engineers; shared allocations are fractional and come from the Platform, DevRel, Design and Security squads.

**Budget:** ≤$100/month (GitHub, CI, small VM, object storage). No model inference costs by default.

---

## 12. Metrics

| Metric | Target |
|---|---:|
| Repositories using Action | 10 |
| Design partners | 3 |
| Regressions caught | ≥2 |
| Time to first PR check | <10 min |
| False-positive rate | <10% |
| Paid conversion | 2 customers/LOIs |
| Manifest coverage | prompts/tools/retrieval ≥90% |

---

## 13. Risks and mitigations

| Risk | Trigger | Mitigation |
|---|---|---|
| Crowded eval market | Low adoption | Focus on context diff/attribution, not generic evals |
| Platform bundles | LangSmith/Braintrust ships diff | Cross-vendor, open-source, CI-native |
| LLM-judge cost/variance | High cost | Deterministic assertions first; customer keys; sampling |
| Prompt content sensitivity | Security review | Self-host/local runner; no content upload by default |
| Weak monetisation | Free usage only | Paid history/SSO/attribution; sell as FinOps add-on |
| Scope creep | Requests for prompt authoring | Stay a test/CI product |

---

## 14. Optional platform integration and the paid-tier gate

Two separate decisions, deliberately not the same decision.

### 14.1 Platform integration (optional)

Integrate with the platform only if all are true:

1. The manifest digest is being referenced by at least one customer's action records.
2. The platform's `action-record` payload extension for `context_manifest_hash` has an approved ADR in `acp-core`.
3. Integration adds no required dependency: the tool still works with the platform absent.

This is never a condition for shipping this product. See [../05-contracts/README.md](../05-contracts/README.md) §4.

### 14.2 Paid tier (the February gate)

Advance to a paid tier only if all are true:

1. **Buyer:** ≥3 repositories have a named platform-team contact with budget.
2. **Pull:** ≥5 teams have asked for cross-repository history or longer retention.
3. **Economics:** hosting history costs less than 30% of the lower plan price at 10 paying repositories.
4. **Support:** expected ticket load is under 1 per paid repository per month.
5. **Focus:** the paid tier does not require building an authoring UI, a registry or a dashboard that competes with eval vendors.

If any is unmet, the tool stays free and is recorded as distribution. Revisit only if the buyer signal changes, not on a calendar.

---

## 15. What to do after each milestone

| After | If pass | If fail |
|---|---|---|
| C0 | Start the 30-day build | Descope to a CLI-only v0.1 and launch that |
| C1 | Add the CI runner | Fix manifest coverage before adding surface |
| C2 | Build attribution | Ship diff and gate without attribution; say so in the README |
| C3 | Public launch | Delay and fix false positives; a noisy gate is worse than no gate |
| C4 | Launch the paid Team tier | Keep it free; record the decision |
| C5 | Reference manifest digests from platform action records | Skip the integration; it is optional |

---

## 16. Definition of done

- CLI + GitHub Action capture and diff prompts, tools, retrieval and model config.
- Regression runner works locally/in CI with customer keys.
- PR annotations and pass/fail status.
- Attribution report ranks likely context causes.
- 10 repos, 3 design partners, 2 real regressions caught.
- Open-source release with docs and examples.
- GA/sell decision recorded.
