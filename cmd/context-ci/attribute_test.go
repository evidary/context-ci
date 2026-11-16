package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// attributeFixture builds two manifests that differ in two classes - the model and the prompt -
// and returns the arguments common to every attribute run against them.
func attributeFixture(t *testing.T) (args []string, results string) {
	t.Helper()
	baseManifest := manifestFor(t, repo(t, "gpt-x"), "base.json")
	head := repo(t, "gpt-y")
	write(t, head, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	headManifest := manifestFor(t, head, "head.json")
	results = t.TempDir()
	return []string{"attribute", "--root", head, "--baseline", baseManifest, "--head", headManifest}, results
}

// TestAttributeStatesConfoundingAndNeverACause is requirement C-08 at the CLI boundary. Two
// classes changed, one regressed case declares one of them: the report names that class as a
// candidate, says the two changes are confounded, and does not say "caused" or "implicated by
// evidence" anywhere.
func TestAttributeStatesConfoundingAndNeverACause(t *testing.T) {
	args, results := attributeFixture(t)
	write(t, results, "base.json", `{"version": 1, "suite": "support-quality", "cases": [
  {"id": "q1", "score": 0.92, "depends_on": ["model.id"]},
  {"id": "q2", "score": 0.80}]}`)
	write(t, results, "head.json", `{"version": 1, "suite": "support-quality", "cases": [
  {"id": "q1", "score": 0.41, "depends_on": ["model.id"]},
  {"id": "q2", "score": 0.80}]}`)
	args = append(args, "--baseline-results", filepath.Join(results, "base.json"), "--results", filepath.Join(results, "head.json"))

	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != ExitOK {
		t.Fatalf("attribute exit = %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"1 of 2 cases regressed and every regressed case declares its dependencies",
		"candidate causes consistent with those declarations, not proven causes",
		"changed classes: model.id, prompt.template_hash",
		"2 context classes changed together, so their effects are confounded",
		"this report is not causal",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("the report does not say %q:\n%s", want, out)
		}
	}
	for _, never := range []string{"caused by", "implicated by evidence", "the cause"} {
		if strings.Contains(out, never) {
			t.Fatalf("the report makes a causal claim (%q):\n%s", never, out)
		}
	}

	stdout.Reset()
	if code := run(append(args, "--format", "github"), &stdout, &stderr); code != ExitOK {
		t.Fatalf("attribute --format github exit = %d: %s", code, stderr.String())
	}
	summary := strings.SplitN(stdout.String(), "\n", 2)[0]
	if !strings.HasPrefix(summary, "::notice title=context-ci attribute::") || !strings.Contains(summary, "confounded") {
		t.Fatalf("the summary annotation does not carry the confounding statement:\n%s", summary)
	}
}

// TestAttributeRefusesResultsItWouldMisread covers the two inputs that used to be accepted in
// silence. Both are exit 3: the file was read and cannot be used.
func TestAttributeRefusesResultsItWouldMisread(t *testing.T) {
	const good = `{"version": 1, "cases": [{"id": "q1", "score": 0.9, "depends_on": ["model.id"]}]}`
	for name, tc := range map[string]struct{ head, want string }{
		"a future version":         {`{"version": 99, "cases": [{"id": "q1", "score": 0.4}]}`, "version 99 is not supported"},
		"a class that is not real": {`{"version": 1, "cases": [{"id": "q1", "score": 0.4, "depends_on": ["model.identifier"]}]}`, `"model.identifier"`},
	} {
		t.Run(name, func(t *testing.T) {
			args, results := attributeFixture(t)
			write(t, results, "base.json", good)
			write(t, results, "head.json", tc.head)
			args = append(args, "--baseline-results", filepath.Join(results, "base.json"), "--results", filepath.Join(results, "head.json"))

			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != ExitManifest {
				t.Fatalf("exit = %d, want %d\n%s%s", code, ExitManifest, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("the error does not say %s: %s", tc.want, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("a refused file still produced a report:\n%s", stdout.String())
			}
		})
	}
}
