// Package config parses and validates .context-ci.yaml.
//
// Validation is strict on purpose. A configuration typo that is silently ignored
// is worse than a failure: a team that writes `gate.fail_on: warn` and gets a
// silently defaulted `fail` gate will turn the gate off rather than debug it, and
// then nothing is protected at all.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sigilary/context-ci/internal/diff"
	"github.com/sigilary/context-ci/internal/manifest"
)

// Version is the configuration schema version this package accepts.
const Version = 1

// DefaultFile is the file the CLI looks for when --config is not given.
const DefaultFile = ".context-ci.yaml"

// Source names a file whose content is hashed into the manifest.
//
// The keys are the ones the product specification's example configuration uses.
// SourceRetrieval is the retrieval configuration file, and its hash lands in
// manifest.retrieval.filters_hash.
const (
	SourcePrompt     = "prompt"
	SourceRendered   = "rendered"
	SourceTools      = "tools"
	SourceRetrieval  = "retrieval"
	SourceGuardrails = "guardrails"
)

// knownSources is the closed set of source keys. An unknown key is a typo, and a
// typo in a source means a file that never gets hashed. A file that fits none of
// them belongs in capture.files, which takes any name.
var knownSources = map[string]bool{
	SourcePrompt: true, SourceRendered: true, SourceTools: true,
	SourceRetrieval: true, SourceGuardrails: true,
}

// Config is .context-ci.yaml.
type Config struct {
	Version   int               `yaml:"version"`
	Capture   Capture           `yaml:"capture"`
	Model     Model             `yaml:"model"`
	Retrieval Retrieval         `yaml:"retrieval"`
	Policy    Policy            `yaml:"policy"`
	Severity  map[string]string `yaml:"severity"`
	Gate      Gate              `yaml:"gate"`
}

// Capture selects how the manifest is produced and which files it covers.
type Capture struct {
	Path string `yaml:"path"`
	// Sources are the five files with a meaning of their own: each is one path.
	Sources map[string]string `yaml:"sources"`
	// Files are named groups of path patterns: prompt fragments, few-shot files, anything else
	// that steers the model. Every matched file is hashed and compared by path, and each group
	// is a diff class of its own, files.<group>.
	Files map[string]Patterns `yaml:"files"`
	// AgentFiles turns the built-in discovery of coding-agent files on or off. It is on unless
	// this says false, because the files it finds steer an agent whether or not anyone listed
	// them here.
	AgentFiles *bool `yaml:"agent_files"`
}

// Patterns is one path pattern or a list of them.
type Patterns []string

// UnmarshalYAML accepts `group: path` as well as `group: [a, b]`, because the one-file case is
// the common one and a list of one is noise.
func (p *Patterns) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*p = Patterns{node.Value}
		return nil
	case yaml.SequenceNode:
		out := make(Patterns, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: a pattern is a string, got %s", item.Line, kindName(item.Kind))
			}
			out = append(out, item.Value)
		}
		*p = out
		return nil
	}
	return fmt.Errorf("line %d: expected a pattern or a list of patterns, got %s", node.Line, kindName(node.Kind))
}

// Model is the model configuration under test.
type Model struct {
	Provider string `yaml:"provider"`
	ID       string `yaml:"id"`
	AliasOf  string `yaml:"alias_of"`
	Params   Params `yaml:"params"`
}

// Params holds sampling settings as written, so the digest depends on the literal
// text and never on how a YAML parser decided to represent a number.
type Params struct {
	Temperature Literal `yaml:"temperature"`
	MaxTokens   *int    `yaml:"max_tokens"`
	Seed        Literal `yaml:"seed"`
	TopP        Literal `yaml:"top_p"`
}

// Retrieval is the retrieval configuration under test.
type Retrieval struct {
	IndexID               string `yaml:"index_id"`
	EmbeddingModel        string `yaml:"embedding_model"`
	EmbeddingModelVersion string `yaml:"embedding_model_version"`
	Chunker               string `yaml:"chunker"`
	TopK                  int    `yaml:"top_k"`
}

// Policy is the guardrail configuration under test.
type Policy struct {
	Version string `yaml:"version"`
}

// Gate is what `context-ci check` enforces.
type Gate struct {
	FailOn          string  `yaml:"fail_on"`
	CallsPerDay     int     `yaml:"calls_per_day"`
	InputUSDPerMTok float64 `yaml:"input_usd_per_mtok"`
}

// Literal keeps a YAML scalar exactly as written. A temperature of 0 and a
// temperature of 0.0 are the same setting but different bytes, and the digest must
// depend on neither the parser nor the platform, so the literal text is carried
// through and compared as text.
type Literal string

// UnmarshalYAML takes the scalar's source text rather than a parsed value.
func (l *Literal) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected a scalar, got %s", kindName(node.Kind))
	}
	*l = Literal(node.Value)
	return nil
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	case yaml.AliasNode:
		return "an alias"
	}
	return "a non-scalar"
}

// Default is the configuration used when a repository has none: no declared model, no named
// source, and the built-in discovery of coding-agent files. It is what lets `check --base-ref`
// say something useful in a repository nobody has configured.
func Default() *Config {
	return &Config{Version: Version, Capture: Capture{Path: manifest.CaptureAsserted}}
}

// Load reads and validates a configuration file.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return Parse(raw)
}

// Parse validates configuration bytes. Unknown fields are rejected, because a
// misspelled key is a rule the team believes is in force and is not.
func Parse(raw []byte) (*Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	// A second document in the same file was ignored, so the half of the file that was
	// read could disagree with the half that was written - and the tools file now
	// refuses that, so leaving config permissive would make the two disagree on one
	// rule.
	var extra any
	if err := dec.Decode(&extra); err != nil {
		if !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		return nil, errors.New("config: the file holds more than one YAML document")
	}
	if cfg.Capture.Path == "" {
		// asserted is the only path a configuration file can claim, so writing it is optional.
		cfg.Capture.Path = manifest.CaptureAsserted
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// AgentFilesEnabled reports whether the built-in discovery of coding-agent files is on.
func (c *Config) AgentFilesEnabled() bool {
	return c.Capture.AgentFiles == nil || *c.Capture.AgentFiles
}

// HasModel reports whether the configuration declares a model at all.
func (c *Config) HasModel() bool {
	m := c.Model
	return m.ID != "" || m.Provider != "" || m.AliasOf != "" || m.Params != (Params{})
}

// groupName is what a capture.files group may be called. A dot is excluded so that a group can
// never be mistaken for one of the built-in classes, which all contain one.
var groupName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Validate rejects a configuration that could not produce a meaningful gate.
func (c *Config) Validate() error {
	if c.Version != Version {
		return fmt.Errorf("config: version %d is not supported, want %d", c.Version, Version)
	}

	switch c.Capture.Path {
	case manifest.CaptureAsserted:
	case manifest.CaptureInstrumented:
		return errors.New("config: capture.path instrumented needs a running application and an SDK; use asserted for the CLI")
	case manifest.CaptureObserved:
		return errors.New("config: capture.path observed needs a recorded sample; use asserted for the CLI")
	default:
		return fmt.Errorf("config: capture.path %q is not one of asserted, instrumented, observed", c.Capture.Path)
	}

	keys := make([]string, 0, len(c.Capture.Sources))
	for k := range c.Capture.Sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !knownSources[k] {
			known := make([]string, 0, len(knownSources))
			for name := range knownSources {
				known = append(known, name)
			}
			sort.Strings(known)
			return fmt.Errorf("config: capture.sources.%s is not a known source (%s); "+
				"a file of any other kind goes under capture.files, in a group you name",
				k, strings.Join(known, ", "))
		}
		if c.Capture.Sources[k] == "" {
			return fmt.Errorf("config: capture.sources.%s is empty", k)
		}
	}
	if _, ok := c.Capture.Sources[SourceRendered]; ok {
		if _, ok := c.Capture.Sources[SourcePrompt]; !ok {
			return errors.New("config: capture.sources.rendered is the rendered form of capture.sources.prompt, which is not set")
		}
	}

	groups := make([]string, 0, len(c.Capture.Files))
	for g := range c.Capture.Files {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	for _, g := range groups {
		if !groupName.MatchString(g) {
			return fmt.Errorf("config: capture.files.%s is not a usable group name: "+
				"use lower-case letters, digits, '-' and '_', starting with a letter", g)
		}
		if len(c.Capture.Files[g]) == 0 {
			return fmt.Errorf("config: capture.files.%s lists no pattern", g)
		}
		for _, pattern := range c.Capture.Files[g] {
			if err := ValidatePattern(pattern); err != nil {
				return fmt.Errorf("config: capture.files.%s: %w", g, err)
			}
		}
	}

	if len(c.Capture.Sources) == 0 && len(c.Capture.Files) == 0 && !c.AgentFilesEnabled() {
		return errors.New("config: capture.sources and capture.files are empty and capture.agent_files is false, " +
			"so the manifest would cover nothing")
	}

	// A model is optional, because a repository may gate files and call no model of its own. A
	// half-declared one is refused here, with the configuration key in the message, rather than
	// left for the manifest validator to refuse later.
	if c.HasModel() {
		if c.Model.ID == "" {
			return errors.New("config: model.id is required when a model is declared")
		}
		if !manifest.IsProvider(c.Model.Provider) {
			if c.Model.Provider == "" {
				return fmt.Errorf("config: model.provider is required: one of %s", strings.Join(manifest.Providers, ", "))
			}
			return fmt.Errorf("config: model.provider %q is not one of %s", c.Model.Provider, strings.Join(manifest.Providers, ", "))
		}
	}
	if c.Model.Params.MaxTokens != nil && *c.Model.Params.MaxTokens < 0 {
		return errors.New("config: model.params.max_tokens cannot be negative")
	}
	if c.Retrieval.TopK < 0 {
		return errors.New("config: retrieval.top_k cannot be negative")
	}

	for name, sev := range c.Severity {
		if !diff.IsKnownClass(name) {
			return fmt.Errorf("config: severity.%s is not a diff class; run 'context-ci classes' for the list", name)
		}
		if group, ok := strings.CutPrefix(name, diff.FilesClassPrefix); ok {
			if _, configured := c.Capture.Files[group]; !configured {
				return fmt.Errorf("config: severity.%s names a group capture.files does not define", name)
			}
		}
		switch diff.Severity(sev) {
		case diff.SeverityFail, diff.SeverityWarn, diff.SeverityIgnore:
		default:
			return fmt.Errorf("config: severity.%s is %q, want fail, warn or ignore", name, sev)
		}
	}

	switch c.Gate.FailOn {
	case "", "fail", "warn", "never":
	default:
		return fmt.Errorf("config: gate.fail_on is %q, want fail, warn or never", c.Gate.FailOn)
	}
	if c.Gate.CallsPerDay < 0 {
		return errors.New("config: gate.calls_per_day cannot be negative")
	}
	if c.Gate.InputUSDPerMTok < 0 {
		return errors.New("config: gate.input_usd_per_mtok cannot be negative")
	}
	return nil
}

// ValidatePattern rejects a pattern that could never match or that points outside the root. A
// pattern is a slash-separated path in which `*` and `?` match within one segment, `[...]` is a
// character class, and a segment that is exactly `**` matches any number of directories.
func ValidatePattern(pattern string) error {
	switch {
	case pattern == "":
		return errors.New("a pattern is empty")
	case strings.Contains(pattern, `\`):
		return fmt.Errorf("pattern %q uses a backslash; write paths with forward slashes on every platform", pattern)
	case strings.HasPrefix(pattern, "/") || (len(pattern) > 1 && pattern[1] == ':'):
		return fmt.Errorf("pattern %q is absolute; a pattern is relative to the repository root", pattern)
	}
	for _, segment := range strings.Split(strings.TrimSuffix(pattern, "/"), "/") {
		switch {
		case segment == "..":
			return fmt.Errorf("pattern %q leaves the repository root", pattern)
		case segment == "":
			return fmt.Errorf("pattern %q has an empty path segment", pattern)
		case segment == "**":
			continue
		case strings.Contains(segment, "**"):
			return fmt.Errorf("pattern %q: ** must be a whole path segment, as in prompts/**/*.md", pattern)
		}
		if _, err := path.Match(segment, ""); err != nil {
			return fmt.Errorf("pattern %q is malformed: %v", pattern, err)
		}
	}
	return nil
}

// SeverityOverrides converts the configured severities into the form diff wants.
func (c *Config) SeverityOverrides() map[string]diff.Severity {
	if len(c.Severity) == 0 {
		return nil
	}
	out := make(map[string]diff.Severity, len(c.Severity))
	for k, v := range c.Severity {
		out[k] = diff.Severity(v)
	}
	return out
}

// FailOn returns the effective gate threshold, defaulting to fail so that a
// configuration that says nothing is still protective.
func (c *Config) FailOn() string {
	if c.Gate.FailOn == "" {
		return "fail"
	}
	return c.Gate.FailOn
}
