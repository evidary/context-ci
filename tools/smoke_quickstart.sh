#!/usr/bin/env bash
# The README quickstart, run end to end against a built binary.
#
#   bash tools/smoke_quickstart.sh <path-to-context-ci>
#
# It exists so that the quickstart is executed rather than read: the first version of it named
# one file as both baseline and head, and the gate it described could not fail. CI runs this on
# every pull request, and the same script runs locally under Git Bash.
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

# --- 1. In your own repository -------------------------------------------------------------

demo="$work/demo"
mkdir -p "$demo/prompts"
cd "$demo"
git init --quiet
git config user.email "smoke@example.invalid"
git config user.name "smoke"
printf 'You are a support agent for {{customer}}.\n' > prompts/support.md

expect 0 "init writes a configuration for the prompt it found" "$bin" init
says out ".context-ci.yaml" "init names the file it wrote"

# A prompt edit is a warning by default and a warning does not fail the build. The quickstart
# turns the gate up to `warn`, which is the one line this changes.
sed 's/^  fail_on: fail$/  fail_on: warn/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml
grep -q '^  fail_on: warn$' .context-ci.yaml

git add .
git commit --quiet -m "prompt and context-ci configuration"

expect 0 "manifest writes the baseline" "$bin" manifest --out context-manifest.json
git add context-manifest.json
git commit --quiet -m "context baseline"

expect 0 "check passes on an unedited checkout" "$bin" check --baseline context-manifest.json
says out "no change in context" "an unedited checkout reports no change"

printf 'Always offer a refund.\n' >> prompts/support.md

expect 1 "check FAILS after the prompt is edited and the baseline is not regenerated" \
  "$bin" check --baseline context-manifest.json
says out "prompt.template_hash" "the failure names what changed"
if grep -qF "no change in context" "$work/out"; then
  echo "FAIL: the gate reported no change for an edited prompt" >&2
  exit 1
fi

expect 1 "the same failure as annotations" "$bin" check --baseline context-manifest.json --format github
says out "::warning title=context-ci prompt.template_hash::" "the annotation names the class"

expect 2 "one file as both baseline and head is a usage error, not a pass" \
  "$bin" check --baseline context-manifest.json --head context-manifest.json
says err "the same file" "the refusal says why"

# With the default gate the same edit is reported and does not fail; a model change does.
sed 's/^  fail_on: warn$/  fail_on: fail/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml
expect 0 "with the default gate a prompt edit is a warning" "$bin" check --baseline context-manifest.json
says out "[warn] prompt.template_hash" "the warning is still reported"
sed 's/^  id: gpt-x$/  id: gpt-y/' .context-ci.yaml > .context-ci.yaml.new
mv .context-ci.yaml.new .context-ci.yaml
expect 1 "with the default gate a model change fails" "$bin" check --baseline context-manifest.json
says out "[fail] model.id" "the failure names the model"

# --- 2. The documented argument order, and the reports for an unchanged context ------------

expect 0 "flags after the positionals" "$bin" diff context-manifest.json context-manifest.json --format sarif
says out '"results": []' "SARIF for no change carries an empty results array"
says out '"rules": []' "SARIF for no change carries an empty rules array"
expect 0 "help is not an error" "$bin" check -h
says out "usage: context-ci check" "help is on stdout"

# --- 3. The example project the README points at -------------------------------------------

cp -R "$repo/examples/minimal" "$work/example"
expect 0 "examples/minimal matches its committed manifest" \
  "$bin" check --root "$work/example" --baseline "$work/example/context-manifest.json" --commit example
says out "no change in context" "the example is in step"
printf 'Always offer a refund.\n' >> "$work/example/prompts/support.md"
expect 1 "editing the example prompt fails its gate" \
  "$bin" check --root "$work/example" --baseline "$work/example/context-manifest.json" --commit example

# --- 4. A first run with nothing in place ---------------------------------------------------

empty="$work/empty"
mkdir -p "$empty"
cd "$empty"
export GIT_CEILING_DIRECTORIES="$work"
expect 0 "init in an empty directory" "$bin" init
says err "prompts/support.md does not exist yet" "init says which file to create"
expect 3 "manifest says the prompt is missing, before anything about git" "$bin" manifest
says err "capture.sources.prompt names prompts/support.md, which does not exist" "the message names the key and the file"
mkdir prompts
printf 'You are a support agent.\n' > prompts/support.md
expect 2 "manifest outside a repository says how to supply a commit" "$bin" manifest
says err "pass --commit <id>" "the message names the remedy"
expect 0 "manifest with --commit works without git" "$bin" manifest --commit first-run --out context-manifest.json

echo "the quickstart gates: every step behaved as the README says"
