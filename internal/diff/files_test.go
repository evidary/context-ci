package diff

import (
	"strings"
	"testing"

	"github.com/evidary/context-ci/internal/manifest"
)

func withFiles(files ...manifest.File) *manifest.Manifest {
	m := base()
	m.Files = files
	return m
}

func file(group, path, fill string, hidden int) manifest.File {
	return manifest.File{Group: group, Path: path, Hash: strings.Repeat(fill, 64), HiddenCharacters: hidden}
}

func entriesOf(r *Result, class Class) []Entry {
	var out []Entry
	for _, e := range r.Entries {
		if e.Class == class {
			out = append(out, e)
		}
	}
	return out
}

// TestFilesAreComparedByPath covers a named group: each file is its own entry, under the
// group's class, with the path a reviewer can open.
func TestFilesAreComparedByPath(t *testing.T) {
	before := withFiles(
		file("prompts", "prompts/a.md", "1", 0),
		file("prompts", "prompts/b.md", "2", 0),
		file("prompts", "prompts/c.md", "3", 0),
	)
	after := withFiles(
		file("prompts", "prompts/a.md", "1", 0), // unchanged
		file("prompts", "prompts/b.md", "9", 0), // changed
		file("prompts", "prompts/d.md", "4", 0), // added; c.md removed
	)
	r := Compare(before, after, Options{})
	got := entriesOf(r, "files.prompts")
	if len(got) != 3 {
		t.Fatalf("want 3 entries for files.prompts, got %d: %+v", len(got), r.Entries)
	}
	want := map[string]string{
		"prompts/b.md": "prompts/b.md changed",
		"prompts/c.md": "prompts/c.md was removed",
		"prompts/d.md": "prompts/d.md was added",
	}
	for _, e := range got {
		if want[e.Path] != e.Summary {
			t.Errorf("%s: summary %q, want %q", e.Path, e.Summary, want[e.Path])
		}
		if e.Severity != SeverityWarn {
			t.Errorf("%s: a named group warns by default, got %s", e.Path, e.Severity)
		}
	}
}

// TestAgentGroupsHaveTheirOwnClassesAndDefaults is the distinction the two built-in groups
// exist to make: an edited instruction file warns, and a changed MCP or hook file fails.
func TestAgentGroupsHaveTheirOwnClassesAndDefaults(t *testing.T) {
	before := withFiles(file("agent.instructions", "AGENTS.md", "1", 0), file("agent.config", ".mcp.json", "2", 0))
	after := withFiles(file("agent.instructions", "AGENTS.md", "8", 0), file("agent.config", ".mcp.json", "9", 0))
	r := Compare(before, after, Options{})
	got := classes(r)
	if got[ClassAgentInstructions].Severity != SeverityWarn {
		t.Errorf("agent.instructions = %q, want warn", got[ClassAgentInstructions].Severity)
	}
	if got[ClassAgentConfig].Severity != SeverityFail {
		t.Errorf("agent.config = %q, want fail", got[ClassAgentConfig].Severity)
	}
	if r.Gating() != SeverityFail {
		t.Errorf("gating severity = %q, want fail", r.Gating())
	}
}

// TestUnchangedFilesAreListedAsUnchanged keeps silence from reading as "not looked at".
func TestUnchangedFilesAreListedAsUnchanged(t *testing.T) {
	m := withFiles(file("prompts", "prompts/a.md", "1", 0))
	r := Compare(m, m, Options{})
	if !r.Equal() {
		t.Fatalf("identical file lists produced entries: %+v", r.Entries)
	}
	listed := map[Class]bool{}
	for _, c := range r.Unchanged {
		listed[c] = true
	}
	for _, c := range []Class{"files.prompts", ClassAgentInstructions, ClassAgentConfig, ClassHiddenCharacters} {
		if !listed[c] {
			t.Errorf("%s is not listed as unchanged", c)
		}
	}
}

// TestHiddenCharactersAreReportedOnlyForWhatChanged is the rule that keeps the class usable. A
// file that is new or edited and holds invisible characters fails the gate; a file that already
// held them and was not touched does not, or every pull request would be red for something its
// author did not do.
func TestHiddenCharactersAreReportedOnlyForWhatChanged(t *testing.T) {
	untouched := file("agent.instructions", "old/AGENTS.md", "1", 5)
	before := withFiles(untouched, file("agent.instructions", "AGENTS.md", "2", 0))
	after := withFiles(untouched, file("agent.instructions", "AGENTS.md", "3", 2), file("agent.instructions", "new/CLAUDE.md", "4", 1))
	r := Compare(before, after, Options{})
	hidden := entriesOf(r, ClassHiddenCharacters)
	if len(hidden) != 2 {
		t.Fatalf("want 2 hidden-character entries, got %d: %+v", len(hidden), hidden)
	}
	for _, e := range hidden {
		if e.Path == "old/AGENTS.md" {
			t.Fatalf("an untouched file was reported: %+v", e)
		}
		if e.Severity != SeverityFail {
			t.Errorf("%s: severity %q, want fail", e.Path, e.Severity)
		}
	}
	if !strings.Contains(hidden[0].Summary, "2 hidden characters") || !strings.Contains(hidden[1].Summary, "1 hidden character") {
		t.Errorf("the counts are not stated: %q, %q", hidden[0].Summary, hidden[1].Summary)
	}
}

// TestHiddenCharactersInAPromptAndAToolDescription covers the two places besides a file where a
// model reads text.
func TestHiddenCharactersInAPromptAndAToolDescription(t *testing.T) {
	head := base()
	head.Prompt.TemplateHash = strings.Repeat("7", 64)
	head.Prompt.HiddenCharacters = 3
	described := strings.Repeat("5", 64)
	head.Tools[0].DescriptionHash = &described
	head.Tools[0].HiddenCharacters = 1
	r := Compare(base(), head, Options{})
	hidden := entriesOf(r, ClassHiddenCharacters)
	if len(hidden) != 2 {
		t.Fatalf("want 2 entries, got %+v", hidden)
	}
	sources := map[string]string{}
	for _, e := range hidden {
		sources[e.Source] = e.Subject
	}
	if _, ok := sources["prompt"]; !ok {
		t.Errorf("no entry is marked as coming from the prompt: %+v", hidden)
	}
	if sources["tools"] != "refund.create" {
		t.Errorf("the tool entry does not name its tool: %+v", hidden)
	}
}

// TestAcknowledgedChangesAreReportedAndDoNotGate is the approval path. Without one, a deliberate
// model change could only be merged by weakening the gate for everybody or by merging red.
func TestAcknowledgedChangesAreReportedAndDoNotGate(t *testing.T) {
	head := base()
	head.Model.ID = "gpt-y"
	head.Policy.Version = "2027.01"

	r := Compare(base(), head, Options{Acknowledge: map[string]bool{"model.id": true}})
	got := classes(r)
	if !got[ClassModelID].Acknowledged || got[ClassModelID].Severity != SeverityFail {
		t.Fatalf("model.id should be acknowledged and still fail-class: %+v", got[ClassModelID])
	}
	if got[ClassPolicyVersion].Acknowledged {
		t.Fatal("policy.version was acknowledged without being named")
	}
	if r.Gating() != SeverityFail {
		t.Fatalf("an unacknowledged fail-class change must still gate, got %q", r.Gating())
	}

	r = Compare(base(), head, Options{Acknowledge: map[string]bool{"model.id": true, "policy.version": true}})
	if r.Gating() != SeverityIgnore {
		t.Fatalf("every change was acknowledged, so nothing should gate; got %q", r.Gating())
	}
	if r.Worst() != SeverityFail || len(r.Entries) != 2 {
		t.Fatalf("acknowledged changes must still be reported at their severity: %+v", r.Entries)
	}
}

// TestAcknowledgeAllDoesNotCoverHiddenCharacters: a blanket approval of text nobody could read
// approves nothing, so that class has to be named.
func TestAcknowledgeAllDoesNotCoverHiddenCharacters(t *testing.T) {
	before := withFiles(file("agent.instructions", "AGENTS.md", "1", 0))
	after := withFiles(file("agent.instructions", "AGENTS.md", "2", 6))
	r := Compare(before, after, Options{AcknowledgeAll: true})
	got := classes(r)
	if !got[ClassAgentInstructions].Acknowledged {
		t.Fatal("all did not acknowledge the instruction file")
	}
	if got[ClassHiddenCharacters].Acknowledged {
		t.Fatal("all acknowledged hidden characters")
	}
	if r.Gating() != SeverityFail {
		t.Fatalf("gating = %q, want fail", r.Gating())
	}
	r = Compare(before, after, Options{AcknowledgeAll: true, Acknowledge: map[string]bool{"hidden_characters": true}})
	if r.Gating() != SeverityIgnore {
		t.Fatalf("naming the class should acknowledge it; gating = %q", r.Gating())
	}
}

// TestAnEditedDescriptionCarriesItsSizeAndCost: the projection used to be printed only for a
// token count that moved under an unchanged description, which no edit produces.
func TestAnEditedDescriptionCarriesItsSizeAndCost(t *testing.T) {
	before, after := base(), base()
	one, two := strings.Repeat("1", 64), strings.Repeat("2", 64)
	before.Tools[0].DescriptionHash, after.Tools[0].DescriptionHash = &one, &two
	after.Tools[0].DescriptionTokens = before.Tools[0].DescriptionTokens + 900

	entry := classes(Compare(before, after, Options{CallsPerDay: 10000, InputUSDPerMTok: 3}))[ClassToolDescription]
	for _, want := range []string{"grew by 900 tokens", "$27.00"} {
		if !strings.Contains(entry.Detail, want) {
			t.Errorf("detail %q does not mention %q", entry.Detail, want)
		}
	}

	after.Tools[0].DescriptionTokens = before.Tools[0].DescriptionTokens - 1
	entry = classes(Compare(before, after, Options{}))[ClassToolDescription]
	if !strings.Contains(entry.Detail, "shrank by 1 token,") {
		t.Errorf("singular is wrong: %q", entry.Detail)
	}
}

// TestAModelThatAppearsIsAChange covers a base with no declared model, which is what a baseline
// built from a commit older than the configuration looks like.
func TestAModelThatAppearsIsAChange(t *testing.T) {
	before := base()
	before.Model = manifest.Model{}
	before.Prompt = manifest.Prompt{}
	r := Compare(before, base(), Options{})
	got := classes(r)
	if got[ClassModelID].Before != "-" || got[ClassModelID].After != "gpt-x" {
		t.Errorf("model.id entry = %+v", got[ClassModelID])
	}
	if got[ClassPromptTemplateHash].Before != "-" {
		t.Errorf("prompt entry = %+v", got[ClassPromptTemplateHash])
	}
}

func TestFileClassNames(t *testing.T) {
	for name, want := range map[string]bool{
		"files.prompts":      true,
		"files.few-shot_2":   true,
		"files.":             false,
		"files.Prompts":      false,
		"files.a.b":          false,
		"agent.instructions": true,
		"agent.config":       true,
		"hidden_characters":  true,
		"agent.other":        false,
	} {
		if IsKnownClass(name) != want {
			t.Errorf("IsKnownClass(%q) = %v, want %v", name, !want, want)
		}
	}
}
