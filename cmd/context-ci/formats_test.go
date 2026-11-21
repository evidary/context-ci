package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sigilary/context-ci/internal/attribute"
	"github.com/sigilary/context-ci/internal/diff"
)

func sampleResult() *diff.Result {
	return &diff.Result{
		Entries: []diff.Entry{
			{Class: "retrieval.top_k", Severity: diff.SeverityFail, Summary: "the retrieval depth changed", Before: "5", After: "50"},
			{Class: "prompt.template_hash", Severity: diff.SeverityWarn, Summary: "the prompt template changed"},
			{Class: "model.params", Severity: diff.SeverityIgnore, Summary: "sampling settings changed"},
		},
	}
}

func TestSARIFIsWellFormedAndPinsItsVersion(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeSARIF(&buffer, sampleResult(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Rules   []struct {
						ID    string `json:"id"`
						Level string `json:"level"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Properties map[string]string `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &document); err != nil {
		t.Fatalf("the output is not JSON: %v", err)
	}
	if document.Version != "2.1.0" {
		t.Fatalf("version %q", document.Version)
	}
	if !strings.Contains(document.Schema, "sarif-2.1.0") {
		t.Fatalf("schema %q", document.Schema)
	}
	if len(document.Runs) != 1 {
		t.Fatalf("%d runs", len(document.Runs))
	}
	run := document.Runs[0]
	if run.Tool.Driver.Name != "context-ci" || run.Tool.Driver.Version != "0.4.0" {
		t.Fatalf("tool %+v", run.Tool.Driver)
	}
	if len(run.Results) != 3 {
		t.Fatalf("%d results", len(run.Results))
	}
	// Every result must name a rule that was declared, or a SARIF consumer drops it.
	declared := map[string]bool{}
	for _, rule := range run.Tool.Driver.Rules {
		declared[rule.ID] = true
	}
	for _, result := range run.Results {
		if !declared[result.RuleID] {
			t.Fatalf("result names undeclared rule %q", result.RuleID)
		}
	}
	levels := map[string]string{}
	for _, result := range run.Results {
		levels[result.RuleID] = result.Level
	}
	if levels["retrieval.top_k"] != "error" || levels["prompt.template_hash"] != "warning" || levels["model.params"] != "note" {
		t.Fatalf("levels %v", levels)
	}
	for _, result := range run.Results {
		if result.RuleID == "retrieval.top_k" {
			if result.Properties["before"] != "5" || result.Properties["after"] != "50" {
				t.Fatalf("properties %v", result.Properties)
			}
			if result.Message.Text == "" {
				t.Fatal("a result with no message")
			}
		}
	}
}

func TestSARIFIsDeterministic(t *testing.T) {
	first, second := new(bytes.Buffer), new(bytes.Buffer)
	if err := writeSARIF(first, sampleResult(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if err := writeSARIF(second, sampleResult(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatal("two renders of the same result differ")
	}
}

// TestJUnitReflectsSeverityWithoutInflatingIt: JUnit has no warning level, so a warning must not
// be dressed up as a failure — calling warnings failures is how a team learns to ignore red.
func TestJUnitReflectsSeverityWithoutInflatingIt(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeJUnit(&buffer, sampleResult()); err != nil {
		t.Fatal(err)
	}
	var suite struct {
		XMLName  xml.Name `xml:"testsuite"`
		Name     string   `xml:"name,attr"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Errors   int      `xml:"errors,attr"`
		Skipped  int      `xml:"skipped,attr"`
		Cases    []struct {
			Name    string `xml:"name,attr"`
			Failure *struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
			Skipped *struct {
				Message string `xml:"message,attr"`
			} `xml:"skipped"`
			SystemOut *string `xml:"system-out"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(buffer.Bytes(), &suite); err != nil {
		t.Fatalf("the output is not XML: %v\n%s", err, buffer.String())
	}
	if suite.Tests != 3 || suite.Failures != 1 || suite.Skipped != 1 {
		t.Fatalf("counts %+v", suite)
	}
	// junit-10.xsd requires errors on testsuite; a change is never an error, so it must be stated
	// as zero rather than left out.
	if suite.Errors != 0 || !strings.Contains(buffer.String(), `errors="0"`) {
		t.Fatalf("errors attribute missing or non-zero: %s", buffer.String())
	}
	for _, testCase := range suite.Cases {
		switch testCase.Name {
		case "retrieval.top_k":
			if testCase.Failure == nil {
				t.Fatal("a fail-severity change did not fail its case")
			}
			if !strings.Contains(testCase.Failure.Message, "5 -> 50") {
				t.Fatalf("message %q", testCase.Failure.Message)
			}
		case "prompt.template_hash":
			if testCase.Failure != nil {
				t.Fatal("a warning was reported as a failure")
			}
			if testCase.SystemOut == nil || !strings.Contains(*testCase.SystemOut, "warning:") {
				t.Fatalf("a warning carried no message: %+v", testCase.SystemOut)
			}
		case "model.params":
			if testCase.Skipped == nil {
				t.Fatal("an ignored class was not skipped")
			}
		}
	}
	if !strings.HasPrefix(buffer.String(), "<?xml") {
		t.Fatal("no XML declaration")
	}
}

// TestGitHubAttributionPutsTheClaimOnTheLine pins the annotation lines: one per finding, the
// strength of the claim in the level, and the counts in the summary a reader sees first.
func TestGitHubAttributionPutsTheClaimOnTheLine(t *testing.T) {
	report := attribute.Report{
		Suite:    "support-quality",
		Mode:     attribute.ModeAttribution,
		Changed:  []string{"prompt.rendered_hash", "model.id"},
		Compared: 4, Regressed: 2, Unchanged: 2,
		Implicated: []attribute.Implication{
			{Class: "prompt.rendered_hash", Summary: "the rendered prompt changed", Regressed: 2, Share: 1},
			{Class: "model.id", Summary: "the model changed"},
		},
		Regressions: []string{"q1", "q3"},
	}

	var buffer bytes.Buffer
	writeGitHubAttribution(&buffer, report)

	want := []string{
		"::notice title=context-ci attribute::support-quality: attribution: 4 compared, 2 regressed, 0 improved, 2 unchanged, 0 incomparable",
		"::error title=context-ci attribute prompt.rendered_hash::2 of 2 regressed cases (100%25) declare it: the rendered prompt changed",
		"::notice title=context-ci attribute model.id::the model changed; no regressed case declares it",
		"::warning title=context-ci attribute case q1::regressed by more than 0.001",
		"::warning title=context-ci attribute case q3::regressed by more than 0.001",
	}
	got := strings.Split(strings.TrimSuffix(buffer.String(), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d:\n%s", len(got), len(want), buffer.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// TestGitHubAttributionNeverDressesCorrelationAsEvidence: with no depends_on declarations nothing
// points at a class, so an error-level annotation would claim an attribution the report does not
// make. The regressed cases are still named, because they are the only evidence there is.
func TestGitHubAttributionNeverDressesCorrelationAsEvidence(t *testing.T) {
	report := attribute.Report{
		Suite:    "support-quality",
		Mode:     attribute.ModeCorrelation,
		Changed:  []string{"model.id"},
		Compared: 3, Regressed: 1, Unchanged: 2,
		Implicated:  []attribute.Implication{{Class: "model.id", Summary: "the model changed"}},
		Regressions: []string{"q2"},
	}

	var buffer bytes.Buffer
	writeGitHubAttribution(&buffer, report)
	out := buffer.String()
	if strings.Contains(out, "::error") {
		t.Fatalf("a correlation produced an error annotation:\n%s", out)
	}
	for _, want := range []string{"correlation: 3 compared, 1 regressed", "model.id", "case q2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output does not mention %q:\n%s", want, out)
		}
	}
}

// TestGitHubAttributionEscapesAndNamesNoLocation: a summary comes from a manifest, so it must not
// be able to open a second workflow command, and no annotation may carry a file or a line the
// report has no basis for.
func TestGitHubAttributionEscapesAndNamesNoLocation(t *testing.T) {
	report := attribute.Report{
		Suite:    "support-quality",
		Mode:     attribute.ModeAttribution,
		Changed:  []string{"prompt.template_hash"},
		Compared: 1, Regressed: 1,
		Implicated: []attribute.Implication{{
			Class:     "prompt.template_hash",
			Summary:   "100% of: the, template\nwas rewritten",
			Regressed: 1,
			Share:     1,
		}},
		Regressions: []string{"q1"},
	}

	var buffer bytes.Buffer
	writeGitHubAttribution(&buffer, report)
	out := buffer.String()

	if strings.Contains(out, "file=") || strings.Contains(out, "line=") {
		t.Fatalf("an annotation claims a location the report does not have:\n%s", out)
	}
	if !strings.Contains(out, "100%25 of: the, template%0Awas rewritten") {
		t.Fatalf("the summary was not escaped for a message body:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.Count(line, "::") > 2 {
			t.Fatalf("a line looks like a second command was injected: %q", line)
		}
	}
}

// TestNoChangeIsAValidReportInEveryFormat is the same guarantee through the CLI, with two real
// manifests: no format may print a null where its schema wants a list.
func TestNoChangeIsAValidReportInEveryFormat(t *testing.T) {
	root := repo(t, "gpt-x")
	base := manifestFor(t, root, "base.json")
	head := manifestFor(t, root, "head.json")
	for _, format := range []string{"sarif", "json", "junit", "github", "text"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{"diff", base, head, "--format", format}, &stdout, &stderr); code != ExitOK {
				t.Fatalf("exit = %d: %s", code, stderr.String())
			}
			if stdout.Len() == 0 || strings.Contains(stdout.String(), "null") {
				t.Fatalf("--format %s for an unchanged context:\n%s", format, stdout.String())
			}
		})
	}
}

// TestSARIFResultsCarryNoLocation pins what the CLI reference now says. A manifest holds hashes,
// not the file and line that produced them, so a result has no location to give; if one is ever
// added, the reference's "no locations" sentence has to change with it.
func TestSARIFResultsCarryNoLocation(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeSARIF(&buffer, sampleResult(), "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buffer.String(), "locations") {
		t.Fatalf("a SARIF result carries a location; update cli-reference.md §7:\n%s", buffer.String())
	}
}

func TestFormattingAnEmptyDiffIsValidOutput(t *testing.T) {
	empty := &diff.Result{}
	var sarif, junit bytes.Buffer
	if err := writeSARIF(&sarif, empty, "0.4.0"); err != nil {
		t.Fatal(err)
	}
	if err := writeJUnit(&junit, empty); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(sarif.Bytes(), &document); err != nil {
		t.Fatalf("SARIF for an empty diff is not JSON: %v", err)
	}
	// Well-formed JSON is not enough. The SARIF schema types rules and results as arrays, and a
	// nil slice marshals as null: the report for "nothing changed" - the one a gate writes on
	// every green build - was rejected by a validating consumer.
	if strings.Contains(sarif.String(), "null") {
		t.Fatalf("SARIF for an empty diff carries a null:\n%s", sarif.String())
	}
	sarifRun := document["runs"].([]any)[0].(map[string]any)
	if results, ok := sarifRun["results"].([]any); !ok || len(results) != 0 {
		t.Fatalf("results = %#v, want an empty array", sarifRun["results"])
	}
	driver := sarifRun["tool"].(map[string]any)["driver"].(map[string]any)
	if rules, ok := driver["rules"].([]any); !ok || len(rules) != 0 {
		t.Fatalf("rules = %#v, want an empty array", driver["rules"])
	}
	for _, want := range []string{`"rules": []`, `"results": []`} {
		if !strings.Contains(sarif.String(), want) {
			t.Fatalf("SARIF for an empty diff does not contain %s:\n%s", want, sarif.String())
		}
	}
	// JUnit has no such trap - an empty suite is an element with no children - but it is pinned
	// so that it stays a complete, countable document.
	if !strings.Contains(junit.String(), `<testsuite name="context-ci" tests="0" failures="0" errors="0" skipped="0">`) {
		t.Fatalf("JUnit for an empty diff is not an empty suite:\n%s", junit.String())
	}
	var suite struct {
		Tests int `xml:"tests,attr"`
	}
	if err := xml.Unmarshal(junit.Bytes(), &suite); err != nil {
		t.Fatalf("JUnit for an empty diff is not XML: %v", err)
	}
	if suite.Tests != 0 {
		t.Fatalf("tests %d", suite.Tests)
	}
}
