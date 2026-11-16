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
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/evidary/context-ci/internal/capture"
	"github.com/evidary/context-ci/internal/config"
	"github.com/evidary/context-ci/internal/diff"
	"github.com/evidary/context-ci/internal/manifest"
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
  context-ci init [flags]         write a starter .context-ci.yaml for this checkout
  context-ci manifest [flags]     write %s for this checkout
  context-ci diff <base> <head>   semantic diff between two manifests
  context-ci check [flags]        diff the baseline against the working tree and gate
  context-ci attribute [flags]    list the changed classes a regression is consistent with
  context-ci classes              list the diff classes and their defaults

  context-ci module-manifest      write the platform's module-manifest.v1.json
  context-ci serve [--addr]       GET /healthz and GET /readyz, so the manifest's health_path is real
  context-ci version              print the tool version

Flags may be written before or after a verb's arguments.
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
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "manifest":
		return runManifest(args[1:], stdout, stderr)
	case "module-manifest":
		return runModuleManifest(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "attribute":
		return attributeCommand(args[1:], stdout, stderr)
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

// version is set at build time by the release workflow. It is empty in every other build, and
// filled in from the module version the toolchain recorded, so a binary installed with
// `go install github.com/evidary/context-ci/cmd/context-ci@v0.4.1` reports that version instead of
// "dev". A tool that cannot say which version it is cannot be quoted in a bug report, and its SARIF
// results carry a tool version that means nothing to a consumer that dedupes by it.
var version = ""

func init() {
	module := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		module = info.Main.Version
	}
	version = resolveVersion(version, module)
}

// resolveVersion prefers the linked value, because the release workflow knows the version of the
// tree it tagged; otherwise the module version decides. Go stamps a build inside a checkout with
// the nearest tag, so a local build reports e.g. "0.4.0+dirty"; "(devel)" is what it records when
// there is no VCS to ask, and "dev" is the honest answer there.
func resolveVersion(linked, module string) string {
	if linked != "" {
		return linked
	}
	if module == "" || module == "(devel)" {
		return "dev"
	}
	return strings.TrimPrefix(module, "v")
}

func runManifest(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	var (
		configPath  = fs.String("config", "", "configuration file (default "+config.DefaultFile+")")
		root        = fs.String("root", ".", "repository root to capture")
		out         = fs.String("out", "", "output path (default stdout)")
		commit      = fs.String("commit", "", "commit to record (default: git HEAD)")
		sdkVer      = fs.String("sdk-version", "", "SDK version, for instrumented capture")
		captureFile = fs.String("capture", "", "a capture file written by an SDK, for instrumented capture")
	)
	positional, code, ok := parseArgs(fs, "context-ci manifest [flags]", args, stdout, stderr)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		fmt.Fprintf(stderr, "manifest: unexpected argument %q\n", positional[0])
		return ExitUsage
	}

	cfg, err := loadConfig(*configPath, *root)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitUsage
	}
	// The sources are checked before the commit is asked for. A first run in a fresh directory
	// used to fail three times in a row - no repository, then no commit, then no prompt file -
	// and the file is the one init has already said is missing, so it is reported first.
	if err := capture.CheckSources(cfg, *root); err != nil {
		fmt.Fprintf(stderr, "manifest: %v\n", err)
		return ExitManifest
	}
	resolvedCommit, err := resolveCommit(*commit, *root)
	if err != nil {
		fmt.Fprintf(stderr, "manifest: %v\n", err)
		return ExitUsage
	}

	options := capture.Options{
		Commit:     resolvedCommit,
		Now:        time.Now(),
		SDKVersion: *sdkVer,
	}
	if *captureFile != "" {
		observed, err := capture.LoadObserved(*captureFile)
		if err != nil {
			fmt.Fprintf(stderr, "manifest: %v\n", err)
			return ExitManifest
		}
		options.Observed = observed
	}
	m, err := capture.Build(cfg, *root, options)
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
	var (
		format     = fs.String("format", "text", "text, github, json, sarif or junit")
		configPath = fs.String("config", "", "configuration file, for severity overrides")
		root       = fs.String("root", ".", "repository root, for the configuration")
	)
	positional, code, ok := parseArgs(fs, "context-ci diff <base.json> <head.json> [flags]", args, stdout, stderr)
	if !ok {
		return code
	}
	if len(positional) != 2 {
		fmt.Fprintf(stderr, "diff: need a base and a head manifest, got %d argument(s)\n", len(positional))
		return ExitUsage
	}

	base, err := readManifest(positional[0])
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return ExitManifest
	}
	head, err := readManifest(positional[1])
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

// runCheck is the gate. The baseline is a manifest somebody committed; the head is, unless one is
// named, built from the working tree at the moment of the check.
//
// It used to read the head from context-manifest.json by default. A workflow that also named that
// file as its baseline - which is what the README told a team to write - compared a file with
// itself, printed "no change" and exited 0 whatever had been edited. A gate that cannot fail is
// worse than no gate, so the head is now computed, and naming one file as both sides is refused.
func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	var (
		configPath = fs.String("config", "", "configuration file (default "+config.DefaultFile+")")
		root       = fs.String("root", ".", "repository root")
		base       = fs.String("baseline", "", "base manifest (required)")
		head       = fs.String("head", "", "head manifest to read (default: built from the working tree at --root)")
		commit     = fs.String("commit", "", "commit to record in the head built from the working tree (default: git HEAD)")
		format     = fs.String("format", "text", "text, github, json, sarif or junit")
	)
	positional, code, ok := parseArgs(fs, "context-ci check --baseline <base.json> [flags]", args, stdout, stderr)
	if !ok {
		return code
	}
	if len(positional) != 0 {
		fmt.Fprintf(stderr, "check: unexpected argument %q\n", positional[0])
		return ExitUsage
	}
	if *base == "" {
		fmt.Fprintf(stderr, "check: --baseline is required; without a baseline there is nothing to compare\n")
		return ExitUsage
	}
	if *head != "" && *commit != "" {
		fmt.Fprintf(stderr, "check: --commit names the commit of a head built from the working tree, "+
			"and --head names a manifest that already records one; pass one or the other\n")
		return ExitUsage
	}
	if *head != "" && sameFile(*base, *head) {
		fmt.Fprintf(stderr, "check: --baseline and --head are the same file (%s), so the comparison could only "+
			"ever say nothing changed; omit --head to compare the baseline against the working tree, "+
			"or name a different head manifest\n", *base)
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
	var headManifest *manifest.Manifest
	if *head != "" {
		headManifest, err = readManifest(*head)
		if err != nil {
			fmt.Fprintf(stderr, "check: head: %v\n", err)
			return ExitManifest
		}
	} else {
		if err := capture.CheckSources(cfg, *root); err != nil {
			fmt.Fprintf(stderr, "check: head: %v\n", err)
			return ExitManifest
		}
		resolvedCommit, err := resolveCommit(*commit, *root)
		if err != nil {
			fmt.Fprintf(stderr, "check: %v\n", err)
			return ExitUsage
		}
		headManifest, err = capture.Build(cfg, *root, capture.Options{Commit: resolvedCommit, Now: time.Now()})
		if err != nil {
			fmt.Fprintf(stderr, "check: head: %v\n", err)
			return ExitManifest
		}
		// On stderr, so every --format stays exactly the report and nothing else.
		fmt.Fprintf(stderr, "context-ci: head built from the working tree at %s (digest %s)\n", *root, short(headManifest.Digest))
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

// sameFile reports whether two paths name one file. Comparing the strings is not enough: ./a.json
// and a.json, a symbolic link and its target, and two spellings on a case-insensitive filesystem
// are all one file.
func sameFile(a, b string) bool {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB)
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
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
	case "sarif":
		return writeSARIF(w, r, version)
	case "junit":
		return writeJUnit(w, r)
	case "text", "":
		writeText(w, r)
		return nil
	}
	return fmt.Errorf("unknown format %q, want text, github, json, sarif or junit", format)
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
	// A field this reader does not know is outside the digest: it is dropped before the digest is
	// recomputed, so the file would verify while carrying content nothing checked. The manifest
	// schema has no extension point, so an unknown field is refused rather than ignored - the
	// same rule the configuration and the eval results already follow.
	dec.DisallowUnknownFields()
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

// resolveCommit returns the commit a manifest records: the one the caller named, or git HEAD.
//
// git is asked two questions, so the answer says which thing is missing. The old single
// `git rev-parse HEAD` printed git's own "fatal: not a git repository" in a fresh directory and
// "unknown revision" one step later, and neither told a first-time user what to do next.
func resolveCommit(flagValue, root string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	const remedy = "a manifest records the commit it describes, so that it can be reproduced: " +
		"run this inside a git repository that has at least one commit, or pass --commit <id>"
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("there is no commit to record, because git is not installed or not on PATH; %s", remedy)
	}
	if _, detail, err := git(root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", fmt.Errorf("there is no commit to record, because %s is not a git repository git can read (%s); %s",
			root, detail, remedy)
	}
	head, _, err := git(root, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil || head == "" {
		return "", fmt.Errorf("there is no commit to record, because the git repository at %s has no commits yet; %s",
			root, remedy)
	}
	return head, nil
}

// git runs one git command without a network call. It returns stdout, and the first line of what
// git said on stderr so a failure can quote it.
func git(root string, args ...string) (string, string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	detail := strings.TrimSpace(errOut.String())
	if i := strings.IndexByte(detail, '\n'); i >= 0 {
		detail = strings.TrimSpace(detail[:i])
	}
	if detail == "" && err != nil {
		detail = err.Error()
	}
	return strings.TrimSpace(out.String()), detail, err
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
