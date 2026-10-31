# CLI reference

**Version:** 1.0.0 · **Date:** 31 October 2026 · **Owner:** Context CI squad

The four verbs in [the specification](spec.md) §1. `attribute` is specified but not
yet implemented; the other three are, and this page is the contract for them.

---

## 1. Install and first run

```bash
context-ci manifest          # writes context-manifest.json for this checkout
```

That is the whole quickstart. No account, no network call, no secret.

## 2. The verbs

| Verb | Command | What it does |
|---|---|---|
| Capture | `context-ci manifest` | Writes `context-manifest.json` for the current checkout |
| Compare | `context-ci diff <base> <head>` | Semantic diff between two manifests |
| Gate | `context-ci check` | Diffs against a baseline and exits non-zero above the threshold |
| Explain | `context-ci classes` | Lists every diff class and its default severity |

`attribute` is not implemented yet. It is the differentiator and it needs the eval
results format settled first, so it is deliberately last.

### `manifest`

```bash
context-ci manifest [--config .context-ci.yaml] [--root .] [--out path] \
                    [--commit <sha>] [--sdk-version <ver>]
```

With no `--out`, the manifest goes to stdout and a one-line summary goes to stderr,
so `context-ci manifest > context-manifest.json` behaves.

`--commit` defaults to `git rev-parse HEAD`. If git cannot answer, the command
fails rather than inventing a value: a manifest with no commit cannot be
reproduced, which makes it useless as evidence.

### `diff`

```bash
context-ci diff <base.json> <head.json> [--format text|json] [--config path]
```

Needs no configuration file. Comparing two manifests should never require setup, so
severity overrides are optional here.

### `check`

```bash
context-ci check --baseline <base.json> [--head context-manifest.json] \
                 [--config .context-ci.yaml] [--format text|github|json]
```

`--baseline` is required. Without a baseline there is nothing to compare, and a
gate that silently passes because it had nothing to do is the most dangerous
possible default.

### `classes`

```bash
context-ci classes
```

Prints every class with its default severity. Use it when writing a
`severity:` override, because an unknown key is rejected rather than ignored.

---

## 3. Exit codes

Exit codes are part of the contract, so a pipeline can script them.

| Code | Meaning |
|---:|---|
| 0 | Nothing changed, or nothing at or above the threshold |
| 1 | **The gate failed**: at least one change at or above `gate.fail_on` |
| 2 | Usage or configuration error |
| 3 | A manifest could not be read, or is invalid |

A configuration error is 2, never 1, so a pipeline can tell "the code is fine but
the tool is misconfigured" from "the context regressed". That distinction decides
whether a team looks at their prompt or at their YAML.

---

## 4. Configuration

`.context-ci.yaml`, found at `--root` unless `--config` says otherwise.

```yaml
version: 1

capture:
  path: asserted            # asserted | instrumented | observed
  sources:                  # files whose content is hashed
    prompt: prompts/support.md
    tools: config/tools.yaml
    retrieval: config/retrieval.yaml
    guardrails: config/guardrails.yaml   # optional
    rendered: build/rendered.txt         # optional

model:
  provider: openai
  id: gpt-x
  alias_of: gpt-x-latest    # optional
  params:
    temperature: "0"        # keep it quoted: the literal text is hashed
    max_tokens: 1024
    seed: "42"              # optional
    top_p: "1"              # optional

retrieval:
  index_id: kb-prod
  embedding_model: embed-3
  embedding_model_version: "2026-05"
  chunker: recursive-800
  top_k: 5

policy:
  version: "2026.11"

severity:                   # override any class: fail | warn | ignore
  model.id: fail
  retrieval.top_k: warn

gate:
  fail_on: fail             # fail | warn | never  (default: fail)
  calls_per_day: 20000      # for the token-delta projection
  input_usd_per_mtok: 3     # omit to get tokens without a dollar figure
```

### Rules that are enforced, not documented

- **Unknown keys are rejected.** A misspelled `gat:` would otherwise leave a team
  believing a gate exists.
- **`capture.sources.prompt` is required.** A manifest with no prompt hash proves
  nothing.
- **Source paths must stay inside the repository.** `../secrets.env` is refused.
- **`capture.path: instrumented` and `observed` are refused by the CLI** with a
  message naming what produces them. Only the SDK and a recorded sample can, and
  pretending otherwise would put a weaker manifest behind a stronger label.
- **`severity.<class>` is validated against the real class list.**
- **Sampling values are read as literal text.** `temperature: 0` and `0.0` are the
  same setting but different bytes; the literal keeps the digest independent of the
  parser.

### Why `retrieval` maps to `filters_hash`

The manifest schema publishes `retrieval.filters_hash`, and the specification's
example config names the source `retrieval`. So the hash of the retrieval
configuration file lands in `filters_hash`. It is one file and one field, and the
change it signals — the scope of what can be retrieved moved — is the same.

---

## 5. Determinism

The same inputs produce a byte-identical manifest, excluding `created_at`. That is
requirement C-01, and three rules make it true:

1. **Line endings are normalised.** CRLF and LF hash identically, so a Windows
   checkout is equivalent to a Unix one. Without this, half a team's pull requests
   would report a prompt change on every commit.
2. **Keys sort by UTF-16 code unit**, not by byte, and floats are refused.
3. **Nothing derived from iteration order or the environment is recorded.**

`created_at` is recorded but excluded from the digest, because a timestamp is not a
property of the context.

---

## 6. What the manifest does not contain

- **No prompt text**, unless `Content` capture is enabled and recorded separately.
  Only its hash.
- **No credential-shaped field from a tool schema.** `api_key`, `token`,
  `secret`, `password`, `authorization`, `private_key` and similar are removed
  before hashing, so a default value in a schema cannot leak into a file that gets
  committed.
- **No token counts claimed as exact.** `description_tokens` is a whitespace-word
  estimate and is documented as one; a cost projection built on a guess is labelled
  as a projection.

---

## 7. Output formats

| Format | For |
|---|---|
| `text` (default) | A human reading a log |
| `github` | Workflow commands, so the entry appears on the pull request |
| `json` | Scripting and the next tool |

`github` escapes `%`, `\r`, `\n`, `:` and `,` in every value, so a string from a
manifest cannot open a second workflow command.

---

## 8. Not yet implemented

| Gap | Why it is a gap | Planned |
|---|---|---|
| `attribute` | Needs the eval-results format settled | Next |
| `--format sarif`, `--format junit` | Chosen output not yet decided (spec §12 question 3) | Before v0.1 |
| The GitHub Action wrapper | `action.yml` around the binary | Before v0.1 |
| Prompt registries as a source | Spec §12 question 2 | After v0.1 |
| Python and TypeScript capture SDKs | `instrumented` capture needs them | Before v0.1 |
