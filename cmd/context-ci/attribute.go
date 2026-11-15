package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/evidary/context-ci/internal/attribute"
	"github.com/evidary/context-ci/internal/config"
	"github.com/evidary/context-ci/internal/diff"
)

// attributeCommand implements the fourth verb: given two manifests and two eval runs, say which
// context change the movement points at, and say plainly when it cannot.
func attributeCommand(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("attribute", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath = fs.String("config", "", "configuration file (default "+config.DefaultFile+")")
		root       = fs.String("root", ".", "repository root")
		baseline   = fs.String("baseline", "", "base manifest (required)")
		head       = fs.String("head", ManifestFileName, "head manifest")
		baseResult = fs.String("baseline-results", "", "eval results for the baseline (required)")
		results    = fs.String("results", "", "eval results for the head (required)")
		format     = fs.String("format", "text", "text, github or json")
	)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "attribute: unexpected argument %q\n", fs.Arg(0))
		return ExitUsage
	}
	for name, value := range map[string]string{
		"--baseline":         *baseline,
		"--head":             *head,
		"--baseline-results": *baseResult,
		"--results":          *results,
	} {
		if value == "" {
			fmt.Fprintf(stderr, "attribute: %s is required\n", name)
			return ExitUsage
		}
	}

	cfg, err := loadConfig(*configPath, *root)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitUsage
	}
	baseManifest, err := readManifest(*baseline)
	if err != nil {
		fmt.Fprintf(stderr, "attribute: baseline manifest: %v\n", err)
		return ExitManifest
	}
	headManifest, err := readManifest(*head)
	if err != nil {
		fmt.Fprintf(stderr, "attribute: head manifest: %v\n", err)
		return ExitManifest
	}
	baseResults, err := attribute.Load(*baseResult)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitManifest
	}
	headResults, err := attribute.Load(*results)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return ExitManifest
	}

	compared := diff.Compare(baseManifest, headManifest, diff.Options{
		Severity:        cfg.SeverityOverrides(),
		CallsPerDay:     cfg.Gate.CallsPerDay,
		InputUSDPerMTok: cfg.Gate.InputUSDPerMTok,
	})
	changes := make([]attribute.ClassChange, 0, len(compared.Entries))
	for _, entry := range compared.Entries {
		changes = append(changes, attribute.ClassChange{Class: string(entry.Class), Summary: entry.Summary})
	}
	report := attribute.Attribute(baseResults, headResults, changes)

	switch *format {
	case "json":
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "attribute: %v\n", err)
			return ExitManifest
		}
		fmt.Fprintln(stdout, string(encoded))
	case "github":
		writeGitHubAttribution(stdout, report)
	case "text", "":
		printAttribution(stdout, report)
	default:
		fmt.Fprintf(stderr, "attribute: unknown format %q; use text, github or json\n", *format)
		return ExitUsage
	}
	return ExitOK
}

// printAttribution leads with the qualifier, not the finding: a reader who skims must not be
// able to take a correlation for an attribution.
func printAttribution(w io.Writer, report attribute.Report) {
	fmt.Fprintf(w, "%s: %d compared, %d regressed, %d improved, %d unchanged, %d incomparable\n",
		report.Suite, report.Compared, report.Regressed, report.Improved, report.Unchanged, report.Incomparable)
	fmt.Fprintln(w, report.Explain())
	if len(report.Changed) == 0 {
		fmt.Fprintln(w, "no context class changed, so nothing can be implicated")
		return
	}
	fmt.Fprintf(w, "\nchanged classes: %s\n", report.ClassNames())
	if len(report.Implicated) > 0 {
		fmt.Fprintln(w, "")
		for _, implication := range report.Implicated {
			fmt.Fprintf(w, "  %-24s %d of %d regressed cases (%.0f%%)  %s\n",
				implication.Class, implication.Regressed, report.Regressed, implication.Share*100, implication.Summary)
		}
	}
	if len(report.Regressions) > 0 {
		fmt.Fprintf(w, "\nregressed: %v\n", report.Regressions)
	}
	if len(report.Improvements) > 0 {
		fmt.Fprintf(w, "improved:  %v\n", report.Improvements)
	}
	if report.Undeclared > 0 {
		fmt.Fprintf(w, "\n%d case(s) declare no depends_on, so no evidence points at a class for them\n", report.Undeclared)
	}
}
