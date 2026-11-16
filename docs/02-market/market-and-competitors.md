# Market and competitors: Context CI

**What this file is.**
A market and competitor deep dive for Context CI, written on 29 October 2026, for the team that has to decide what to build and what to charge.
Every price in it was re-observed on that date; every factual claim carries a numbered source resolved in section 12.
It covers the job, the 2026 evidence that the job is now urgent, the buyers, 45 competitor dossiers, the substitutes teams actually use, the pricing landscape, open-source conversion mechanics, distribution, the structural gap, and ten ways this fails.
Where a number is not published we write **not public** rather than estimate silently; where a vendor describes itself we label it **vendor claim**.
Prices move — treat the observed date as part of the price, not as a footnote.

---

## 1. The job and the category

**The job.** Version-control the *inputs* to an AI feature, not the code. Concretely: produce, per commit, a **context manifest** — the exact rendered prompt bytes, the tool definitions and JSON schemas the model was offered, the retrieval configuration (index id, chunker, `top_k`, filters, embedding model and its version), the model id plus parameters (temperature, max tokens, seed, reasoning effort, provider endpoint), and the policy in force (guardrails, allow/deny lists, redaction rules, PII handling). Hash it, store it beside the commit, diff it between two commits, and attribute a behaviour, safety or cost change to the specific entry that moved.

**Why these inputs are unversioned today.** Each input lives somewhere different and none of them is a file in the repository that produced the behaviour:

| Input | Where it usually lives in 2026 | Reviewed? | In git? | Rollback? |
|---|---|---|---|---|
| Rendered prompt | Vendor prompt registry, env var, DB row, or console | No | Sometimes (template only) | Registry "labels", not commits |
| Tool definitions | Code, but the per-agent *selection* is config | Partly | Partly | Redeploy |
| Retrieval config | App config or dashboard | No | Sometimes | Re-index |
| Model id | Alias string in code or env | Yes, as a string | Yes, as a string | Alias floats |
| Model parameters | Code default or request-time override | No | Rarely | None |
| Policy / guardrails | Vendor dashboard | No | No | Manual |

The template may be in git. The **rendered** prompt is not, because rendering depends on runtime variables, retrieved chunks, tool results and provider-side templating. A diff of `prompt.md` is not a diff of what the model saw.

**What breaks, concretely.**

1. **Alias drift with no deploy.** A provider updates the model behind an alias; behaviour changes on a Tuesday with an empty git log. Real precedent: OpenAI shipped a GPT‑4o personality update that made the model sycophantic and then rolled it back — a behaviour regression with no customer code change at all [10].
2. **Deprecation cliffs.** OpenAI retires GA models on at least 6 months' notice, specialised variants on 3 months, and preview models on as little as 2 weeks [8]. Anthropic gives at least 60 days and then requests to retired models fail [9]. When the replacement lands, nobody can prove the old behaviour, because the old context was never recorded as an artifact.
3. **Prompt edited in a UI at 16:00.** No PR, no reviewer, no author beyond a registry audit log the engineer cannot query. The incident review cannot answer "what changed between Monday and Thursday".
4. **Retrieval swap.** Re-embedding a corpus with a new embedding model changes which chunks win. Quality moves on a slice nobody measures, because the eval set was frozen against the old index.
5. **Cost drift by config, not code.** Adding a 900-token tool definition to an agent called 200k times/day at $3 per million input tokens adds 900 × 200,000 × $3 / 1,000,000 = **$540/day** (arithmetic, not a measured case). It arrives as a cloud invoice, weeks later, with no commit to blame.
6. **Audit questions with no answer.** EU AI Act Article 12 requires high-risk systems to log events so that the functioning of the system is traceable in its "specific context and conditions of use"; Article 11 requires technical documentation kept up to date [5][6]. ISO/IEC 42001 asks for traceability and transparency as AIMS properties [7]. "Which model version, prompt and retrieval configuration produced this output in March?" is a question a file listing cannot answer.

**The category.** Not "LLM observability" (post-hoc traces), not "prompt management" (a registry), not "AI code review" (a reviewer of diffs). The closest existing category is **build provenance for non-code inputs** — the thing Nx and Bazel do for task inputs, narrowed to the context that determines model behaviour. Nobody sells that object today. The pitch that survives contact with an engineer is "`git blame` for agent behaviour, enforced by a CI check".

---

## 2. Why now

**Adoption is past the pilot line.** 84% of developers use or plan to use AI tools and 51% of professional developers use them daily; only 33% trust the output, 46% actively distrust it and 3% "highly trust" it [1]. Distrust plus daily use is exactly the condition that creates demand for verification rather than more generation.

**Agents became the unit of work in 2026.** GitHub reports that in September 2026 developers started more than 40% of pull requests with an agent, up from 17% in March; 61% of Copilot users used more than one model that month; and among repositories created in 2026 the average repo containing an agent configuration file logged **eight times** the commits of one without [2]. Model choice is now per-user and per-task, which multiplies the number of distinct contexts the team must reason about.

**The config surface became a file category.** Agent configuration files are now measurable enough for GitHub to report on them [2]. That is the leading indicator for this product: once context is a file, it can be versioned; once it is versioned, it can be diffed and gated. It also sets the trap — registries are pulling prompts *out* of repos. Langfuse now ships an agent skill whose documented purpose is to "migrate the prompts in this codebase to Langfuse" [15]. Migration into a registry is migration away from review.

**Model churn is structural, not incidental.** OpenAI publishes a `shutdown_date` per model and advises testing replacements well before retirement [8]; Anthropic tells customers to "consider thorough testing of your applications with the new models well before the retirement date" [9]. Both vendors are telling customers that a model swap is a change requiring evaluation — and neither ships the artifact that makes the comparison attributable.

**Regulation moved from consultation to dates.** The EU AI Act's remaining provisions apply from 2 August 2026, with Article 50(2) transparency duties for synthetic content phasing in by 2 December 2026 [4]. As currently stated, Articles 11 and 12 obligations bind from 2 December 2027 for Annex III systems and 2 August 2028 for Annex I systems, per Article 113(c) [5][6]. The practical read for a 2026 buyer: the documentation and logging duties are *known*, *dated*, and being designed into processes now, while enforcement is still ahead. That is when tooling gets chosen.

**A management-system standard exists to hang it on.** ISO/IEC 42001:2023 is a certifiable AI management system standard, CHF 225 for the text, whose stated benefits include traceability, transparency and reliability [7]. Certification creates a document request, not a dashboard request.

**Money is in the category even if the wedge is small.** Enterprise generative-AI spend reached $37B in 2025, up from $11.5B in 2024; $19B of it went to applications; coding was the first killer use case at $4B; and 76% of enterprise AI solutions are bought rather than built [3]. Buyers exist and they buy rather than build.

**The point-tool layer is consolidating.** In the window covered by this research: Humanloop's team joined Anthropic and the Humanloop platform is being sunset [23]; Launchable's pricing now resolves into CloudBees [50]; keywordsai.co resolves to Respan [27]; wandb.ai pricing resolves to CoreWeave Forge [29]; Gitar is now sold on Sonar's price list [39]. Five absorptions in one pass. A standalone "eval dashboard" is not a durable company in 2026; an artifact format with a CI gate might be.

---

## 3. Market sizing anchors

No published figure exists for "context manifest tooling". Any number presented as its TAM would be invented, so this section gives anchors and labels what each one is.

| Anchor | Figure | Year | Source | What it is | What it is not |
|---|---|---|---|---|---|
| Enterprise generative-AI spend | $37B (up from $11.5B) | 2025 | Menlo Ventures [3] | Total enterprise genAI spend, bottoms-up model | Not a TAM for CI tooling |
| Coding share of that spend | $4B | 2025 | Menlo Ventures [3] | Spend on coding assistants/agents | Adjacent budget, already allocated |
| Enterprise AI bought vs built | 76% bought | 2025 | Menlo Ventures [3] | Procurement posture | Not a price point |
| Developers on GitHub | 225M | Sep 2026 | GitHub Octoverse [2] | Platform population | Not addressable market |
| Merged pull requests | 1.3B in 12 months | 2026 | GitHub Octoverse [2] | Volume of the CI slot we attach to | Not buyers |
| GitHub Actions jobs | 45B in 12 months | 2026 | GitHub Octoverse [2] | Volume of CI executions | Not spend |
| Developer AI adoption | 84% using/planning, 51% daily | 2025 | Stack Overflow [1] | Adoption | Not willingness to pay |
| Per-seat price of an adjacent tool | $39/seat/mo (LangSmith Plus) | 2026 | LangSmith [11] | Anchors a per-seat ask | Not our price |
| Per-contributor price of engineering intelligence | $29–$59/contributor/mo | 2026 | LinearB [55] | Anchors a per-contributor ask | Not our price |
| AI code tools market forecast (Gartner) | **not verifiable on this date** — gartner.com returned HTTP 403 to automated retrieval | — | — | — | Do not quote it |

**Reading the anchors.** The category we sit inside is CI, and CI's unit economics are per-committer or per-CI-minute, not per-trace. The neighbouring price points that a platform team already approves are $20–$45 per developer per month for review and intelligence tools [34][59], and $29–$79 per month for the whole team on observability tools [14][26].

**Bottom-up sanity check (our arithmetic, not a market forecast).** 1,000 organisations at $250/month blended is $3.0M ARR. 5,000 organisations at $250/month is $15M ARR. At a $39/seat anchor [11], a 40-engineer team with 12 seats on the paid plan is $468/month. The question this document has to answer is not "how big is the market" but "does anyone pay for the gate", which section 8 and section 11 address.

---

## 4. Buyer map

| Buyer | Budget line | Trigger event | Who blocks | What they need to see |
|---|---|---|---|---|
| Individual engineer (OSS adopter) | None — personal time | A regression they cannot explain | Nobody; they just stop using it | Install in under 10 minutes, no account, no key upload |
| Tech lead / engineering manager | Team tooling, $20–$50/dev/mo | A bad release traced to a prompt or model change | Security review of data egress | A gate that fired before merge, with a named cause |
| Platform / developer-experience team | Platform budget, per-committer | Agent sprawl: N teams, N prompt registries | Their own roadmap capacity | A neutral format that does not pick a vendor |
| QA / quality engineering | Test tooling | Eval suite that nobody trusts or gates | The eval owner's sunk cost | Attribution, not another score |
| Compliance / risk / DPO | Audit and assurance | ISO 42001 certification, EU AI Act readiness, a customer security questionnaire | Legal (DPA, data residency) | Exported evidence tied to a commit and a date |
| Finance / FinOps | Cloud and AI spend | Token invoice spike | Procurement (new vendor) | Cost per PR and per feature, attributed to a change |
| Security / AppSec | AppSec tooling | Prompt-injection or data-leak finding | AppSec lead's tool consolidation mandate | Policy diff and enforcement at merge |

**The split that matters.** The **open-source adopter** is an individual engineer with no budget, who installs because a specific regression burned them and who will never talk to sales. The **economic buyer** is an engineering manager or platform lead who has already spent the observability budget and needs a *different* justification: merge safety and audit evidence. These two are not the same person, do not want the same artefact (the engineer wants a diff; the buyer wants a report), and a product that serves only the first will have excellent adoption and no revenue. Section 8 is about that gap.

---

## 5. Competitor dossiers

45 tools, grouped by the slot they occupy. Each entry: what it actually does, price (exact where published, observed 29 Oct 2026), funding/traction where public, strength, and the gap against Context CI.

### 5.1 LLM evaluation and observability (15)

**LangSmith** — Tracing, datasets, evaluators, a prompt hub, and an Insights agent for LangChain and non-LangChain stacks [13]. Price: Developer $0 with 5k traces/month; Plus $39/seat/month then pay-as-you-go; metered in LSU at $1.00 [11]. Funding: $125M at a $1.25B valuation [12]. Strength: it is where the trace already is, so adoption is free for LangChain users. Gap: the manifest is fragmented across a prompt object in LangSmith, a retrieval config in app code and tool schemas elsewhere; nothing is keyed by the deploying commit, so there is no object to diff and no check to fail.

**Langfuse** — Open-source tracing, prompt management with versions and labels, datasets and evals; cloud or self-host [15]. Price: Hobby $0 (50k units/mo), Core $29/mo, Pro $199/mo, Enterprise $2,499/mo, optional Teams add-on $300/mo, extra units $8/100k [14]. Traction: large OSS install base and a paid cloud; acquisition status not verified here. Strength: genuinely useful self-hosted, and prompt versioning that engineers accept. Gap: the registry, not the repository, is the system of record — "version 12" is a label Langfuse assigns, not a hash derived from a commit.

**Braintrust** — Eval-first platform: playgrounds, datasets, scorers, production logging, and a "Loop" agent that iterates on prompts. Price: Starter $0 (1 GB processing, 10k scores), Pro $249/month, Enterprise custom; additional data $3/GB, scores $1.50/1,000 on Pro [16]. Funding: not public on the pricing page. Strength: the best "which version scores better" workflow in the category. Gap: it compares runs you chose to run; it does not reconstruct the context a production request actually used, and it has no merge gate.

**Arize Phoenix** — Open-source, OpenTelemetry-native tracing and evaluation with experiments and a prompt playground [18]; managed as Arize AX. Price: AX Free $0, AX Pro $50/month, Enterprise custom [17]; Phoenix itself is free and self-hostable. Strength: OTel + OpenInference means it can accept almost any instrumentation — the closest architectural cousin to a neutral layer. Gap: instrumentation-first. The manifest is whatever fields you happened to log; it is not a first-class, hashed, diffable document.

**promptfoo** — OSS CLI that runs YAML-declared evals and red-team probes across a matrix of prompts and models, designed to run in CI [19][20]. Price: Community free forever (10k red-team probes/month); Enterprise and On-Premise custom [19]. Strength: this is the real objection — "we already run evals in CI". Gap: it tests a configuration matrix you wrote by hand and returns assertions; it does not capture what production actually sent, does not attribute a failure to one context delta, and has no cross-commit manifest diff.

**DeepEval (Confident AI)** — pytest-style LLM unit tests with a large metric library, run locally and in CI/CD. Price: Free forever $0 (2 seats, 1 project, 5 test runs/week); Starter $200/month; Team $2,000/month; Enterprise custom [21]. Strength: lowest-friction way to get assertions into a pipeline. Gap: metrics library plus cloud dashboard. It answers "did this assertion pass", not "which input changed and what did it cost us".

**Ragas** — Apache-2.0 Python metric library for RAG evaluation (faithfulness, context precision/recall and similar), now under vibrantlabsai [22]. Price: free OSS. Strength: the metrics people cite when they say "we measure retrieval quality". Gap: a library with no state. No history, no artifact, no gate, no attribution.

**Humanloop** — Was prompt management plus evaluation. The team has joined Anthropic and the platform is being sunset [23]. Price: not public / n/a. Signal, not competitor: the standalone prompt-management vendor did not survive as an independent company, which is a direct warning about building a registry-shaped product.

**LangWatch** — Agent testing and observability with simulations, scenarios, custom evals and prompt management; self-host available. Price: Developer €0 (50k events), Growth €29/core-seat/month with 200k events then €5/100k, storage €3/GB beyond 30 days, Enterprise custom [24]. Strength: simulation-first testing is a good fit for agent workflows. Gap: simulations are synthetic by construction; the product does not bind production traces to the commit that produced them.

**PromptLayer** — Prompt registry with versioning, request logging, and eval cells. Price: Free $0 (5 users, 2.5k requests/mo), Pro $49/month (pay-as-you-go $0.003/txn), Team $500/month ($0.002/txn), Enterprise custom [25]. Strength: one of the earliest prompt registries; deployment approvals and RBAC exist at enterprise tier. Gap: registry-centric workflow with no repository diff and no code-review surface. A prompt change is approved in PromptLayer, not in the PR.

**Helicone** — LLM gateway/proxy with request logging, caching, rate limits, alerts and HQL queries. Price: Hobby $0 (10k requests), Pro $79/month, Team $799/month, Enterprise custom [26]. Note: the pricing page carries a banner that Helicone "is no longer part of Mintlify" [26] — status in flux. Strength: proxy deployment means zero code change to start collecting. Gap: a proxy sees payloads, not repository state; it cannot tell you which commit produced a prompt.

**Keywords AI (now Respan)** — keywordsai.co/pricing now resolves to respan.ai: a model router with tracing, evals, red-teaming and budget limits [27]. Price: not public on the fetched page. Strength: routing plus observability in one API is attractive for cost control. Gap: gateway-first; context is an opaque payload, and there is no artifact lineage.

**Opik (Comet)** — Apache-2.0 LLM observability, test suites, assertions and guardrails; same codebase for OSS and cloud. Price: OSS $0, Free cloud $0 (25k spans, 10 members), Pro $19/month (100k spans, 50 members), Enterprise custom [28]. Strength: the most generous free tier in the category and a real OSS posture. Gap: traces and test suites; no manifest object, no diff, no merge gate.

**W&B Weave (now CoreWeave Forge)** — Tracing, scorers, evaluations and asset lineage; wandb.ai/site/pricing now resolves to CoreWeave Forge pricing. Price: Free $0, Pro from $60/month billed monthly, Enterprise custom [29]. Strength: lineage is in the product's DNA — model and dataset lineage is a solved concept there. Gap: lineage is for ML assets, not for prompt + retrieval + policy as a mergeable, diffable artifact; and the product now sits inside a GPU-cloud vendor's bundle.

**Galileo** — Evaluation plus an "Insights" engine that clusters failure modes and prescribes fixes, and "Luna" small models that distil LLM-as-judge evaluators into cheap guardrails (vendor claims: 20+ out-of-box evals, 96% lower cost) [30]. Price: not public — galileo.ai/pricing redirected to Splunk's pricing page on 29 Oct 2026 [30]. Strength: guardrails running on 100% of traffic is a real production need. Gap: it measures outputs and prescribes prompt fixes; it does not own the input diff, and the sales motion is enterprise-negotiated.

### 5.2 AI code review and PR context (9)

**CodeRabbit** — AI review on PRs plus CLI/IDE/agent loops, "learnings" from prior PRs, pre-merge checks, autofix, slop detection. Price: tiers Essentials / Team / Advanced / Enterprise; per-seat prices were not machine-readable at fetch time [31], observed 29 Oct 2026. Strength: the deepest PR-native workflow in the category. Gap: it reviews the code diff. A prompt change is a string change with no oracle, so the reviewer cannot say whether it made the feature worse.

**Qodo** — Agentic PR review plus test generation and a rules system; credit-metered (a credit is $0.012, pooled; packs of 2,500 / 5,000 / 20,000 credits, no permanent free tier) [32]. Price: usage-based, pack prices not stated on the page. Strength: rules plus tests is a credible "quality gate" story. Gap: code semantics only; no model, prompt or retrieval identity anywhere in the model.

**Greptile** — Codebase-graph code review. Price: Starter free (1 developer, 50 credits/month), Pro $1/credit with no seat fees (base review 1 credit, plus 3, apex 10), Enterprise custom [33]. Strength: whole-repo context produces fewer false positives than diff-only reviewers. Gap: repo context, not *model* context. It has no concept of what the LLM feature was told.

**Graphite Diamond** — Stacked PRs, AI review, merge queue and CI optimisation. Price: Hobby free, Starter $20/user/month billed annually, Team $40/user/month billed annually, Enterprise custom [34]. Strength: owns the merge path, which is exactly where a gate must live. Gap: throughput tool. "Did this change degrade the feature" is outside its model, and it has no eval or manifest primitive.

**Cursor Bugbot** — AI code review delivered inside Cursor's plan ladder; cursor.com/pricing now serves an xAI-bundled ladder (Plus $8, Premium $30, Super $100, Ultra $200 per month; Teams $30+/user/month) with "AI code reviews" listed from Premium upward [35]. Price: bundled, not separately priced on that page. Strength: distribution inside the editor where the code is written. Gap: review of code, and bundling means review is a feature of a seat, not a product with its own budget line.

**GitHub Copilot code review** — Native PR review that consumes GitHub AI Credits. Price: Free (limited), Pro $15/month, Pro+ $70/month, Max $200/month, with base credits $10/$39/$100 and flex allotment on top [36]. Strength: unbeatable distribution — zero install, already in the PR. Gap: it reviews code and its own model choice varies by user (61% of Copilot users used more than one model in September 2026 [2]) — the exact unversioned-context problem, inside the reviewer.

**Sourcegraph / Amp** — Code search and code graph, plus the Amp agent. Price: Hobby free (pay-as-you-go for hosted orbs), Individual $20/month with 45,000 minutes, Teams no extra charge, Enterprise custom [37]. Strength: deepest code graph, and a real agent product. Gap: an agent that changes code, not a verifier of behaviour. No manifest, no gate.

**Codacy** — SAST, quality, coverage and "AI guardrails" with policy checks including unapproved model calls. Price: Developer free forever, Team from $18/dev/month annual or $21 monthly, Business custom; free forever for OSS [38]. Strength: it already sells a *policy* gate in CI, including a rule about model calls — the closest existing rule primitive. Gap: static rules over code. It cannot express "this prompt change lowered faithfulness by 6 points on the refund slice".

**Sonar (SonarQube + Advanced Security + Gitar)** — Static analysis across 30+ languages, advanced SAST/SCA, and AI code review via Gitar; the vendor claims 7M developers and 75% of the Fortune 100 (vendor claim) [39]. Price: Team starts at $34/month including Advanced Security; Gitar AI Code Review $20/user/month billed annually; Enterprise custom [39]. Strength: quality-gate brand recognition and a billing relationship with exactly the buyer we want. Gap: "AI code assurance" means assurance about AI-*written* code. It says nothing about AI *behaviour*.

### 5.3 CI, affected graphs and build systems (7)

**Nx** — Monorepo build system with a project graph, affected-task computation, remote cache, distributed execution and "self-healing CI" that proposes fixes for failed tasks [40]. Price: Hobby free (50k credits), Team $29/month with $29 of usage, then $5.50 per 10k credits, $19 per contributor, $2.25 per concurrent CI connection, $99 dedicated compute, Enterprise custom [40]. Strength: the mental model we are copying — *task inputs are hashed, and a change to an input selects what runs*. Gap: inputs are files and env vars. A model version, a retrieval index or a guardrail setting is not an input it can hash, and it has no notion of semantic diff or behavioural attribution.

**Turborepo** — JS/TS build system with local and remote caching, positioned as "the build system for coding agents"; OSS, remote cache through Vercel [41]. Price: free OSS; hosted cache bundled with Vercel (not separately published). Strength: task-hash caching is table stakes and it normalised it. Gap: task hashing only; no artifact semantics beyond files.

**Bazel** — OSS, hermetic, multi-language builds with an action graph and remote execution/caching [42]. Price: free, Apache-2.0. Strength: hermeticity is philosophically the right answer — pin every input, rebuild and compare. Gap: no LLM primitives, and the adoption cost is prohibitive for the teams that have the problem.

**moonrepo** — OSS monorepo task runner (moon) plus toolchain version manager (proto); no paid tier published on the site as of 29 Oct 2026 [43]. Strength: clean task-hash model, strong DX reputation. Gap: same as Nx and Turborepo, with a smaller team and no commercial surface to fund a context feature.

**Depot** — Fast CI runners, container builds, test analytics and sandboxes. Price: Developer $20/month, Startup $200/month, Business custom; overage $0.00005/second/vCPU, $0.006/minute for a base 2-vCPU Linux runner, $0.000001 per passing test [44]. Strength: developers adopt it for speed in an afternoon, which is the adoption curve we want. Gap: speed, not semantics. It will never tell you a prompt regressed.

**Buildkite** — Hybrid CI: your agents, their orchestration. Price: Free (5 users, 10 concurrent jobs, 2,000 Linux vCPU minutes), Pro $30 per active user/month, Enterprise custom [45]. Strength: the platform team's choice at scale, and a natural host for a Context CI step. Gap: it runs pipelines; it has no opinion about what the pipeline should verify. Partner, not competitor, unless they build the check themselves.

**GitHub Actions** — The substrate: 45B jobs in 12 months, 1,440 jobs per second [2]. Price: GitHub Team $4/user/month and Enterprise $21/user/month, plus Actions minutes [46]. Strength: distribution and the required-status-check primitive. Gap: substrate. It is also the single largest platform risk — if GitHub ships a native context manifest, it inherits every repository.

### 5.4 Test, coverage and reporting in the same CI slot (8)

**Codecov** — Coverage upload, PR comments, status checks, patch coverage, flaky detection. Price: Developer free (open source unlimited, private 250 uploads), Team and Enterprise tiers; exact per-seat prices were not machine-readable at fetch time [47], observed 29 Oct 2026. Strength: the best template for our UX — one status check, one comment with a delta, one number that can block a merge. Gap: coverage is a number you upload, computed from code execution. It has no concept of a manifest, and it cannot attribute a change in *behaviour*.

**Coveralls** — Coverage reporting for private repos with unlimited users; Enterprise Cloud minimum order 40 seats at $1,400/month, i.e. $35/seat/month [48]. Strength: simple, long-lived, understood by buyers. Gap: same as Codecov, smaller surface.

**Datadog CI Visibility** — Pipeline and test tracing inside Datadog, adjacent to its Agent Observability product. Price: per-product, published on the Datadog pricing page [49]; exact CI Visibility unit price was not extracted on this date. Strength: if the company already pays Datadog, the marginal purchase is easy. Gap: telemetry, not artifacts. Datadog stores what happened; a context manifest must exist *before* anything runs, and Datadog's model has no pre-execution object and no semantic diff.

**Launchable** — Predictive test selection; pricing now resolves into CloudBees' custom-quote page [50]. Price: not public. Strength: the "run fewer tests, catch the same failures" pitch is the same budget as ours. Gap: it selects tests from history; it knows nothing about model or prompt identity.

**Trunk** — Flaky-test quarantine, merge queue and "Dynamic CI" run/skip decisions. Price: Free and Team both list $0/committer/month with 1M test spans per committer; Enterprise custom [51], observed 29 Oct 2026 — the paid boundary has moved to usage and the overage rate is not stated on the page. Strength: it owns the merge queue and the flaky-test narrative. Gap: test health only. A prompt is not a test, and a flaky verdict is not a context regression.

**Mergify** — Merge queue, CI insights and a test engine with quarantine. Price: Free for OSS and private teams up to 5 users, Max up to 100 users (price not shown on the page), Enterprise custom [52]. Strength: merge automation with a clear per-contributor model. Gap: queue mechanics; no artifact semantics.

**Danger** — OSS DSL for enforcing PR conventions from CI (Ruby, JS, Kotlin, Swift, Python) [53]. Price: free OSS. Strength: this is the plumbing we would imitate — a small runner that posts a structured comment and can fail a check. Gap: rules over the diff, no context model.

**Reviewdog** — OSS (MIT) tool that posts linter/analyzer results as review comments across CI providers [54]. Strength: the reference implementation of "any tool can comment on a PR from CI". Gap: same as Danger — transport, not semantics.

### 5.5 Engineering intelligence that claims to explain delivery (6)

**LinearB** — Delivery metrics, workflow automation, AI impact measurement and AI code review. Price: Essentials $29/user/month billed annually (50+ developers, GitHub Cloud only, 1,000 AI credits/user, extra credits from $0.015), Enterprise $59/user/month [55]. Strength: sells to exactly the VP Engineering who signs off on "why is delivery slower". Gap: measures humans and adoption. It can tell you AI usage rose; it cannot tell you which context change made the product worse.

**Jellyfish** — Engineering intelligence with an AI Impact suite including **AI token cost management** and vendor comparison. Price: not public on the pricing page [56]. Strength: already owns the "AI spend" conversation inside finance-aware orgs. Gap: cost is aggregated by team, vendor and model — not by prompt, retrieval config or merge. The cost half of our thesis is being sold, but without the attribution.

**Faros** — Now positioned as a "Token Engineering platform": connecting AI coding spend to shipped outcomes and routing tasks by evals built from your own code [57]. Price: not public. Strength: closest philosophical neighbour — it agrees that AI spend must be tied to outcomes and governed by evals. Gap: the unit is the task and the token, not the context manifest; there is no artifact, no diff, no merge gate.

**DX** — Developer-experience and delivery-intelligence platform with AI measurement and an "AI enablement" product that claims to build the context layer agents need. Price: not public; getdx.com/pricing redirects to the homepage [58]. Strength: the strongest research brand in the space and a real seat at the exec table. Gap: survey- and telemetry-based measurement. "Context layer" here means organisational knowledge for agents, not a versioned manifest.

**Swarmia** — Engineering effectiveness with modular pricing. Price: Standard $45/developer/month, Enterprise $55/developer/month billed annually; individual modules: AI adoption and cost $5, developer surveys $9, software capitalisation $18, productivity and AI impact $23; free for companies up to 9 developers [59]. Strength: module pricing makes the "AI cost" line easy to add to an existing contract. Gap: dashboards about adoption and cost, no per-change attribution.

**Sleuth** — Has pivoted to distributing AI assets: Skills and MCP servers synchronised to Claude Code, Codex, Cursor, Gemini CLI, Copilot and others through a CLI [60]. Price: not public. Strength: this is the most interesting adjacent move in the set — it is *config distribution*, one half of "version control the inputs". Gap: it distributes configuration to developers' machines; it does not keep a per-commit record of the context a feature is configured with, and it has no diff or gate.

---

## 6. Substitutes and the real competition

The competitor is almost never a vendor. It is one of these.

| Substitute | Cost | Failure mode | Why it survives | Signal it is in use |
|---|---|---|---|---|
| Do nothing | $0 | Regressions attributed to "the model being weird"; no rollback | Invisible cost; nobody is blamed for a change nobody made | No eval in CI; prompt edits via dashboard |
| Prompt files in git + code review | $0, ~10 min/PR | Reviews the template, not the rendered prompt, retrieval or parameters; reviewer has no oracle | Feels like engineering rigour; passes audit optics | `prompts/` directory, no manifest |
| Spreadsheet of prompt versions | 1–2 hours/week of one person | Stale within a sprint; no link to code; no diff | Cheap, legible to non-engineers, survives reorgs | A Confluence page titled "Prompt log" |
| LangSmith/Langfuse traces read manually after an incident | Seat cost already paid | Post-hoc, non-reproducible, depends on someone remembering a filter | The trace is genuinely useful once you know where to look | Screenshots of traces in incident channels |
| Eval dashboards nobody gates on | $200–$2,000/mo | Score drifts, nobody looks, no merge blocking | Buying the dashboard *is* the risk mitigation for the manager | Eval suite green while users complain |
| A staff engineer who "knows" | Their salary | Single point of failure; leaves; knowledge is not diffable | Fastest answer available today, and usually right | One person is @-mentioned on every prompt question |
| Vendor prompt registry used as the system of record | Seat cost | Version numbers assigned by the vendor; no tie to the commit; migration rewrites history | It is where the prompt editor already is | Prompts edited in a UI, repo has stale copies |

The honest read: substitutes 1, 2 and 6 dominate volume, and substitute 5 is what we actually have to displace at the buyer level. Substitutes 4 and 7 are the incumbent vendors, and they are good enough that "better eval" is not a wedge. The only wedge that survives is **attribution** — naming the change — plus **enforcement** at merge.

---

## 7. Pricing landscape

All prices observed 29 October 2026, USD unless stated.

| Vendor | Product | Unit | Price | URL | Model |
|---|---|---|---|---|---|
| LangSmith | Observability/eval | seat + LSU | $0 dev, $39/seat/mo, LSU $1.00 | langchain.com/pricing | Open-core-ish, sales-led enterprise |
| Langfuse | Observability/eval | units + seat | $0 / $29 / $199 / $2,499 per month | langfuse.com/pricing | Open-core, self-host free |
| Braintrust | Eval platform | platform fee + usage | $0 / $249 per month; $3/GB; $1.50 per 1k scores | braintrust.dev/pricing | Sales-led above Pro |
| Arize AX / Phoenix | Observability/eval | tier | $0 / $50 per month / custom | arize.com/pricing | Open-core (Phoenix OSS) |
| promptfoo | Eval CLI | tier | Community $0; Enterprise/on-prem custom | promptfoo.dev/pricing | Free-forever OSS + sales-led |
| DeepEval / Confident AI | Eval + cloud | tier | $0 / $200 / $2,000 per month | confident-ai.com/pricing | Open-core |
| Ragas | Metrics library | — | Free (Apache-2.0) | github.com/vibrantlabsai/ragas | Free OSS only |
| Humanloop | Prompt mgmt/eval | — | not public (platform sunset) | humanloop.com | Discontinued |
| LangWatch | Agent test/obs | seat + events | €0 / €29 per core-seat/mo; €5 per 100k events | langwatch.ai/pricing | Open-core |
| PromptLayer | Prompt registry | tier + txn | $0 / $49 / $500 per month; $0.002–$0.003 per txn | promptlayer.com/pricing | Sales-led |
| Helicone | LLM gateway | tier + usage | $0 / $79 / $799 per month | helicone.ai/pricing | Sales-led |
| Keywords AI / Respan | Router + obs | — | not public | respan.ai | Sales-led |
| Opik (Comet) | Observability/eval | tier | $0 / $19 per month / custom | comet.com/site/pricing | Open-core (Apache-2.0) |
| W&B Weave / CoreWeave Forge | Observability/eval | tier | $0 / from $60 per month / custom | coreweave.com/forge-pricing | Sales-led, bundle |
| Galileo | Eval + guardrails | — | not public (pricing redirects to Splunk) | galileo.ai | Sales-led |
| CodeRabbit | AI review | seat, tiered | Essentials/Team/Advanced/Enterprise; list prices not machine-readable | coderabbit.ai/pricing | Sales-led |
| Qodo | AI review | credits | $0.012/credit; packs 2,500–20,000 credits | qodo.ai/pricing | Sales-led |
| Greptile | AI review | credits | $0 or $1/credit; base 1, plus 3, apex 10 | greptile.com/pricing | Sales-led |
| Graphite | Review + merge queue | seat | $0 / $20 / $40 per user/mo annual | graphite.com/pricing | Sales-led |
| Cursor | Editor + review | seat bundle | $0 / $8 / $30 / $100 / $200 per month; Teams $30+ | cursor.com/pricing | Bundled subscription |
| GitHub Copilot | Review + agent | seat + credits | $0 / $15 / $70 / $200 per month | github.com/features/copilot/plans | Bundled subscription |
| Amp (Sourcegraph) | Agent | seat + minutes | $0 / $20 per month; Teams no charge; Enterprise custom | ampcode.com/pricing | Free tier + sales-led |
| Codacy | Quality/SAST | seat | $0 / from $18 per dev/mo annual ($21 monthly) | codacy.com/pricing | Sales-led |
| Sonar | Quality + AI review | seat + tier | Team from $34/mo; Gitar $20/user/mo annual | sonarsource.com/plans-and-pricing | Sales-led |
| Nx | Build system | credits + seat | $0 / $29 per month; $5.50 per 10k credits; $19/contributor | nx.dev/pricing | Open-core, usage |
| Turborepo | Build system | — | Free OSS (remote cache via Vercel) | turborepo.dev | Free OSS + platform bundle |
| Bazel | Build system | — | Free (Apache-2.0) | bazel.build | Free OSS |
| moonrepo | Monorepo tool | — | No paid tier published | moonrepo.dev | Free OSS |
| Depot | CI runners | seat + usage | $20 / $200 per month; $0.006/min; $0.000001/test | depot.dev/pricing | Usage |
| Buildkite | CI | active user | $0 / $30 per active user/mo / custom | buildkite.com/pricing | Sales-led |
| GitHub Actions | CI substrate | seat + minutes | Team $4/user/mo; Enterprise $21/user/mo + minutes | github.com/pricing | Platform |
| Codecov | Coverage | tier | Free (OSS); paid tiers, list prices not machine-readable | about.codecov.io/pricing | Sales-led |
| Coveralls | Coverage | seat | Enterprise Cloud from 40 seats at $1,400/mo ($35/seat) | coveralls.io/pricing | Sales-led |
| Datadog CI Visibility | CI telemetry | per product | Published per product on pricing page; exact CI unit not extracted | datadoghq.com/pricing | Usage |
| Launchable (CloudBees) | Test selection | — | not public (custom quote) | cloudbees.com/pricing | Sales-led |
| Trunk | Test health | committer + spans | Free and Team both $0/committer; 1M spans/committer; Enterprise custom | trunk.io/pricing | Usage / sales-led |
| Mergify | Merge queue | users + tier | Free ≤5 users; Max ≤100 users, price not shown; Enterprise custom | mergify.com/pricing | Sales-led |
| Danger | PR rules | — | Free OSS | danger.systems | Free OSS |
| Reviewdog | Review comments | — | Free (MIT) | github.com/reviewdog/reviewdog | Free OSS |
| LinearB | Eng intelligence | contributor | $29 / $59 per contributor/mo annual | linearb.io/pricing | Sales-led |
| Jellyfish | Eng intelligence | — | not public | jellyfish.co/pricing | Sales-led |
| Faros | Token engineering | — | not public | faros.ai | Sales-led |
| DX | Eng intelligence | — | not public | getdx.com | Sales-led |
| Swarmia | Eng intelligence | developer | $45 / $55 per dev/mo; modules $5–$23 | swarmia.com/pricing | Sales-led, modular |
| Sleuth | AI asset distribution | — | not public | sleuth.io/pricing | Sales-led |

**What the table says.** Six of the 45 are free-forever OSS with no commercial tier at all — Ragas, Turborepo, Bazel, moonrepo, Danger and Reviewdog — so they cannot be a business, only a dependency. Seven are open-core with a paid cloud or enterprise tier: Langfuse, Arize Phoenix, promptfoo, DeepEval, LangWatch, Opik and Nx. The remaining 32 are proprietary, bundled, or sales-led. Bundling is the 2026 trend — Copilot, Cursor, Graphite, Sonar with Gitar, CoreWeave with Weave — and a standalone per-seat product is being squeezed from above.

---

## 8. Open-source strategy analysis

**What actually converted.** Five mechanisms, with the evidence available in this research:

1. **Hosted convenience on an OSS core.** Langfuse gives away the full platform self-hosted and sells cloud at $29–$2,499/month [14]. The conversion is "I do not want to run ClickHouse, Postgres and Redis for this". LangSmith is the mirror image: proprietary, but free at the bottom and $39/seat above [11], funded at $1.25B to sustain it [12].
2. **Usage metering above a free allowance.** Nx: 50k credits free, then credits and per-contributor fees [40]. promptfoo: free community, paid enterprise and on-prem [19]. Opik: free cloud, $19/month Pro [28].
3. **Governance features.** SSO, RBAC, audit logs, retention, data residency. This is where every vendor puts the paywall — Langfuse's Teams add-on is literally SSO + RBAC + Slack support for $300/month [14].
4. **Deployment shape.** Self-host, on-prem, BYOC, air-gap. Trunk, Buildkite, Codacy, Sonar and promptfoo all price this as enterprise-only [51][45][38][39][19].
5. **Platform absorption.** Turborepo → Vercel, W&B Weave → CoreWeave Forge [29], Launchable → CloudBees [50], Humanloop → Anthropic [23], Gitar → Sonar [39], keywordsai → Respan [27]. When the OSS tool has no independent budget line, the exit is acquisition or shutdown.

**Conversion rates and failure rates.** Not public. No vendor in this set publishes install-to-paid conversion, and no independent study of OSS devtool conversion is available on this date. What *is* observable is the absorption pattern: five of the 45 vendors changed hands or shut down in the window covered here, all of them in the "adjacent to evaluation" band. Treat "OSS → company" as a low-base-rate event that requires a specific paid boundary, not a general hope.

**What this means for Context CI.**

- **Licence.** Apache-2.0 for the CLI, the runner and the manifest schema; no AGPL on the runtime. Rationale: the manifest must be *implementable by competitors and consumable by auditors*. A copyleft runtime makes CI legal review slower and makes the format impossible to standardise. The moat is the format plus the accumulated history, not the source.
- **Free forever, non-negotiable:** capture, hash, diff, local history, the CI gate, the GitHub/GitLab comment, and the schema. If the gate is paid, the OSS core is a demo and adoption dies.
- **Paid boundary that survives:** (a) hosted manifest history beyond N days and cross-repo aggregation; (b) SSO/RBAC/audit export and retention policy; (c) approval workflows and policy packs; (d) organisation dashboards and cost attribution across teams; (e) supported self-host and air-gap; (f) SLA. Every one of these is a thing a compliance buyer asks for in the first call and a solo engineer never asks for — the two audiences from section 4 get different products from the same binary.
- **Never paywall:** the word "diff", the word "gate", or the ability to read your own manifests.

---

## 9. Distribution

Ranked by expected installs per unit of effort for a team with no capital. Estimates below are planning judgements, not measured data.

| Rank | Channel | Specific action | Effort | Expected first-year installs |
|---|---|---|---|---|
| 1 | Framework integrations | Ship first-class capture for OpenAI Agents SDK, LangChain/LangGraph, Vercel AI SDK, LlamaIndex; PR the snippet into their docs | 25 person-days | 250–500 |
| 2 | GitHub Actions marketplace + reusable workflow | A one-line `uses: context-ci/gate@v1` step plus a copy-paste workflow; README optimised for search, not browse | 10 person-days | 150–400 |
| 3 | Design-partner outbound | 50 hand-picked teams with LLM features in production; install it for them on a call | 30 person-days | 40–80 (highest conversion, lowest scale) |
| 4 | Comparison content | "Your eval suite passed and users still complained: what a trace cannot tell you", "LangSmith vs a context manifest" — honest, technical, with reproductions | 15 person-days | 100–250 |
| 5 | Spec publication | Publish the manifest JSON Schema and a conformance suite; invite Langfuse/Opik/Phoenix to emit it | 12 person-days | 50–150 (compounding via citations) |
| 6 | Community launch | One Show HN, one r/LLMDevs, one r/ExperiencedDevs post, each with a real regression reproduction | 5 person-days | 80–200, decays in 72 hours |
| 7 | Conference and meetup talks | AI Engineer meetups, local platform-engineering meetups; live demo of attribution | 10 person-days | 30–80 |
| 8 | Newsletter sponsorship | Paid; excluded by the capital constraint | — | — |

**The honest note on the Actions marketplace.** It is crowded and it is not a discovery surface. The listing shows generic utilities at the top (checkout, cache, setup, linters, PR-summary bots) [61], and GitHub does not publish a total action count. Browse-driven discovery is close to zero; search-driven discovery ("prompt", "context", "regression") is where a new action can win, which is why the action's name and description matter more than its icon. The marketplace's real value is the *install mechanic* — a badge, a version tag, and a workflow snippet — not traffic.

**The one channel that compounds.** The schema. If the manifest format is published and other tools emit it, every integration is distribution that costs nothing to maintain, and the format becomes the thing buyers ask vendors to support. That is the only channel on this list that does not decay.

---

## 10. The structural gap

**What eval vendors cannot do, and why it is architectural rather than a roadmap gap.**

1. **They are metered on traffic; a gate prevents traffic.** LangSmith meters traces and LSUs [11], Langfuse meters units [14], Braintrust meters processed data and scores [16], Datadog meters per product [49]. A pre-merge check produces no traces because nothing has run yet. The unit that makes them money does not exist at the moment our product is most valuable.
2. **Their system of record is the trace, which is post-hoc and immutable.** A trace records what happened; a manifest must exist *before* execution and be addressable by content hash. Retrofitting "the artifact that produced this" into a trace store means adding a second, pre-execution object with different lifecycle semantics — a schema change across their whole product, not a feature flag.
3. **Their version numbers are theirs.** Langfuse prompt versions and labels [15], LangSmith prompt commits [13], PromptLayer deployment approvals [25] — in every case the version identifier is assigned by the vendor. It is not derived from the repository, so it cannot be tied to the commit that shipped, and it does not survive migrating registries.
4. **Attribution needs a counterfactual they never stored.** To say "this regression came from the chunker change", you must re-run the *old* context against the *new* inputs — which requires the retrieval snapshot, the exact rendered prompt, the tool schemas and the parameters from before. A trace usually has the rendered prompt and the response, often not the retrieval index identity, and almost never the guardrail policy.
5. **Their business model depends on being the system of record.** If git becomes the source of truth and the vendor becomes a sync target, the registry's lock-in disappears. That is a cannibalisation decision, not a product decision, and it is the same reason code hosts were slow to adopt external CI formats.

**If LangSmith or Braintrust ships a "context diff".** The plausible version is narrow: diff two prompt commits, compare two experiment runs, show the score delta. That is useful and it will ship. What it will not do, because of the above: hash the retrieval configuration and index identity; run pre-merge with no traces; block a merge as a required GitHub check with a named cause; cover a stack that is not theirs. **Response:** be the neutral layer that ingests their exports, publish the schema so their prompt objects can be *referenced* rather than replaced, and make the gate work identically on GitHub, GitLab and Buildkite. Precedent: Codecov survived GitHub's native coverage because the gate UX and multi-CI neutrality mattered more than the badge.

---

## 11. What would make this fail

1. **"We already have evals."** The most common objection, and often true in the sense that a suite exists. *Early signal:* in the first 10 design partners, 6+ ask "how is this different from promptfoo/LangSmith" and none reach a second integration; time-to-first-diff exceeds one day.
2. **Capture is too invasive.** If a manifest requires an SDK swap, a proxy or a framework migration, the install funnel dies at step two. *Early signal:* quickstart completion under 40%; support tickets about instrumentation dominate.
3. **Attribution is not trustworthy because the system is non-deterministic.** A gate that flips pass/fail on rerun is worse than no gate. *Early signal:* verdict instability above 2% on the same commit pair; users disabling the check "just for this PR".
4. **The counterfactual is too expensive.** Re-running old context against new inputs costs tokens and minutes on the customer's key. *Early signal:* median gate time above 10 minutes, or a per-PR cost the buyer notices; teams dropping to nightly runs.
5. **The platform absorbs it.** GitHub ships native manifests, or OpenAI/Anthropic ship a hosted "context record" that becomes the default. *Early signal:* fewer than 5 external implementers of the schema within two quarters; an Anthropic/GitHub job posting or changelog that describes our primitive.
6. **There is no buyer until there is an incident.** Observability budget is already allocated; a gate is a new line item that only appears after a bad release. *Early signal:* pilots convert to paid below 10%; every deal waits for a security questionnaire that never closes.
7. **Compliance buyers want certification, not a JSON file.** An auditor accepts a process and an attestation; a manifest is evidence *for* a process. *Early signal:* procurement asks "are you ISO 42001 certified" in the first call; deals stall on "who signs this".
8. **The free tier is too good or not good enough.** Too good: nobody upgrades, because self-host plus local history is the whole product. Too weak: the gate needs a server, so installs stall. *Early signal:* over 90% of installs self-host forever with under 1% conversion; or trial-to-install below 15%.
9. **Manifests contain secrets and customer data.** Rendered prompts carry PII, retrieved documents and occasionally keys. Storing them is a security review. *Early signal:* first enterprise security review flags manifest storage; DPA negotiation exceeds 60 days; a leaked manifest in a public repo becomes a case study.
10. **Fifty people, no on-call, one bad release.** A CI gate is on the critical path of every merge; downtime is visible and unforgivable. *Early signal:* p95 issue response above 5 days; churn after a release that broke a pipeline; a maintainer burnout spike in the first quarter after launch.

Cross-cutting: 1, 4 and 6 are the same failure wearing different clothes — **the product must show value before an incident**, or it only sells to teams that just had one. The design answer is to make the diff useful on day one without any gate at all: a PR comment that says "this PR changed the retrieval `top_k` from 8 to 20 and added 900 tokens per request" is worth reading even if the team never turns the gate on.

---

## 12. Sources

1. `https://survey.stackoverflow.co/2025/ai` — 84% using/planning AI, 51% daily, 46% distrust vs 33% trust, 3% highly trust, sentiment 60%.
2. `https://github.blog/news-insights/octoverse/octoverse-commits-pushes-reach-new-highs-in-2026-as-developers-build-with-agents/` — 17.9B pushes, 34.3B commits, 45B Actions jobs, 1.3B merged PRs, 40% agent-started PRs (Sep 2026) vs 17% (Mar), 8× commits for agent-config repos, 61% multi-model Copilot users, 225M developers.
3. `https://menlovc.com/perspective/2025-the-state-of-generative-ai-in-the-enterprise/` — $37B enterprise genAI spend 2025, $11.5B 2024, $19B apps, $4B coding, 76% bought.
4. `https://artificialintelligenceact.eu/implementation-timeline/` — 27 Jul 2026, 2 Aug 2026 general application, 2 Dec 2026 Article 50(2) and prohibitions, 2027–2028 stage.
5. `https://artificialintelligenceact.eu/article/11/` — technical documentation, Annex IV, entry into force 2 Dec 2027 / 2 Aug 2028 per Art 113(c).
6. `https://artificialintelligenceact.eu/article/12/` — record-keeping, logging for traceability, same staged dates.
7. `https://www.iso.org/standard/42001` — ISO/IEC 42001:2023 AI management system, CHF 225, traceability/transparency benefits.
8. `https://developers.openai.com/api/docs/deprecations` — 6-month GA / 3-month variant / ~2-week preview notice, `shutdown_date` field.
9. `https://platform.claude.com/docs/en/about-claude/model-deprecations` — 60-day notice, retired models fail, testing guidance.
10. `https://openai.com/index/sycophancy-in-gpt-4o/` — model update rolled back for sycophancy; behaviour change with no customer code change.
11. `https://www.langchain.com/pricing` — LangSmith Developer $0/5k traces, Plus $39/seat, LSU $1.00, Enterprise custom.
12. `https://www.langchain.com/blog/series-b` — $125M at $1.25B valuation.
13. `https://docs.langchain.com/langsmith/prompt-engineering-concepts` — LangSmith prompt objects and workflow.
14. `https://langfuse.com/pricing` — $0/$29/$199/$2,499, Teams add-on $300, $8/100k units.
15. `https://langfuse.com/docs/prompt-management/get-started` — prompt management and the agent skill that migrates prompts out of a codebase.
16. `https://www.braintrust.dev/pricing` — $0/$249/custom, $3/GB, $1.50 per 1k scores, Loop agent, Brainstore.
17. `https://arize.com/pricing/` — AX Free $0, AX Pro $50, Enterprise custom.
18. `https://arize.com/docs/phoenix` — Phoenix OSS, OTel/OpenInference, tracing/evals/experiments.
19. `https://www.promptfoo.dev/pricing/` — Community free forever, Enterprise and On-Premise custom.
20. `https://www.promptfoo.dev/docs/installation/` — CLI install, Node ≥22, CI usage.
21. `https://www.confident-ai.com/pricing` — Free $0, Starter $200, Team $2,000, Enterprise custom.
22. `https://github.com/vibrantlabsai/ragas` — Apache-2.0 RAG metrics library (repo moved from explodinggradients).
23. `https://humanloop.com/` — team joined Anthropic; platform sunset.
24. `https://langwatch.ai/pricing` — Developer €0, Growth €29/core-seat, €5/100k events, €3/GB storage.
25. `https://www.promptlayer.com/pricing` — $0/$49/$500, $0.002–$0.003 per txn.
26. `https://www.helicone.ai/pricing` — $0/$79/$799/custom; "no longer part of Mintlify" banner.
27. `https://www.respan.ai/` — redirect from keywordsai.co/pricing; router + observability + evals.
28. `https://www.comet.com/site/pricing/` — Opik OSS $0, Free cloud $0, Pro $19, Enterprise custom.
29. `https://coreweave.com/forge-pricing` — redirect from wandb.ai/site/pricing; Free $0, Pro from $60/mo.
30. `https://galileo.ai/` — eval + Insights + Luna guardrails (vendor claims); pricing redirects to Splunk.
31. `https://www.coderabbit.ai/pricing` and `https://docs.coderabbit.ai/faq` — tiers and capabilities.
32. `https://www.qodo.ai/pricing/` — $0.012/credit, packs 2,500/5,000/20,000, no permanent free tier.
33. `https://www.greptile.com/pricing` — free 50 credits, $1/credit, base 1 / plus 3 / apex 10.
34. `https://graphite.com/pricing` — Hobby free, Starter $20, Team $40 per user/mo annual, Enterprise custom.
35. `https://cursor.com/pricing` — xAI-bundled ladder $0/$8/$30/$100/$200, Teams $30+, AI code reviews from Premium.
36. `https://github.com/features/copilot/plans` — Free/Pro $15/Pro+ $70/Max $200, base credits $10/$39/$100.
37. `https://ampcode.com/pricing` — Hobby free, Individual $20/mo, Teams no charge, Enterprise custom.
38. `https://www.codacy.com/pricing` — Developer free, Team from $18/dev/mo annual ($21 monthly).
39. `https://www.sonarsource.com/plans-and-pricing/` — Team from $34/mo, Gitar $20/user/mo annual, 7M developers and 75% of Fortune 100 (vendor claim).
40. `https://nx.dev/pricing` and `https://nx.dev/docs/features/ci-features/self-healing-ci` — credits, $19/contributor, self-healing CI.
41. `https://turborepo.dev/` — OSS build system, remote cache, "build system for coding agents".
42. `https://bazel.build/` — OSS hermetic multi-language build.
43. `https://moonrepo.dev/` — moon and proto, no paid tier published.
44. `https://depot.dev/pricing` — Developer $20, Startup $200, $0.006/min, $0.000001/test.
45. `https://buildkite.com/pricing` — Free, Pro $30/active user/mo, Enterprise custom.
46. `https://github.com/pricing` — Team $4/user, Enterprise $21/user.
47. `https://about.codecov.io/pricing/` — free OSS, paid tiers, list prices not machine-readable.
48. `https://coveralls.io/pricing` — Enterprise Cloud minimum 40 seats at $1,400/mo.
49. `https://www.datadoghq.com/pricing/` — per-product pricing including CI Visibility and Agent Observability.
50. `https://www.cloudbees.com/pricing` — redirect from launchableinc.com/pricing; custom quote.
51. `https://trunk.io/pricing` — Free and Team $0/committer with 1M test spans, Enterprise custom.
52. `https://mergify.com/pricing` — Free ≤5 users, Max ≤100 users, Enterprise custom.
53. `https://danger.systems/` — OSS PR rules DSL across languages.
54. `https://github.com/reviewdog/reviewdog` — MIT, posts analyzer results as review comments.
55. `https://linearb.io/pricing` — Essentials $29, Enterprise $59 per contributor/mo annual.
56. `https://jellyfish.co/pricing/` — AI Impact suite incl. token cost management; prices not public.
57. `https://www.faros.ai/` — "Token Engineering platform", AI spend to outcomes.
58. `https://getdx.com/` — AI measurement and AI enablement; pricing not public.
59. `https://www.swarmia.com/pricing/` — Standard $45, Enterprise $55 per dev/mo, modules $5–$23.
60. `https://sleuth.io/pricing/` — AI Skills/MCP distribution to Claude Code, Codex, Cursor and others.
61. `https://github.com/marketplace?type=actions` — Actions marketplace listing; generic utilities dominate the surface.
