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
	"regexp"
	"sort"
	"strings"

	"github.com/sigilary/context-ci/internal/manifest"
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

// Classes for files matched by name instead of declared one by one.
const (
	// ClassAgentInstructions is a file a coding agent reads as instructions: AGENTS.md, CLAUDE.md,
	// a rules directory, a skill.
	ClassAgentInstructions Class = "agent.instructions"
	// ClassAgentConfig is a file that decides what a coding agent may run or reach: an MCP server
	// list, hooks, permissions.
	ClassAgentConfig Class = "agent.config"
	// ClassHiddenCharacters is text a model reads and a reviewer cannot see, in a file, a prompt
	// or a tool description that is new or changed.
	ClassHiddenCharacters Class = "hidden_characters"
)

// FilesClassPrefix starts the class of every group a configuration defines under capture.files:
// the group `examples` is the class `files.examples`. The set is open, because the groups are
// the team's own.
const FilesClassPrefix = "files."

var filesGroup = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// FileClass returns the class of a file group as a manifest records it.
func FileClass(group string) Class {
	switch group {
	case string(ClassAgentInstructions), string(ClassAgentConfig):
		return Class(group)
	}
	return Class(FilesClassPrefix + group)
}

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
	case ClassAgentConfig:
		// A new MCP server or hook gives an agent something to run. An edited instruction
		// file is the commonest deliberate change there is, so it warns, as a prompt does.
		return SeverityFail
	case ClassHiddenCharacters:
		// Nobody reviews what they cannot see.
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
		ClassAgentInstructions, ClassAgentConfig, ClassHiddenCharacters,
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	return all
}

// IsKnownClass reports whether a configuration key names a real class: one of Classes, or the
// class of a file group. Whether that group is configured is for the caller that has the
// configuration to say.
func IsKnownClass(name string) bool {
	for _, c := range Classes() {
		if string(c) == name {
			return true
		}
	}
	group, ok := strings.CutPrefix(name, FilesClassPrefix)
	return ok && filesGroup.MatchString(group)
}

// Entry is one change.
type Entry struct {
	Class    Class    `json:"class"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
	Before   string   `json:"before,omitempty"`
	After    string   `json:"after,omitempty"`
	Detail   string   `json:"detail,omitempty"`
	// Path and Line say where the change is, when a file holds it. Compare fills Path for a
	// file group; the caller that knows the configuration fills the rest.
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
	// Acknowledged marks a change somebody with the authority to do so has accepted for this
	// comparison. It is still reported, at its own severity; it no longer fails the gate.
	Acknowledged bool `json:"acknowledged,omitempty"`
	// Source and Subject tell the caller which configured source an entry came from ("prompt" or
	// "tools") and which tool it names, so it can find the file and the line. They are not part
	// of a report.
	Source  string `json:"-"`
	Subject string `json:"-"`
}

// TextDiff is the changed text of one file, for a reviewer who would otherwise see two hashes.
type TextDiff struct {
	Path string `json:"path"`
	// Diff is unified-diff hunks, without the file header.
	Diff string `json:"diff"`
	// Omitted is how many further lines of it were left out to keep a report readable.
	Omitted int `json:"omitted,omitempty"`
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
	// Acknowledge lists the classes whose changes are accepted for this comparison, and
	// AcknowledgeAll accepts every class except hidden characters, which have to be named: a
	// blanket approval of text nobody could read approves nothing.
	Acknowledge    map[string]bool
	AcknowledgeAll bool
}

func (o Options) acknowledged(c Class) bool {
	if o.Acknowledge[string(c)] {
		return true
	}
	return o.AcknowledgeAll && c != ClassHiddenCharacters
}

func (o Options) entry(c Class, summary, before, after, detail string) Entry {
	return Entry{
		Class:        c,
		Severity:     o.severityFor(c),
		Summary:      summary,
		Before:       before,
		After:        after,
		Detail:       detail,
		Acknowledged: o.acknowledged(c),
	}
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
	// Texts holds the changed text of the files behind the entries, when the caller could read
	// both sides of them.
	Texts []TextDiff `json:"text_diffs,omitempty"`
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

// Gating returns the most severe severity among the changes nobody acknowledged, which is what
// the gate decides on.
func (r *Result) Gating() Severity {
	worst := SeverityIgnore
	for _, e := range r.Entries {
		if !e.Acknowledged && rank(e.Severity) > rank(worst) {
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
		res.Entries = append(res.Entries, opts.entry(class, summary, before, after, detail))
	}
	unchanged := func(class Class) { res.Unchanged = append(res.Unchanged, class) }

	if base.Model.ID == head.Model.ID {
		unchanged(ClassModelID)
	} else {
		add(ClassModelID, "the model changed", orDash(base.Model.ID), orDash(head.Model.ID), "")
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
		add(ClassModelProvider, "the model provider changed", orDash(base.Model.Provider), orDash(head.Model.Provider), "")
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
			orDash(short(base.Prompt.TemplateHash)), orDash(short(head.Prompt.TemplateHash)), "")
		if head.Prompt.HiddenCharacters > 0 {
			add(ClassHiddenCharacters, fmt.Sprintf("the prompt template contains %s", hiddenCount(head.Prompt.HiddenCharacters)),
				"", "", hiddenDetail)
			res.Entries[len(res.Entries)-1].Source = "prompt"
		}
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
			orDash(short(base.Prompt.RenderedHash)), orDash(short(head.Prompt.RenderedHash)), "")
	}

	if before, after := variablesFingerprint(base.Prompt.Variables), variablesFingerprint(head.Prompt.Variables); before == after {
		unchanged(ClassPromptVariables)
	} else {
		add(ClassPromptVariables, "the prompt variables changed", before, after, "")
	}

	compareTools(base, head, opts, res)
	compareFiles(base, head, opts, res)

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
		add(ClassRetrievalFiltersHash, "the retrieval configuration file changed",
			orDash(short(deref(base.Retrieval.FiltersHash))), orDash(short(deref(head.Retrieval.FiltersHash))),
			"its values differ; comments, key order and formatting are not compared")
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
		add(ClassPolicyGuardrailsHash, "the guardrails configuration file changed",
			orDash(short(deref(base.Policy.GuardrailsHash))), orDash(short(deref(head.Policy.GuardrailsHash))),
			"its values differ; comments, key order and formatting are not compared")
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

const hiddenDetail = "a reviewer cannot see these characters and a model reads them; remove them, or acknowledge this class by name"

func hiddenCount(n int) string {
	if n == 1 {
		return "1 hidden character"
	}
	return fmt.Sprintf("%d hidden characters", n)
}

// compareFiles reports each file of each group by path. A group is a class, so one severity
// covers every file in it; the entries are per file, because "something under prompts/ changed"
// is not what a reviewer needs to hear.
func compareFiles(base, head *manifest.Manifest, opts Options, res *Result) {
	type key struct{ group, path string }
	before := make(map[key]manifest.File, len(base.Files))
	after := make(map[key]manifest.File, len(head.Files))
	groups := map[string]bool{string(ClassAgentInstructions): true, string(ClassAgentConfig): true}
	var keys []key
	for _, f := range base.Files {
		k := key{f.Group, f.Path}
		before[k] = f
		groups[f.Group] = true
		keys = append(keys, k)
	}
	for _, f := range head.Files {
		k := key{f.Group, f.Path}
		after[k] = f
		groups[f.Group] = true
		if _, ok := before[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].group != keys[j].group {
			return keys[i].group < keys[j].group
		}
		return keys[i].path < keys[j].path
	})

	changed := map[string]bool{}
	hidden := false
	for _, k := range keys {
		b, hadBefore := before[k]
		a, hasAfter := after[k]
		class := FileClass(k.group)
		var e Entry
		switch {
		case !hadBefore:
			e = opts.entry(class, fmt.Sprintf("%s was added", k.path), "", short(a.Hash), "")
		case !hasAfter:
			e = opts.entry(class, fmt.Sprintf("%s was removed", k.path), short(b.Hash), "", "")
		case b.Hash != a.Hash:
			e = opts.entry(class, fmt.Sprintf("%s changed", k.path), short(b.Hash), short(a.Hash), "")
		default:
			continue
		}
		e.Path = k.path
		changed[k.group] = true
		res.Entries = append(res.Entries, e)
		// Reported for a file that is new or changed, and not for one that was already like
		// this: a finding nobody in this pull request caused would be red on every one of them.
		if hasAfter && a.HiddenCharacters > 0 {
			h := opts.entry(ClassHiddenCharacters,
				fmt.Sprintf("%s contains %s", k.path, hiddenCount(a.HiddenCharacters)), "", "", hiddenDetail)
			h.Path = k.path
			hidden = true
			res.Entries = append(res.Entries, h)
		}
	}
	for group := range groups {
		if !changed[group] {
			res.Unchanged = append(res.Unchanged, FileClass(group))
		}
	}
	// The prompt and the tool descriptions add their own entries of this class, so it is
	// unchanged only if nothing anywhere reported it.
	if !hidden {
		for _, e := range res.Entries {
			if e.Class == ClassHiddenCharacters {
				hidden = true
				break
			}
		}
	}
	if !hidden {
		res.Unchanged = append(res.Unchanged, ClassHiddenCharacters)
	}
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
		first := len(res.Entries)
		switch {
		case !hadBefore:
			toolsChanged = true
			res.Entries = append(res.Entries, opts.entry(ClassTools, fmt.Sprintf("tool %q was added", name), "", name, ""))
			if a.HiddenCharacters > 0 {
				res.Entries = append(res.Entries, opts.entry(ClassHiddenCharacters,
					fmt.Sprintf("the description of tool %q contains %s", name, hiddenCount(a.HiddenCharacters)), "", "", hiddenDetail))
			}
		case !hasAfter:
			toolsChanged = true
			res.Entries = append(res.Entries, opts.entry(ClassTools, fmt.Sprintf("tool %q was removed", name), name, "", ""))
		default:
			if b.SchemaHash != a.SchemaHash {
				toolsChanged = true
				res.Entries = append(res.Entries, opts.entry(ClassTools,
					fmt.Sprintf("the definition of tool %q changed", name), short(b.SchemaHash), short(a.SchemaHash),
					"its parameter schema or another field of the definition, other than the description"))
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
				// The size and what it costs ride on the entry for the edit. They used to be
				// printed only for a count that moved under an unchanged description, which no
				// edit can produce, so the projection a team configured was never shown.
				detail := ""
				if delta != 0 {
					detail = fmt.Sprintf("the description %s by %s, estimated as words", changeWord(delta), tokens(abs(delta)))
					if projection := costProjection(delta, opts); projection != "" {
						detail += ": " + projection
					}
				}
				res.Entries = append(res.Entries, opts.entry(ClassToolDescription,
					fmt.Sprintf("the description of tool %q changed", name), short(beforeHash), short(afterHash), detail))
				if a.HiddenCharacters > 0 {
					res.Entries = append(res.Entries, opts.entry(ClassHiddenCharacters,
						fmt.Sprintf("the description of tool %q contains %s", name, hiddenCount(a.HiddenCharacters)), "", "", hiddenDetail))
				}
			} else if delta != 0 {
				// Same text, different count means the estimate changed under the same
				// description, which is a capture bug rather than an edit.
				toolsChanged = true
				res.Entries = append(res.Entries, opts.entry(ClassToolDescriptionTokens,
					fmt.Sprintf("the description of tool %q %s by %s", name, changeWord(delta), tokens(abs(delta))),
					tokens(b.DescriptionTokens), tokens(a.DescriptionTokens), costProjection(delta, opts)))
			}
		}
		for i := first; i < len(res.Entries); i++ {
			res.Entries[i].Source, res.Entries[i].Subject = "tools", name
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

func tokens(n int) string {
	if n == 1 {
		return "1 token"
	}
	return fmt.Sprintf("%d tokens", n)
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
