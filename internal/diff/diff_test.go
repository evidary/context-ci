package diff

import (
	"strings"
	"testing"

	"github.com/context-ci/context-ci/internal/manifest"
)

func base() *manifest.Manifest {
	alias := "x-latest"
	filters := strings.Repeat("f", 64)
	guard := strings.Repeat("9", 64)
	return &manifest.Manifest{
		ManifestVersion: 1,
		CreatedAt:       "2026-12-01T10:00:00Z",
		Commit:          "9c1f2ab",
		Capture:         manifest.Capture{Path: manifest.CaptureAsserted},
		Model: manifest.Model{
			Provider: "openai",
			ID:       "gpt-x",
			AliasOf:  &alias,
			Params:   manifest.ModelParam{Temperature: strPtr("0"), MaxTokens: intPtr(1024)},
		},
		Prompt: manifest.Prompt{
			TemplateHash: strings.Repeat("a", 64),
			RenderedHash: strings.Repeat("b", 64),
			Variables:    map[string]string{"tone": "string"},
		},
		Tools: []manifest.Tool{
			{Name: "refund.create", SchemaHash: strings.Repeat("c", 64), DescriptionTokens: 12},
			{Name: "ticket.read", SchemaHash: strings.Repeat("d", 64), DescriptionTokens: 5},
		},
		Retrieval: manifest.Retrieval{
			IndexID: "kb-prod", EmbeddingModel: "embed-3", EmbeddingModelVersion: "2026-05",
			Chunker: "recursive-800", TopK: 5, FiltersHash: &filters,
		},
		Policy: manifest.Policy{Version: "2026.11", GuardrailsHash: &guard},
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func classes(r *Result) map[Class]Entry {
	out := make(map[Class]Entry, len(r.Entries))
	for _, e := range r.Entries {
		out[e.Class] = e
	}
	return out
}

// TestNoChangeHasNoEntries guards the most common outcome in a real repository.
func TestNoChangeHasNoEntries(t *testing.T) {
	r := Compare(base(), base(), Options{})
	if !r.Equal() {
		t.Fatalf("identical manifests produced %d entries", len(r.Entries))
	}
	if r.Worst() != SeverityIgnore {
		t.Fatalf("worst severity = %q, want ignore", r.Worst())
	}
}

// TestClassifiesTheTenSpecifiedClasses is requirement C-04. Each case mutates one
// field and asserts the class and the default severity the specification assigns.
func TestClassifiesTheTenSpecifiedClasses(t *testing.T) {
	cases := []struct {
		name    string
		class   Class
		want    Severity
		mutate  func(*manifest.Manifest)
		summary string
	}{
		{
			name: "model.id changed", class: ClassModelID, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Model.ID = "gpt-y" },
			summary: "the model changed",
		},
		{
			name: "model.alias_of changed", class: ClassModelAliasOf, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Model.AliasOf = strPtr("elsewhere") },
			summary: "the model alias resolves differently",
		},
		{
			name: "model.params changed", class: ClassModelParams, want: SeverityWarn,
			mutate:  func(m *manifest.Manifest) { m.Model.Params.Temperature = strPtr("0.7") },
			summary: "sampling parameters changed",
		},
		{
			name: "prompt.rendered_hash changed", class: ClassPromptRenderedHash, want: SeverityWarn,
			mutate:  func(m *manifest.Manifest) { m.Prompt.RenderedHash = strings.Repeat("1", 64) },
			summary: "the rendered prompt changed",
		},
		{
			name: "tool removed", class: ClassTools, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Tools = m.Tools[:1] },
			summary: "was removed",
		},
		{
			name: "tool added", class: ClassTools, want: SeverityFail,
			mutate: func(m *manifest.Manifest) {
				m.Tools = append(m.Tools, manifest.Tool{Name: "refund.reverse", SchemaHash: strings.Repeat("e", 64)})
			},
			summary: "was added",
		},
		{
			name: "tool schema changed", class: ClassTools, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Tools[0].SchemaHash = strings.Repeat("2", 64) },
			summary: "schema of tool",
		},
		{
			name: "tool description grew", class: ClassToolDescriptionTokens, want: SeverityWarn,
			mutate:  func(m *manifest.Manifest) { m.Tools[0].DescriptionTokens = 912 },
			summary: "grew by 900 tokens",
		},
		{
			name: "retrieval.embedding_model changed", class: ClassRetrievalEmbedding, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Retrieval.EmbeddingModel = "embed-4" },
			summary: "the embedding model changed",
		},
		{
			name: "retrieval.top_k changed", class: ClassRetrievalTopK, want: SeverityWarn,
			mutate:  func(m *manifest.Manifest) { m.Retrieval.TopK = 20 },
			summary: "the retrieval depth changed",
		},
		{
			name: "retrieval.filters_hash changed", class: ClassRetrievalFiltersHash, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Retrieval.FiltersHash = strPtr(strings.Repeat("0", 64)) },
			summary: "the retrieval filter changed",
		},
		{
			name: "policy.version changed", class: ClassPolicyVersion, want: SeverityFail,
			mutate:  func(m *manifest.Manifest) { m.Policy.Version = "2027.01" },
			summary: "the policy version changed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head := base()
			tc.mutate(head)
			r := Compare(base(), head, Options{})
			got := classes(r)
			entry, ok := got[tc.class]
			if !ok {
				t.Fatalf("no %s entry; got %v", tc.class, classNames(r))
			}
			if entry.Severity != tc.want {
				t.Fatalf("%s severity = %q, want %q", tc.class, entry.Severity, tc.want)
			}
			if !strings.Contains(entry.Summary, tc.summary) {
				t.Fatalf("%s summary = %q, want it to mention %q", tc.class, entry.Summary, tc.summary)
			}
		})
	}
}

// TestEveryClassIsReachable is the guard against a class that is declared, has a
// severity, appears in configuration validation, and can never actually be emitted.
func TestEveryClassIsReachable(t *testing.T) {
	mutations := map[Class]func(*manifest.Manifest){
		ClassModelID:               func(m *manifest.Manifest) { m.Model.ID = "y" },
		ClassModelAliasOf:          func(m *manifest.Manifest) { m.Model.AliasOf = nil },
		ClassModelParams:           func(m *manifest.Manifest) { m.Model.Params.Seed = strPtr("1") },
		ClassPromptRenderedHash:    func(m *manifest.Manifest) { m.Prompt.RenderedHash = strings.Repeat("1", 64) },
		ClassTools:                 func(m *manifest.Manifest) { m.Tools[0].SchemaHash = strings.Repeat("2", 64) },
		ClassToolDescriptionTokens: func(m *manifest.Manifest) { m.Tools[0].DescriptionTokens = 99 },
		ClassToolDescription:       func(m *manifest.Manifest) { h := strings.Repeat("5", 64); m.Tools[0].DescriptionHash = &h },
		ClassRetrievalEmbedding:    func(m *manifest.Manifest) { m.Retrieval.EmbeddingModelVersion = "2027-01" },
		ClassRetrievalTopK:         func(m *manifest.Manifest) { m.Retrieval.TopK = 9 },
		ClassRetrievalFiltersHash:  func(m *manifest.Manifest) { m.Retrieval.FiltersHash = nil },
		ClassPolicyVersion:         func(m *manifest.Manifest) { m.Policy.Version = "x" },
		ClassModelProvider:         func(m *manifest.Manifest) { m.Model.Provider = "anthropic" },
		ClassPromptTemplateHash:    func(m *manifest.Manifest) { m.Prompt.TemplateHash = strings.Repeat("3", 64) },
		ClassPromptVariables:       func(m *manifest.Manifest) { m.Prompt.Variables["extra"] = "string" },
		ClassRetrievalIndexID:      func(m *manifest.Manifest) { m.Retrieval.IndexID = "other" },
		ClassRetrievalChunker:      func(m *manifest.Manifest) { m.Retrieval.Chunker = "fixed-400" },
		ClassPolicyGuardrailsHash:  func(m *manifest.Manifest) { m.Policy.GuardrailsHash = nil },
		ClassCapturePath:           func(m *manifest.Manifest) { m.Capture.Path = manifest.CaptureObserved },
	}

	for _, class := range Classes() {
		mutate, ok := mutations[class]
		if !ok {
			t.Fatalf("class %s is declared but the test has no mutation for it", class)
		}
		head := base()
		mutate(head)
		r := Compare(base(), head, Options{})
		if _, found := classes(r)[class]; !found {
			t.Fatalf("class %s was never emitted; entries: %v", class, classNames(r))
		}
	}
}

// TestSeverityOverride covers the configuration path: a team that changes a model
// on purpose should not be able to fail forever.
func TestSeverityOverride(t *testing.T) {
	head := base()
	head.Model.ID = "gpt-y"
	r := Compare(base(), head, Options{Severity: map[string]Severity{
		string(ClassModelID): SeverityWarn,
	}})
	if got := classes(r)[ClassModelID].Severity; got != SeverityWarn {
		t.Fatalf("override was ignored: %q", got)
	}
	if r.Worst() != SeverityWarn {
		t.Fatalf("worst = %q, want warn", r.Worst())
	}
}

func TestIgnoreRemovesTheThreat(t *testing.T) {
	head := base()
	head.Policy.Version = "2027.01"
	r := Compare(base(), head, Options{Severity: map[string]Severity{
		string(ClassPolicyVersion): SeverityIgnore,
	}})
	if r.Worst() != SeverityIgnore {
		t.Fatalf("worst = %q, want ignore", r.Worst())
	}
}

// TestCostProjection covers the requirement that a token delta comes with a
// projection, and the honesty rule that no dollar figure is invented without a
// configured price.
func TestCostProjection(t *testing.T) {
	head := base()
	head.Tools[0].DescriptionTokens = 912

	withoutPrice := classes(Compare(base(), head, Options{CallsPerDay: 10000}))[ClassToolDescriptionTokens]
	if !strings.Contains(withoutPrice.Detail, "tokens per day") {
		t.Fatalf("no projection in the detail: %q", withoutPrice.Detail)
	}
	if strings.Contains(withoutPrice.Detail, "$") {
		t.Fatalf("a dollar figure appeared without a configured price: %q", withoutPrice.Detail)
	}

	withPrice := classes(Compare(base(), head, Options{
		CallsPerDay: 10000, InputUSDPerMTok: 3,
	}))[ClassToolDescriptionTokens]
	if !strings.Contains(withPrice.Detail, "$") {
		t.Fatalf("no dollar figure with a configured price: %q", withPrice.Detail)
	}
	// 900 tokens x 10000 calls = 9,000,000 tokens/day = 9 MTok x $3 = $27/day.
	if !strings.Contains(withPrice.Detail, "$27.00") {
		t.Fatalf("projection arithmetic is wrong: %q", withPrice.Detail)
	}
}

// TestOrderIsStable keeps the output of two runs identical, so a diff in a CI log
// is comparable across builds.
func TestOrderIsStable(t *testing.T) {
	head := base()
	head.Model.ID = "gpt-y"
	head.Policy.Version = "2027.01"
	head.Retrieval.TopK = 50

	first := Compare(base(), head, Options{})
	for i := 0; i < 20; i++ {
		again := Compare(base(), head, Options{})
		if len(again.Entries) != len(first.Entries) {
			t.Fatalf("entry count changed between runs")
		}
		for j := range first.Entries {
			if again.Entries[j].Class != first.Entries[j].Class {
				t.Fatalf("entry %d class changed: %s vs %s",
					j, again.Entries[j].Class, first.Entries[j].Class)
			}
		}
	}
}

// TestPromptEditsDoNotBlockEveryBuild locks a decision that is easy to get wrong
// and expensive to get wrong: editing a prompt must warn, not fail. Prompts are
// what this audience changes most often, and a default that failed them all would
// be switched off within a week.
func TestPromptEditsDoNotBlockEveryBuild(t *testing.T) {
	head := base()
	head.Prompt.TemplateHash = strings.Repeat("7", 64)
	head.Prompt.RenderedHash = strings.Repeat("8", 64)

	r := Compare(base(), head, Options{})
	if r.Worst() != SeverityWarn {
		t.Fatalf("worst severity for a prompt edit = %q, want warn", r.Worst())
	}
	for _, e := range r.Entries {
		if e.Severity == SeverityFail {
			t.Fatalf("%s failed the build on a prompt edit", e.Class)
		}
	}
}

// TestDerivedRenderedHashDoesNotDoubleReport covers a case that made ordinary use
// of the tool look broken: with no rendered sample, capture sets rendered_hash
// equal to template_hash, so one prompt edit produced two identical entries.
func TestDerivedRenderedHashDoesNotDoubleReport(t *testing.T) {
	baseM, headM := base(), base()
	// Model the no-rendered-sample case: both hashes track the template.
	baseM.Prompt.TemplateHash = strings.Repeat("a", 64)
	baseM.Prompt.RenderedHash = strings.Repeat("a", 64)
	headM.Prompt.TemplateHash = strings.Repeat("b", 64)
	headM.Prompt.RenderedHash = strings.Repeat("b", 64)

	r := Compare(baseM, headM, Options{})
	if len(r.Entries) != 1 {
		t.Fatalf("one prompt edit produced %d entries: %v", len(r.Entries), classNames(r))
	}
	if r.Entries[0].Class != ClassPromptTemplateHash {
		t.Fatalf("entry class = %s, want the template class", r.Entries[0].Class)
	}
}

// TestRealRenderedChangeIsStillReported is the other half of the previous test: a
// rendered sample that differs from the template must still produce its own entry.
func TestRealRenderedChangeIsStillReported(t *testing.T) {
	baseM, headM := base(), base()
	headM.Prompt.TemplateHash = strings.Repeat("a", 64) // template unchanged
	headM.Prompt.RenderedHash = strings.Repeat("c", 64) // rendered output differs from base

	r := Compare(baseM, headM, Options{})
	if _, ok := classes(r)[ClassPromptRenderedHash]; !ok {
		t.Fatalf("a changed rendered hash was suppressed: %v", classNames(r))
	}
	if len(r.Entries) != 1 {
		t.Fatalf("expected exactly the rendered entry, got %v", classNames(r))
	}
}

// TestUnknownClassesAreDetectable backs configuration validation: a typo in a
// severity override must be catchable rather than silently inert.
func TestUnknownClassesAreDetectable(t *testing.T) {
	if IsKnownClass("model.ed") {
		t.Fatal("a typo was accepted as a class")
	}
	if !IsKnownClass(string(ClassModelID)) {
		t.Fatal("a real class was rejected")
	}
	if len(Classes()) < 17 {
		t.Fatalf("class count = %d, the specification's ten plus the extras are expected", len(Classes()))
	}
}

func classNames(r *Result) []string {
	out := make([]string, len(r.Entries))
	for i, e := range r.Entries {
		out[i] = string(e.Class)
	}
	return out
}
