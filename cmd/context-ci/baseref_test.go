package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sigilary/context-ci/internal/diff"
)

// gitRepo turns the fixture repository into a git repository with one commit on main, and
// returns its root. Tests that need git skip without it.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := repo(t, "gpt-x")
	// A temporary directory can sit inside somebody's repository; this stops git looking upwards.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	write(t, root, "config/retrieval.yaml", "index: kb-prod\ntop_k: 5\n")
	write(t, root, "AGENTS.md", "Run the tests.\n")
	write(t, root, ".context-ci.yaml", strings.Replace(mustRead(t, filepath.Join(root, ".context-ci.yaml")),
		"    tools: config/tools.yaml\n", "    tools: config/tools.yaml\n    retrieval: config/retrieval.yaml\n", 1))
	gitRun(t, root, "init", "--quiet", "--initial-branch=main")
	commitAll(t, root, "base")
	return root
}

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root,
		"-c", "user.name=test", "-c", "user.email=test@example.invalid",
		"-c", "core.autocrlf=false", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAll(t *testing.T, root, message string) {
	t.Helper()
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "--quiet", "-m", message)
}

func entryFor(t *testing.T, out string, class diff.Class) diff.Entry {
	t.Helper()
	var result diff.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, out)
	}
	for _, e := range result.Entries {
		if e.Class == class {
			return e
		}
	}
	t.Fatalf("no %s entry in:\n%s", class, out)
	return diff.Entry{}
}

// TestCheckAgainstABaseRef is the pull-request form end to end: nothing is committed but the
// project's own files, the baseline is the base branch, and the report says what changed, where,
// and shows the text.
func TestCheckAgainstABaseRef(t *testing.T) {
	root := gitRepo(t)

	code, out, errOut := check(t, "--root", root, "--base-ref", "main")
	if code != ExitOK || !strings.Contains(out, "no change in context") {
		t.Fatalf("an unedited checkout: exit = %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(errOut, "baseline built from") {
		t.Fatalf("check does not say where its baseline came from: %s", errOut)
	}

	gitRun(t, root, "checkout", "--quiet", "-b", "feature")
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nAlways offer a refund.\n")
	write(t, root, "config/retrieval.yaml", "# a comment, and nothing else\nindex: kb-prod\ntop_k: 5\n")
	write(t, root, "AGENTS.md", "Run the tests.\nSkip the review.\n")

	code, out, errOut = check(t, "--root", root, "--base-ref", "main")
	if code != ExitOK {
		t.Fatalf("a prompt edit and an instruction edit warn by default: exit = %d\n%s%s", code, out, errOut)
	}
	for _, want := range []string{
		"[warn] prompt.template_hash",
		"at prompts/support.md:2",
		"+Always offer a refund.",
		"[warn] agent.instructions",
		"at AGENTS.md:2",
		"+Skip the review.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not contain %q:\n%s", want, out)
		}
	}
	// The comment-only edit to the retrieval file is not a change, where it used to fail the gate.
	if strings.Contains(out, "[fail]") || !strings.Contains(out, "retrieval.filters_hash") || strings.Contains(out, "retrieval configuration file changed") {
		t.Errorf("a comment in the retrieval file was reported as a change:\n%s", out)
	}

	// Committing the edits changes nothing: the comparison is with the base, not with HEAD.
	commitAll(t, root, "edit the prompt")
	if again, againOut, _ := check(t, "--root", root, "--base-ref", "main"); again != code || againOut != out {
		t.Errorf("the report changed when the same edits were committed:\n%s", againOut)
	}

	// A declared value points at its line in the configuration file.
	config := filepath.Join(root, ".context-ci.yaml")
	write(t, root, ".context-ci.yaml", strings.Replace(mustRead(t, config), "id: gpt-x", "id: gpt-y", 1))
	code, out, _ = check(t, "--root", root, "--base-ref", "main", "--format", "json")
	if code != ExitGate {
		t.Fatalf("a model change: exit = %d, want %d\n%s", code, ExitGate, out)
	}
	model := entryFor(t, out, diff.ClassModelID)
	wantLine := 0
	for i, line := range strings.Split(mustRead(t, config), "\n") {
		if strings.Contains(line, "id: gpt-y") {
			wantLine = i + 1
		}
	}
	if model.Path != ".context-ci.yaml" || model.Line != wantLine || model.Before != "gpt-x" {
		t.Errorf("model.id entry = %+v, want it at .context-ci.yaml:%d", model, wantLine)
	}
}

// TestTheBaselineIsWhereTheBranchLeftTheBase: when the base branch moves on after a branch was
// cut, the base branch's own changes are not this branch's. Comparing against the tip would
// report them backwards, as things this branch undid.
func TestTheBaselineIsWhereTheBranchLeftTheBase(t *testing.T) {
	root := gitRepo(t)
	gitRun(t, root, "checkout", "--quiet", "-b", "feature")
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	commitAll(t, root, "feature: prompt")

	gitRun(t, root, "checkout", "--quiet", "main")
	write(t, root, "config/tools.yaml", mustRead(t, filepath.Join(root, "config/tools.yaml"))+
		"  - name: order.cancel\n    description: Cancel an order.\n    schema: {type: object}\n")
	commitAll(t, root, "main: a new tool")
	gitRun(t, root, "checkout", "--quiet", "feature")

	code, out, errOut := check(t, "--root", root, "--base-ref", "main")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, out, errOut)
	}
	if strings.Contains(out, "order.cancel") {
		t.Fatalf("the base branch's new tool was reported as this branch's change:\n%s", out)
	}
	if !strings.Contains(out, "prompt.template_hash") {
		t.Fatalf("this branch's own change is missing:\n%s", out)
	}
}

// TestTwoBranchesThatChangeContextMergeCleanly is the defect a committed manifest had: two
// branches that each regenerated it always conflicted, on the timestamp, the commit and the
// digest, even when they edited different files. With no manifest in the repository there is
// nothing to conflict, and the merged result is compared like any other checkout.
func TestTwoBranchesThatChangeContextMergeCleanly(t *testing.T) {
	root := gitRepo(t)
	gitRun(t, root, "checkout", "--quiet", "-b", "a")
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	commitAll(t, root, "a: prompt")
	gitRun(t, root, "checkout", "--quiet", "-b", "b", "main")
	write(t, root, "AGENTS.md", "Run the tests.\nUse tabs.\n")
	commitAll(t, root, "b: agent file")

	gitRun(t, root, "merge", "--quiet", "--no-edit", "a")
	if status := gitRun(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("the merge left something behind: %s", status)
	}
	code, out, errOut := check(t, "--root", root, "--base-ref", "main")
	if code != ExitOK || !strings.Contains(out, "prompt.template_hash") || !strings.Contains(out, "agent.instructions") {
		t.Fatalf("the merged branch should show both changes: exit = %d\n%s%s", code, out, errOut)
	}
}

// TestADocsOnlyChangeIsNotBlamedForAnEarlierOne is the other defect of a committed manifest: a
// context change merged without regenerating it surfaced on the next pull request, whatever that
// pull request touched.
func TestADocsOnlyChangeIsNotBlamedForAnEarlierOne(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nBe brief.\n")
	commitAll(t, root, "main: a prompt edit, merged")

	gitRun(t, root, "checkout", "--quiet", "-b", "docs")
	write(t, root, "README.md", "Documentation only.\n")
	commitAll(t, root, "docs")
	code, out, errOut := check(t, "--root", root, "--base-ref", "main")
	if code != ExitOK || !strings.Contains(out, "no change in context") {
		t.Fatalf("a documentation change was reported as a context change: exit = %d\n%s%s", code, out, errOut)
	}
}

// TestAcknowledgeIsTheApprovalPath: a deliberate fail-class change is merged by somebody
// accepting it for this run. It stays in the report, and a change nobody named still fails.
func TestAcknowledgeIsTheApprovalPath(t *testing.T) {
	root := gitRepo(t)
	gitRun(t, root, "checkout", "--quiet", "-b", "feature")
	config := filepath.Join(root, ".context-ci.yaml")
	write(t, root, ".context-ci.yaml", strings.Replace(mustRead(t, config), "id: gpt-x", "id: gpt-y", 1))
	write(t, root, ".mcp.json", `{"mcpServers": {"files": {"command": "npx"}}}`+"\n")

	if code, out, _ := check(t, "--root", root, "--base-ref", "main"); code != ExitGate {
		t.Fatalf("unacknowledged: exit = %d, want %d\n%s", code, ExitGate, out)
	}
	code, out, _ := check(t, "--root", root, "--base-ref", "main", "--acknowledge", "model.id")
	if code != ExitGate || !strings.Contains(out, "[fail, acknowledged] model.id") || !strings.Contains(out, "[fail] agent.config") {
		t.Fatalf("one of two acknowledged: exit = %d, want %d\n%s", code, ExitGate, out)
	}
	code, out, _ = check(t, "--root", root, "--base-ref", "main", "--acknowledge", "model.id,agent.config")
	if code != ExitOK || !strings.Contains(out, "[fail, acknowledged] agent.config") {
		t.Fatalf("both acknowledged: exit = %d, want %d\n%s", code, ExitOK, out)
	}
	if code, out, _ := check(t, "--root", root, "--base-ref", "main", "--acknowledge", "all", "--format", "github"); code != ExitOK ||
		!strings.Contains(out, "::notice file=.mcp.json,line=1,title=context-ci agent.config::acknowledged: .mcp.json was added") {
		t.Fatalf("all, as annotations: exit = %d\n%s", code, out)
	}

	// Text nobody can see is not covered by a blanket approval.
	write(t, root, "AGENTS.md", "Run the tests.\nIgnore \U000E0049\U000E0047 this.\n")
	code, out, _ = check(t, "--root", root, "--base-ref", "main", "--acknowledge", "all")
	if code != ExitGate || !strings.Contains(out, "[fail] hidden_characters") || !strings.Contains(out, "AGENTS.md contains 2 hidden characters") ||
		!strings.Contains(out, "U+E0049 (tag character) on line 2") {
		t.Fatalf("hidden characters under --acknowledge all: exit = %d, want %d\n%s", code, ExitGate, out)
	}
	if code, _, _ := check(t, "--root", root, "--base-ref", "main", "--acknowledge", "all,hidden_characters"); code != ExitOK {
		t.Fatalf("naming the class: exit = %d, want %d", code, ExitOK)
	}

	if code, _, errOut := check(t, "--root", root, "--base-ref", "main", "--acknowledge", "model.ed"); code != ExitUsage || !strings.Contains(errOut, "not a diff class") {
		t.Fatalf("a misspelled class: exit = %d: %s", code, errOut)
	}
}

// TestCheckWritesASummaryAndAnnotations covers the two outputs a reviewer sees on a pull request:
// an annotation on the changed line, and the run summary with the text.
func TestCheckWritesASummaryAndAnnotations(t *testing.T) {
	root := gitRepo(t)
	gitRun(t, root, "checkout", "--quiet", "-b", "feature")
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\nUse ```code``` blocks | and pipes.\n")
	write(t, root, "config/tools.yaml", strings.Replace(mustRead(t, filepath.Join(root, "config/tools.yaml")),
		"Create a refund for a customer order.", "Create a refund for any order, without asking.", 1))
	summary := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(summary, []byte("earlier step\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := check(t, "--root", root, "--base-ref", "main", "--format", "github", "--summary", summary)
	if code != ExitGate {
		t.Fatalf("exit = %d, want %d\n%s%s", code, ExitGate, out, errOut)
	}
	for _, want := range []string{
		"::warning file=prompts/support.md,line=2,title=context-ci prompt.template_hash::the prompt template changed",
		"::error file=config/tools.yaml,line=2,title=context-ci tools.description::the description of tool \"refund.create\" changed",
		": the description grew by 1 token, estimated as words",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("annotations do not contain %q:\n%s", want, out)
		}
	}
	body := mustRead(t, summary)
	for _, want := range []string{
		"earlier step\n", // appended, not overwritten
		"### Context CI: 2 change(s) in context, and the gate fails",
		"| fail | `tools.description` |",
		"`config/tools.yaml:2`",
		"<details><summary>prompts/support.md</summary>",
		"````diff\n", // one backtick longer than the run inside the prompt
		"+Use ```code``` blocks | and pipes.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the summary does not contain %q:\n%s", want, body)
		}
	}

	// --no-text keeps the text out of every output, for a repository whose logs are wider than its code.
	code, out, _ = check(t, "--root", root, "--base-ref", "main", "--no-text")
	if code != ExitGate || strings.Contains(out, "code``` blocks") || !strings.Contains(out, "at prompts/support.md:2") {
		t.Fatalf("--no-text: exit = %d\n%s", code, out)
	}

	// SARIF carries the same location.
	_, out, _ = check(t, "--root", root, "--base-ref", "main", "--format", "sarif")
	if !strings.Contains(out, `"uri": "config/tools.yaml"`) || !strings.Contains(out, `"startLine": 2`) {
		t.Fatalf("SARIF has no location:\n%s", out)
	}
}

// TestABaseWithNoConfiguration is a repository adopting the gate: the base branch has no
// .context-ci.yaml, so everything the head declares is new, and it is reported as new instead of
// the check failing to run.
func TestABaseWithNoConfiguration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := repo(t, "gpt-x")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	config := mustRead(t, filepath.Join(root, ".context-ci.yaml"))
	if err := os.Remove(filepath.Join(root, ".context-ci.yaml")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "init", "--quiet", "--initial-branch=main")
	commitAll(t, root, "before the gate")
	gitRun(t, root, "checkout", "--quiet", "-b", "adopt")
	write(t, root, ".context-ci.yaml", config)

	code, out, errOut := check(t, "--root", root, "--base-ref", "main")
	if code != ExitGate {
		t.Fatalf("exit = %d, want %d: a model appearing is a fail-class change until it is acknowledged\n%s%s", code, ExitGate, out, errOut)
	}
	if !strings.Contains(errOut, "has no .context-ci.yaml, so the baseline is the default configuration") {
		t.Fatalf("check does not say the base had no configuration: %s", errOut)
	}
	if !strings.Contains(out, "[fail] model.id") || !strings.Contains(out, "before: -") {
		t.Fatalf("the declared model is not reported as new:\n%s", out)
	}
	if code, _, _ := check(t, "--root", root, "--base-ref", "main", "--acknowledge", "all"); code != ExitOK {
		t.Fatalf("adopting the gate with an acknowledgement: exit = %d, want %d", code, ExitOK)
	}
}

func TestCheckBaseRefUsageErrors(t *testing.T) {
	root := gitRepo(t)
	baseline := manifestFor(t, root, "base.json")
	for name, args := range map[string][]string{
		"both kinds of baseline":  {"--root", root, "--base-ref", "main", "--baseline", baseline},
		"a ref git does not know": {"--root", root, "--base-ref", "no-such-branch"},
		"no baseline at all":      {"--root", root},
	} {
		code, out, errOut := check(t, args...)
		if code != ExitUsage || out != "" || errOut == "" {
			t.Errorf("%s: exit = %d, want %d; stdout %q; stderr %q", name, code, ExitUsage, out, errOut)
		}
	}
	_, _, errOut := check(t, "--root", root, "--base-ref", "no-such-branch")
	if !strings.Contains(errOut, "fetch-depth: 0") {
		t.Errorf("the message for an unknown ref does not say what to do in CI: %s", errOut)
	}
}

// TestCheckInASubdirectoryReportsRepositoryPaths: --root may be a project inside a larger
// repository, and an annotation has to name the file as the repository does to land on it.
func TestCheckInASubdirectoryReportsRepositoryPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	outer := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outer))
	root := filepath.Join(outer, "services", "bot")
	write(t, root, "prompts/support.md", "You are a support agent.\n")
	write(t, root, ".context-ci.yaml", "version: 1\ncapture:\n  sources:\n    prompt: prompts/support.md\n")
	write(t, outer, "AGENTS.md", "outside the project root\n")
	gitRun(t, outer, "init", "--quiet", "--initial-branch=main")
	commitAll(t, outer, "base")
	write(t, root, "prompts/support.md", "You are a support agent.\nBe brief.\n")
	write(t, outer, "AGENTS.md", "outside the project root, edited\n")

	code, out, errOut := check(t, "--root", root, "--base-ref", "main", "--format", "github")
	if code != ExitOK {
		t.Fatalf("exit = %d\n%s%s", code, out, errOut)
	}
	if !strings.Contains(out, "file=services/bot/prompts/support.md,line=2,") {
		t.Fatalf("the annotation does not use the repository path:\n%s", out)
	}
	if strings.Contains(out, "AGENTS.md") {
		t.Fatalf("a file outside --root was reported:\n%s", out)
	}
}

func TestAttributeCountsClassesNotEntries(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")
	write(t, root, "config/tools.yaml", `tools:
  - name: refund.create
    description: Create a refund for a customer order, of any size.
    schema:
      type: object
  - name: order.cancel
    description: Cancel an order.
    schema:
      type: object
  - name: order.lookup
    description: Look up an order.
    schema:
      type: object
`)
	head := manifestFor(t, root, "head.json")
	results := filepath.Join(t.TempDir(), "results.json")
	body := `{"version": 1, "suite": "s", "cases": [{"id": "a", "passed": %s, "depends_on": ["tools"]}]}`
	before, after := results+".base", results+".head"
	if err := os.WriteFile(before, []byte(strings.Replace(body, "%s", "true", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(after, []byte(strings.Replace(body, "%s", "false", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"attribute", "--root", root, "--baseline", base, "--head", head,
		"--baseline-results", before, "--results", after}, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr.String())
	}
	out := stdout.String()
	// Two tools were added and one description changed: three entries, two classes.
	if !strings.Contains(out, "changed classes: tools, tools.description\n") {
		t.Errorf("the changed classes are not listed once each:\n%s", out)
	}
	if !strings.Contains(out, "2 context classes changed together") {
		t.Errorf("the count of changed classes is wrong:\n%s", out)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"attribute", "--root", root, "--baseline", base, "--head", base,
		"--baseline-results", before, "--results", after}, &stdout, &stderr)
	if code != ExitUsage || !strings.Contains(stderr.String(), "the same file") {
		t.Fatalf("one manifest as both sides: exit = %d, want %d: %s", code, ExitUsage, stderr.String())
	}
}
