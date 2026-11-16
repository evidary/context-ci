package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/evidary/context-ci/internal/config"
)

const toolsFixture = `tools:
  - name: refund.create
    description: Create a refund for a customer order.
    schema:
      type: object
`

// initIn runs the verb and returns its exit code and its two streams.
func initIn(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"init"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// TestInitWritesAConfigTheOtherVerbsAccept is the whole requirement of the verb: the file it
// writes is one check accepts without an edit, and one manifest can use as written. If either
// half failed, init would be a longer way of writing a file a team then has to debug.
func TestInitWritesAConfigTheOtherVerbsAccept(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "You are a support agent for {{customer}}.\n")
	write(t, root, "config/tools.yaml", toolsFixture)

	code, stdout, stderr := initIn(t, "--root", root)
	if code != ExitOK {
		t.Fatalf("init exit = %d, want %d: %s", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, config.DefaultFile) {
		t.Fatalf("stdout does not name the file it wrote: %q", stdout)
	}
	if strings.Contains(stderr, "does not exist yet") {
		t.Fatalf("init reported a missing file for a checkout that has both sources: %s", stderr)
	}

	body := mustRead(t, filepath.Join(root, config.DefaultFile))
	if !strings.Contains(body, "# ") || !strings.Contains(body, "version: 1") {
		t.Fatalf("the file is not the commented configuration it claims to be:\n%s", body)
	}
	if _, err := config.Load(filepath.Join(root, config.DefaultFile)); err != nil {
		t.Fatalf("the generated configuration does not load: %v", err)
	}

	// manifest is the harder reader: it hashes the sources the file names.
	head := manifestFor(t, root, "head.json")
	if raw := mustRead(t, head); !strings.Contains(raw, "refund.create") {
		t.Fatalf("the generated configuration did not reach the tools source:\n%s", raw)
	}

	// check builds its head from the same checkout, so against a baseline written a moment ago
	// it has to find nothing - and has to say so about a manifest it built, not one it re-read.
	var checkOut, checkErr bytes.Buffer
	if code := run([]string{"check", "--root", root, "--baseline", head, "--commit", "9c1f2ab"}, &checkOut, &checkErr); code != ExitOK {
		t.Fatalf("check exit = %d, want %d with the generated configuration: %s", code, ExitOK, checkErr.String())
	}
	if !strings.Contains(checkOut.String(), "no change in context") {
		t.Fatalf("check found a change in a checkout nobody edited:\n%s", checkOut.String())
	}
}

// TestInitDoesNotTouchAnExistingFile covers the refusal, with a malformed file as the fixture:
// the file a team has is theirs even when it is wrong, and a silent replacement would delete the
// only record of what they gate on.
func TestInitDoesNotTouchAnExistingFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "prompt.\n")
	const broken = "version: [1\n"
	write(t, root, config.DefaultFile, broken)

	code, _, stderr := initIn(t, "--root", root)
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d when the file already exists", code, ExitUsage)
	}
	if !strings.Contains(stderr, "--force") {
		t.Fatalf("the refusal does not say how to proceed: %q", stderr)
	}
	if got := mustRead(t, filepath.Join(root, config.DefaultFile)); got != broken {
		t.Fatalf("the existing file was changed:\n%s", got)
	}
}

// TestInitForceReplacesAMalformedFile is the other half of the same rule: --force is the only way
// past the refusal, and what it leaves behind is valid, so a team recovering from a broken
// configuration gets a working one.
func TestInitForceReplacesAMalformedFile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "prompt.\n")
	write(t, root, config.DefaultFile, "version: [1\n")

	code, _, stderr := initIn(t, "--root", root, "--force")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d: %s", code, ExitOK, stderr)
	}
	cfg, err := config.Load(filepath.Join(root, config.DefaultFile))
	if err != nil {
		t.Fatalf("--force left a configuration that does not load: %v", err)
	}
	if cfg.Version != config.Version {
		t.Fatalf("version = %d, want %d", cfg.Version, config.Version)
	}
	if got := cfg.Capture.Sources[config.SourcePrompt]; got != "prompts/support.md" {
		t.Fatalf("the replacement does not describe the checkout: prompt source = %q", got)
	}
}

// TestInitReportsAnUnwritableDestination covers the write failure. A regular file where the
// directory must be is used because it fails on every platform; a read-only directory is the
// obvious case but Windows does not implement it, so it is covered separately below.
func TestInitReportsAnUnwritableDestination(t *testing.T) {
	root := t.TempDir()
	write(t, root, "occupied", "not a directory\n")
	target := filepath.Join(root, "occupied", config.DefaultFile)

	code, _, stderr := initIn(t, "--root", root, "--config", target)
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d for a destination that cannot be written", code, ExitUsage)
	}
	if !strings.Contains(stderr, config.DefaultFile) {
		t.Fatalf("the failure does not name the destination: %q", stderr)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("something was written where nothing could be written")
	}
}

func TestInitReportsAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a directory's mode does not stop a file being created inside it on Windows")
	}
	root := t.TempDir()
	write(t, root, "prompts/support.md", "prompt.\n")
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })
	// A process with CAP_DAC_OVERRIDE - root, which is how many containers run a test suite - can
	// write into a read-only directory, and then this test would prove the opposite of its name.
	probe := filepath.Join(locked, "probe")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err == nil {
		os.Remove(probe)
		t.Skip("this process can write into a read-only directory, so the case cannot be staged")
	}

	code, _, stderr := initIn(t, "--root", root, "--config", filepath.Join(locked, config.DefaultFile))
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d for a read-only directory", code, ExitUsage)
	}
	if !strings.Contains(stderr, "write") {
		t.Fatalf("the failure does not say it could not write: %q", stderr)
	}
}

// TestInitNamesOnlyFilesTheCheckoutHas is the invariant that keeps the generated file useful:
// every source it names, except the documented prompt default, is a file the checkout has. A
// configuration naming a path nobody created fails the first manifest for a reason the team then
// has to discover.
func TestInitNamesOnlyFilesTheCheckoutHas(t *testing.T) {
	cases := []struct {
		name        string
		files       map[string]string
		dirs        []string
		wantSources map[string]string
		wantMissing string
	}{
		{
			name:        "nothing to find",
			wantSources: map[string]string{config.SourcePrompt: defaultPrompt},
			wantMissing: defaultPrompt,
		},
		{
			name:        "a prompt in a conventional place",
			files:       map[string]string{"prompts/system.md": "You are a support agent.\n"},
			wantSources: map[string]string{config.SourcePrompt: "prompts/system.md"},
		},
		{
			name: "every optional source present",
			files: map[string]string{
				"prompts/support.md":     "You are a support agent for {{customer}}.\n",
				"config/tools.yaml":      toolsFixture,
				"config/retrieval.yaml":  "top_k: 5\n",
				"config/guardrails.yaml": "block: pii\n",
				"build/rendered.txt":     "rendered prompt\n",
			},
			wantSources: map[string]string{
				config.SourcePrompt:     "prompts/support.md",
				config.SourceTools:      "config/tools.yaml",
				config.SourceRetrieval:  "config/retrieval.yaml",
				config.SourceGuardrails: "config/guardrails.yaml",
				config.SourceRendered:   "build/rendered.txt",
			},
		},
		{
			name:        "a directory with a source name is not a source",
			dirs:        []string{"prompts/support.md"},
			wantSources: map[string]string{config.SourcePrompt: defaultPrompt},
			wantMissing: defaultPrompt,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, body := range tc.files {
				write(t, root, rel, body)
			}
			for _, rel := range tc.dirs {
				if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
			}

			code, _, stderr := initIn(t, "--root", root)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d: %s", code, ExitOK, stderr)
			}
			cfg, err := config.Load(filepath.Join(root, config.DefaultFile))
			if err != nil {
				t.Fatalf("the generated configuration does not load: %v", err)
			}
			if len(cfg.Capture.Sources) != len(tc.wantSources) {
				t.Fatalf("sources = %v, want %v", cfg.Capture.Sources, tc.wantSources)
			}
			for kind, want := range tc.wantSources {
				if got := cfg.Capture.Sources[kind]; got != want {
					t.Errorf("source %s = %q, want %q", kind, got, want)
				}
			}
			for kind, rel := range cfg.Capture.Sources {
				if rel == tc.wantMissing {
					continue
				}
				info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
				if err != nil || !info.Mode().IsRegular() {
					t.Errorf("source %s names %q, which is not a file in the checkout (%v)", kind, rel, err)
				}
			}
			if tc.wantMissing != "" && !strings.Contains(stderr, tc.wantMissing) {
				t.Errorf("init did not report the missing prompt %q: %s", tc.wantMissing, stderr)
			}
			if tc.wantMissing == "" && strings.Contains(stderr, "does not exist yet") {
				t.Errorf("init reported a missing file it did not name: %s", stderr)
			}

			// Where the prompt exists the file is usable as written, so the manifest must build.
			promptPath := filepath.Join(root, filepath.FromSlash(cfg.Capture.Sources[config.SourcePrompt]))
			if info, err := os.Stat(promptPath); err == nil && info.Mode().IsRegular() {
				manifestFor(t, root, "head.json")
			}
		})
	}
}

// TestInitWithoutAPromptSaysWhichFileToCreate covers the one exception to the rule above. The
// prompt source is required by the validator, so init names a documented default and the next
// command fails until the file exists - which only works if init said so.
func TestInitWithoutAPromptSaysWhichFileToCreate(t *testing.T) {
	root := t.TempDir()

	code, _, stderr := initIn(t, "--root", root)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d: %s", code, ExitOK, stderr)
	}
	if !strings.Contains(stderr, defaultPrompt) {
		t.Fatalf("init did not name the file it expects: %s", stderr)
	}
	if body := mustRead(t, filepath.Join(root, config.DefaultFile)); !strings.Contains(body, defaultPrompt) {
		t.Fatalf("the file does not say which source it is waiting for:\n%s", body)
	}

	var stdout, errOut bytes.Buffer
	if code := run([]string{"manifest", "--root", root, "--commit", "9c1f2ab"}, &stdout, &errOut); code != ExitManifest {
		t.Fatalf("manifest exit = %d, want %d: %s", code, ExitManifest, errOut.String())
	}
	if !strings.Contains(errOut.String(), defaultPrompt) {
		t.Fatalf("manifest does not name the missing prompt: %s", errOut.String())
	}

	// Doing exactly what init asked for is enough to make the file usable, with no edit to it.
	write(t, root, defaultPrompt, "You are a support agent.\n")
	head := manifestFor(t, root, "head.json")
	errOut.Reset()
	if code := run([]string{"check", "--root", root, "--baseline", head, "--commit", "9c1f2ab"}, &stdout, &errOut); code != ExitOK {
		t.Fatalf("check exit = %d, want %d after creating the file init named: %s", code, ExitOK, errOut.String())
	}
}

// TestInitWritesWhereItIsTold covers --config, and that the other verbs read the file it wrote:
// a flag that produces a file nothing else can find would be worse than no flag.
func TestInitWritesWhereItIsTold(t *testing.T) {
	root := t.TempDir()
	write(t, root, "prompts/support.md", "You are a support agent.\n")
	target := filepath.Join(root, "ci", "context.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	code, _, stderr := initIn(t, "--root", root, "--config", target)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d: %s", code, ExitOK, stderr)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("init did not write the path it was given: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, config.DefaultFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("init also wrote the default path")
	}

	head := filepath.Join(t.TempDir(), "head.json")
	var out, errOut bytes.Buffer
	if code := run([]string{"manifest", "--root", root, "--config", target, "--commit", "9c1f2ab", "--out", head}, &out, &errOut); code != ExitOK {
		t.Fatalf("manifest exit = %d with the generated configuration: %s", code, errOut.String())
	}
	errOut.Reset()
	if code := run([]string{"check", "--root", root, "--config", target, "--baseline", head, "--commit", "9c1f2ab"}, &out, &errOut); code != ExitOK {
		t.Fatalf("check exit = %d with the generated configuration: %s", code, errOut.String())
	}
}
