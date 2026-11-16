#!/usr/bin/env bash
# The README quickstart, run end to end against a built binary.
#
#   bash tools/smoke_quickstart.sh <path-to-context-ci>
#
# It exists so that the quickstart is executed rather than read: the first version of it named
# one file as both baseline and head, and the gate it described could not fail. CI runs this on
# every pull request, and the same script runs locally under Git Bash.
#
# Nothing here commits a manifest. The baseline is the base branch, built by `check --base-ref`.
set -euo pipefail

if [ "$#" -ne 1 ] || [ ! -x "$1" ]; then
  echo "usage: bash tools/smoke_quickstart.sh <path-to-context-ci>" >&2
  exit 2
fi
bin="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# expect <exit code> <what is being shown> <command...>
# Leaves stdout in $work/out and stderr in $work/err for the assertions that follow.
expect() {
  local want="$1" what="$2" got=0
  shift 2
  "$@" >"$work/out" 2>"$work/err" || got=$?
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: $what: exit $got, want $want" >&2
    cat "$work/out" "$work/err" >&2
    exit 1
  fi
  echo "ok   exit $got  $what"
}

# says <file> <text> <what is being shown>
says() {
  if ! grep -qF -- "$2" "$work/$1"; then
    echo "FAIL: $3: expected to find: $2" >&2
    cat "$work/$1" >&2
    exit 1
  fi
}

# --- 1. With no configuration at all --------------------------------------------------------

export GIT_CEILING_DIRECTORIES="$work"
zero="$work/zero"
mkdir -p "$zero"
cd "$zero"
git init --quiet --initial-branch=main
git config user.email "smoke@example.invalid"
git config user.name "smoke"
git config core.autocrlf false
printf 'Run the tests before every commit.\n' > AGENTS.md
git add . && git commit --quiet -m "agent instructions"
git checkout --quiet -b feature

expect 0 "an unconfigured repository is checked with the defaults" "$bin" check --base-ref main
says out "no change in context" "an unedited checkout reports no change"
says err "the defaults are in use" "the run says which configuration it used"

printf 'Skip the review.\n' >> AGENTS.md
printf '{"mcpServers": {"files": {"command": "npx"}}}\n' > .mcp.json
expect 1 "a new MCP server fails the gate with no configuration" "$bin" check --base-ref main
says out "[warn] agent.instructions" "the instruction edit is a warning"
says out "at AGENTS.md:2" "the report points at the changed line"
says out "+Skip the review." "the report shows the text that changed"
says out "[fail] agent.config" "the new MCP configuration is a failure"
expect 0 "acknowledging the change lets it through, and still reports it" \
  "$bin" check --base-ref main --acknowledge agent.config
says out "[fail, acknowledged] agent.config" "an acknowledged change stays in the report"

# --- 2. In your own repository -------------------------------------------------------------

demo="$work/demo"
mkdir -p "$demo/prompts"
cd "$demo"
git init --quiet --initial-branch=main
git config user.email "smoke@example.invalid"
git config user.name "smoke"
git config core.autocrlf false
printf 'You are a support agent for {{customer}}.\n' > prompts/support.md

expect 0 "init writes a configuration for the prompt it found" "$bin" init
says out ".context-ci.yaml" "init names the file it wrote"
git add .
git commit --quiet -m "prompt and context-ci configuration"
git checkout --quiet -b edit-prompt

expect 0 "check passes on an unedited branch" "$bin" check --base-ref main
says out "no change in context" "an unedited branch reports no change"

printf 'Always offer a refund.\n' >> prompts/support.md

# A prompt edit is a warning by default, and a warning is reported without failing the build.
expect 0 "a prompt edit is reported as a warning" "$bin" check --base-ref main
says out "[warn] prompt.template_hash" "the report names what changed"
says out "at prompts/support.md:2" "and where"
says out "+Always offer a refund." "and shows the line"
if grep -qF "no change in context" "$work/out"; then
  echo "FAIL: the gate reported no change for an edited prompt" >&2
  exit 1
fi

expect 0 "the same report as annotations" "$bin" check --base-ref main --format github
says out "::warning file=prompts/support.md,line=2,title=context-ci prompt.template_hash::" "the annotation lands on the line"

expect 0 "the same report as a job summary" "$bin" check --base-ref main --summary "$work/summary.md"
says summary.md "### Context CI: 1 change(s) in context" "the summary has a heading"
says summary.md "+Always offer a refund." "the summary shows the line"

# Committing the edit changes nothing: the comparison is with the base branch.
git commit --quiet -am "offer refunds"
expect 0 "the committed edit is still reported" "$bin" check --base-ref main
says out "[warn] prompt.template_hash" "the report is the same after the commit"

# The gate turned up to `warn` fails on it.
sed 's/^  fail_on: fail$/  fail_on: warn/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml
grep -q '^  fail_on: warn$' .context-ci.yaml
expect 1 "with gate.fail_on: warn the prompt edit FAILS" "$bin" check --base-ref main
sed 's/^  fail_on: warn$/  fail_on: fail/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml

# A model change fails by default, and is merged by acknowledging it.
sed 's/^  id: gpt-x$/  id: gpt-y/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml
expect 1 "a model change fails" "$bin" check --base-ref main
says out "[fail] model.id" "the failure names the model"
says out "at .context-ci.yaml:" "and the line that declares it"
expect 0 "an acknowledged model change passes" "$bin" check --base-ref main --acknowledge model.id
expect 2 "a ref git does not know is a usage error, not a pass" "$bin" check --base-ref no-such-branch
says err "fetch-depth: 0" "the message says what a CI checkout needs"

# --- 3. Manifest files, for comparing two points in time ------------------------------------

expect 0 "manifest writes a file" "$bin" manifest --out "$work/head.json"
expect 0 "check accepts a manifest file as the baseline" "$bin" check --baseline "$work/head.json"
says out "no change in context" "a manifest of this tree equals this tree"
expect 2 "one file as both baseline and head is a usage error, not a pass" \
  "$bin" check --baseline "$work/head.json" --head "$work/head.json"
says err "the same file" "the refusal says why"
expect 2 "two kinds of baseline is a usage error" "$bin" check --baseline "$work/head.json" --base-ref main
expect 0 "flags after the positionals" "$bin" diff "$work/head.json" "$work/head.json" --format sarif
says out '"results": []' "SARIF for no change carries an empty results array"
says out '"rules": []' "SARIF for no change carries an empty rules array"
expect 0 "help is not an error" "$bin" check -h
says out "usage: context-ci check" "help is on stdout"

# --- 4. The example project the README points at -------------------------------------------

example="$work/example"
cp -R "$repo/examples/minimal" "$example"
cd "$example"
git init --quiet --initial-branch=main
git config user.email "smoke@example.invalid"
git config user.name "smoke"
git config core.autocrlf false
git add . && git commit --quiet -m "the example"
expect 0 "examples/minimal is unchanged against its own commit" "$bin" check --base-ref HEAD
says out "no change in context" "the example is in step"
printf 'Always offer a refund.\n' >> prompts/support.md
expect 1 "editing the example prompt fails its gate" "$bin" check --base-ref HEAD
says out "prompt.template_hash" "the failure names what changed"

# --- 5. A first run with nothing in place ---------------------------------------------------

empty="$work/empty"
mkdir -p "$empty"
cd "$empty"
expect 0 "init in an empty directory" "$bin" init
says err "no prompt file found" "init says the prompt was left out"
expect 2 "manifest outside a repository says how to supply a commit" "$bin" manifest
says err "pass --commit <id>" "the message names the remedy"
expect 0 "the file init wrote works as it is" "$bin" manifest --commit first-run --out context-manifest.json
expect 2 "check --base-ref outside a repository is a usage error" "$bin" check --base-ref main

echo "the quickstart gates: every step behaved as the README says"
