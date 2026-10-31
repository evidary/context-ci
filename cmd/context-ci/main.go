// Command context-ci answers one question in a pull request: did the context the
// model sees change, and does that change matter?
//
// It makes no network call, requires no account, and needs no secret. Exit codes
// are part of the contract so a CI pipeline can script them:
//
//	0  nothing to report, or nothing above the threshold
//	1  the gate failed: a change at or above the configured threshold
//	2  usage or configuration error
//	3  a manifest could not be read or is invalid
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/context-ci/context-ci/internal/capture"
	"github.com/context-ci/context-ci/internal/config"
	"github.com/context-ci/context-ci/internal/diff"
	"github.com/context-ci/context-ci/internal/manifest"
)

// Exit codes. See the package comment.
const (
	ExitOK       = 0
	ExitGate     = 1
	ExitUsage    = 2
	ExitManifest = 3
)

// ManifestFileName is the artefact the other verbs read.
const ManifestFileName = "context-manifest.json"

const usage = `context-ci — fail the build when the context a model sees changes

usage:
  context-ci manifest [flags]     write %s for this checkout
  context-ci diff <base> <head>   semantic diff between two manifests
  context-ci check [flags]        diff against the baseline and gate
  context-ci classes              list the diff classes and their defaults

Run 'context-ci <verb> -h' for the flags of one verb.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, usage, ManifestFileName)
		return ExitUsage
	}
	switch args[0] {
	case "manifest":
		return runManifest(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "classes":
		return runClasses(stdout)
	case "-h", "--help", "help":
		fmt.Fprintf(stdout, usage, ManifestFileName)
		return ExitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "context-ci %s\n", version)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "context-ci: unknown verb %q\n\n", args[0])
		fmt.Fprintf(stderr, usage, ManifestFileName)
		return ExitUsage
	}
}

// version is set from the VERSION file at release time; "dev" otherwise.
var version = "dev"

func runManifest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath = fs.String("config", "", "configuration file (default "+config.DefaultFile+")")
		root       = fs.String("root", ".", "repository root to capture")
		out        = fs.String("out", "", "output path (default stdout)")
		commit     = fs.String("commit", "", "commit to record (default: git HEAD)")
		sdkVer     = fs.String("sdk-version", "", "SDK version, for instrumented capture")
	)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "manifest: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}

	cfg, err := loadConfig(*configPath, *root)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitUsage
	}
	resolvedCommit := *commit
	if resolvedCommit == "" {
		resolvedCommit, err = gitHEAD(*root)
		if err != nil {
			fmt.Fprintf(stderr, "manifest: cannot determine the commit: %v\n"+
				"pass --commit, because a manifest without a commit cannot be reproduced\n", err)
			return ExitUsage
		}
	}

	m, err := capture.Build(cfg, *root, capture.Options{
		Commit:     resolvedCommit,
		Now:        time.Now(),
		SDKVersion: *sdkVer,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitManifest
	}

	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "manifest: encode: %v\n", err)
		return ExitManifest
	}
	body = append(body, '\n')

	if *out == "" || *out == "-" {
		stdout.Write(body)
		fmt.Fprintf(stderr, "context-ci: digest %s\n", m.Digest)
		return ExitOK
	}
	if err := os.WriteFile(*out, body, 0o644); err != nil {
		fmt.Fprintf(stderr, "manifest: write %s: %v\n", *out, err)
		return ExitManifest
	}
	fmt.Fprintf(stdout, "wrote %s (digest %s)\n", *out, m.Digest)
	return ExitOK
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		format     = fs.String("format", "text", "text or json")
		configPath = fs.String("config", "", "configuration file, for severity overrides")
		root       = fs.String("root", ".", "repository root, for the configuration")
	)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 2 {
		fmt.Fprintf(stderr, "diff: need a base and a head manifest\n")
		return ExitUsage
	}

	base, err := readManifest(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return ExitManifest
	}
	head, err := readManifest(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return ExitManifest
	}

	opts, err := diffOptions(*configPath, *root)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitUsage
	}
	result := diff.Compare(base, head, opts)
	if err := writeResult(stdout, *format, result); err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return ExitUsage
	}
	return ExitOK
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath = fs.String("config", "", "configuration file (default "+config.DefaultFile+")")
		root       = fs.String("root", ".", "repository root")
		head       = fs.String("head", ManifestFileName, "head manifest")
		base       = fs.String("baseline", "", "base manifest (required)")
		format     = fs.String("format", "text", "text, github or json")
	)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "check: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}
	if *base == "" {
		fmt.Fprintf(stderr, "check: --baseline is required; without a baseline there is nothing to compare\n")
		return ExitUsage
	}

	cfg, err := loadConfig(*configPath, *root)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitUsage
	}
	baseManifest, err := readManifest(*base)
	if err != nil {
		fmt.Fprintf(stderr, "check: baseline: %v\n", err)
		return ExitManifest
	}
	headManifest, err := readManifest(*head)
	if err != nil {
		fmt.Fprintf(stderr, "check: head: %v\n", err)
		return ExitManifest
	}

	opts := diff.Options{
		Severity:        cfg.SeverityOverrides(),
		CallsPerDay:     cfg.Gate.CallsPerDay,
		InputUSDPerMTok: cfg.Gate.InputUSDPerMTok,
	}
	result := diff.Compare(baseManifest, headManifest, opts)

	if err := writeResult(stdout, *format, result); err != nil {
		fmt.Fprintf(stderr, "check: %v\n", err)
		return ExitUsage
	}
	if failed(result, cfg.FailOn()) {
		return ExitGate
	}
	return ExitOK
}

func runClasses(stdout io.Writer) int {
	classes := diff.Classes()
	fmt.Fprintf(stdout, "%-30s %s\n", "class", "default")
	for _, c := range classes {
		fmt.Fprintf(stdout, "%-30s %s\n", c, diff.DefaultSeverity(c))
	}
	fmt.Fprintf(stdout, "\nOverride with severity.<class>: fail|warn|ignore in %s\n", config.DefaultFile)
	return ExitOK
}

// failed reports whether the result breaches the configured threshold.
func failed(r *diff.Result, failOn string) bool {
	switch failOn {
	case "never":
		return false
	case "warn":
		return r.Worst() == diff.SeverityFail || r.Worst() == diff.SeverityWarn
	default: // "fail"
		return r.Worst() == diff.SeverityFail
	}
}

func writeResult(w io.Writer, format string, r *diff.Result) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case "github":
		writeGitHub(w, r)
		return nil
	case "text", "":
		writeText(w, r)
		return nil
	}
	return fmt.Errorf("unknown format %q, want text, github or json", format)
}

func writeText(w io.Writer, r *diff.Result) {
	if r.Equal() {
		fmt.Fprintf(w, "no change in context\n  digest %s\n", r.HeadDigest)
		return
	}
	fmt.Fprintf(w, "%d change(s) in context\n\n", len(r.Entries))
	for _, e := range r.Entries {
		fmt.Fprintf(w, "  [%s] %s\n", e.Severity, e.Class)
		fmt.Fprintf(w, "    %s\n", e.Summary)
		if e.Before != "" || e.After != "" {
			fmt.Fprintf(w, "    before: %s\n    after:  %s\n", orDash(e.Before), orDash(e.After))
		}
		if e.Detail != "" {
			fmt.Fprintf(w, "    %s\n", e.Detail)
		}
	}
	if len(r.Unchanged) > 0 {
		fmt.Fprintf(w, "\nunchanged: %s\n", joinClasses(r.Unchanged))
	}
	fmt.Fprintf(w, "\ndigest %s -> %s\n", short(r.BaseDigest), short(r.HeadDigest))
}

// writeGitHub emits workflow commands, so the failure appears on the pull request
// next to the diff instead of only in a log.
func writeGitHub(w io.Writer, r *diff.Result) {
	if r.Equal() {
		fmt.Fprintf(w, "::notice title=%s::no change in context (%s)\n",
			escapeProperty("context-ci"), escapeData(short(r.HeadDigest)))
		return
	}
	for _, e := range r.Entries {
		level := "warning"
		if e.Severity == diff.SeverityFail {
			level = "error"
		}
		if e.Severity == diff.SeverityIgnore {
			level = "notice"
		}
		// The title is a property and the rest is the message. They have different
		// escaping rules: a colon in a message is legal and must survive, while a
		// colon in a property would end the property list early.
		fmt.Fprintf(w, "::%s title=%s::%s",
			level, escapeProperty("context-ci "+string(e.Class)), escapeData(e.Summary))
		if e.Before != "" || e.After != "" {
			fmt.Fprintf(w, " (%s -> %s)", escapeData(orDash(e.Before)), escapeData(orDash(e.After)))
		}
		if e.Detail != "" {
			fmt.Fprintf(w, " %s", escapeData(e.Detail))
		}
		fmt.Fprintln(w)
	}
}

// escapeData escapes a message. GitHub unescapes only these three sequences in a
// message, so escaping anything else would show the escape itself to the reader.
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// escapeProperty escapes a property value, where a colon or comma would end the
// property list and let a manifest value start a second workflow command.
func escapeProperty(s string) string {
	return strings.NewReplacer(
		"%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C",
	).Replace(s)
}

// readManifest reads a manifest and proves it is the manifest it claims to be.
//
// Recomputing the digest is not optional. Without it the digest is decorative: a
// baseline committed to a repository could be edited so the gate sees no change,
// and the file would keep reporting the digest of a document it no longer
// contains. Both files would show the same digest while disagreeing about the
// model, which is the worst possible failure for a tool whose output is evidence.
func readManifest(path string) (*manifest.Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest.Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s is not a manifest: %w", path, err)
	}
	// Anything after the first document is unread by this decoder, so a file could
	// hold a manifest and a second value that some other reader would trust. Refusing
	// the trailing bytes keeps every reader looking at the same document.
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s holds more than one JSON document, so different readers would see different manifests", path)
	}
	stored := m.Digest
	recomputed, err := m.Compute()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if stored == "" {
		return nil, fmt.Errorf("%s carries no digest, so it cannot be shown to be unedited", path)
	}
	if !strings.EqualFold(stored, recomputed) {
		return nil, fmt.Errorf("%s was modified after it was produced: the stored digest is %s "+
			"but its content hashes to %s", path, short(stored), short(recomputed))
	}
	return &m, nil
}

func loadConfig(path, root string) (*config.Config, error) {
	if path == "" {
		path = filepath.Join(root, config.DefaultFile)
	}
	cfg, err := config.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config: %s not found; create one, or pass --config", path)
		}
		return nil, err
	}
	return cfg, nil
}

// diffOptions loads the optional configuration for severity overrides. `diff`
// works without a configuration file, because comparing two manifests should never
// require setup.
func diffOptions(path, root string) (diff.Options, error) {
	if path == "" {
		candidate := filepath.Join(root, config.DefaultFile)
		if _, err := os.Stat(candidate); err != nil {
			return diff.Options{}, nil
		}
		path = candidate
	}
	cfg, err := config.Load(path)
	if err != nil {
		return diff.Options{}, err
	}
	return diff.Options{
		Severity:        cfg.SeverityOverrides(),
		CallsPerDay:     cfg.Gate.CallsPerDay,
		InputUSDPerMTok: cfg.Gate.InputUSDPerMTok,
	}, nil
}

// gitHEAD reads the commit without a network call and without requiring the git
// binary to be configured, because CI images vary.
func gitHEAD(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "…"
}

func joinClasses(cs []diff.Class) string {
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = string(c)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
