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
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/evidary/context-ci/internal/diff"
	"github.com/evidary/context-ci/internal/manifest"
)

// Version is the configuration schema version this package accepts.
const Version = 1

// DefaultFile is the file the CLI looks for when --config is not given.
const DefaultFile = ".context-ci.yaml"

// Source names a file whose content is hashed into the manifest.
//
// The keys are the ones the product specification's example configuration uses.
// SourceRetrieval is the retrieval configuration file, and its hash lands in
// manifest.retrieval.filters_hash: that is the field the manifest schema publishes,
// and the one whose change means the scope of what can be retrieved has moved.
const (
	SourcePrompt     = "prompt"
	SourceRendered   = "rendered"
	SourceTools      = "tools"
	SourceRetrieval  = "retrieval"
	SourceGuardrails = "guardrails"
)

// knownSources is the closed set of source keys. An unknown key is a typo, and a
// typo in a source means a file that never gets hashed.
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
	Path    string            `yaml:"path"`
	Sources map[string]string `yaml:"sources"`
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
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

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
	case "":
		return errors.New("config: capture.path is required")
	default:
		return fmt.Errorf("config: capture.path %q is not one of asserted, instrumented, observed", c.Capture.Path)
	}

	if len(c.Capture.Sources) == 0 {
		return errors.New("config: capture.sources is empty, so the manifest would cover nothing")
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
			return fmt.Errorf("config: capture.sources.%s is not a known source (%s)", k, strings.Join(known, ", "))
		}
		if c.Capture.Sources[k] == "" {
			return fmt.Errorf("config: capture.sources.%s is empty", k)
		}
	}
	if _, ok := c.Capture.Sources[SourcePrompt]; !ok {
		return errors.New("config: capture.sources.prompt is required: a manifest with no prompt hash proves nothing")
	}

	if c.Model.ID == "" {
		return errors.New("config: model.id is required")
	}
	// These three are refused here, with the configuration key in the message, rather than left
	// for the manifest validator to refuse later: the manifest rejects them too, and a team should
	// hear about its own file from the command that read it.
	if !manifest.IsProvider(c.Model.Provider) {
		if c.Model.Provider == "" {
			return fmt.Errorf("config: model.provider is required: one of %s", strings.Join(manifest.Providers, ", "))
		}
		return fmt.Errorf("config: model.provider %q is not one of %s", c.Model.Provider, strings.Join(manifest.Providers, ", "))
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
