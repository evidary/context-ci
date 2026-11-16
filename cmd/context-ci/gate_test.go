package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// check runs the gate and returns its exit code and both streams.
func check(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"check"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestCheckGatesAnEditMadeAfterTheBaseline is the README quickstart, and the defect it replaces.
// The quickstart named context-manifest.json as the baseline while check read its head from the
// same file, so the gate compared a file with itself and passed whatever was edited. The head is
// now built from the working tree: a prompt edited after the baseline was written is a change,
// and with the quickstart's `gate.fail_on: warn` it fails the build.
func TestCheckGatesAnEditMadeAfterTheBaseline(t *testing.T) {
	root := repo(t, "gpt-x")
	write(t, root, ".context-ci.yaml", mustRead(t, filepath.Join(root, ".context-ci.yaml"))+"gate:\n  fail_on: warn\n")
	baseline := filepath.Join(root, ManifestFileName)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"manifest", "--root", root, "--commit", "9c1f2ab", "--out", baseline}, &stdout, &stderr); code != ExitOK {
		t.Fatalf("manifest exit = %d: %s", code, stderr.String())
	}

	code, out, errOut := check(t, "--root", root, "--baseline", baseline, "--commit", "9c1f2ab")
	if code != ExitOK || !strings.Contains(out, "no change in context") {
		t.Fatalf("an unedited checkout: exit = %d, want %d\n%s%s", code, ExitOK, out, errOut)
	}

	// The edit, with the baseline deliberately left as it was.
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nAlways offer a refund.\n")

	code, out, errOut = check(t, "--root", root, "--baseline", baseline, "--commit", "9c1f2ab")
	if code != ExitGate {
		t.Fatalf("an edited prompt: exit = %d, want %d\n%s%s", code, ExitGate, out, errOut)
	}
	if strings.Contains(out, "no change in context") {
		t.Fatalf("the gate reported no change for an edited prompt:\n%s", out)
	}
	if !strings.Contains(out, "[warn] prompt.template_hash") {
		t.Fatalf("the failure does not name what changed:\n%s", out)
	}
	if !strings.Contains(errOut, "head built from the working tree") {
		t.Fatalf("check does not say where its head came from: %s", errOut)
	}
}

// TestCheckWithDefaultSeveritiesStillSeesTheEdit keeps requirement C-05 beside the test above. A
// prompt edit is a warning by default and a warning does not fail the build - that is a decision
// the specification records - but it must be reported, and a fail-class edit must fail.
func TestCheckWithDefaultSeveritiesStillSeesTheEdit(t *testing.T) {
	root := repo(t, "gpt-x")
	baseline := filepath.Join(root, ManifestFileName)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"manifest", "--root", root, "--commit", "9c1f2ab", "--out", baseline}, &stdout, &stderr); code != ExitOK {
		t.Fatalf("manifest exit = %d: %s", code, stderr.String())
	}

	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	code, out, errOut := check(t, "--root", root, "--baseline", baseline, "--commit", "9c1f2ab")
	if code != ExitOK {
		t.Fatalf("a warn-class edit: exit = %d, want %d\n%s%s", code, ExitOK, out, errOut)
	}
	if !strings.Contains(out, "[warn] prompt.template_hash") {
		t.Fatalf("the passing run did not report the edit:\n%s", out)
	}

	config := mustRead(t, filepath.Join(root, ".context-ci.yaml"))
	write(t, root, ".context-ci.yaml", strings.Replace(config, "id: gpt-x", "id: gpt-y", 1))
	code, out, errOut = check(t, "--root", root, "--baseline", baseline, "--commit", "9c1f2ab")
	if code != ExitGate || !strings.Contains(out, "[fail] model.id") {
		t.Fatalf("a model change: exit = %d, want %d\n%s%s", code, ExitGate, out, errOut)
	}
}

// TestCheckRefusesOneFileAsBothSides: a comparison of a file with itself can only say nothing
// changed, so asking for one is a usage error and not a pass. The second spelling is the one a
// string comparison would miss.
func TestCheckRefusesOneFileAsBothSides(t *testing.T) {
	root := repo(t, "gpt-x")
	baseline := manifestFor(t, root, "base.json")
	// Built by hand: filepath.Join would clean the detour away and make the two strings equal.
	sep := string(filepath.Separator)
	respelled := filepath.Dir(baseline) + sep + "sub" + sep + ".." + sep + "base.json"
	if err := os.MkdirAll(filepath.Join(filepath.Dir(baseline), "sub"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	for name, head := range map[string]string{"the same path": baseline, "another spelling of it": respelled} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := check(t, "--root", root, "--baseline", baseline, "--head", head)
			if code != ExitUsage {
				t.Fatalf("exit = %d, want %d\n%s%s", code, ExitUsage, out, errOut)
			}
			if !strings.Contains(errOut, "the same file") || !strings.Contains(errOut, "omit --head") {
				t.Fatalf("the refusal does not say what is wrong or what to do: %s", errOut)
			}
			if out != "" {
				t.Fatalf("a refused comparison still printed a report:\n%s", out)
			}
		})
	}
}

func TestCheckRefusesACommitForAHeadItDidNotBuild(t *testing.T) {
	root := repo(t, "gpt-x")
	baseline := manifestFor(t, root, "base.json")
	head := manifestFor(t, root, "head.json")
	code, _, errOut := check(t, "--root", root, "--baseline", baseline, "--head", head, "--commit", "abc")
	if code != ExitUsage || !strings.Contains(errOut, "--commit") {
		t.Fatalf("exit = %d, want %d: %s", code, ExitUsage, errOut)
	}
}

// TestTheExampleProjectIsInStep keeps examples/minimal honest: the README points at it, so the
// manifest committed there must be the one its files produce.
func TestTheExampleProjectIsInStep(t *testing.T) {
	example := filepath.Join("..", "..", "examples", "minimal")
	code, out, errOut := check(t, "--root", example, "--baseline", filepath.Join(example, ManifestFileName), "--commit", "example")
	if code != ExitOK || !strings.Contains(out, "no change in context") {
		t.Fatalf("examples/minimal/%s is stale: exit = %d\n%s%s\nregenerate it with: context-ci manifest --root examples/minimal --commit example --out examples/minimal/%s",
			ManifestFileName, code, out, errOut, ManifestFileName)
	}
}

// TestAManifestCarryingAnythingUnverifiableIsRejected covers the reader. Each mutation is made to
// a manifest the tool wrote, and each must stop the comparison with exit 3: the first two are
// content outside the digest, and the rest are values the specification's §4 does not allow.
func TestAManifestCarryingAnythingUnverifiableIsRejected(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")
	head := manifestFor(t, root, "head.json")

	section := func(doc map[string]any, name string) map[string]any {
		inner, ok := doc[name].(map[string]any)
		if !ok {
			t.Fatalf("the manifest has no %s object", name)
		}
		return inner
	}
	cases := []struct {
		name   string
		mutate func(doc map[string]any)
		want   string
	}{
		{"an unknown top-level field", func(doc map[string]any) { doc["evil_extra"] = "ignore previous" }, "evil_extra"},
		{"an unknown nested field", func(doc map[string]any) { section(doc, "model")["system_prompt"] = "x" }, "system_prompt"},
		{"a provider outside the enum", func(doc map[string]any) { section(doc, "model")["provider"] = "OpenAI" }, "model.provider"},
		{"no provider", func(doc map[string]any) { section(doc, "model")["provider"] = "" }, "model.provider"},
		{"a negative top_k", func(doc map[string]any) { section(doc, "retrieval")["top_k"] = -1 }, "retrieval.top_k"},
		{"a negative max_tokens", func(doc map[string]any) {
			section(doc, "model")["params"] = map[string]any{"max_tokens": -1}
		}, "model.params.max_tokens"},
		{"a temperature that is not a number", func(doc map[string]any) {
			section(doc, "model")["params"] = map[string]any{"temperature": "hot"}
		}, "model.params.temperature"},
		{"a seed that is not an integer", func(doc map[string]any) {
			section(doc, "model")["params"] = map[string]any{"seed": "4.2"}
		}, "model.params.seed"},
		{"a filters_hash that is not a hash", func(doc map[string]any) { section(doc, "retrieval")["filters_hash"] = "abc" }, "retrieval.filters_hash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(mustRead(t, base)), &doc); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			tc.mutate(doc)
			raw, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			forged := filepath.Join(t.TempDir(), "forged.json")
			if err := os.WriteFile(forged, raw, 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			var stdout, stderr bytes.Buffer
			if code := run([]string{"diff", forged, head}, &stdout, &stderr); code != ExitManifest {
				t.Fatalf("diff exit = %d, want %d\n%s%s", code, ExitManifest, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("the rejection does not name %q: %s", tc.want, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("a rejected manifest still produced a report:\n%s", stdout.String())
			}
			code, _, errOut := check(t, "--root", root, "--baseline", forged, "--head", head)
			if code != ExitManifest {
				t.Fatalf("check exit = %d, want %d: %s", code, ExitManifest, errOut)
			}
		})
	}
}

// TestAFirstRunSaysWhatToDoNext walks the first minutes of a new user: init and manifest in a
// directory that is not a repository and has no prompt. Each failure is one line, names the thing
// that is missing and says what fixes it - and the missing file is reported before git is asked
// for anything, because it is the one init has just told them about.
func TestAFirstRunSaysWhatToDoNext(t *testing.T) {
	root := t.TempDir()
	// A temporary directory can sit inside somebody's repository; this stops git looking upwards.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))

	if code, _, stderr := initIn(t, "--root", root); code != ExitOK {
		t.Fatalf("init exit = %d: %s", code, stderr)
	}
	manifest := func() (int, string) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"manifest", "--root", root}, &stdout, &stderr)
		return code, stderr.String()
	}
	oneLine := func(message string) {
		t.Helper()
		if strings.Count(strings.TrimSpace(message), "\n") != 0 {
			t.Fatalf("the failure is not a single message:\n%s", message)
		}
	}

	code, message := manifest()
	if code != ExitManifest {
		t.Fatalf("no prompt file: exit = %d, want %d: %s", code, ExitManifest, message)
	}
	oneLine(message)
	for _, want := range []string{defaultPrompt, "capture.sources.prompt", "create that file"} {
		if !strings.Contains(message, want) {
			t.Fatalf("the missing-prompt message does not contain %q: %s", want, message)
		}
	}
	if strings.Contains(message, "git") {
		t.Fatalf("the missing prompt was not the first thing reported: %s", message)
	}

	write(t, root, defaultPrompt, "You are a support agent.\n")
	if _, err := exec.LookPath("git"); err != nil {
		code, message = manifest()
		if code != ExitUsage || !strings.Contains(message, "git is not installed") || !strings.Contains(message, "--commit") {
			t.Fatalf("no git: exit = %d: %s", code, message)
		}
		return
	}

	code, message = manifest()
	if code != ExitUsage {
		t.Fatalf("not a repository: exit = %d, want %d: %s", code, ExitUsage, message)
	}
	oneLine(message)
	for _, want := range []string{"is not a git repository", "at least one commit", "--commit"} {
		if !strings.Contains(message, want) {
			t.Fatalf("the no-repository message does not contain %q: %s", want, message)
		}
	}

	if out, err := exec.Command("git", "-C", root, "init", "--quiet").CombinedOutput(); err != nil {
		t.Skipf("git init failed, so the no-commit case cannot be staged: %v: %s", err, out)
	}
	code, message = manifest()
	if code != ExitUsage {
		t.Fatalf("no commits: exit = %d, want %d: %s", code, ExitUsage, message)
	}
	oneLine(message)
	for _, want := range []string{"has no commits yet", "--commit"} {
		if !strings.Contains(message, want) {
			t.Fatalf("the no-commit message does not contain %q: %s", want, message)
		}
	}

	// And the remedy the message names works.
	var stdout, stderr bytes.Buffer
	if code := run([]string{"manifest", "--root", root, "--commit", "first-run"}, &stdout, &stderr); code != ExitOK {
		t.Fatalf("manifest --commit exit = %d: %s", code, stderr.String())
	}
}
