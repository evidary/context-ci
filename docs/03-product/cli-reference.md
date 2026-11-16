# CLI reference

**Version:** 1.1.0 · **Date:** 16 November 2026 · **Owner:** Context CI squad

The four verbs in [the specification](spec.md) §1. All four are implemented, and this page is the
contract for them. The rest of this page also covers the verbs that serve them — `init`,
`classes`, `module-manifest`, `serve` and `version` — none of which changes what a gate decides.

---

## 1. Install and first run

```bash
context-ci check --base-ref origin/main    # what this branch changes, compared with where it left main
```

That is the whole quickstart. No account, no network call, no secret, nothing to commit and no
configuration: with no `.context-ci.yaml`, `check` compares the coding-agent files the repository
holds (§4, *Agent files*). `context-ci init` writes a configuration that adds a prompt, tool
definitions, retrieval and guardrail files, other files in named groups, and a declared model.
[`examples/minimal`](../../examples/minimal) is a project in that shape, and
`tools/smoke_quickstart.sh` runs these commands end to end in CI.

What a first run meets, each reported as one line that says what to do:

| Situation | What the tool says | Exit |
|---|---|---:|
| No `.context-ci.yaml` at `--root`, and no `--config` | `no .context-ci.yaml at <root>, so the defaults are in use`, on stderr, and the run continues | 0 |
| `--config` names a file that is not there | the file is not found | 2 |
| `init` found no prompt file | the `prompt` line is written commented out, and stderr names the key to point at the file | 0 |
| A configured source file is missing | `capture.sources.<kind> names <path>, which does not exist under <root>: create that file, or point capture.sources.<kind> at the file that holds it` | 3 |
| `check --base-ref` and git does not know the ref, or the directory is not a repository | says so, and that a CI checkout needs the base branch, which `actions/checkout` fetches with `fetch-depth: 0` | 2 |
| `manifest` with no commit to record | says whether there is no repository, no commit or no git, and `pass --commit <id>` | 2 |

**The manifest is declared, not observed.** The tool hashes files in the repository and copies
`model`, `retrieval` and `policy` from the configuration. It does not inspect a running
application, so the model id, the parameters, the retrieval settings and the policy version are
what the configuration asserts. `manifest --capture` replaces one field, the rendered prompt
hash, with the hash of text an SDK recorded at request-assembly time; see [`manifest`](#manifest).

In a pull request:

```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened, labeled, unlabeled]
jobs:
  context:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }      # the baseline is built from the base branch
      - uses: evidary/context-ci@v0.9.0
        with:
          # A reviewer accepts a deliberate fail-class change by applying this label.
          acknowledge: ${{ contains(github.event.pull_request.labels.*.name, 'context-change-approved') && 'all' || '' }}
```

`action.yml` is at the repository root, so the action is `evidary/context-ci@<ref>`; there is no
`action/` subdirectory. Its inputs:

| Input | Default | Meaning |
|---|---|---|
| `command` | `check` | `check`, `manifest` or `attribute` |
| `base-ref` | the pull request's base branch | The ref `check` compares with. Outside a pull request it, or `baseline`, is required: the action fails instead of passing on an empty comparison |
| `baseline` | none | A manifest file to compare with, instead of `base-ref`. Required for `attribute` |
| `acknowledge` | none | Passed to `check --acknowledge` |
| `summary` | `true` | Write the report to the job summary |
| `text` | `true` | Include the changed lines of files. `false` passes `--no-text` |
| `format` | `github` | As `--format` |
| `root`, `config`, `head` | none | As the flags |
| `version` | the action's ref | A release to run instead |

Named with a release tag (`@v0.9.0`) or given `version`, the action downloads that release's
binary for the runner's operating system and architecture and compares it with the release's
`SHA256SUMS`; a mismatch fails the step. Named with a branch or a commit, or when the download
fails, it builds the binary from its own checkout, installing the Go version its `go.mod` names if
the runner has none. [spec.md](spec.md) §7 says which of these paths have run on a hosted runner.

## 2. The verbs

| Verb | Command | What it does |
|---|---|---|
| Scaffold | `context-ci init` | Writes a commented `.context-ci.yaml` for the current checkout |
| Capture | `context-ci manifest` | Writes the manifest for the current checkout, to stdout or to `--out`. `check --base-ref` does not need one |
| Compare | `context-ci diff <base> <head>` | Semantic diff between two manifests |
| Gate | `context-ci check --base-ref <ref>` | Compares the working tree with the commit where the branch left `<ref>`, says what changed and where, and exits non-zero above the threshold |
| Explain | `context-ci classes` | Lists every diff class and its default severity |
| Attribute | `context-ci attribute` | **Experimental.** Lists the changed context classes a regression is consistent with, when the regressed cases declare what they depend on |
| Discover | `context-ci module-manifest` | The platform's `module-manifest.v1.json`. Deliberately **not** `manifest`, which already means the tool manifest above |
| Probe | `context-ci serve` | `GET /healthz` and `GET /readyz`, so the manifest's `health_path` is real |
| Version | `context-ci version` | Prints the tool version, including the module version the toolchain stamped into a `go install`ed binary |

`attribute` refuses to guess: when no regressed case declares `depends_on` it reports a correlation
and says so. See [`attribute`](#attribute).

**Flags may be written before or after a verb's arguments.** `context-ci diff a.json b.json --format
sarif` and `context-ci diff --format sarif a.json b.json` are the same command, and a bare `--` ends
the flags for a file whose name starts with a dash. `context-ci <verb> -h` prints that verb's flags
on stdout and exits 0.

### `init`

```bash
context-ci init [--root .] [--config path] [--force]
```

Writes a commented `.context-ci.yaml` for the checkout, then says on stderr what still has to be
changed before the file is true. It exists because the alternative is copying the example in §4 by
hand, and a configuration typo is a gate that looks on while being off.

| Flag | Default | Behaviour |
|---|---|---|
| `--root` | `.` | The checkout the file describes; the same directory `manifest`, `check` and `attribute` read the file from |
| `--config` | `.context-ci.yaml` under `--root` | Where to write it. A directory that does not exist is an error, not something to create |
| `--force` | off | Replace a file that already exists. Without it `init` refuses and changes nothing |

What it writes, and what it refuses to invent:

- **A source is named only when the checkout has the file.** Each kind is probed at a short fixed
  list, and a kind with no file is written as a comment rather than as a path nobody created.
  The lists are these, in preference order:

  | Source | Probed |
  |---|---|
  | `prompt` | `prompts/support.md`, `prompts/system.md`, `prompts/system-prompt.md`, `prompts/prompt.md`, `prompt.md`, `PROMPT.md` |
  | `tools` | `config/tools.yaml`, `config/tools.yml`, `config/tools.json`, `tools.yaml`, `tools.yml`, `tools.json` |
  | `retrieval` | `config/retrieval.yaml`, `config/retrieval.yml`, `retrieval.yaml`, `retrieval.yml` |
  | `guardrails` | `config/guardrails.yaml`, `config/guardrails.yml`, `guardrails.yaml`, `guardrails.yml` |
  | `rendered` | `build/rendered.txt`, `build/rendered.md` |

  The prompt is treated like the others. It used to be written live when nothing was found,
  pointing at a file nobody had created, so the first `manifest` after `init` failed.
- **`model.provider` and `model.id` are placeholders** — `openai` and `gpt-x` — and the file and
  stderr both say so. Nothing in a repository says which model a team calls, so a guess presented
  as a fact would gate on a model nobody uses. `retrieval` and `policy` are commented out entirely
  for the same reason: no file this tool may read holds their values.
- **The file is parsed before it is written**, by the same strict loader `check` uses, so `init`
  cannot produce a configuration the next command rejects. `cmd/context-ci/init_test.go` runs
  `manifest` and `check` against the generated file rather than trusting that claim.
- **The gate is written explicitly** as `gate.fail_on: fail`, which is also the default, so the
  protective setting is visible in the file a team reviews.
- **`capture.files` and `capture.agent_files` are written as commented examples**, because which
  other files steer the model is not something a tool can know.

It stats a handful of candidate paths and writes one file. It makes no network call, needs no
account and no secret, and does not run `git`.

### `manifest`

```bash
context-ci manifest [--config .context-ci.yaml] [--root .] [--out path] \
                    [--commit <sha>] [--sdk-version <ver>] [--capture <file>]
```

`--capture` reads a file written by an SDK at request-assembly time — see
[sdk/python](../../sdk/python/README.md) and [sdk/typescript](../../sdk/typescript/README.md) — and
produces an **instrumented** manifest: `capture.path` becomes `instrumented`, `capture.sdk_version`
records which SDK produced it, and `rendered_hash` becomes the digest of the text that was actually
sent rather than of a file in the repository. `rendered_sample_hash` is left null, because a live
capture is not a sample.

The capture supplies the rendered prompt and, when it has it, the template. Tools, retrieval and
policy still come from the configuration: they are configuration, and recording them twice would
create two sources of truth for the same fact. Variable *values* are never taken from a capture —
the manifest records types, because it is a file teams commit and share.

With no `--out`, the manifest goes to stdout and a one-line summary goes to stderr,
so `context-ci manifest > context-manifest.json` behaves. A warning on stderr lists every file,
prompt or tool description that already contains hidden characters (§6).

`--commit` defaults to git's `HEAD`. If git cannot answer, the command fails with exit 2 rather
than inventing a value: a manifest with no commit cannot be traced to the tree it describes. The
commit is recorded and is not part of the digest, so the same context has the same digest on every
commit. The message says which case it is — the directory is not a git repository, the
repository has no commits yet, or git is not installed — and that `--commit <id>` supplies the
value by hand. Every configured source is checked first, so a missing prompt file is reported
before the commit is.

What the manifest holds is what the repository declares. The hashes are computed from the files
named under `capture.sources` and `capture.files` and the agent files found by name; `model.provider`, `model.id`, `model.params`, `retrieval.*` and
`policy.version` are copied from `.context-ci.yaml` unchanged. Nothing here contacts a provider or
reads a request, so a manifest cannot show that the application uses the model it names.

### `diff`

```bash
context-ci diff <base.json> <head.json> [--format text|github|json|sarif|junit] [--config path]
```

Needs no configuration file. Comparing two manifests should never require setup, so
severity overrides are optional here. The flags work in either position.

Both files are read strictly. A manifest is rejected with exit 3 when its digest does not match
its content, when it holds a field the schema does not define — an unknown field is outside the
digest, so it would be content nothing verified — or when a restricted value in
[spec.md](spec.md) §4 is outside what is allowed: a `model.provider` that is not `openai`,
`anthropic`, `google` or `other`, a sampling value that is not a number, a negative `top_k`,
`max_tokens` or `description_tokens`, or a hash that is not 64 lowercase hex characters. `check`
and `attribute` read manifests the same way.

### `check`

```bash
context-ci check --base-ref <ref> [--root .] [--config .context-ci.yaml] \
                 [--acknowledge <classes>|all] [--summary <file>] [--no-text] \
                 [--format text|github|json|sarif|junit]
context-ci check --baseline <base.json> [--head <head.json>] [--commit <sha>] ...
```

Exactly one of `--base-ref` and `--baseline` is required. Without a baseline there is nothing to
compare, and a gate that silently passes because it had nothing to do is the most dangerous
possible default.

| Flag | Default | Behaviour |
|---|---|---|
| `--base-ref` | none | A git ref, such as `origin/main`. The baseline is built from the **merge base** of that ref and `HEAD`: the commit where this branch left it. Changes the base branch made afterwards are not this branch's, and are not reported |
| `--baseline` | none | A manifest file to compare against instead |
| `--head` | built from the working tree | Read this manifest instead of building one, such as an instrumented manifest written with `manifest --capture` |
| `--commit` | git `HEAD` | The commit recorded in a head `check` builds. Combining it with `--head` is a usage error |
| `--acknowledge` | none | Comma-separated classes, or `all`. See below |
| `--summary` | none | Append the report as Markdown to this file, such as `$GITHUB_STEP_SUMMARY` |
| `--no-text` | off | Leave the changed lines of files out of every output |
| `--root` | `.` | The project directory: where `.context-ci.yaml` is found, and what its paths are relative to. It may be a subdirectory of the repository; only files under it are read, and reported paths are relative to the repository |

**The baseline is built, not stored.** With `--base-ref`, `check` reads the files as they were at
the merge base, through git, and applies the configuration that commit had. If that commit has no
`.context-ci.yaml`, or one this version cannot read, the defaults are used and stderr says so:
adopting the tool, or adding a source, reports what was added. A source the base configuration
names that the base commit does not hold is left out with a warning instead of failing the run.
The head is always built strictly from the working tree, so an uncommitted edit is seen. One line
on stderr names the commit the baseline came from and both digests; stdout is only the report.

Nothing is committed for this, which removes three defects of a committed manifest: two branches
that each regenerated it conflicted every time, a change merged without regenerating it was
reported on the next unrelated pull request, and an author could regenerate it to make the check
pass without anyone reviewing the change.

**Where, and what.** For every entry that is about a file, the report gives the path and the first
changed line, and for prompt, tool, instruction and group files it prints the changed lines from
`git diff`, at most 60 per file:

```
2 change(s) in context

  [fail] model.id
    the model changed
    at .context-ci.yaml:44
    before: gpt-x
    after:  gpt-y
  [warn] prompt.template_hash
    the prompt template changed
    at prompts/support.md:2
    before: a0f2faf62e33…
    after:  3b02226794ca…

  prompts/support.md
    @@ -1 +1,2 @@
     You are a support agent for {{customer}}.
    +Always offer a refund.

unchanged: agent.config, agent.instructions, capture.path, ...

digest 4e8a5e80e124… -> 7fea460a8434…
```

A declared value points at its line in the configuration file; a tool entry at the line that names
the tool. With `--baseline` there is no commit to compare text with, so entries carry a path where
one is known and no text.

**What fails the build.** `check` exits 1 for a change at or above `gate.fail_on`, which defaults
to `fail`, unless it is acknowledged. A prompt edit is `warn` by default, so with the default gate
it is reported and the exit code is 0; set `gate.fail_on: warn`, or
`severity: {prompt.template_hash: fail}`, to make it block. When the configuration names no
`rendered` source the rendered hash is the template hash, and the edit is reported once, as
`prompt.template_hash`.

**Acknowledging a change.** A deliberate model upgrade is a `fail`, and the answer is not to
lower its severity for everyone afterwards. `--acknowledge model.id` accepts changes of that class
for this run: they are printed as `[fail, acknowledged]`, written as `notice` annotations that
start with `acknowledged:`, marked in JSON and SARIF, and do not affect the exit code. A change
nobody named still fails. `--acknowledge all` accepts every class except `hidden_characters`,
which has to be named, because text a reviewer cannot see is the one thing a blanket approval
should not cover. An unknown class is a usage error. Who may acknowledge is decided by whatever
supplies the flag; in the workflow in §1 it is whoever may label the pull request.

**One file as both sides is a usage error.** `check --baseline m.json --head m.json` exits 2,
whichever way the two paths are spelled: a file compared with itself can only report that nothing
changed.

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
| 0 | Nothing changed, or nothing at or above the threshold. Also `-h`: asking for help is not an error |
| 1 | **The gate failed**: at least one change at or above `gate.fail_on` that was not acknowledged |
| 2 | Usage or configuration error, including `check` given one file as both baseline and head, a `--base-ref` git cannot resolve, an unknown class in `--acknowledge`, and no commit to record |
| 3 | A manifest could not be read, or is invalid. `module-manifest` also uses it for a build with no released identity, and `serve` for a server that failed after it started: neither is a usage error, and both mean the document the tool was asked for could not be produced |

A configuration error is 2, never 1, so a pipeline can tell "the code is fine but
the tool is misconfigured" from "the context regressed". That distinction decides
whether a team looks at their prompt or at their YAML.

Every way `init` fails is 2 — refusing to replace a file without `--force`, a destination it cannot
write, a flag it does not know — because nothing was compared and nothing regressed.

---

## 4. Configuration

`.context-ci.yaml`, found at `--root` unless `--config` says otherwise. It is optional: without it
the defaults are `capture.path: asserted`, `agent_files: true`, `gate.fail_on: fail` and nothing
else. `context-ci init` writes a commented file in this shape; the values it writes for `model` are
placeholders.

```yaml
version: 1

capture:
  path: asserted            # the only path a configuration can claim
  sources:                  # every one is optional
    prompt: prompts/support.md
    tools: config/tools.yaml
    retrieval: config/retrieval.yaml
    guardrails: config/guardrails.yaml
    rendered: build/rendered.txt         # needs prompt
  files:                    # other files that steer the model, in groups you name
    prompts: ["prompts/**/*.md"]
    examples: few_shot/*.jsonl
  agent_files: true         # the default

model:                      # optional as a whole; provider and id go together
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
  files.examples: fail

gate:
  fail_on: fail             # fail | warn | never  (default: fail)
  calls_per_day: 20000      # for the projection when a tool description grows
  input_usd_per_mtok: 3     # omit to get tokens without a dollar figure
```

### File groups

`capture.files` maps a group name to one pattern or a list. Each group is a diff class,
`files.<group>`, `warn` by default, and each matched file is hashed as text and reported by its
path: added, removed or changed.

- A group name is lowercase letters, digits, `_` and `-`, starting with a letter.
- `*` and `?` match within one path segment, `[...]` is a character class, and `**` as a whole
  segment matches any number of directories. A pattern with no wildcard also matches everything
  under it, so `prompts` is the directory. Matching is case-sensitive, on every operating system.
- There is no negation: a pattern cannot exclude what another matched.
- A pattern with no wildcard that matches nothing is an error, because it names a file somebody
  expected. A wildcard that matches nothing is a warning.
- Files are listed through git where there is a repository, so ignored files are not matched; an
  untracked file that is not ignored is. Symbolic links are not followed, and a link git checked
  out as a text file, as it does on Windows, is still a link.
- The file named as `sources.prompt` is left out of every group, so a prompt edit is reported once.
- More than 5,000 matched files is refused.

### Agent files

Unless `capture.agent_files: false`, files that coding agents read are found by name, with no
configuration, in two groups:

| Class | Default | Files |
|---|---|---|
| `agent.instructions` | `warn` | `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` and `SKILL.md` at any depth; `.github/copilot-instructions.md`; `.github/instructions/`, `.github/prompts/`, `.github/agents/`, `.github/chatmodes/`; `.claude/agents/`, `.claude/commands/`, `.claude/rules/`, `.claude/skills/`; `.agents/skills/`; `.cursor/rules/`, `.cursorrules`; `.windsurf/rules/`, `.windsurfrules`; `.clinerules`; `.roo/rules/`, `.roorules`; `.continue/rules/`; `.kiro/steering/`; `.junie/guidelines.md` |
| `agent.config` | `fail` | `.mcp.json`, `.vscode/mcp.json`, `.cursor/mcp.json`, `.roo/mcp.json`, `.kiro/settings/mcp.json`; `.claude/settings.json`, `.claude/hooks/`, `.claude-plugin/`; `.cursor/hooks.json`; `.codex/config.toml`; `.gemini/settings.json`; `.aider.conf.yml` |

The second group fails by default because it is where a repository grants an agent a new server, a
new command to run or a wider permission. The list is in `internal/capture/agent.go`. It is a list
of names known in November 2026, and a tool that reads its instructions from somewhere else is not
covered until its path is added there or to `capture.files`.

### Tool definitions

`sources.tools` is a YAML or JSON file in any of these shapes:

| Shape | Looks like |
|---|---|
| This tool's own | `tools: [{name, description, schema}]` |
| Chat-completions function tools | a list of `{type: function, function: {name, description, parameters}}` |
| The flattened function form | a list of `{type: function, name, description, parameters}` |
| `input_schema` tools | a list of `{name, description, input_schema}` |
| Function declarations | `[{functionDeclarations: [{name, description, parameters}]}]` |
| A saved MCP `tools/list` response | `{tools: [{name, description, inputSchema, ...}]}`, with or without the JSON-RPC `result` envelope |

A list may be bare or under `tools:`. The description is hashed alone; everything else in a
definition, under whatever key, is hashed together, so no field can change unseen, and converting a
file from one shape to another is not a schema change. A provider-hosted tool with a `type` and no
name is named by its type. A top-level key that belongs to none of these shapes is refused, and so
is one page of a longer `tools/list` response (`nextCursor` set), because the tools on the other
pages would be invisible.

These shapes were written from the formats as documented when this version was built and are
covered by fixtures in `internal/capture/sources_test.go`. They have not been checked against
files exported from each vendor's current tooling. There is no MCP client here: the tool reads a
file somebody saved, and a server whose live tool list changes is not seen until that file does.

### Rules that are enforced, not documented

- **Unknown keys are rejected.** A misspelled `gat:` would otherwise leave a team
  believing a gate exists.
- **`model` is optional, and `provider` and `id` go together.** `provider` is one of `openai`,
  `anthropic`, `google`, `other`: `OpenAI` and `openai` would otherwise be two providers.
- **`sources.rendered` needs `sources.prompt`.**
- **`retrieval.top_k` and `model.params.max_tokens` cannot be negative**, and a sampling value
  must be a number: `temperature` and `top_p` a decimal, `seed` an integer.
- **Paths and patterns must stay inside the root.** `../secrets.env` is refused.
- **`capture.path: instrumented` and `observed` are refused by the CLI** with a
  message naming what produces them.
- **`severity.<class>` is validated against the real class list**, plus `files.<group>` for a
  group the same file configures.
- **Sampling values are read as literal text.** `temperature: 0` and `0.0` are the
  same setting but different bytes; the literal keeps the digest independent of the
  parser.

### How `retrieval` and `guardrails` files are hashed

The hash of the retrieval file is `retrieval.filters_hash` and the hash of the guardrails file is
`policy.guardrails_hash`; a change to either is a `fail`. A `.yaml`, `.yml` or `.json` file is
parsed and its **values** are hashed, so a comment, the order of keys, quoting and indentation are
not changes. Until version 0.9 the bytes were hashed, and a comment failed the gate with a message
about retrieval scope. A file with another extension is hashed as text with line endings
normalised. A YAML or JSON file that does not parse is an error in the working tree; in a baseline
built from git it is hashed as text, with a warning.

---

## 5. Determinism

The same files and configuration produce the same digest. That is requirement C-01, and four
rules make it true:

1. **Line endings are normalised.** CRLF and LF hash identically, so a Windows
   checkout is equivalent to a Unix one.
2. **Keys sort by UTF-16 code unit**, not by byte, and floats are refused.
3. **Nothing derived from iteration order or the environment is recorded.** Files are sorted by
   path.
4. **`created_at` and `commit` are recorded and excluded from the digest.** When and where a
   manifest was built are not properties of the context. A manifest built from a commit through
   git and one built from a checkout of that commit have the same digest, which is what lets
   `check --base-ref` compare the two.

---

## 6. What the manifest does not contain

- **No observation of the running application.** Every value is a hash of a file in the
  repository or a value copied from `.context-ci.yaml`. The one exception is `prompt.rendered_hash`
  in a manifest written with `--capture`, which is the hash of text an SDK recorded. A manifest
  therefore says what a team declared, and `capture.path` says which of the two it is.
- **No prompt text.** The manifest schema carries an optional `content` field for a copy a
  team opted into, and it is checked against its hash on load, but no verb writes one: the
  prompt reaches the manifest as a hash and nothing else.
- **No credential value from a tool definition.** Before hashing, a string under a key such as
  `api_key`, `token`, `secret`, `password`, `authorization` or `private_key`, a literal value
  (`default`, `example`, `const`, `enum`) of a property with such a name, and any string shaped
  like a known token are replaced with `<redacted>`. The parameter stays: until version 0.9 the
  whole key was dropped, so a tool gaining a `token` parameter was not a change.
- **Hidden characters are counted, not hidden.** `hidden_characters` on a prompt, tool or file is
  the number of invisible or direction-changing characters in it. `check` reports a `fail`-class
  `hidden_characters` entry when a new or changed file, prompt or tool description contains any,
  with the code point and line of the first. Files that already had them are listed by `manifest`
  on stderr and are not reported on every later check. This is a deterministic character check and
  nothing more: it does not read meaning, and visible text that instructs a model to misbehave is a
  reviewer's job, which is why the text is shown.
- **No token counts claimed as exact.** `description_tokens` is a whitespace-word
  estimate and is documented as one; a cost projection built on a guess is labelled
  as a projection.

---

## 7. Output formats

| Format | For |
|---|---|
| `text` (default) | A human reading a log |
| `github` | Workflow commands with `file=` and `line=`, so the entry appears on the changed line of the pull request |
| `json` | Scripting and the next tool |
| `sarif` | A SARIF 2.1.0 report: one rule per changed class and one result per change, with a location where the change is about a file |
| `junit` | A test report, for dashboards that only read XML |

`github` escapes `%`, `\r` and `\n` in every message, and additionally `:` and `,` in the
title property, so a string from a manifest cannot open a second workflow command. The two
sets differ on purpose: GitHub unescapes only the first three in a message, so escaping a
colon there would show the escape to the reader.

`diff` and `check` write one annotation per change and carry `severity` as the level; an
acknowledged change is a `notice`. `check --summary <file>` appends the same report as Markdown, a
table of changes followed by the changed text of each file in a collapsed block.
`attribute` has no file to point at, so it writes the findings the report can name — see
[`attribute --format github`](#attribute---format-github) for what each level means there.

### `sarif` and `junit`, and why severity is not translated

| Severity | SARIF `level` | JUnit |
|---|---|---|
| `fail` | `error` | `<failure>` |
| `warn` | `warning` | passes, with the warning in `<system-out>` |
| `ignore` | `note` | `<skipped>` |
| acknowledged | `note`, with `properties.acknowledged` | passes, with the message in `<system-out>` |

JUnit has no warning level. The two dishonest options are to call a warning a failure, which
teaches a team to ignore red, and to drop it, which hides the change. So a `warn` entry passes its
test case and carries its message in `<system-out>`; only `fail` becomes `<failure>`. A team that
wants warnings to break the build sets `severity:` in the configuration, where that decision
belongs.

SARIF output is deterministic: the same diff renders byte-identically twice, so it can be committed
or diffed. Every result names a rule that the run declares, because a SARIF consumer drops a result
whose rule is missing.

**Locations.** A result or annotation about a file carries the file's path, relative to the
repository, and a line: the first changed line, the line that names the tool, or the line of the
key in `.context-ci.yaml` for a declared value. `diff`, which compares two manifest files, has no
repository to look in and writes no locations. SARIF with locations, including an
acknowledged result, was validated against the OASIS `sarif-schema-2.1.0.json` on 16 November 2026.
It has not been uploaded to code scanning.

**A diff with no change is a complete report.** `rules` and `results` are empty arrays, and the
JUnit suite is an empty `<testsuite tests="0" ...>`. Until 16 November 2026 the SARIF for an
unchanged context carried `"rules": null, "results": null`, which the schema rejects.

Both formats were validated against their published schemas on 13 November 2026: SARIF 2.1.0
against the OASIS `sarif-schema-2.1.0.json`, and JUnit against the `junit-10.xsd` the Jenkins xunit
plugin ships, with a diff containing one `fail`, one `warn` and one `ignore` entry. That last part
matters: `errors` is a required attribute on `testsuite` in that schema, so it is written as `0` —
a change is never a report that failed to be produced. That validation did not include a diff with
no entries, which is how the `null` above went unnoticed. On 16 November 2026 the SARIF for an
unchanged context and for a two-change diff were validated against the OASIS
`sarif-schema-2.1.0.json` with Python `jsonschema`: both pass, and the unchanged report with `null`
in place of its two arrays fails on exactly `rules` and `results`. JUnit was not re-validated
against the XSD; its empty suite is pinned by a test.

---

### `attribute`

```bash
context-ci attribute --baseline base.json --head context-manifest.json \
                     --baseline-results base-eval.json --results head-eval.json \
                     [--format text|github|json]
```

**Experimental.** It is only as good as the `depends_on` declarations in the eval results, which
no eval framework writes for you, and it has been run against fixtures only. The format of its
input and output may change.

Two manifests and two eval runs. It reports what moved, and — when the regressed cases say what
they depend on — which changed context classes that movement is consistent with. **It lists
candidate causes. It does not establish a cause.** A `depends_on` declaration is the suite
author's statement about a case, not a measurement, and two eval runs cannot separate two changes
made together.

```json
{"version": 1, "suite": "support-quality", "cases": [
  {"id": "q1", "score": 0.92, "depends_on": ["prompt.template_hash"]},
  {"id": "q3", "score": 0.90, "depends_on": ["model.id"]}
]}
```

A results file is read strictly, and refused with exit 3 when it would be misread: a `version`
other than `1`, an unknown field, a case with no `id` or a repeated one, a `score` outside 0..1,
or a `depends_on` entry that is not a context class — `context-ci classes` prints the list. A
misspelled class could never match a change, so it would silently remove the case from every
attribution. Only `score` and `passed` are compared; `cost_usd` is accepted and not used, so
nothing here attributes cost.

**The distinction the verb exists to keep is attribution versus correlation.**

| The regressed cases declare `depends_on` | What the report says |
|---|---|
| At least one does | `attribution`: a regressed case that depends on one of the changed classes makes that class a candidate, with a share over all regressed cases |
| None does | `correlation`: the scores moved and the configuration moved, and nothing ties them together |

The mode is decided by the cases that regressed. A declaration on a case that did not regress is
not evidence about the ones that did.

```
support-quality: 3 compared, 3 regressed, 0 improved, 0 unchanged, 0 incomparable
3 of 3 cases regressed and every regressed case declares its dependencies; 3 of them declare a
dependency on a class that changed, so the classes below are candidate causes consistent with those
declarations, not proven causes.

changed classes: model.id, prompt.template_hash
2 context classes changed together, so their effects are confounded: this report is not causal, and
it cannot say which of those changes, if any, caused a movement in the scores.

  prompt.template_hash     3 of 3 regressed cases (100%)  the prompt template changed
  model.id                 0 of 3 regressed cases (0%)  the model changed
```

(Each sentence is one line in the output; it is wrapped here.) The second paragraph is requirement
C-08: it is printed whenever more than one class changed, in every mode, and the JSON report
carries the same fact as `"confounded": true`.

The same numbers with the declarations removed:

```
3 of 3 cases regressed, but no regressed case declares which context classes it depends on, so this
is a correlation: the scores moved and the configuration moved, and nothing here ties them together
beyond that.
```

Every sentence is computed from the counts it states. The other things the first line can say:

| Situation | Sentence |
|---|---|
| Some regressed cases declare and some do not | `2 of the 3 regressed cases declare dependencies` in place of `every regressed case declares its dependencies` |
| The regressed cases declare, but not a class that changed | `none of them declares a dependency on a class that changed, so no changed class is a candidate cause on this evidence` |
| Cases regressed and no class changed | `no context class changed between the two manifests, so nothing in the context is a candidate cause` |
| Nothing regressed and something improved | `No case regressed by more than 0.001 (2 improved), so there is no regression to attribute.` |
| Nothing moved | `No case moved by more than 0.001, so there is nothing to attribute.` |

Three rules worth knowing:

- **A movement smaller than 0.001 is not a movement.** Model scores do not repeat exactly, and a
  report that calls a rounding wobble a regression trains its reader to ignore it.
- **A case in one run and not the other is incomparable, not unchanged.** A vanished case is a
  result, and dropping it would quietly shrink the denominator.
- **A class that changed with no evidence against it is printed with zero**, not omitted. Silence
  would read as innocence.
- **Classes are counted, not entries.** Three tools changing is one changed class, `tools`, and is
  not reported as three things confounded with each other.
- **One file as both manifests is refused**, as it is for `check`.

One thing that surprises: with no `rendered` source in the configuration, a prompt edit is the
class `prompt.template_hash`, and `prompt.rendered_hash` is reported as unchanged. A case that
should follow prompt edits in that setup declares `prompt.template_hash`.

`attribute` never exits non-zero for a regression: gating is `check`'s job, and a verb that
reports and fails at once cannot be used to explain a build that already failed.

#### `attribute --format github`

The same report as workflow annotations, so a regression appears on the pull request instead of
only in the job log:

```
::notice title=context-ci attribute::support-quality: attribution: 2 compared, 1 regressed, 0 improved, 1 unchanged, 0 incomparable
::error title=context-ci attribute model.id::1 of 1 regressed cases (100%25) declare it: the model changed
::warning title=context-ci attribute case q1::regressed by more than 0.001
```

When more than one class changed, the summary notice ends with the confounding statement — `... 0
incomparable. 2 context classes changed together, so their effects are confounded: ...` — so the
first annotation a reader sees qualifies the ones below it.

| Annotation | Level | Why that level |
|---|---|---|
| The summary: suite, mode and the five counts | `notice` | It is the qualifier, not a finding, and it must be readable before the rest |
| A class that a regressed case declares a dependency on | `error` | A declaration ties a regression to this change: a candidate cause, not a proven one |
| A regressed case | `warning` | The score moved; the case name is what a reader needs to go and look |
| A changed class that nothing points at | `notice` | It changed. Printing it with zero beats silence that reads as innocence |

The level states how strong the claim is, not whether the build failed: `attribute` exits 0
whatever it finds, so an `error` annotation here never fails a job. `100%25` is how `100%` is
written on the wire; GitHub unescapes it back to `100%` when it renders the message.

**No annotation carries `file=` or `line=`.** An attribution report names eval cases and context
classes, and neither records where it lives. An invented location would send a reader to an
unrelated line, which is worse than no location at all — so the annotation says what it knows and
nothing else.

In a workflow, run the verb as its own step rather than capturing its output, because annotations
are read from the step's log:

```yaml
- uses: evidary/context-ci@v0.9.0
  with:
    command: attribute
    baseline: context-manifest.base.json
    head: context-manifest.json
    baseline-results: evals/base.json
    results: evals/head.json
    format: github
```

GitHub shows at most 10 warning and 10 error annotations per step (and 20 notices) on the pull
request; anything beyond that stays in the log. The summary line always carries the counts, so a
truncated list cannot make a report look empty.

## 8. Not yet implemented

| Gap | Why it is a gap | Planned |
|---|---|---|
| CI-provider verification | GitHub Actions is exercised by this repository's own CI. The GitLab CI and CircleCI commands were run in those providers' images, not on their runners. Dated in [spec.md](spec.md) §7 | When a user of either asks |
| A comment on the pull request | The report goes to annotations and the job summary. A comment needs a token with write access, which a pull request from a fork does not get | After launch, if asked for |
| Live MCP servers | Tool definitions are read from a file somebody saved. Nothing connects to a server | Not scheduled |
| Token counts | `description_tokens` is a count of words. No tokenizer is bundled | Not scheduled |
| Observed capture of anything but the prompt | The manifest's model, parameters, retrieval settings and policy version are declared in `.context-ci.yaml`. No code reads them from a running application, so a change made outside the repository is not seen | Not scheduled |
| Cost attribution | `attribute` compares `score` and `passed`. An eval case's `cost_usd` is accepted and unused, and no cost event is emitted | Not scheduled |
| Prompts from a registry | **Undecided, so not built.** [spec.md](spec.md) §12 question 2 asks whether registries are a source at v0.1 or only files, and the repository records no answer. What a source would need is listed below, and the binary makes no network call | After v0.1 |

### Why prompts from a registry are still deferred

This row used to say "Spec §12 question 2" and nothing else, which named the open question and not
the reason. The reason is that the specification does not define the feature, and cannot without
changing a requirement it states twice.

- **The definition is an open question.** [spec.md](spec.md) §12 question 2 is "Do we support prompt
  registries (Langfuse, LangSmith) as a source at v0.1, or only files?" It has no recorded answer in
  any document here. §2 states the intent — "Registries are the thing we read from, not the thing we
  become" — and §3 defines the asserted capture path as files in the repository. Neither says what a
  registry source *is*, so implementing one would mean inventing the contract this page exists to
  record.
- **Six things would have to be specified, and none are.** The configuration key, given that the
  validator accepts exactly `prompt`, `rendered`, `tools`, `retrieval` and `guardrails` and rejects
  every other key; which registry, and which of its API versions; how one version is selected (label,
  tag, numeric version, or "latest"); whether the template, the rendered text, or both are fetched;
  where credentials come from, given that C-10 requires the tool to work with no account and no
  network; and what happens when the registry is unreachable during a check.
- **Requirement C-02 forbids the network call it would make.** [spec.md](spec.md) §9 C-02 is "no
  outbound network call in any mode", and §7 repeats "Network: None". A registry source fetches a
  prompt over HTTP by definition, so shipping one is an amendment to the requirement, which is a
  specification decision and not an implementation detail.
- **The digest story is unspecified.** A fetched prompt has no file whose bytes the digest covers.
  Whether the manifest records the registry's version id, the hash of the fetched text, or both,
  decides whether a baseline committed to the repository can still be verified and diffed with no
  network — and it must be, because `check` reads that baseline offline.

So no registry code exists and no configuration key accepts one. Before this row can be removed, §12
question 2 needs an answer, and the answer needs to settle each point above, in particular how a
fetched prompt is pinned so `diff` and `check` stay offline. Adding `init` did not make that decision,
and `init` writes only file sources for exactly this reason.
