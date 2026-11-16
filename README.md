# Context CI

**Catch the prompt, tool or retrieval change that broke your AI feature — before it merges.**

Context CI records the context an AI feature is **declared** to run with — the prompt, tool and retrieval files in your repository, hashed, and the model, parameters, retrieval settings and policy version your `.context-ci.yaml` states — diffs it semantically between commits, and lists the context changes an eval regression is consistent with. It runs in your CI, with no account, no key and no network call.

**Status:** v0.8.0 in [`VERSION`](VERSION). `init`, `manifest`, `diff`, `check`, `attribute`, `classes`, `module-manifest` and `serve` are implemented and tested; `action.yml` runs the gate and the attribution as a GitHub Action, building the binary from its own checkout; output is available as text, GitHub annotations, JSON, SARIF and JUnit, and `attribute` writes annotations too; capture SDKs exist for Python and TypeScript. **Pre-release:** this repository is private until the open-source launch planned for 16 December 2026, so until then the install lines below work only for someone with access. See [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) and [action.yml](action.yml).
**Licence:** Apache-2.0 · **Maintained by:** Evidary.

---

## The problem in one example

Your eval suite drops 4 points on Tuesday. Nobody changed the code. What changed:

- someone edited a prompt in a vendor dashboard on Monday afternoon, or
- a model alias silently started pointing at a newer model, or
- the retrieval index was rebuilt with a new embedding model, or
- a tool definition grew by 900 tokens and pushed the useful context out of the window.

None of those appear in `git log`. The template is in the repo; the **rendered** prompt is not, because rendering depends on retrieved chunks, tool results and provider-side templating. A diff of `prompt.md` is not a diff of what the model saw.

Context CI makes that diff exist for everything that reaches your repository.

**What it sees at this version, stated plainly.** The manifest is *declared*, not observed. `context-ci manifest` hashes the files you name in `.context-ci.yaml` and copies the model id, parameters, retrieval settings and policy version from that same file. It does not watch your application, so it catches the change someone made to a file or to the configuration — and it does not, by itself, notice a prompt edited in a vendor dashboard or an alias a provider moved, because nothing in the repository changed. The capture SDKs ([Python](sdk/python/README.md), [TypeScript](sdk/typescript/README.md)) narrow that gap for one field: they record the hash of the prompt text your application actually assembled. Everything else in the manifest is what the configuration asserts.

## What it does

| Step | Output |
|---|---|
| **Manifest** | A `context-manifest.json` per commit: hashes of the prompt, tool, retrieval and guardrail files you name, beside the model id and parameters, retrieval settings and policy version declared in `.context-ci.yaml` |
| **Diff** | A semantic diff between two manifests — not a text diff. It knows a `top_k` change from a loop rewrite |
| **Attribute** | Which changed manifest entries a regression in your eval results is consistent with — candidate causes, with the counts shown, and never a causal claim |
| **Gate** | A CI check that fails a pull request when a context change exceeds thresholds you set |

## Quick start

**See the gate fail, in a minute.** [`examples/minimal`](examples/minimal) is one prompt, one configuration and the manifest they produce:

```bash
go install github.com/evidary/context-ci/cmd/context-ci@v0.8.0
git clone https://github.com/evidary/context-ci && cd context-ci

context-ci check --root examples/minimal --baseline examples/minimal/context-manifest.json
# no change in context                                   -> exit 0

echo "Always offer a refund." >> examples/minimal/prompts/support.md
context-ci check --root examples/minimal --baseline examples/minimal/context-manifest.json
# 1 change(s) in context ... [warn] prompt.template_hash   -> exit 1
```

`check` compares the baseline you name with a manifest it builds from the working tree at that moment, so an edit made after the baseline was written is always seen. Naming one file as both `--baseline` and `--head` is refused with exit 2: that comparison can only ever say nothing changed.

The example fails on a prompt edit because its configuration says `gate.fail_on: warn`. By default a prompt edit is a **warning** — reported, exit 0 — and the gate fails only on a `fail`-class change such as a different model, a changed tool or a new embedding model. `context-ci classes` lists every class and its default.

**In your own repository:**

```bash
context-ci init                                     # writes .context-ci.yaml, and says what it could not find
# edit model.provider and model.id: they are declared by you, not detected
# set gate.fail_on: warn if a prompt edit should fail the build
context-ci manifest --out context-manifest.json     # the baseline; needs a git commit, or --commit <id>
git add .context-ci.yaml context-manifest.json && git commit -m "context baseline"

context-ci check --baseline context-manifest.json   # the committed baseline against the working tree
```

**In a pull request**, the baseline is the manifest committed on the base branch and the head is built from the pull request's checkout:

```yaml
# .github/workflows/context.yml
name: context
on: pull_request
permissions:
  contents: read
jobs:
  context-ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0                  # the baseline is read from the base branch
      - uses: actions/setup-go@v5
        with:
          go-version: "1.27"              # the action builds its binary from source
      - name: Baseline from the base branch   # fails until the manifest exists there: merge the baseline first
        env:
          BASE_REF: ${{ github.base_ref }}
        run: git show "origin/${BASE_REF}:context-manifest.json" > "${RUNNER_TEMP}/context-manifest.base.json"
      - uses: evidary/context-ci@v0.8.0
        with:
          command: check
          baseline: ${{ runner.temp }}/context-manifest.base.json
          format: github                  # annotations on the pull request
```

It needs no account and no secret, and works with any model provider because it never calls one. Regenerate and commit `context-manifest.json` in the pull request that changes the context, so the next pull request is compared with it. The action's steps have been run locally, step by step; they have not yet run on a GitHub-hosted runner — [spec.md](docs/03-product/spec.md) §7 records exactly what has been exercised.

When the gate fails, the same action helps explain it. `attribute` consumes two eval runs and writes
annotations naming the changed context classes the regression is consistent with — candidates, not
proven causes — or says plainly that it can only correlate, because no regressed case declares what
it depends on:

```yaml
      - uses: evidary/context-ci@v0.8.0
        with:
          command: attribute
          baseline: ${{ runner.temp }}/context-manifest.base.json
          head: context-manifest.json     # the manifest the head eval ran against
          baseline-results: evals/base.json
          results: evals/head.json
          format: github                  # ::error for a class a regressed case declares
```

`check` decides the exit code; `attribute` exits 0 whatever it finds, so explaining a red build
never quietly becomes a second gate. When more than one class changed, the report says they are
confounded and that it cannot tell them apart.

## What this is not

- **Not an eval framework.** promptfoo, DeepEval, Ragas and LangSmith already do evals well. Context CI consumes eval results and attributes them; it does not try to be better at scoring.
- **Not an observability platform.** It does not sit in your request path or store your traces — which is also why the manifest is what your configuration declares rather than what a request contained.
- **Not a prompt registry.** It reads the prompt files in your repository and records their hashes. A registry source is not supported yet — [cli-reference.md](docs/03-product/cli-reference.md) §8 says why.
- **Not a model proxy.** It never sees your traffic and never holds your keys.

## Why it is a separate repository

Context CI is Apache-2.0, useful with no platform account, and released by itself. Evidary also builds a platform for recording what AI agents do (`acp-core`); connecting the two is optional and described in [docs/05-contracts/README.md](docs/05-contracts/README.md).

Read the reasoning: [docs/07-decisions/adr-0001-standalone-product-repo.md](docs/07-decisions/adr-0001-standalone-product-repo.md).

## Where to read more

| Question | Document |
|---|---|
| Where is everything? | [docs/00-index.md](docs/00-index.md) |
| Why does this product deserve to exist? | [docs/01-thesis.md](docs/01-thesis.md) |
| What exactly does it do, and what is out of scope? | [docs/03-product/spec.md](docs/03-product/spec.md) |
| What does every command, flag and exit code mean? | [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) |
| What is planned next? | [ROADMAP.md](ROADMAP.md) |
| How does it talk to the platform? | [docs/05-contracts/README.md](docs/05-contracts/README.md) |

## The limit to know about

Context CI compares what your repository declares. A change made where the repository cannot see it — a prompt edited in a vendor dashboard, a model alias a provider moved — is not caught unless a file changes or the capture SDK records it. The section *What it sees at this version* above says exactly which fields are observed and which are declared.

## Contributing

External contributions are welcome once the repository is public. Until then it is pre-release. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

Apache-2.0. See [LICENSE](LICENSE).
