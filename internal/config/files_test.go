package config

import (
	"reflect"
	"testing"
)

// TestFilesAndDefaultsParse covers what version 0.9 added: named groups of patterns, written as
// one string or a list, a model and a prompt that may be left out, and a capture path that may
// be left unsaid because only one value is accepted.
func TestFilesAndDefaultsParse(t *testing.T) {
	cfg, err := Parse([]byte(`
version: 1
capture:
  files:
    prompts: ["prompts/**/*.md", "shared/tone.md"]
    examples: few_shot/*.jsonl
severity:
  files.examples: fail
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Capture.Path != "asserted" {
		t.Errorf("capture.path = %q, want the default asserted", cfg.Capture.Path)
	}
	if want := (Patterns{"prompts/**/*.md", "shared/tone.md"}); !reflect.DeepEqual(cfg.Capture.Files["prompts"], want) {
		t.Errorf("prompts = %v, want %v", cfg.Capture.Files["prompts"], want)
	}
	if want := (Patterns{"few_shot/*.jsonl"}); !reflect.DeepEqual(cfg.Capture.Files["examples"], want) {
		t.Errorf("examples = %v, want %v", cfg.Capture.Files["examples"], want)
	}
	if cfg.HasModel() {
		t.Error("a configuration with no model block reports a model")
	}
	if !cfg.AgentFilesEnabled() {
		t.Error("agent file discovery should be on unless it is switched off")
	}
}

func TestAgentFilesCanBeSwitchedOff(t *testing.T) {
	cfg, err := Parse([]byte("version: 1\ncapture: {sources: {prompt: p.md}, agent_files: false}\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.AgentFilesEnabled() {
		t.Error("agent_files: false was ignored")
	}
}

// TestDefaultIsValid: the configuration used when a repository has none must pass the same
// validation as one somebody wrote.
func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("Default() does not validate: %v", err)
	}
}

func TestValidatePattern(t *testing.T) {
	for _, ok := range []string{"AGENTS.md", "prompts/", "prompts/**/*.md", "**/SKILL.md", ".cursor/rules/**", "a/[ab]?.txt"} {
		if err := ValidatePattern(ok); err != nil {
			t.Errorf("ValidatePattern(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "../x", "a/../../x", "/abs", "C:/x", `a\b`, "a//b", "a/**b", "a/[b"} {
		if err := ValidatePattern(bad); err == nil {
			t.Errorf("ValidatePattern(%q) accepted a bad pattern", bad)
		}
	}
}
