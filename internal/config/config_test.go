package config

import (
	"strings"
	"testing"

	"github.com/evidary/context-ci/internal/diff"
)

const minimal = `
version: 1
capture:
  path: asserted
  sources:
    prompt: prompts/support.md
model:
  provider: openai
  id: gpt-x
`

func TestParseMinimal(t *testing.T) {
	cfg, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.FailOn() != "fail" {
		t.Fatalf("default fail_on = %q, want fail: a configuration that says nothing must still protect", cfg.FailOn())
	}
	if cfg.SeverityOverrides() != nil {
		t.Fatal("a configuration with no overrides produced overrides")
	}
}

// TestTemperatureKeepsItsLiteralText covers the determinism rule: the digest must
// not depend on how a YAML parser chose to represent a number.
func TestTemperatureKeepsItsLiteralText(t *testing.T) {
	for _, literal := range []string{"0", "0.0", "0.70", "1e-1"} {
		body := minimal + "  params:\n    temperature: " + literal + "\n"
		cfg, err := Parse([]byte(body))
		if err != nil {
			t.Fatalf("Parse(%q) error = %v", literal, err)
		}
		if got := string(cfg.Model.Params.Temperature); got != literal {
			t.Fatalf("temperature %q became %q, so the digest would depend on the parser", literal, got)
		}
	}
}

func TestFullConfigurationParses(t *testing.T) {
	body := `
version: 1
capture:
  path: asserted
  sources:
    prompt: prompts/support.md
    tools: config/tools.yaml
    retrieval: config/retrieval.yaml
model:
  provider: openai
  id: gpt-x
  alias_of: gpt-x-latest
  params:
    temperature: "0"
    max_tokens: 1024
retrieval:
  index_id: kb-prod
  embedding_model: embed-3
  embedding_model_version: "2026-05"
  chunker: recursive-800
  top_k: 5
policy:
  version: "2026.11"
severity:
  model.id: fail
  retrieval.top_k: warn
gate:
  fail_on: warn
  calls_per_day: 10000
  input_usd_per_mtok: 3
`
	cfg, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Gate.FailOn != "warn" || cfg.Gate.CallsPerDay != 10000 || cfg.Gate.InputUSDPerMTok != 3 {
		t.Fatalf("gate was not parsed: %+v", cfg.Gate)
	}
	overrides := cfg.SeverityOverrides()
	if overrides[string(diff.ClassRetrievalTopK)] != diff.SeverityWarn {
		t.Fatalf("severity override was not carried: %v", overrides)
	}
}

func TestValidationRejects(t *testing.T) {
	cases := map[string]string{
		"wrong version": `
version: 2
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
`,
		"instrumented without an sdk": `
version: 1
capture: {path: instrumented, sources: {prompt: p.md}}
model: {provider: openai, id: m}
`,
		"nothing to cover": `
version: 1
capture: {path: asserted, sources: {}, agent_files: false}
model: {provider: openai, id: m}
`,
		"a rendered sample with no prompt": `
version: 1
capture: {path: asserted, sources: {rendered: build/r.txt}}
`,
		"a group name with a dot": `
version: 1
capture: {files: {agent.rules: "rules/**"}}
`,
		"a group with no pattern": `
version: 1
capture: {files: {prompts: []}}
`,
		"a pattern that leaves the root": `
version: 1
capture: {files: {prompts: "../other/*.md"}}
`,
		"an absolute pattern": `
version: 1
capture: {files: {prompts: "/etc/*.md"}}
`,
		"a pattern with a backslash": `
version: 1
capture: {files: {prompts: "prompts\\*.md"}}
`,
		"** inside a segment": `
version: 1
capture: {files: {prompts: "prompts/**.md"}}
`,
		"a malformed character class": `
version: 1
capture: {files: {prompts: "prompts/[a.md"}}
`,
		"a severity for a group that is not defined": `
version: 1
capture: {files: {prompts: "prompts/*.md"}}
severity: {files.examples: fail}
`,
		"parameters for a model nobody named": `
version: 1
capture: {sources: {prompt: p.md}}
model: {params: {temperature: "0"}}
`,
		"unknown source key": `
version: 1
capture: {path: asserted, sources: {prompt: p.md, promt: typo.md}}
model: {provider: openai, id: m}
`,
		"no model id": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai}
`,
		"no model provider": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
`,
		"a provider outside the documented set": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: OpenAI, id: m}
`,
		"negative top_k": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
retrieval: {top_k: -1}
`,
		"negative max_tokens": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m, params: {max_tokens: -5}}
`,
		"unknown severity class": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
severity: {model.ed: fail}
`,
		"bad severity value": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
severity: {model.id: block}
`,
		"bad fail_on": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
gate: {fail_on: sometimes}
`,
		"negative calls": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
gate: {calls_per_day: -1}
`,
		"unknown top level field": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai, id: m}
gat: {fail_on: never}
`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(body))
			if err == nil {
				t.Fatal("Parse() accepted an invalid configuration")
			}
			if !strings.HasPrefix(err.Error(), "config:") {
				t.Fatalf("error is not attributed to config: %v", err)
			}
		})
	}
}

// TestUnknownFieldMessageIsUseful keeps the failure actionable. A silently ignored
// `gat:` would leave the team believing a gate exists.
func TestUnknownFieldMessage(t *testing.T) {
	body := strings.Replace(minimal, "model:", "gat:\n  fail_on: never\nmodel:", 1)
	_, err := Parse([]byte(body))
	if err == nil {
		t.Fatal("Parse() accepted an unknown field")
	}
	if !strings.Contains(err.Error(), "gat") {
		t.Fatalf("the error does not name the offending field: %v", err)
	}
}
