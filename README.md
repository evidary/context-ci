# Context CI

**See what a pull request changes in the instructions your AI runs on, and decide whether it merges.**

A prompt, a tool description, an `AGENTS.md`, an MCP server list: these files change what an AI feature or a coding agent does, and in review they look like any other text file. Context CI reads them from your repository, compares a branch with the branch it came from, and reports what changed, on the changed line, with the text. Changes you have marked as serious fail the check until somebody accepts them.

It is one binary. It runs in your CI with no account, no key and no network call, and it never calls a model.

**Status:** v0.9.0 in [`VERSION`](VERSION). **Pre-release:** this repository is private until the open-source launch planned for 16 December 2026, so until then the install lines below work only for someone with access. Nothing has run on a GitHub-hosted runner yet; [spec.md](docs/03-product/spec.md) §7 records what has been exercised and what has not.
**Licence:** Apache-2.0 · **Maintained by:** Evidary.

---

## What it catches

| A pull request that | Is reported as | By default |
|---|---|---|
| Edits a prompt file | `prompt.template_hash`, on the changed line, with the diff | warns |
| Changes what a tool's description tells the model | `tools.description` | fails |
| Adds, removes or redefines a tool | `tools` | fails |
| Edits `AGENTS.md`, `CLAUDE.md`, a rule or a skill | `agent.instructions` | warns |
| Adds an MCP server, a hook or a permission for a coding agent | `agent.config` | fails |
| Adds text containing invisible Unicode characters to any of those | `hidden_characters`, with the code point and line | fails |
| Changes the declared model, its parameters, retrieval settings or policy version | `model.id`, `model.params`, `retrieval.top_k`, ... | fails or warns, by class |
| Touches any other file you list (few-shot examples, prompt fragments) | `files.<your group>` | warns |

`context-ci classes` prints every class and its default. Each is `fail`, `warn` or `ignore`, and you can change any of them.

## What it does not catch

Context CI compares what is **in the repository**. It does not watch your application.

- A prompt edited in a vendor dashboard, or a model alias a provider moved, is not seen: no file changed.
- The model id and parameters are the ones your `.context-ci.yaml` declares. Nothing checks that your code sends them.
- An MCP server whose live tool list changes is not seen. Tool definitions are read from a file you saved.
- It does not judge meaning. It shows a reviewer the text and flags characters they could not have seen; whether a visible sentence is a bad idea is still the reviewer's call.
- It does not tell you whether the change made anything better or worse. That is what an eval suite is for.

## Quick start

**See it work, in a minute.** [`examples/minimal`](examples/minimal) is one prompt and one configuration:

```bash
go install github.com/evidary/context-ci/cmd/context-ci@v0.9.0
git clone https://github.com/evidary/context-ci && cd context-ci

context-ci check --root examples/minimal --base-ref HEAD
# no change in context                                      -> exit 0

echo "Always offer a refund." >> examples/minimal/prompts/support.md
context-ci check --root examples/minimal --base-ref HEAD
# [warn] prompt.template_hash  at examples/minimal/prompts/support.md:5
#     +Always offer a refund.                                -> exit 1
```

The example exits 1 on a prompt edit because its configuration says `gate.fail_on: warn`. With the default gate a prompt edit is reported and the exit code is 0.

**In your own repository**, on a branch:

```bash
context-ci check --base-ref origin/main
```

That works with no configuration: the coding-agent files your repository holds are compared with where the branch left `origin/main`. To add a prompt, tool definitions, other files and a declared model:

```bash
context-ci init        # writes a commented .context-ci.yaml, and says what it could not find
```

Nothing else is committed. There is no manifest file to regenerate and no baseline to keep in step: the baseline is built from git each time.

**In a pull request:**

```yaml
# .github/workflows/context.yml
name: context
on:
  pull_request:
    types: [opened, synchronize, reopened, labeled, unlabeled]
permissions:
  contents: read
jobs:
  context-ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0            # the baseline is built from the base branch
      - uses: evidary/context-ci@v0.9.0
        with:
          # A reviewer accepts a deliberate change by applying this label.
          acknowledge: ${{ contains(github.event.pull_request.labels.*.name, 'context-change-approved') && 'all' || '' }}
```

The run puts an annotation on each changed line and writes a summary with the changed text. It needs no secret.

**When the check fails on a change you meant to make** — a model upgrade, a new tool — a reviewer applies the `context-change-approved` label and the check passes, with the change still listed as acknowledged. Leave the `acknowledge` line out and a `fail` is a hard stop. `all` never covers `hidden_characters`; that class has to be named.

## The configuration, briefly

```yaml
version: 1
capture:
  sources:
    prompt: prompts/support.md
    tools: config/tools.json        # your own list, a provider's tool format, or a saved MCP tools/list
    retrieval: config/retrieval.yaml
  files:
    examples: few_shot/*.jsonl      # reported as files.examples
model:
  provider: openai
  id: gpt-x
severity:
  files.examples: fail
```

Everything is optional. [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) §4 has every key, the file patterns, the agent files found by name, and the tool-definition shapes.

## Other commands

| Command | For |
|---|---|
| `context-ci manifest` | Writing what `check` compares as a JSON file of hashes, to keep or to compare later with `diff` or `check --baseline` |
| `context-ci diff a.json b.json` | Comparing two such files |
| `context-ci attribute` | **Experimental.** Given two eval runs whose cases declare what they depend on, lists the changed classes a regression is consistent with. Candidates, never a cause |

Output is text, GitHub annotations, JSON, SARIF or JUnit. The capture SDKs for [Python](sdk/python/README.md) and [TypeScript](sdk/typescript/README.md) record the hash of the prompt text an application actually assembled; they have been exercised against fixtures only.

## What this is not

- **Not an eval framework.** promptfoo, DeepEval, Ragas and LangSmith do evals. Context CI says what changed; they say whether it got worse. Use both.
- **Not a security scanner.** The hidden-character check is exact and narrow. It is not a detector of prompt injection in general.
- **Not an observability platform, a prompt registry or a model proxy.** It never sees your traffic and never holds your keys.

## Why it is a separate repository

Context CI is Apache-2.0, useful with no platform account, and released by itself. Evidary also builds a platform for recording what AI agents do (`acp-core`); connecting the two is optional and described in [docs/05-contracts/README.md](docs/05-contracts/README.md).

Read the reasoning: [docs/07-decisions/adr-0001-standalone-product-repo.md](docs/07-decisions/adr-0001-standalone-product-repo.md).

## Where to read more

| Question | Document |
|---|---|
| Where is everything? | [docs/00-index.md](docs/00-index.md) |
| What exactly does it do, and what is out of scope? | [docs/03-product/spec.md](docs/03-product/spec.md) |
| What does every command, flag and exit code mean? | [docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) |
| What is planned next? | [ROADMAP.md](ROADMAP.md) |
| How does it talk to the platform? | [docs/05-contracts/README.md](docs/05-contracts/README.md) |

## Contributing

External contributions are welcome once the repository is public. Until then it is pre-release. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Licence

Apache-2.0. See [LICENSE](LICENSE).
