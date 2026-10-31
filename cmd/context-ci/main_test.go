package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repo writes a minimal working tree with a configuration and returns its root.
func repo(t *testing.T, modelID string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\n")
	write(t, root, "config/tools.yaml", `tools:
  - name: refund.create
    description: Create a refund for a customer order.
    schema:
      type: object
`)
	write(t, root, ".context-ci.yaml", `version: 1
capture:
  path: asserted
  sources:
    prompt: prompts/support.md
    tools: config/tools.yaml
model:
  provider: openai
  id: `+modelID+`
retrieval:
  index_id: kb-prod
  embedding_model: embed-3
  top_k: 5
policy:
  version: "2026.11"
`)
	return root
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// manifestFor produces a manifest file for a root and returns its path.
func manifestFor(t *testing.T, root, name string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "--root", root, "--commit", "9c1f2ab", "--out", out}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("manifest exit = %d, stderr: %s", code, stderr.String())
	}
	return out
}

// TestManifestWritesADigest covers the first verb, and that the artefact is
// readable by the second.
func TestManifestWritesADigest(t *testing.T) {
	root := repo(t, "gpt-x")
	path := manifestFor(t, root, "base.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	digest, _ := doc["digest"].(string)
	if len(digest) != 64 {
		t.Fatalf("digest = %q", digest)
	}
	if doc["commit"] != "9c1f2ab" {
		t.Fatalf("commit = %v", doc["commit"])
	}
}

// TestManifestIsReproducible is C-01 at the CLI boundary: two runs, different
// output files, same digest.
func TestManifestIsReproducible(t *testing.T) {
	root := repo(t, "gpt-x")
	first, _ := os.ReadFile(manifestFor(t, root, "a.json"))
	second, _ := os.ReadFile(manifestFor(t, root, "b.json"))

	var a, b map[string]any
	json.Unmarshal(first, &a)
	json.Unmarshal(second, &b)
	if a["digest"] != b["digest"] {
		t.Fatalf("digests differ between runs: %v vs %v", a["digest"], b["digest"])
	}
	if a["created_at"] == b["created_at"] {
		t.Log("created_at matched; the runs were in the same second, which is fine")
	}
}

// TestCheckPassesOnWarnAndFailsOnFail is requirement C-05.
func TestCheckPassesOnWarnAndFailsOnFail(t *testing.T) {
	baseRoot := repo(t, "gpt-x")
	base := manifestFor(t, baseRoot, "base.json")

	t.Run("a warn-class change passes", func(t *testing.T) {
		root := repo(t, "gpt-x")
		write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
		head := manifestFor(t, root, "head.json")

		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--root", root, "--baseline", base, "--head", head}, &stdout, &stderr)
		if code != ExitOK {
			t.Fatalf("exit = %d, want %d\n%s", code, ExitOK, stdout.String())
		}
		if !strings.Contains(stdout.String(), "prompt.rendered_hash") {
			t.Fatalf("the passing run did not report the warning:\n%s", stdout.String())
		}
	})

	t.Run("a fail-class change fails the gate", func(t *testing.T) {
		root := repo(t, "gpt-y")
		head := manifestFor(t, root, "head.json")

		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--root", root, "--baseline", base, "--head", head}, &stdout, &stderr)
		if code != ExitGate {
			t.Fatalf("exit = %d, want %d\n%s", code, ExitGate, stdout.String())
		}
	})
}

// TestCheckOutputNamesTheChangeAndThreshold is requirement C-06: a non-expert has
// to be able to act on the message.
func TestCheckOutputNamesTheChangeAndThreshold(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	root := repo(t, "gpt-y")
	head := manifestFor(t, root, "head.json")

	var stdout, stderr bytes.Buffer
	run([]string{"check", "--root", root, "--baseline", base, "--head", head}, &stdout, &stderr)

	out := stdout.String()
	for _, want := range []string{"model.id", "fail", "the model changed", "gpt-x", "gpt-y"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output does not contain %q:\n%s", want, out)
		}
	}
}

// TestCheckHonoursFailOnWarn covers the configuration path at the CLI boundary.
func TestCheckHonoursFailOnWarn(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	root := repo(t, "gpt-x")
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	write(t, root, ".context-ci.yaml", mustRead(t, filepath.Join(root, ".context-ci.yaml"))+"gate:\n  fail_on: warn\n")
	head := manifestFor(t, root, "head.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--root", root, "--baseline", base, "--head", head}, &stdout, &stderr)
	if code != ExitGate {
		t.Fatalf("exit = %d, want %d with fail_on: warn\n%s", code, ExitGate, stdout.String())
	}
}

// TestGitHubFormatIsEscaped covers the injection risk: a value from a manifest must
// not be able to open a second workflow command.
func TestGitHubFormatIsEscaped(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	root := repo(t, "gpt-y")
	head := manifestFor(t, root, "head.json")

	var stdout, stderr bytes.Buffer
	run([]string{"check", "--root", root, "--baseline", base, "--head", head, "--format", "github"}, &stdout, &stderr)

	out := stdout.String()
	if !strings.Contains(out, "::error") {
		t.Fatalf("no error annotation:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Count(line, "::") > 2 {
			t.Fatalf("a line looks like a second command was injected: %q", line)
		}
	}
}

func TestDiffJSONFormat(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	head := manifestFor(t, repo(t, "gpt-y"), "head.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"diff", "--format", "json", base, head}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("diff exit = %d, stderr: %s", code, stderr.String())
	}
	var result struct {
		Entries []struct {
			Class    string `json:"class"`
			Severity string `json:"severity"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("diff output is not JSON: %v", err)
	}
	if len(result.Entries) == 0 {
		t.Fatal("diff found no entries between two different models")
	}
	if result.Entries[0].Class != "model.id" || result.Entries[0].Severity != "fail" {
		t.Fatalf("first entry = %+v", result.Entries[0])
	}
}

// TestTamperedManifestIsRejected is the guard against the most dangerous failure
// this tool could have. A baseline lives in the repository, so if the digest were
// not rechecked it could be edited to hide a regression, and both the baseline and
// the head would report the same digest while disagreeing about the model.
func TestTamperedManifestIsRejected(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")

	forged := filepath.Join(t.TempDir(), "forged.json")
	raw, err := os.ReadFile(base)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	model, _ := doc["model"].(map[string]any)
	model["id"] = "gpt-y" // change the content, keep the digest
	forgedRaw, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(forged, forgedRaw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	head := manifestFor(t, repo(t, "gpt-y"), "head.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--root", root, "--baseline", forged, "--head", head}, &stdout, &stderr)
	if code != ExitManifest {
		t.Fatalf("exit = %d, want %d for a forged baseline\n%s", code, ExitManifest, stderr.String())
	}
	if !strings.Contains(stderr.String(), "modified after it was produced") {
		t.Fatalf("error does not explain the rejection: %s", stderr.String())
	}
}

// TestManifestWithoutDigestIsRejected covers a hand-written manifest, which would
// otherwise slip past the gate with no provenance at all.
func TestManifestWithoutDigestIsRejected(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")

	raw, _ := os.ReadFile(base)
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	delete(doc, "digest")
	stripped := filepath.Join(t.TempDir(), "stripped.json")
	strippedRaw, _ := json.MarshalIndent(doc, "", "  ")
	os.WriteFile(stripped, strippedRaw, 0o644)

	head := manifestFor(t, root, "head.json")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"diff", stripped, head}, &stdout, &stderr); code != ExitManifest {
		t.Fatalf("exit = %d, want %d for a manifest with no digest", code, ExitManifest)
	}
}

// TestGitHubMessageEscaping covers the difference between the two escaping rules.
// Escaping a colon in the message body would show "%3A" to the reader, because
// GitHub only unescapes percent, carriage return and newline there.
func TestGitHubMessageEscaping(t *testing.T) {
	if got, want := escapeData("a: b, c"), "a: b, c"; got != want {
		t.Fatalf("escapeData(%q) = %q, want %q: a colon in a message is legal", "a: b, c", got, want)
	}
	if got, want := escapeData("line\nbreak"), "line%0Abreak"; got != want {
		t.Fatalf("escapeData(newline) = %q, want %q", got, want)
	}
	if got, want := escapeData("100%"), "100%25"; got != want {
		t.Fatalf("escapeData(percent) = %q, want %q", got, want)
	}
	if got, want := escapeProperty("a: b, c"), "a%3A b%2C c"; got != want {
		t.Fatalf("escapeProperty(%q) = %q, want %q: a colon would end the property list", "a: b, c", got, want)
	}
}

// TestGitHubOutputKeepsPunctuationReadable is the end-to-end version: a message with
// a colon and a comma must appear intact.
func TestGitHubOutputKeepsPunctuationReadable(t *testing.T) {
	base := manifestFor(t, repo(t, "gpt-x"), "base.json")
	root := repo(t, "gpt-y")
	head := manifestFor(t, root, "head.json")

	var stdout, stderr bytes.Buffer
	run([]string{"check", "--root", root, "--baseline", base, "--head", head, "--format", "github"}, &stdout, &stderr)

	out := stdout.String()
	if strings.Contains(out, "%2C") || strings.Contains(out, "%3A") {
		t.Fatalf("punctuation in the message was escaped and will be shown literally:\n%s", out)
	}
	if !strings.Contains(out, "gpt-x -> gpt-y") {
		t.Fatalf("the message body was mangled:\n%s", out)
	}
}

func TestUsageErrors(t *testing.T) {
	root := repo(t, "gpt-x")
	cases := [][]string{
		{},
		{"unknown-verb"},
		{"diff"},
		{"diff", "one.json"},
		{"check", "--root", root},
		{"manifest", "--root", root, "--commit", "abc", "extra"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != ExitUsage {
			t.Fatalf("run(%v) exit = %d, want %d", args, code, ExitUsage)
		}
	}
}

func TestMissingConfigIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "--root", t.TempDir(), "--commit", "abc"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("error does not say the file is missing: %s", stderr.String())
	}
}

func TestClassesVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"classes"}, &stdout, &stderr); code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"model.id", "retrieval.top_k", "policy.version", "fail"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("classes output does not mention %q:\n%s", want, stdout.String())
		}
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(raw)
}
