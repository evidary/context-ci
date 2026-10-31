package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func sample() *Manifest {
	alias := "gpt-x-latest"
	filters := strings.Repeat("f", 64)
	guard := strings.Repeat("9", 64)
	return &Manifest{
		ManifestVersion: 1,
		CreatedAt:       "2026-12-01T10:00:00Z",
		Commit:          "9c1f2ab",
		Capture:         Capture{Path: CaptureAsserted},
		Model: Model{
			Provider: "openai",
			ID:       "gpt-x",
			AliasOf:  &alias,
			Params: ModelParam{
				Temperature: ptr("0"),
				MaxTokens:   ptrInt(1024),
			},
		},
		Prompt: Prompt{
			TemplateHash: strings.Repeat("a", 64),
			RenderedHash: strings.Repeat("b", 64),
			Variables:    map[string]string{"tone": "string", "locale": "string"},
		},
		Tools: []Tool{
			{Name: "refund.create", SchemaHash: strings.Repeat("c", 64), DescriptionHash: ptr(strings.Repeat("e", 64)), DescriptionTokens: 120},
			{Name: "ticket.read", SchemaHash: strings.Repeat("d", 64), DescriptionHash: ptr(strings.Repeat("f", 64)), DescriptionTokens: 40},
		},
		Retrieval: Retrieval{
			IndexID:               "kb-prod",
			EmbeddingModel:        "embed-3",
			EmbeddingModelVersion: "2026-05",
			Chunker:               "recursive-800",
			TopK:                  5,
			FiltersHash:           &filters,
		},
		Policy: Policy{Version: "2026.11", GuardrailsHash: &guard},
	}
}

func ptr(s string) *string { return &s }
func ptrInt(i int) *int    { return &i }

// TestDigestIsStable pins one concrete digest. A change to the canonicalisation or
// to the set of digested fields shows up as a failing constant rather than as an
// unexplained change in every manifest a customer has stored.
//
// The value was reproduced independently by a Python canonicaliser written from the
// specification, so this constant is not merely a snapshot of this implementation's
// own output: two implementations agreed on the same bytes before it was pinned.
func TestDigestIsStable(t *testing.T) {
	m := sample()
	got, err := m.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	const want = "e342a0e16c0586531d21b7759cdeb1f1d67102b7cd9eade78a8965fa1c8bccf0"
	if got != want {
		t.Fatalf("digest = %s\npinned  %s\nif this is intentional then every stored "+
			"manifest stops matching and the SDKs need the same change", got, want)
	}
}

// TestDigestIgnoresCreatedAt is the property that makes the tool usable: two runs
// over the same repository are the same context even though the clock moved.
func TestDigestIgnoresCreatedAt(t *testing.T) {
	first := sample()
	d1, err := first.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	second := sample()
	second.CreatedAt = "2030-01-01T00:00:00Z"
	d2, err := second.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if d1 != d2 {
		t.Fatalf("created_at changed the digest: %s vs %s", d1, d2)
	}
}

// TestDigestIgnoresContent covers the opt-in content capture. Content is redundant
// with its hash, and including it would make the digest depend on whether the team
// happened to enable capture that day.
func TestDigestIgnoresContent(t *testing.T) {
	plain := sample()
	d1, err := plain.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	withContent := sample()
	body := "hello"
	sum := sha256.Sum256([]byte(body))
	withContent.Content = []Content{{Kind: "prompt", Hash: hex.EncodeToString(sum[:]), Body: body}}
	d2, err := withContent.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}
	if d1 != d2 {
		t.Fatalf("captured content changed the digest: %s vs %s", d1, d2)
	}
}

// TestDigestNoticesEveryField walks every behaviour-bearing field and asserts the
// digest changes. A field that does not change the digest is a field the gate
// cannot protect, and that is a silent hole rather than a visible bug.
func TestDigestNoticesEveryField(t *testing.T) {
	base := sample()
	baseDigest, err := base.Compute()
	if err != nil {
		t.Fatalf("Compute() error = %v", err)
	}

	cases := map[string]func(*Manifest){
		"commit":                   func(m *Manifest) { m.Commit = "abcdef0" },
		"model.provider":           func(m *Manifest) { m.Model.Provider = "anthropic" },
		"model.id":                 func(m *Manifest) { m.Model.ID = "gpt-y" },
		"model.alias_of":           func(m *Manifest) { m.Model.AliasOf = ptr("elsewhere") },
		"model.params.temperature": func(m *Manifest) { m.Model.Params.Temperature = ptr("7") },
		"model.params.max_tokens":  func(m *Manifest) { m.Model.Params.MaxTokens = ptrInt(2048) },
		"model.params.seed":        func(m *Manifest) { m.Model.Params.Seed = ptr("42") },
		"prompt.template_hash":     func(m *Manifest) { m.Prompt.TemplateHash = strings.Repeat("1", 64) },
		"prompt.rendered_hash":     func(m *Manifest) { m.Prompt.RenderedHash = strings.Repeat("2", 64) },
		"prompt.variables":         func(m *Manifest) { m.Prompt.Variables["tone"] = "enum" },
		"tools.added": func(m *Manifest) {
			m.Tools = append(m.Tools, Tool{Name: "x", SchemaHash: strings.Repeat("7", 64), DescriptionHash: ptr(strings.Repeat("7", 64))})
		},
		"tools.removed":            func(m *Manifest) { m.Tools = m.Tools[:1] },
		"tools.schema_hash":        func(m *Manifest) { m.Tools[0].SchemaHash = strings.Repeat("8", 64) },
		"tools.description_hash":   func(m *Manifest) { m.Tools[0].DescriptionHash = ptr(strings.Repeat("8", 64)) },
		"tools.description_tokens": func(m *Manifest) { m.Tools[0].DescriptionTokens = 121 },
		"retrieval.index_id":       func(m *Manifest) { m.Retrieval.IndexID = "kb-staging" },
		"retrieval.embedding":      func(m *Manifest) { m.Retrieval.EmbeddingModel = "embed-4" },
		"retrieval.top_k":          func(m *Manifest) { m.Retrieval.TopK = 20 },
		"retrieval.filters_hash":   func(m *Manifest) { m.Retrieval.FiltersHash = ptr(strings.Repeat("0", 64)) },
		"retrieval.chunker":        func(m *Manifest) { m.Retrieval.Chunker = "fixed-400" },
		"policy.version":           func(m *Manifest) { m.Policy.Version = "2027.01" },
		"policy.guardrails_hash":   func(m *Manifest) { m.Policy.GuardrailsHash = nil },
		"capture.path":             func(m *Manifest) { m.Capture.Path = CaptureObserved },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := sample()
			mutate(m)
			got, err := m.Compute()
			if err != nil {
				t.Fatalf("Compute() error = %v", err)
			}
			if got == baseDigest {
				t.Fatalf("%s does not change the digest, so the gate cannot see it change", name)
			}
		})
	}
}

// TestDeterministicAcrossEncodings guards the canonical form rather than the hash:
// the same document reached by different routes must serialise identically.
func TestDeterministicAcrossEncodings(t *testing.T) {
	body := map[string]any{
		"b": json.Number("2"),
		"a": "x",
		"c": map[string]any{"z": true, "y": nil},
	}
	first, err := Canonicalise(body)
	if err != nil {
		t.Fatalf("Canonicalise() error = %v", err)
	}
	for i := 0; i < 50; i++ {
		again, err := Canonicalise(body)
		if err != nil {
			t.Fatalf("Canonicalise() error = %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("canonical form is not deterministic:\n%s\n%s", first, again)
		}
	}
	if want := `{"a":"x","b":2,"c":{"y":null,"z":true}}`; string(first) != want {
		t.Fatalf("canonical form = %s, want %s", first, want)
	}
}

// TestCanonicaliseRejectsFloats is the rule that keeps two implementations in
// agreement: 1, 1.0 and 1e0 are the same number with three spellings.
func TestCanonicaliseRejectsFloats(t *testing.T) {
	_, err := Canonicalise(map[string]any{"temperature": json.Number("0.7")})
	if err == nil {
		t.Fatal("Canonicalise() accepted a float")
	}
	if !strings.Contains(err.Error(), "floating point") {
		t.Fatalf("error does not explain the rule: %v", err)
	}
}

// TestKeysSortByUTF16CodeUnit covers the one place where sorting by code point
// gives a different answer from sorting by UTF-16 code unit.
func TestKeysSortByUTF16CodeUnit(t *testing.T) {
	// U+1F600 is above the BMP. In UTF-16 it is encoded as the surrogate pair
	// D83D DE00, so it sorts before U+FFFD, which is a single unit FFFD. Sorting by
	// code point would put U+FFFD first.
	astral := "\U0001F600"
	replacement := "\uFFFD"
	got, err := Canonicalise(map[string]any{astral: 1, replacement: 2})
	if err != nil {
		t.Fatalf("Canonicalise() error = %v", err)
	}
	want := `{"` + astral + `":1,"` + replacement + `":2}`
	if string(got) != want {
		t.Fatalf("key order = %s, want %s", got, want)
	}
}

func TestValidateRejectsIncompleteManifests(t *testing.T) {
	cases := map[string]func(*Manifest){
		"version":        func(m *Manifest) { m.ManifestVersion = 2 },
		"commit":         func(m *Manifest) { m.Commit = "" },
		"capture path":   func(m *Manifest) { m.Capture.Path = "guessed" },
		"model id":       func(m *Manifest) { m.Model.ID = "" },
		"template hash":  func(m *Manifest) { m.Prompt.TemplateHash = "short" },
		"rendered hash":  func(m *Manifest) { m.Prompt.RenderedHash = strings.Repeat("z", 64) },
		"duplicate tool": func(m *Manifest) { m.Tools = append(m.Tools, m.Tools[0]) },
		"tool hash":      func(m *Manifest) { m.Tools[0].SchemaHash = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := sample()
			mutate(m)
			if err := m.Validate(); err == nil {
				t.Fatal("Validate() accepted an invalid manifest")
			}
			if _, err := m.Compute(); err == nil {
				t.Fatal("Compute() accepted an invalid manifest")
			}
		})
	}
}

func TestValidateAcceptsSample(t *testing.T) {
	if err := sample().Validate(); err != nil {
		t.Fatalf("Validate() rejected the sample: %v", err)
	}
}
