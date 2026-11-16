// Package manifest defines the context manifest and its digest.
//
// The digest is the product. Everything else — the diff, the attribution, the CI
// gate — assumes that the same repository state produces the same digest on any
// operating system, in any order, on any machine. If that is not true the diff is
// noise and the gate is a coin toss.
//
// Two rules make it true, and both are enforced here rather than documented and
// hoped for:
//
//  1. Serialisation is canonical: keys sorted by UTF-16 code unit, no
//     insignificant whitespace, no floats anywhere in the document.
//  2. created_at is excluded from the digest. It is the only field allowed to
//     differ between two runs over the same inputs, because a timestamp is not a
//     property of the context.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Version is the manifest schema version this package writes.
const Version = 1

// Capture paths, in order of strength. A sample-based manifest is weaker
// evidence, and the field exists so it can never be presented as equivalent.
const (
	CaptureInstrumented = "instrumented"
	CaptureAsserted     = "asserted"
	CaptureObserved     = "observed"
)

// Providers are the values model.provider may take, as the specification's §4 lists them. The set
// is closed so that "OpenAI", "openai " and "open-ai" cannot be three providers in three manifests
// and a model.provider change between two of them; "other" is the documented escape.
var Providers = []string{"openai", "anthropic", "google", "other"}

// IsProvider reports whether name is one of Providers.
func IsProvider(name string) bool {
	for _, p := range Providers {
		if name == p {
			return true
		}
	}
	return false
}

// decimalPattern and integerPattern are what a sampling value may look like. An exponent is
// accepted because a configuration may write one and the literal text is what is hashed; a word,
// an empty string and NaN are not numbers under any spelling.
var (
	decimalPattern = regexp.MustCompile(`^-?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)
	integerPattern = regexp.MustCompile(`^-?\d+$`)
)

// ErrFloat is returned when a value that is not an integer reaches the
// canonicaliser. A float has more than one legal spelling, so it cannot be part
// of a digest that two implementations must agree on.
var ErrFloat = errors.New("manifest: floating point values are not permitted")

// Manifest is context-manifest.json, schema version 1.
type Manifest struct {
	ManifestVersion int       `json:"manifest_version"`
	CreatedAt       string    `json:"created_at"`
	Commit          string    `json:"commit"`
	Capture         Capture   `json:"capture"`
	Model           Model     `json:"model"`
	Prompt          Prompt    `json:"prompt"`
	Tools           []Tool    `json:"tools"`
	Retrieval       Retrieval `json:"retrieval"`
	Policy          Policy    `json:"policy"`
	Digest          string    `json:"digest,omitempty"`
	Content         []Content `json:"content,omitempty"`
}

// Capture records how the manifest was produced.
type Capture struct {
	Path       string `json:"path"`
	SDKVersion string `json:"sdk_version,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// Model is the model and the parameters that change behaviour.
type Model struct {
	Provider string     `json:"provider"`
	ID       string     `json:"id"`
	AliasOf  *string    `json:"alias_of"`
	Params   ModelParam `json:"params"`
}

// ModelParam holds sampling settings. Every value is a string or an integer,
// never a float, for the same reason the canonicaliser refuses floats.
type ModelParam struct {
	Temperature *string `json:"temperature,omitempty"`
	MaxTokens   *int    `json:"max_tokens,omitempty"`
	Seed        *string `json:"seed,omitempty"`
	TopP        *string `json:"top_p,omitempty"`
}

// Prompt holds hashes, not content. Content is opt-in and lives in Content.
type Prompt struct {
	TemplateHash       string            `json:"template_hash"`
	RenderedHash       string            `json:"rendered_hash"`
	RenderedSampleHash *string           `json:"rendered_sample_hash"`
	Variables          map[string]string `json:"variables"`
}

// Tool is one callable the model can reach.
type Tool struct {
	Name              string  `json:"name"`
	SchemaHash        string  `json:"schema_hash"`
	DescriptionHash   *string `json:"description_hash"`
	DescriptionTokens int     `json:"description_tokens"`
}

// Retrieval is the whole retrieval configuration. Changing any of it changes what
// the model sees, which is why the diff reports a change to any field of it.
type Retrieval struct {
	IndexID               string  `json:"index_id"`
	EmbeddingModel        string  `json:"embedding_model"`
	EmbeddingModelVersion string  `json:"embedding_model_version"`
	Chunker               string  `json:"chunker"`
	TopK                  int     `json:"top_k"`
	FiltersHash           *string `json:"filters_hash"`
}

// Policy is the guardrail configuration in force.
type Policy struct {
	Version        string  `json:"version"`
	GuardrailsHash *string `json:"guardrails_hash"`
}

// Content is an opted-in copy of something that would otherwise only be hashed.
// It exists so a reviewer can see a prompt diff without the manifest becoming a
// leak by default.
type Content struct {
	Kind string `json:"kind"`
	Hash string `json:"hash"`
	Body string `json:"body"`
}

// ContentKinds are the kinds a Content entry may declare. A captured prompt, the
// rendered prompt, and a tool description are the three things that steer a model.
var ContentKinds = []string{"prompt", "rendered", "tool_description"}

// Validate checks that the plaintext is the plaintext. The manifest digest
// deliberately excludes content, so without this the body is an unchecked
// annotation: a reviewer would read text the model was never given, next to a hash
// that says so and a verifier that reports nothing wrong.
func (c Content) Validate() error {
	if !isSHA256Hex(c.Hash) {
		return fmt.Errorf("manifest: content %q hash is not 64 hex characters", c.Kind)
	}
	known := false
	for _, k := range ContentKinds {
		if c.Kind == k {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("manifest: content kind %q is not one of %s", c.Kind, strings.Join(ContentKinds, ", "))
	}
	sum := sha256.Sum256([]byte(c.Body))
	if got := hex.EncodeToString(sum[:]); got != c.Hash {
		return fmt.Errorf("manifest: content %q body does not match its hash (body hashes to %s, entry says %s)",
			c.Kind, got, c.Hash)
	}
	return nil
}

// Compute sets Digest and returns it. The digest covers the document with
// created_at, digest and content removed: a timestamp is not part of the context,
// content is deliberately redundant with its hash, and a digest cannot cover
// itself.
func (m *Manifest) Compute() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	body, err := m.digestBody()
	if err != nil {
		return "", err
	}
	canonical, err := Canonicalise(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	m.Digest = hex.EncodeToString(sum[:])
	return m.Digest, nil
}

// digestBody returns the manifest as a generic document with the excluded fields
// removed, so the canonicaliser sees exactly what the digest covers.
func (m *Manifest) digestBody() (map[string]any, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var generic map[string]any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("manifest: decode: %w", err)
	}
	delete(generic, "created_at")
	delete(generic, "digest")
	delete(generic, "content")
	return generic, nil
}

// Validate rejects a manifest that could not be compared meaningfully. It is
// deliberately strict: a manifest with a missing field produces a diff that says
// "nothing changed" for something that was never recorded.
func (m *Manifest) Validate() error {
	switch {
	case m.ManifestVersion != Version:
		return fmt.Errorf("manifest: version %d is not supported, want %d", m.ManifestVersion, Version)
	case m.Commit == "":
		return errors.New("manifest: commit is required")
	case m.Capture.Path == "":
		return errors.New("manifest: capture.path is required")
	}
	switch m.Capture.Path {
	case CaptureInstrumented, CaptureAsserted, CaptureObserved:
	default:
		return fmt.Errorf("manifest: capture.path %q is not one of instrumented, asserted, observed", m.Capture.Path)
	}
	if m.Model.ID == "" {
		return errors.New("manifest: model.id is required")
	}
	if !IsProvider(m.Model.Provider) {
		return fmt.Errorf("manifest: model.provider %q is not one of %s", m.Model.Provider, strings.Join(Providers, ", "))
	}
	// Sampling values are strings so that they never depend on float formatting, which only helps
	// if the string is a number: "hot" would hash, diff and gate like any other value.
	for _, p := range []struct {
		what    string
		value   *string
		pattern *regexp.Regexp
		want    string
	}{
		{"model.params.temperature", m.Model.Params.Temperature, decimalPattern, "a decimal number"},
		{"model.params.top_p", m.Model.Params.TopP, decimalPattern, "a decimal number"},
		{"model.params.seed", m.Model.Params.Seed, integerPattern, "an integer"},
	} {
		if p.value != nil && !p.pattern.MatchString(*p.value) {
			return fmt.Errorf("manifest: %s is %q, want %s written as a string", p.what, *p.value, p.want)
		}
	}
	// A count below zero is not a setting anyone chose; it is a corrupted or hand-edited file, and
	// a diff of it would report a change between two values neither of which can be run.
	if m.Model.Params.MaxTokens != nil && *m.Model.Params.MaxTokens < 0 {
		return fmt.Errorf("manifest: model.params.max_tokens is %d and cannot be negative", *m.Model.Params.MaxTokens)
	}
	if m.Retrieval.TopK < 0 {
		return fmt.Errorf("manifest: retrieval.top_k is %d and cannot be negative", m.Retrieval.TopK)
	}
	for _, h := range []struct {
		what  string
		value *string
	}{
		{"prompt.rendered_sample_hash", m.Prompt.RenderedSampleHash},
		{"retrieval.filters_hash", m.Retrieval.FiltersHash},
		{"policy.guardrails_hash", m.Policy.GuardrailsHash},
	} {
		if h.value != nil && !isSHA256Hex(*h.value) {
			return fmt.Errorf("manifest: %s is not 64 hex characters", h.what)
		}
	}
	if m.Prompt.TemplateHash == "" || m.Prompt.RenderedHash == "" {
		return errors.New("manifest: prompt.template_hash and prompt.rendered_hash are required")
	}
	for _, h := range []struct {
		what  string
		value string
	}{
		{"prompt.template_hash", m.Prompt.TemplateHash},
		{"prompt.rendered_hash", m.Prompt.RenderedHash},
	} {
		if !isSHA256Hex(h.value) {
			return fmt.Errorf("manifest: %s is not 64 hex characters", h.what)
		}
	}
	seen := make(map[string]bool, len(m.Tools))
	for _, t := range m.Tools {
		if t.Name == "" {
			return errors.New("manifest: every tool needs a name")
		}
		if seen[t.Name] {
			return fmt.Errorf("manifest: tool %q appears twice", t.Name)
		}
		seen[t.Name] = true
		if !isSHA256Hex(t.SchemaHash) {
			return fmt.Errorf("manifest: tool %q schema_hash is not 64 hex characters", t.Name)
		}
		if t.DescriptionTokens < 0 {
			return fmt.Errorf("manifest: tool %q description_tokens is %d and cannot be negative", t.Name, t.DescriptionTokens)
		}
	}
	// A tool's description steers the model as much as its schema does, and the only
	// record of it used to be a token count - which cannot detect an edit that keeps
	// the length. The hash is required; the count is not trusted for anything.
	for _, t := range m.Tools {
		if t.DescriptionHash == nil || !isSHA256Hex(*t.DescriptionHash) {
			return fmt.Errorf("manifest: tool %q has no description_hash; a token count cannot record what the description said", t.Name)
		}
	}
	kinds := make(map[string]bool, len(m.Content))
	for _, c := range m.Content {
		if kinds[c.Kind] {
			return fmt.Errorf("manifest: content %q appears twice", c.Kind)
		}
		kinds[c.Kind] = true
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// isSHA256Hex accepts lowercase hex only.
//
// hex.DecodeString accepts uppercase, so a manifest could carry the same hash in two
// spellings: both validated, but they compare unequal, so a baseline and a head that
// describe the same context reported a change, and two spellings of one digest were
// two different strings in the canonical form. One spelling, enforced at the door.
func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Canonicalise serialises a document the one way the specification permits, so
// two implementations and two operating systems produce the same bytes.
//
// Keys sort by UTF-16 code unit rather than by byte, which differs for characters
// above U+FFFF, and integers print as plain digits. Floats are refused.
func Canonicalise(v any) ([]byte, error) {
	var b strings.Builder
	if err := encodeValue(&b, v, 0); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

const maxDepth = 32

func encodeValue(b *strings.Builder, v any, depth int) error {
	if depth > maxDepth {
		return errors.New("manifest: document nests too deeply")
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
		return nil
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		return nil
	case string:
		return encodeString(b, x)
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return fmt.Errorf("%w: %q is not an integer", ErrFloat, x.String())
		}
		b.WriteString(strconv.FormatInt(n, 10))
		return nil
	case int:
		b.WriteString(strconv.Itoa(x))
		return nil
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
		return nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encodeString(b, k); err != nil {
				return err
			}
			b.WriteByte(':')
			if err := encodeValue(b, x[k], depth+1); err != nil {
				return err
			}
		}
		b.WriteByte('}')
		return nil
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := encodeValue(b, item, depth+1); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	default:
		return fmt.Errorf("manifest: cannot canonicalise %T", v)
	}
}

var escapes = map[rune]string{
	0x08: "\\b",
	0x09: "\\t",
	0x0A: "\\n",
	0x0C: "\\f",
	0x0D: "\\r",
	0x22: "\\\"",
	0x5C: "\\\\",
}

func encodeString(b *strings.Builder, s string) error {
	b.WriteByte('"')
	for _, r := range s {
		if esc, ok := escapes[r]; ok {
			b.WriteString(esc)
			continue
		}
		if r < 0x20 {
			fmt.Fprintf(b, "\\u%04x", r)
			continue
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return nil
}

// utf16Less compares two strings by UTF-16 code unit, which is what the
// specification requires and what a plain byte comparison gets wrong above
// U+FFFF.
func utf16Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}
