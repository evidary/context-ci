// Package diff computes the semantic difference between two context manifests.
//
// A text diff of the JSON is useless: key order and the timestamp alone produce
// noise, and the real question is behavioural — did the model change, did a tool
// gain or lose, did retrieval start looking somewhere else. So the diff is
// organised by what changes behaviour, and every entry names a class that a
// configuration can reclassify.
//
// The ten classes the C-04 fixture suite in diff_test.go classifies one by one are
// implemented here as the first ten constants, and each carries the default severity
// the specification names. `tools[].description_hash` — ADR-0007's addition — and
// the other extras below them exist because a change that is invisible to the gate
// is a hole, not a simplification.
package diff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/evidary/context-ci/internal/manifest"
)

// Class names one kind of change. These strings are configuration keys, so they
// are stable and are never renamed without a manifest version bump.
type Class string

// The ten classes the C-04 fixture suite covers.
const (
	ClassModelID               Class = "model.id"
	ClassModelAliasOf          Class = "model.alias_of"
	ClassModelParams           Class = "model.params"
	ClassPromptRenderedHash    Class = "prompt.rendered_hash"
	ClassTools                 Class = "tools"
	ClassToolDescriptionTokens Class = "tools.description_tokens"
	ClassRetrievalEmbedding    Class = "retrieval.embedding_model"
	ClassRetrievalTopK         Class = "retrieval.top_k"
	ClassRetrievalFiltersHash  Class = "retrieval.filters_hash"
	ClassPolicyVersion         Class = "policy.version"
)

// Additional classes. Each one changes what the model sees, so leaving it out
// would let a real change pass the gate silently.
const (
	ClassModelProvider        Class = "model.provider"
	ClassPromptTemplateHash   Class = "prompt.template_hash"
	ClassPromptVariables      Class = "prompt.variables"
	ClassRetrievalIndexID     Class = "retrieval.index_id"
	ClassRetrievalChunker     Class = "retrieval.chunker"
	ClassPolicyGuardrailsHash Class = "policy.guardrails_hash"
	ClassCapturePath          Class = "capture.path"
	// ClassToolDescription is the hash of a tool's description. It exists separately
	// from ClassToolDescriptionTokens because a count cannot see an edit that keeps
	// the length: "issue a refund" and "ignore prior rules" are the same token count.
	ClassToolDescription Class = "tools.description"
)

// Severity is how a change is treated by the gate.
type Severity string

const (
	SeverityFail   Severity = "fail"
	SeverityWarn   Severity = "warn"
	SeverityIgnore Severity = "ignore"
)

// DefaultSeverity returns the severity the specification assigns to a class. The
// defaults exist so that doing nothing is safe.
//
// The extras follow the class they shadow rather than being stricter for its own
// sake. ClassPromptTemplateHash is warn, not fail, because editing a prompt is the
// most common deliberate change in this product's audience: a default that failed
// every one of them would be turned off within a week, and then nothing would be
// gated at all.
func DefaultSeverity(c Class) Severity {
	switch c {
	case ClassModelID, ClassModelAliasOf, ClassTools,
		ClassRetrievalEmbedding, ClassRetrievalFiltersHash, ClassPolicyVersion,
		ClassPolicyGuardrailsHash:
		return SeverityFail
	case ClassModelParams, ClassPromptRenderedHash, ClassPromptTemplateHash,
		ClassToolDescriptionTokens, ClassRetrievalTopK:
		return SeverityWarn
	case ClassModelProvider:
		// A provider swap changes tokenisation, safety behaviour and cost at once.
		return SeverityFail
	case ClassPromptVariables, ClassRetrievalIndexID, ClassRetrievalChunker, ClassCapturePath:
		return SeverityWarn
	case ClassToolDescription:
		// A tool description is prompt-injection surface that the model reads as
		// instructions. Same severity as the schema it sits beside.
		return SeverityFail
	}
	return SeverityWarn
}

// Classes returns every class this package can emit, sorted. Configuration is
// validated against it, so a typo in a severity override fails instead of being
// quietly ignored.
func Classes() []Class {
	all := []Class{
		ClassModelID, ClassModelAliasOf, ClassModelParams,
		ClassPromptRenderedHash, ClassTools, ClassToolDescriptionTokens,
		ClassRetrievalEmbedding, ClassRetrievalTopK, ClassRetrievalFiltersHash,
		ClassPolicyVersion,
		ClassModelProvider, ClassPromptTemplateHash, ClassPromptVariables,
		ClassRetrievalIndexID, ClassRetrievalChunker, ClassPolicyGuardrailsHash,
		ClassCapturePath, ClassToolDescription,
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	return all
}

// IsKnownClass reports whether a configuration key names a real class.
func IsKnownClass(name string) bool {
	for _, c := range Classes() {
		if string(c) == name {
			return true
		}
	}
	return false
}

// Entry is one change.
type Entry struct {
	Class    Class    `json:"class"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
	Before   string   `json:"before,omitempty"`
	After    string   `json:"after,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// Options controls how a comparison is performed.
type Options struct {
	// Severity overrides the default for a class.
	Severity map[string]Severity
	// CallsPerDay and InputUSDPerMTok turn a token delta into a projection. With
	// no price configured the projection reports tokens only, because a dollar
	// figure invented from a guessed price is worse than no figure.
	CallsPerDay     int
	InputUSDPerMTok float64
}

func (o Options) severityFor(c Class) Severity {
	if s, ok := o.Severity[string(c)]; ok {
		return s
	}
	return DefaultSeverity(c)
}

// Result is the outcome of a comparison.
type Result struct {
	BaseDigest string  `json:"base_digest"`
	HeadDigest string  `json:"head_digest"`
	Entries    []Entry `json:"entries"`
	Unchanged  []Class `json:"unchanged"`
}

// Equal reports whether two manifests describe the same context. The digest is the
// definitive answer; the entry list is the explanation.
func (r *Result) Equal() bool { return len(r.Entries) == 0 }

// Worst returns the most severe severity present, or SeverityIgnore when nothing
// changed.
func (r *Result) Worst() Severity {
	worst := SeverityIgnore
	for _, e := range r.Entries {
		if rank(e.Severity) > rank(worst) {
			worst = e.Severity
		}
	}
	return worst
}

func rank(s Severity) int {
	switch s {
	case SeverityFail:
		return 3
	case SeverityWarn:
		return 2
	case SeverityIgnore:
		return 1
	}
	return 0
}

// Compare produces the semantic difference between two manifests. Entries are
// ordered by class so the output is stable regardless of map iteration.
func Compare(base, head *manifest.Manifest, opts Options) *Result {
	res := &Result{
		BaseDigest: base.Digest,
		HeadDigest: head.Digest,
		Entries:    []Entry{},
		Unchanged:  []Class{},
	}
	add := func(class Class, summary, before, after, detail string) {
		res.Entries = append(res.Entries, Entry{
			Class:    class,
			Severity: opts.severityFor(class),
			Summary:  summary,
			Before:   before,
			After:    after,
			Detail:   detail,
		})
	}
	unchanged := func(class Class) { res.Unchanged = append(res.Unchanged, class) }

	if base.Model.ID == head.Model.ID {
		unchanged(ClassModelID)
	} else {
		add(ClassModelID, "the model changed", base.Model.ID, head.Model.ID, "")
	}

	if deref(base.Model.AliasOf) == deref(head.Model.AliasOf) {
		unchanged(ClassModelAliasOf)
	} else {
		add(ClassModelAliasOf, "the model alias resolves differently",
			orDash(deref(base.Model.AliasOf)), orDash(deref(head.Model.AliasOf)), "")
	}

	if base.Model.Provider == head.Model.Provider {
		unchanged(ClassModelProvider)
	} else {
		add(ClassModelProvider, "the model provider changed", base.Model.Provider, head.Model.Provider, "")
	}

	if before, after := paramsFingerprint(base.Model.Params), paramsFingerprint(head.Model.Params); before == after {
		unchanged(ClassModelParams)
	} else {
		add(ClassModelParams, "sampling parameters changed", before, after, "")
	}

	if base.Prompt.TemplateHash == head.Prompt.TemplateHash {
		unchanged(ClassPromptTemplateHash)
	} else {
		add(ClassPromptTemplateHash, "the prompt template changed",
			short(base.Prompt.TemplateHash), short(head.Prompt.TemplateHash), contentHint(base, head, "template"))
	}

	// When no rendered sample was supplied, capture sets rendered_hash equal to
	// template_hash. Reporting both would describe one edit twice, so the derived
	// entry is suppressed while the manifest itself still carries both fields.
	renderedIsDerived := base.Prompt.RenderedHash == base.Prompt.TemplateHash &&
		head.Prompt.RenderedHash == head.Prompt.TemplateHash

	if base.Prompt.RenderedHash == head.Prompt.RenderedHash || renderedIsDerived {
		unchanged(ClassPromptRenderedHash)
	} else {
		add(ClassPromptRenderedHash, "the rendered prompt changed",
			short(base.Prompt.RenderedHash), short(head.Prompt.RenderedHash), contentHint(base, head, "rendered"))
	}

	if before, after := variablesFingerprint(base.Prompt.Variables), variablesFingerprint(head.Prompt.Variables); before == after {
		unchanged(ClassPromptVariables)
	} else {
		add(ClassPromptVariables, "the prompt variables changed", before, after, "")
	}

	compareTools(base, head, opts, res)

	if base.Retrieval.EmbeddingModel == head.Retrieval.EmbeddingModel &&
		base.Retrieval.EmbeddingModelVersion == head.Retrieval.EmbeddingModelVersion {
		unchanged(ClassRetrievalEmbedding)
	} else {
		add(ClassRetrievalEmbedding, "the embedding model changed",
			fmt.Sprintf("%s@%s", orDash(base.Retrieval.EmbeddingModel), orDash(base.Retrieval.EmbeddingModelVersion)),
			fmt.Sprintf("%s@%s", orDash(head.Retrieval.EmbeddingModel), orDash(head.Retrieval.EmbeddingModelVersion)),
			"every document must be re-embedded, so recall changes for the whole index")
	}

	if base.Retrieval.TopK == head.Retrieval.TopK {
		unchanged(ClassRetrievalTopK)
	} else {
		add(ClassRetrievalTopK, "the retrieval depth changed",
			fmt.Sprint(base.Retrieval.TopK), fmt.Sprint(head.Retrieval.TopK), "")
	}

	if deref(base.Retrieval.FiltersHash) == deref(head.Retrieval.FiltersHash) {
		unchanged(ClassRetrievalFiltersHash)
	} else {
		add(ClassRetrievalFiltersHash, "the retrieval filter changed, so the scope of what can be retrieved changed",
			orDash(short(deref(base.Retrieval.FiltersHash))), orDash(short(deref(head.Retrieval.FiltersHash))), "")
	}

	if base.Retrieval.IndexID == head.Retrieval.IndexID {
		unchanged(ClassRetrievalIndexID)
	} else {
		add(ClassRetrievalIndexID, "the retrieval index changed", base.Retrieval.IndexID, head.Retrieval.IndexID, "")
	}

	if base.Retrieval.Chunker == head.Retrieval.Chunker {
		unchanged(ClassRetrievalChunker)
	} else {
		add(ClassRetrievalChunker, "the chunking strategy changed",
			orDash(base.Retrieval.Chunker), orDash(head.Retrieval.Chunker), "")
	}

	if base.Policy.Version == head.Policy.Version {
		unchanged(ClassPolicyVersion)
	} else {
		add(ClassPolicyVersion, "the policy version changed", base.Policy.Version, head.Policy.Version,
			"a policy change can weaken a guardrail without touching the prompt")
	}

	if deref(base.Policy.GuardrailsHash) == deref(head.Policy.GuardrailsHash) {
		unchanged(ClassPolicyGuardrailsHash)
	} else {
		add(ClassPolicyGuardrailsHash, "the guardrails configuration changed",
			orDash(short(deref(base.Policy.GuardrailsHash))), orDash(short(deref(head.Policy.GuardrailsHash))), "")
	}

	if base.Capture.Path == head.Capture.Path {
		unchanged(ClassCapturePath)
	} else {
		add(ClassCapturePath, "the capture path changed, so the two manifests are not equally strong evidence",
			base.Capture.Path, head.Capture.Path, "")
	}

	sort.SliceStable(res.Entries, func(i, j int) bool { return res.Entries[i].Class < res.Entries[j].Class })
	sort.Slice(res.Unchanged, func(i, j int) bool { return res.Unchanged[i] < res.Unchanged[j] })
	return res
}

func compareTools(base, head *manifest.Manifest, opts Options, res *Result) {
	before := indexTools(base.Tools)
	after := indexTools(head.Tools)

	names := make([]string, 0, len(before)+len(after))
	for n := range before {
		names = append(names, n)
	}
	for n := range after {
		if _, ok := before[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	toolsChanged := false
	for _, name := range names {
		b, hadBefore := before[name]
		a, hasAfter := after[name]
		switch {
		case !hadBefore:
			toolsChanged = true
			res.Entries = append(res.Entries, Entry{
				Class:    ClassTools,
				Severity: opts.severityFor(ClassTools),
				Summary:  fmt.Sprintf("tool %q was added", name),
				After:    name,
			})
		case !hasAfter:
			toolsChanged = true
			res.Entries = append(res.Entries, Entry{
				Class:    ClassTools,
				Severity: opts.severityFor(ClassTools),
				Summary:  fmt.Sprintf("tool %q was removed", name),
				Before:   name,
			})
		default:
			if b.SchemaHash != a.SchemaHash {
				toolsChanged = true
				res.Entries = append(res.Entries, Entry{
					Class:    ClassTools,
					Severity: opts.severityFor(ClassTools),
					Summary:  fmt.Sprintf("the schema of tool %q changed", name),
					Before:   short(b.SchemaHash),
					After:    short(a.SchemaHash),
				})
			}
			// The hash is the signal; the token delta is detail on the same entry, not a
			// second entry. A description can be rewritten entirely without changing its
			// length, and that case used to produce no entry at all.
			beforeHash, afterHash := "", ""
			if b.DescriptionHash != nil {
				beforeHash = *b.DescriptionHash
			}
			if a.DescriptionHash != nil {
				afterHash = *a.DescriptionHash
			}
			delta := a.DescriptionTokens - b.DescriptionTokens
			if beforeHash != afterHash {
				toolsChanged = true
				detail := ""
				if delta != 0 {
					detail = fmt.Sprintf("the description is %d tokens %s", abs(delta), changeWord(delta))
				}
				res.Entries = append(res.Entries, Entry{
					Class:    ClassToolDescription,
					Severity: opts.severityFor(ClassToolDescription),
					Summary:  fmt.Sprintf("the description of tool %q changed", name),
					Before:   short(beforeHash),
					After:    short(afterHash),
					Detail:   detail,
				})
			} else if delta != 0 {
				// Same text, different count means the estimate changed under the same
				// description, which is a capture bug rather than an edit.
				toolsChanged = true
				res.Entries = append(res.Entries, Entry{
					Class:    ClassToolDescriptionTokens,
					Severity: opts.severityFor(ClassToolDescriptionTokens),
					Summary:  fmt.Sprintf("the description of tool %q %s by %d tokens", name, changeWord(delta), abs(delta)),
					Before:   fmt.Sprintf("%d tokens", b.DescriptionTokens),
					After:    fmt.Sprintf("%d tokens", a.DescriptionTokens),
					Detail:   costProjection(delta, opts),
				})
			}
		}
	}
	if !toolsChanged {
		res.Unchanged = append(res.Unchanged, ClassTools, ClassToolDescriptionTokens, ClassToolDescription)
	}
}

func indexTools(tools []manifest.Tool) map[string]manifest.Tool {
	out := make(map[string]manifest.Tool, len(tools))
	for _, t := range tools {
		out[t.Name] = t
	}
	return out
}

// costProjection converts a per-call token delta into a daily figure. It reports
// tokens only when no price is configured, because an invented price is worse than
// an absent one.
func costProjection(delta int, opts Options) string {
	if opts.CallsPerDay <= 0 {
		return ""
	}
	daily := delta * opts.CallsPerDay
	if opts.InputUSDPerMTok <= 0 {
		return fmt.Sprintf("%+d tokens per day at %d calls per day; no token price configured, so no dollar figure is given",
			daily, opts.CallsPerDay)
	}
	usd := float64(daily) / 1_000_000 * opts.InputUSDPerMTok
	return fmt.Sprintf("%+d tokens per day at %d calls per day, about $%.2f per day at $%.2f per million input tokens",
		daily, opts.CallsPerDay, usd, opts.InputUSDPerMTok)
}

func paramsFingerprint(p manifest.ModelParam) string {
	parts := []string{
		"temperature=" + orDash(deref(p.Temperature)),
		"max_tokens=" + orDash(derefInt(p.MaxTokens)),
		"seed=" + orDash(deref(p.Seed)),
		"top_p=" + orDash(deref(p.TopP)),
	}
	return strings.Join(parts, " ")
}

// variablesFingerprint renders the map unambiguously.
//
// The previous version joined "name:value" pairs with spaces, which collided:
// {"a": "1 b:2"} and {"a": "1", "b": "2"} both produced "a:1 b:2", so a real change to
// the prompt variables was reported as unchanged and the gate exited 0. JSON encoding
// sorts map keys and quotes every value, so no value can forge a separator.
func variablesFingerprint(v map[string]string) string {
	if len(v) == 0 {
		return "none"
	}
	b, err := json.Marshal(v)
	if err != nil {
		// A map of strings cannot fail to encode. If it ever does, returning a value
		// that cannot equal another manifest's is safer than returning one that might.
		return "unencodable:" + fmt.Sprint(len(v))
	}
	return string(b)
}

func contentHint(base, head *manifest.Manifest, kind string) string {
	if hasContent(base, kind) && hasContent(head, kind) {
		return "both manifests carry the captured text, so a text diff is possible"
	}
	return "content was not captured, so only the hash shows the change"
}

func hasContent(m *manifest.Manifest, kind string) bool {
	for _, c := range m.Content {
		if c.Kind == kind {
			return true
		}
	}
	return false
}

func changeWord(delta int) string {
	if delta > 0 {
		return "grew"
	}
	return "shrank"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(n *int) string {
	if n == nil {
		return ""
	}
	return fmt.Sprint(*n)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func short(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12] + "…"
}
