package capture

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/evidary/context-ci/internal/config"
	"github.com/evidary/context-ci/internal/manifest"
)

var fixedTime = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

func parseTools(t *testing.T, body string) []manifest.Tool {
	t.Helper()
	tools, err := ParseTools([]byte(body))
	if err != nil {
		t.Fatalf("ParseTools() error = %v\n%s", err, body)
	}
	return tools
}

// TestToolShapes: the same two tools, written the six ways a repository is likely to hold them,
// are the same two tools. Before this, every shape but the first was rejected with exit 3, so a
// team had to write a converter before the gate could see a single tool.
func TestToolShapes(t *testing.T) {
	shapes := map[string]string{
		"native": `
tools:
  - name: refund.create
    description: Create a refund.
    schema: {type: object, properties: {amount: {type: number}}}
  - name: order.lookup
    description: Look up an order.
    schema: {type: object}
`,
		"chat completions, bare list": `[
  {"type": "function", "function": {"name": "refund.create", "description": "Create a refund.",
     "parameters": {"type": "object", "properties": {"amount": {"type": "number"}}}}},
  {"type": "function", "function": {"name": "order.lookup", "description": "Look up an order.",
     "parameters": {"type": "object"}}}
]`,
		"flattened function tools": `{"tools": [
  {"type": "function", "name": "refund.create", "description": "Create a refund.",
     "parameters": {"type": "object", "properties": {"amount": {"type": "number"}}}},
  {"type": "function", "name": "order.lookup", "description": "Look up an order.",
     "parameters": {"type": "object"}}
]}`,
		"input_schema": `
- name: refund.create
  description: Create a refund.
  input_schema: {type: object, properties: {amount: {type: number}}}
- name: order.lookup
  description: Look up an order.
  input_schema: {type: object}
`,
		"a saved tools/list response": `{"jsonrpc": "2.0", "id": 1, "result": {"tools": [
  {"name": "refund.create", "description": "Create a refund.",
     "inputSchema": {"type": "object", "properties": {"amount": {"type": "number"}}}},
  {"name": "order.lookup", "description": "Look up an order.", "inputSchema": {"type": "object"}}
]}}`,
		"function declarations": `{"tools": [{"functionDeclarations": [
  {"name": "refund.create", "description": "Create a refund.",
     "parameters": {"type": "object", "properties": {"amount": {"type": "number"}}}},
  {"name": "order.lookup", "description": "Look up an order.", "parameters": {"type": "object"}}
]}]}`,
	}

	// The two provider shapes that wrap a definition in {type: function} hash that field too, so
	// they agree with each other and not with the shapes that have no such field. What must hold
	// for every shape is the names, the description hashes, and that a schema edit is seen.
	for name, body := range shapes {
		tools := parseTools(t, body)
		if len(tools) != 2 || tools[0].Name != "order.lookup" || tools[1].Name != "refund.create" {
			t.Fatalf("%s: got %+v", name, tools)
		}
		if got, want := *tools[1].DescriptionHash, hashString("Create a refund."); got != want {
			t.Errorf("%s: description hash = %s, want %s", name, got, want)
		}
		edited := parseTools(t, strings.Replace(body, "number", "integer", 1))
		if edited[1].SchemaHash == tools[1].SchemaHash {
			t.Errorf("%s: changing a parameter's type did not change the schema hash", name)
		}
		if edited[0].SchemaHash != tools[0].SchemaHash {
			t.Errorf("%s: editing one tool changed the other's hash", name)
		}
	}

	same := [][2]string{
		{"native", "input_schema"},
		{"native", "a saved tools/list response"},
		{"native", "function declarations"},
		{"chat completions, bare list", "flattened function tools"},
	}
	for _, pair := range same {
		a, b := parseTools(t, shapes[pair[0]]), parseTools(t, shapes[pair[1]])
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%q and %q describe the same tools and produced different entries", pair[0], pair[1])
		}
	}
}

// TestEveryFieldOfADefinitionIsHashed: a field this reader has no name for is still part of the
// tool. An annotation that says a tool is read-only, a strict flag, an output schema: changing
// any of them changes the hash.
func TestEveryFieldOfADefinitionIsHashed(t *testing.T) {
	base := `{"tools": [{"name": "a", "description": "d", "inputSchema": {"type": "object"}, "annotations": {"readOnlyHint": true}}]}`
	for _, edit := range [][2]string{
		{`"readOnlyHint": true`, `"readOnlyHint": false`},
		{`"annotations": {"readOnlyHint": true}`, `"annotations": {"readOnlyHint": true}, "outputSchema": {"type": "object"}`},
		{`"name": "a",`, `"name": "a", "strict": true,`},
	} {
		edited := strings.Replace(base, edit[0], edit[1], 1)
		if edited == base {
			t.Fatalf("the edit %q did not apply", edit[0])
		}
		if parseTools(t, base)[0].SchemaHash == parseTools(t, edited)[0].SchemaHash {
			t.Errorf("changing %s to %s left the hash unchanged", edit[0], edit[1])
		}
	}
}

func TestToolsFileMistakesAreRefused(t *testing.T) {
	for name, body := range map[string]string{
		"a misspelled list key":       "tool:\n  - name: a\n",
		"a tool with no name":         "tools:\n  - description: d\n",
		"the same name twice":         "tools:\n  - name: a\n  - name: a\n",
		"two schemas for one tool":    "tools:\n  - name: a\n    schema: {}\n    parameters: {}\n",
		"a description that is a map": "tools:\n  - name: a\n    description: {text: d}\n",
		"one page of a longer list":   `{"tools": [{"name": "a"}], "nextCursor": "abc"}`,
		"a scalar":                    "just text\n",
		"two documents":               "tools: []\n---\ntools: []\n",
	} {
		if _, err := ParseTools([]byte(body)); err == nil {
			t.Errorf("%s: ParseTools accepted it", name)
		}
	}
	for name, body := range map[string]string{"an empty file": "", "an empty mapping": "{}\n", "an empty list": "tools: []\n"} {
		if tools := parseTools(t, body); len(tools) != 0 {
			t.Errorf("%s: got %d tools", name, len(tools))
		}
	}
}

func TestAProviderHostedToolIsNamedByItsType(t *testing.T) {
	tools := parseTools(t, `[{"type": "web_search"}, {"type": "function", "name": "a", "parameters": {}}]`)
	if len(tools) != 2 || tools[1].Name != "web_search" {
		t.Fatalf("got %+v", tools)
	}
}

// TestStructuredSourcesIgnoreCommentsAndOrder is the false failure this fixes: a comment added to
// the retrieval file failed the gate, at the default severity, with "the retrieval filter changed".
func TestStructuredSourcesIgnoreCommentsAndOrder(t *testing.T) {
	root, cfg := fixture(t, "prompt\n")
	write(t, root, "config/retrieval.yaml", "index: kb-prod\ntop_k: 5\nfilters:\n  - public\n  - docs\n")
	write(t, root, "config/guardrails.yaml", "block: [pii]\nmode: strict\n")
	before := build(t, cfg, root, fixedTime)

	write(t, root, "config/retrieval.yaml", "# who owns this file\ntop_k: 5 # the depth\nindex: \"kb-prod\"\n\nfilters: [public, docs]\n")
	write(t, root, "config/guardrails.yaml", "mode: strict\n# reordered\nblock:\n  - pii\n")
	after := build(t, cfg, root, fixedTime)
	if *before.Retrieval.FiltersHash != *after.Retrieval.FiltersHash {
		t.Error("a comment, a reordered key and a reformatted list changed the retrieval hash")
	}
	if *before.Policy.GuardrailsHash != *after.Policy.GuardrailsHash {
		t.Error("a comment and a reordered key changed the guardrails hash")
	}
	if before.Digest != after.Digest {
		t.Error("the digest changed although no value did")
	}

	write(t, root, "config/retrieval.yaml", "index: kb-prod\ntop_k: 20\nfilters: [public, docs]\n")
	changed := build(t, cfg, root, fixedTime)
	if *changed.Retrieval.FiltersHash == *before.Retrieval.FiltersHash {
		t.Error("changing top_k in the retrieval file did not change its hash")
	}
	write(t, root, "config/retrieval.yaml", "index: kb-prod\ntop_k: 5\nfilters: [docs, public]\n")
	reordered := build(t, cfg, root, fixedTime)
	if *reordered.Retrieval.FiltersHash == *before.Retrieval.FiltersHash {
		t.Error("the order of a list is a value, and reordering it did not change the hash")
	}
}

func TestAStructuredSourceThatDoesNotParse(t *testing.T) {
	root, cfg := fixture(t, "prompt\n")
	write(t, root, "config/retrieval.yaml", "index: [unterminated\n")
	if _, err := Build(cfg, root, Options{Commit: "c"}); err == nil || !strings.Contains(err.Error(), "config/retrieval.yaml is not valid YAML") {
		t.Fatalf("Build() error = %v, want it to name the file that does not parse", err)
	}
	// A baseline is built leniently: the bytes are compared, and the reader is told.
	var warnings []string
	m, err := Build(cfg, root, Options{Commit: "c", Lenient: true, Warn: func(w string) { warnings = append(warnings, w) }})
	if err != nil {
		t.Fatalf("a lenient build failed: %v", err)
	}
	if m.Retrieval.FiltersHash == nil || len(warnings) != 1 || !strings.Contains(warnings[0], "does not parse") {
		t.Fatalf("hash %v, warnings %v", m.Retrieval.FiltersHash, warnings)
	}
}

// TestATextSourceIsStillHashedAsText: only YAML and JSON are read as values.
func TestATextSourceIsStillHashedAsText(t *testing.T) {
	root, cfg := fixture(t, "prompt\n")
	cfg.Capture.Sources[config.SourceGuardrails] = "config/guardrails.txt"
	write(t, root, "config/guardrails.txt", "never discuss pricing\n")
	before := build(t, cfg, root, fixedTime)
	write(t, root, "config/guardrails.txt", "never discuss pricing\n# not a comment here\n")
	after := build(t, cfg, root, fixedTime)
	if *before.Policy.GuardrailsHash == *after.Policy.GuardrailsHash {
		t.Fatal("an added line in a text file did not change its hash")
	}
}

func TestMatchPattern(t *testing.T) {
	for _, c := range []struct {
		pattern, name string
		want          bool
	}{
		{"AGENTS.md", "AGENTS.md", true},
		{"AGENTS.md", "docs/AGENTS.md", false},
		{"AGENTS.md", "agents.md", false},
		{"**/AGENTS.md", "AGENTS.md", true},
		{"**/AGENTS.md", "a/b/c/AGENTS.md", true},
		{"**/AGENTS.md", "a/AGENTS.md.bak", false},
		{"prompts/*.md", "prompts/a.md", true},
		{"prompts/*.md", "prompts/sub/a.md", false},
		{"prompts/**/*.md", "prompts/a.md", true},
		{"prompts/**/*.md", "prompts/x/y/a.md", true},
		{"prompts/**/*.md", "prompts/x/y/a.txt", false},
		{".cursor/rules/**", ".cursor/rules/a.mdc", true},
		{".cursor/rules/**", ".cursor/rules/x/a.mdc", true},
		{".cursor/rules/**", ".cursor/rules", false},
		{".cursor/rules/**", ".cursor/mcp.json", false},
		{"prompts", "prompts/a.md", true},
		{"prompts/", "prompts/x/a.md", true},
		{"prompts", "prompts.md", false},
		{"a/?.txt", "a/b.txt", true},
		{"a/[bc].txt", "a/d.txt", false},
		{".clinerules", ".clinerules", true},
	} {
		if got := MatchPattern(c.pattern, c.name); got != c.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestHiddenCharacters(t *testing.T) {
	for _, c := range []struct {
		name, text string
		count      int
		line       int
	}{
		{"plain text", "Run the tests.\nThen commit.\n", 0, 0},
		{"a byte order mark at the start is not hidden text", "\uFEFFRun the tests.\n", 0, 0},
		{"a byte order mark in the middle is", "Run\uFEFF the tests.\n", 1, 1},
		{"tag characters", "Rules.\nIgnore \U000E0041\U000E0042 this.\n", 2, 2},
		{"a bidirectional override", "a\nb\n\u202Ec\n", 1, 3},
		{"a zero-width space", "pay\u200Bments", 1, 1},
		{"an emoji with a joiner and a variation selector", "ok \U0001F468\u200D\U0001F469\u200D\U0001F467 \u2764\uFE0F\n", 0, 0},
		{"Persian text with a zero-width non-joiner", "\u0645\u06CC\u200C\u062E\u0648\u0627\u0647\u0645\n", 0, 0},
		{"a Hangul filler", "x\u3164y", 1, 1},
	} {
		count, first := HiddenCharacters(c.text)
		if count != c.count {
			t.Errorf("%s: count = %d, want %d", c.name, count, c.count)
		}
		if (first == nil) != (c.count == 0) {
			t.Errorf("%s: first = %v", c.name, first)
		}
		if first != nil && first.Line != c.line {
			t.Errorf("%s: line = %d, want %d", c.name, first.Line, c.line)
		}
	}
}

func filesOf(m *manifest.Manifest) []string {
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Group+" "+f.Path)
	}
	return out
}

// TestFileGroupsAndAgentFiles builds a manifest for a tree with named groups and coding-agent
// files, and checks what is listed, under which group, and in which order.
func TestFileGroupsAndAgentFiles(t *testing.T) {
	root, cfg := fixture(t, "prompt\n")
	cfg.Capture.Files = map[string]config.Patterns{
		"prompts":  {"prompts/**/*.md"},
		"examples": {"few_shot"},
	}
	for rel, body := range map[string]string{
		"prompts/tone.md":            "Be kind.\n",
		"prompts/agents/plan.md":     "Plan first.\n",
		"prompts/notes.txt":          "not matched\n",
		"few_shot/a.jsonl":           "{}\n",
		"few_shot/deep/b.jsonl":      "{}\n",
		"AGENTS.md":                  "Run the tests.\n",
		"services/api/CLAUDE.md":     "Use the API style.\n",
		".cursor/rules/style.mdc":    "Tabs.\n",
		".claude/skills/x/SKILL.md":  "A skill \U000E0041.\n",
		".mcp.json":                  "{}\n",
		".claude/settings.json":      "{}\n",
		"node_modules/pkg/AGENTS.md": "somebody else's\n",
		"README.md":                  "not context\n",
	} {
		write(t, root, rel, body)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	m := build(t, cfg, root, fixedTime)
	want := []string{
		"agent.config .claude/settings.json",
		"agent.config .mcp.json",
		"agent.instructions .claude/skills/x/SKILL.md",
		"agent.instructions .cursor/rules/style.mdc",
		"agent.instructions AGENTS.md",
		"agent.instructions services/api/CLAUDE.md",
		"examples few_shot/a.jsonl",
		"examples few_shot/deep/b.jsonl",
		// prompts/support.md is the prompt source, so the group does not list it a second time.
		"prompts prompts/agents/plan.md",
		"prompts prompts/tone.md",
	}
	if got := filesOf(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("files:\n got %q\nwant %q", got, want)
	}
	for _, f := range m.Files {
		wantHidden := 0
		if f.Path == ".claude/skills/x/SKILL.md" {
			wantHidden = 1
		}
		if f.HiddenCharacters != wantHidden {
			t.Errorf("%s: hidden_characters = %d, want %d", f.Path, f.HiddenCharacters, wantHidden)
		}
	}

	off := false
	cfg.Capture.AgentFiles = &off
	for _, f := range build(t, cfg, root, fixedTime).Files {
		if strings.HasPrefix(f.Group, "agent.") {
			t.Fatalf("agent_files: false still listed %s", f.Path)
		}
	}
}

func TestAFilePatternThatMatchesNothing(t *testing.T) {
	root, cfg := fixture(t, "prompt\n")
	cfg.Capture.Files = map[string]config.Patterns{"examples": {"few_shot/a.jsonl"}}
	if _, err := Build(cfg, root, Options{Commit: "c"}); err == nil || !strings.Contains(err.Error(), "capture.files.examples names few_shot/a.jsonl") {
		t.Fatalf("a path written in full and not there should be an error naming the key; got %v", err)
	}
	// A wildcard that matches nothing may be right, so it is said and not refused.
	cfg.Capture.Files = map[string]config.Patterns{"examples": {"few_shot/*.jsonl"}}
	var warnings []string
	if _, err := Build(cfg, root, Options{Commit: "c", Warn: func(w string) { warnings = append(warnings, w) }}); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "few_shot/*.jsonl matches no file") {
		t.Fatalf("warnings = %v", warnings)
	}
}

// TestAManifestWithNoModelAndNoPrompt is the zero-configuration case: the defaults, in a
// repository that has an agent file and nothing else.
func TestAManifestWithNoModelAndNoPrompt(t *testing.T) {
	root := t.TempDir()
	write(t, root, "AGENTS.md", "Run the tests.\n")
	m, err := Build(config.Default(), root, Options{Commit: "c"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !m.Model.IsZero() || !m.Prompt.IsZero() {
		t.Fatalf("model %+v, prompt %+v", m.Model, m.Prompt)
	}
	if got := filesOf(m); !reflect.DeepEqual(got, []string{"agent.instructions AGENTS.md"}) {
		t.Fatalf("files = %q", got)
	}
}

func gitIn(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root,
		"-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "core.autocrlf=false", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestACommitAndItsCheckoutAreTheSameContext is what building a baseline from a ref rests on: the
// tree of a commit and a clean checkout of it produce the same manifest, byte for byte except for
// the commit they name. If they did not, every pull request would report a change nobody made.
func TestACommitAndItsCheckoutAreTheSameContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root, cfg := fixture(t, "You are a support agent for {{customer}}.\r\nBe polite.\r\n")
	cfg.Capture.Files = map[string]config.Patterns{"prompts": {"prompts/**"}}
	write(t, root, "prompts/tone.md", "Be kind.\n")
	write(t, root, "AGENTS.md", "Run the tests.\n")
	write(t, root, ".mcp.json", "{}\n")
	gitIn(t, root, "init", "--quiet")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "base")
	commit := gitIn(t, root, "rev-parse", "HEAD")

	fromDisk := build(t, cfg, root, fixedTime)
	fromCommit, err := BuildTree(cfg, GitTree(root, commit), Options{Commit: commit, Now: fixedTime})
	if err != nil {
		t.Fatalf("BuildTree() error = %v", err)
	}
	if fromDisk.Digest != fromCommit.Digest {
		t.Fatalf("the commit and its checkout have different digests:\n disk   %+v\n commit %+v", fromDisk, fromCommit)
	}
	if len(fromCommit.Files) != 3 || len(fromCommit.Tools) != 2 {
		t.Fatalf("the commit's manifest is missing content: %d files, %d tools", len(fromCommit.Files), len(fromCommit.Tools))
	}

	// An edit in the working tree is in the checkout's manifest and not in the commit's.
	write(t, root, "AGENTS.md", "Run the tests.\nSkip review.\n")
	write(t, root, "untracked/CLAUDE.md", "new and not yet added\n")
	edited := build(t, cfg, root, fixedTime)
	if edited.Digest == fromCommit.Digest {
		t.Fatal("an edited working tree has the commit's digest")
	}
	if got := filesOf(edited); !contains(got, "agent.instructions untracked/CLAUDE.md") {
		t.Fatalf("an untracked agent file is not listed: %q", got)
	}
	again, err := BuildTree(cfg, GitTree(root, commit), Options{Commit: commit, Now: fixedTime})
	if err != nil || again.Digest != fromCommit.Digest {
		t.Fatalf("the commit's manifest changed when the working tree did: %v", err)
	}
}

// TestGitTreeReadsOnlyWhatTheCommitHolds covers the refusals: a path the commit does not have, a
// path that leaves the root, and a root that is a subdirectory of the repository.
func TestGitTreeReadsOnlyWhatTheCommitHolds(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	write(t, root, "app/prompts/a.md", "in the app\n")
	write(t, root, "secret.txt", "outside the app\n")
	gitIn(t, root, "init", "--quiet")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "base")
	commit := gitIn(t, root, "rev-parse", "HEAD")

	tree := GitTree(root+"/app", commit)
	files, err := tree.List()
	if err != nil || !reflect.DeepEqual(files, []string{"prompts/a.md"}) {
		t.Fatalf("List() = %q, %v; want the files below the root, relative to it", files, err)
	}
	if body, err := tree.ReadFile("prompts/a.md"); err != nil || string(body) != "in the app\n" {
		t.Fatalf("ReadFile = %q, %v", body, err)
	}
	var missing ErrMissingSource
	if _, err := tree.ReadFile("prompts/b.md"); !errors.As(err, &missing) {
		t.Fatalf("a path the commit lacks: %v", err)
	}
	var outside ErrOutsideRoot
	if _, err := tree.ReadFile("../secret.txt"); !errors.As(err, &outside) {
		t.Fatalf("a path above the root: %v", err)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
