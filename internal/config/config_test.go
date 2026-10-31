package config

import (
	"strings"
	"testing"

	"github.com/context-ci/context-ci/internal/diff"
)

const minimal = `
version: 1
capture:
  path: asserted
  sources:
    prompt: prompts/support.md
model:
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
model: {id: m}
`,
		"instrumented without an sdk": `
version: 1
capture: {path: instrumented, sources: {prompt: p.md}}
model: {id: m}
`,
		"no sources": `
version: 1
capture: {path: asserted, sources: {}}
model: {id: m}
`,
		"no prompt source": `
version: 1
capture: {path: asserted, sources: {tools: t.yaml}}
model: {id: m}
`,
		"unknown source key": `
version: 1
capture: {path: asserted, sources: {prompt: p.md, promt: typo.md}}
model: {id: m}
`,
		"no model id": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {provider: openai}
`,
		"unknown severity class": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
severity: {model.ed: fail}
`,
		"bad severity value": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
severity: {model.id: block}
`,
		"bad fail_on": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
gate: {fail_on: sometimes}
`,
		"negative calls": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
gate: {calls_per_day: -1}
`,
		"unknown top level field": `
version: 1
capture: {path: asserted, sources: {prompt: p.md}}
model: {id: m}
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
