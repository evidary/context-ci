# Context CI — thesis

**Version:** 1.0.0 · **Date:** 29 October 2026
**Evidence:** [02-market/market-and-competitors.md](02-market/market-and-competitors.md) (61 sources, prices observed 29 Oct 2026)

---

## 1. The bet

> **In 2026 the inputs that determine AI behaviour — prompts, tool definitions, retrieval configuration, model parameters, policy — are the only part of the stack that is not version-controlled. Everything that breaks in production now comes from that gap. Version the inputs, gate the changes, and attribute the regressions, and you get the one object nobody else sells: a diff for behaviour.**

The bet has three parts, and all three must hold:

1. Context changes are a real and recurring cause of incidents, cost spikes and audit questions — not a hypothetical.
2. Those changes can be captured without asking the developer to rewrite their application.
3. Somebody will pay for the history and the gate, even though the capture itself is free.

Part 3 is the weak one. Section 7 is about it.

## 2. The evidence

| Claim | Evidence | Source |
|---|---|---|
| Developers use AI daily and do not trust it | 84% use or plan to; 51% of professionals daily; 46% distrust vs 33% trust; only 3% highly trust | Stack Overflow 2025, via market research §2 |
| Agents are now the unit of work | 40%+ of pull requests started with an agent in Sep 2026, up from 17% in March | GitHub, via market research §2 |
| Context configuration is now a file category | Repos with an agent config file logged 8× the commits of those without | GitHub, via market research §2 |
| Model churn is structural | OpenAI publishes `shutdown_date` per model; specialised variants on 3 months' notice, previews as little as 2 weeks | OpenAI docs, via market research §1 |
| Config, not code, moves cost | Adding a 900-token tool definition to a 200k-call/day agent at $3/M input adds ~$540/day | arithmetic in market research §1 |
| Documentation and logging duties are dated | EU AI Act Art. 11/12 for Annex III from 2 Dec 2027, Annex I from 2 Aug 2028; Art. 50(2) phasing from 2 Dec 2026 | EU AI Act, via market research §2 |
| A management-system standard exists to hang it on | ISO/IEC 42001:2023, certifiable, traceability and transparency named as benefits | ISO, via market research §2 |
| The point-tool layer is consolidating | Humanloop → Anthropic (sunset), Launchable → CloudBees, W&B → CoreWeave, Keywords AI → Respan, Gitar → Sonar, all in the research window | market research §2, §8 |

## 3. Who buys it

Two buyers, and confusing them is the main commercial risk.

| Buyer | What they want | What they pay for |
|---|---|---|
| **The adopter** — an individual engineer or a small team lead | A CI check that stops a dumb context change reaching production | **Nothing.** They install, they star, they file issues. They are distribution, not revenue |
| **The economic buyer** — platform engineering lead, AI platform team, quality or compliance owner | Cross-repository history, attribution, an audit-ready record of what changed and who approved it | A Team or Business plan |

The open-source product must be genuinely complete for the adopter. The paid product must be something the adopter cannot use alone: **history retention across many repositories, attribution dashboards, SSO, and an exportable record.**

## 4. The wedge

> **One repository, one AI feature, one PR check that fails when the context diff correlates with a measurable regression.**

The first version does not need to be clever. It needs to be *believable*: a developer installs it, it produces a manifest, it diffs against the base branch, and on the first real regression it points at the right line. That moment is the whole product.

What makes it adoptable in a week:

- No account, no key, no data leaving the CI runner.
- Works with any provider, because it never calls one.
- Twenty seconds of CI time.
- A failure message that names the change, not just a score.

## 5. What nobody else does

The research covers 45 competitors across five segments. The pattern:

| Segment | Why they cannot occupy this position |
|---|---|
| LLM observability and evals (LangSmith, Langfuse, Braintrust, Arize, promptfoo, DeepEval) | They meter traffic. A pre-merge gate produces **zero traces**, so it sits outside their revenue model. Their object is a trace; ours is a diff |
| AI code review (CodeRabbit, Qodo, Greptile, Graphite, Copilot review) | They review code diffs and never see the rendered context, the retrieval index or the model parameters |
| CI and build systems (Nx, Turborepo, Bazel, moonrepo) | They already do affected-input analysis — for files. None models prompt, tool schema, retrieval config or policy as an input |
| Coverage and CI reporting (Codecov, Datadog CI Visibility, Trunk) | They report on tests, not on the context that determines model behaviour |
| Engineering intelligence (LinearB, Jellyfish, Faros, DX) | They infer delivery health from events. They do not record configuration |

**The structural gap, stated precisely:** every eval vendor prices per trace, per seat or per evaluation run. A merge gate consumes none of those units. The only way for an eval vendor to sell this is to change what they meter, which is why the gap has stayed open.

## 6. The four things that must be true

| # | Must be true | How we will know | When |
|---|---|---|---|
| 1 | A context manifest can be produced for a real application without the developer rewriting it | An outside team produces a manifest for their own repo in under an hour | Week 4 |
| 2 | A semantic diff identifies a real regression that a text diff would miss | Demonstrated on a seeded regression, with the correct attribution | Week 6 |
| 3 | Installation is under five minutes and CI cost under 30 seconds | Timed, by someone outside the team, from public docs | Week 8 |
| 4 | A team-level buyer exists, not just individual adopters | Five conversations where a named person owns the budget | Week 12 |

If (4) fails, this is a well-loved open-source tool with no business — which is the failure case in section 7, and the point at which the product should stay free and be used as distribution for the platform.

## 7. The honest failure case

**The most likely failure is not technical.** It is absorption or irrelevance:

- **Absorption.** A registry or observability vendor ships a context diff as a feature. Five acquisitions in the research window show that standalone point tools in this layer get bought or die. Langfuse already ships a documented "migrate your prompts to Langfuse" skill — migration into a registry is migration away from review.
- **Irrelevance.** Teams keep prompts in the repo, review them by hand, and consider that sufficient. The gate then has nothing to add for them. The research's own conclusion is that "prompt files in git plus a staff engineer" is the real competitor, and it is a strong one.
- **Free-forever.** Every adopter loves it, nobody buys. Mitigated only by making the paid boundary a capability a single developer cannot use: multi-repo history and attribution.

**The trap to avoid:** becoming an eval dashboard. Once this product tries to score outputs, it competes with promptfoo and LangSmith on their terms and loses on distribution. Stay narrow: capture, diff, attribute, gate.

## 8. Why this repo, why this licence

Apache-2.0 because the credibility of this product is the point. It is the one thing in the portfolio that a stranger can read end to end and conclude the team is real. It must be forkable, readable and free of references to closed code.

Repository separation is recorded in [07-decisions/adr-0001-standalone-product-repo.md](07-decisions/adr-0001-standalone-product-repo.md).

## 9. What we refuse to build

| Refused | Why |
|---|---|
| An eval framework or scoring library | Crowded, and it destroys the positioning |
| A prompt registry | Being disrupted; also puts us against the vendors we integrate with |
| A model proxy or gateway | Would make us a traffic path and a cost line |
| A hosted trace store | Different product, different buyer, and a data-protection burden |
| Anything that requires an account for the core check | Kills the adoption mechanic |
