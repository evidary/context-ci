# Roadmap

**Status on 16 November 2026:** version 0.9.0, pre-release. The repository is public and has not been announced.
**Public launch target:** 16 December 2026.

This page is what is true now.

---

## Built

| Area | What exists |
|---|---|
| Capture | A prompt, tool definitions in the common provider shapes or a saved MCP `tools/list`, retrieval and guardrail files hashed by value, any other files in named groups, coding-agent files found by name, and a declared model. A count of hidden characters in each. Python and TypeScript capture SDKs for the rendered prompt, exercised against fixtures only |
| Compare | `diff`, with a severity for each class of change; `classes` lists them |
| Gate | `check --base-ref`, which builds the baseline from git and the head from the working tree, reports each change with its file, line and text, and exits 1 at or above the configured threshold unless the change is acknowledged |
| Attribute | `attribute`, experimental: lists the changed classes a regression in two eval runs is consistent with, and says when it can only correlate |
| Setup | `init`, the GitHub Action in `action.yml`, and `examples/minimal` |
| Output | Text, GitHub annotations on the changed line, a Markdown job summary, JSON, SARIF and JUnit |

[docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) §8 lists what is not implemented.

## Before the public launch

| Item | Why it is not done |
|---|---|
| Run the action on a real pull request | CI and the action pass on hosted Linux, macOS and Windows runners. The action's default base branch on a `pull_request` event has not been exercised, because no pull request has been opened against this repository since it was written |
| Check the tool-definition shapes against files exported from each vendor's tooling | They were written from the documented formats and are covered by fixtures only |
| Run it against more public repositories that hold agent files and prompts | Three were tried and every report was a real edit; that is too few to call a false-positive rate |
| Publish the 0.9.0 release | The documents pin `v0.9.0`, and the tag does not exist yet. Until it does, the install lines and `uses: sigilary/context-ci@v0.9.0` do not resolve |
| Someone outside the team installs from the README | Nobody has been asked yet |
| Tag the release the documents pin | The install lines name a version that has to exist as a tag |
| Decide on signed releases and an SBOM | Releases carry SHA-256 checksums only |
| Publish the manifest as a JSON Schema, with a stability promise | Open decision D-01 in [docs/07-decisions/README.md](docs/07-decisions/README.md) |
| A way to contact the maintainers other than GitHub | No project mailbox exists yet |

## After launch

Plans, not commitments. Each depends on what the first users ask for.

| Item | Note |
|---|---|
| Prompts from a registry as a source | Undecided; it conflicts with the "no network call" requirement. See cli-reference §8 |
| Observed capture beyond the prompt | Today the model, parameters and retrieval settings are declared, not observed |
| A comment on the pull request | The report goes to annotations and the job summary today |
| Reading a live MCP server's tool list | Today a saved file is read |
| Hosted history across repositories | The planned paid boundary. Nothing is built, and the decision whether to sell it is planned for the end of February 2027 |

## Not doing

| Not doing | Why |
|---|---|
| An eval framework | Others do it well; this tool consumes their results |
| Prompt authoring or a registry UI | A different product |
| A model proxy or gateway | It would put the tool in the request path |
| IDE-first distribution | The gate belongs in CI |
