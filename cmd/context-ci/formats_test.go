package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/evidary/context-ci/internal/diff"
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
