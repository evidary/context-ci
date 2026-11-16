# Roadmap

**Status on 16 November 2026:** version 0.8.0, pre-release. The repository is private.
**Public launch target:** 16 December 2026.

The original plans are kept for history in [docs/04-plan/](docs/04-plan/30-day-launch.md). This page is
what is true now.

---

## Built

| Area | What exists |
|---|---|
| Capture | `manifest` from files and declared configuration (asserted); the rendered prompt hash from an SDK capture (instrumented). Python and TypeScript capture SDKs |
| Compare | `diff`, with a severity for each class of change; `classes` lists them |
| Gate | `check`, which builds its head from the working tree and exits 1 at or above the configured threshold |
| Attribute | `attribute`, which lists the changed classes a regression in two eval runs is consistent with, and says when it can only correlate |
| Setup | `init`, the GitHub Action in `action.yml`, and `examples/minimal` |
| Output | Text, GitHub annotations, JSON, SARIF and JUnit |

[docs/03-product/cli-reference.md](docs/03-product/cli-reference.md) §8 lists what is not implemented.

## Before the public launch

| Item | Why it is not done |
|---|---|
| Run CI and the action on GitHub-hosted runners | Everything was run locally; nothing has run on a hosted runner yet |
| Someone outside the team installs from the README | Impossible while the repository is private |
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
| Hosted history across repositories | The planned paid boundary. Nothing is built, and the decision whether to sell it is planned for the end of February 2027 |

## Not doing

| Not doing | Why |
|---|---|
| An eval framework | Others do it well; this tool consumes their results |
| Prompt authoring or a registry UI | A different product |
| A model proxy or gateway | It would put the tool in the request path |
| IDE-first distribution | The gate belongs in CI |
