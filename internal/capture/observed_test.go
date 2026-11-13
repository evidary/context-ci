package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evidary/context-ci/internal/config"
	"github.com/evidary/context-ci/internal/manifest"
)

func observedFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capture.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadObservedReadsACapture(t *testing.T) {
	observed, err := LoadObserved(observedFixture(t, `{
  "capture_version": 1,
  "sdk": "context-ci-python",
  "sdk_version": "0.1.0",
  "rendered": "You are a support agent. Order 9912 is late.",
  "template": "You are a support agent. Order {{order}} is {{state}}."
}`))
	if err != nil {
		t.Fatal(err)
	}
	if observed.SDKVersion != "0.1.0" || !strings.Contains(observed.Rendered, "9912") {
		t.Fatalf("observed %+v", observed)
	}
}

func TestLoadObservedRefusesWhatWouldProveNothing(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field":  `{"capture_version":1,"sdk_version":"0.1.0","rendered":"x","rendered_txt":"y"}`,
		"wrong version":  `{"capture_version":2,"sdk_version":"0.1.0","rendered":"x"}`,
		"no rendered":    `{"capture_version":1,"sdk_version":"0.1.0"}`,
		"no sdk version": `{"capture_version":1,"rendered":"x"}`,
		"empty render":   `{"capture_version":1,"sdk_version":"0.1.0","rendered":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadObserved(observedFixture(t, body)); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

// TestAnObservedCaptureMarksTheManifestInstrumented is the point of the feature: the manifest has
// to say how it was produced, because an instrumented capture and an asserted one are not the
// same evidence.
func TestAnObservedCaptureMarksTheManifestInstrumented(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "You are a support agent. Order {{order}} is {{state}}.\n")
	write(t, root, "config/tools.yaml", "{}\n")
	cfg := &config.Config{}
	cfg.Version = 1
	cfg.Capture.Path = "asserted"
	cfg.Capture.Sources = map[string]string{"prompt": "prompts/support.md", "tools": "config/tools.yaml"}
	cfg.Model.Provider = "openai"
	cfg.Model.ID = "gpt-x"
	cfg.Policy.Version = "2026.11"

	observed := &Observed{
		CaptureVersion: 1,
		SDK:            "context-ci-python",
		SDKVersion:     "0.1.0",
		Rendered:       "You are a support agent. Order 9912 is late.",
		Template:       "You are a support agent. Order {{order}} is {{state}}.",
	}
	built, err := Build(cfg, root, Options{Commit: strings.Repeat("a", 40), SDKVersion: "ignored", Observed: observed})
	if err != nil {
		t.Fatal(err)
	}
	if built.Capture.Path != "instrumented" {
		t.Fatalf("capture path %q, want instrumented", built.Capture.Path)
	}
	if built.Capture.SDKVersion != "0.1.0" {
		t.Fatalf("sdk version %q", built.Capture.SDKVersion)
	}
	if built.Capture.Reason != "" {
		t.Fatalf("the asserted-capture reason survived: %q", built.Capture.Reason)
	}
	if built.Prompt.RenderedSampleHash != nil {
		t.Fatal("a live capture was recorded as a sample")
	}
	if built.Prompt.RenderedHash != hashString(observed.Rendered) {
		t.Fatalf("rendered hash is not the digest of the captured text")
	}
	if built.Prompt.TemplateHash != hashString(observed.Template) {
		t.Fatalf("template hash is not the digest of the captured template")
	}
	if err := built.Validate(); err != nil {
		t.Fatalf("the instrumented manifest is invalid: %v", err)
	}
}

// TestTheSameCaptureProducesTheSameDigest: an instrumented manifest is evidence, so two runs of
// the same captured text must agree byte for byte apart from the timestamp.
func TestTheSameCaptureProducesTheSameDigest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "template\n")
	write(t, root, "config/tools.yaml", "{}\n")
	cfg := &config.Config{}
	cfg.Version = 1
	cfg.Capture.Path = "instrumented"
	cfg.Capture.Sources = map[string]string{"prompt": "prompts/support.md", "tools": "config/tools.yaml"}
	cfg.Model.Provider = "openai"
	cfg.Model.ID = "gpt-x"
	cfg.Policy.Version = "2026.11"

	make := func() *manifest.Manifest {
		m, err := Build(cfg, root, Options{
			Commit:   strings.Repeat("b", 40),
			Observed: &Observed{CaptureVersion: 1, SDKVersion: "0.1.0", Rendered: "same text\n"},
		})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	first, second := make(), make()
	firstDigest, err := first.Compute()
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := second.Compute()
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatal("the same capture produced two digests")
	}
}
