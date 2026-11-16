package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"

	"github.com/evidary/context-ci/internal/attribute"
	"github.com/evidary/context-ci/internal/diff"
)

// sarifVersion is the schema this output targets. Pinned rather than floating: a consumer parses
// one version, and "latest" is not a version.
const sarifVersion = "2.1.0"

// writeSARIF renders the diff as SARIF 2.1.0, which is what GitHub code scanning and most
// security dashboards read. One rule per context class, one result per changed class.
func writeSARIF(w io.Writer, r *diff.Result, toolVersion string) error {
	type rule struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		ShortDescription struct {
			Text string `json:"text"`
		} `json:"shortDescription"`
		DefaultConfiguration struct {
			Level string `json:"level"`
		} `json:"defaultConfiguration"`
	}
	type result struct {
		RuleID  string `json:"ruleId"`
		Level   string `json:"level"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
		Properties map[string]string `json:"properties,omitempty"`
	}
	document := struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name           string `json:"name"`
					Version        string `json:"version"`
					InformationURI string `json:"informationUri"`
					Rules          []rule `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []result `json:"results"`
		} `json:"runs"`
	}{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: sarifVersion,
	}

	var run = struct {
		Tool struct {
			Driver struct {
				Name           string `json:"name"`
				Version        string `json:"version"`
				InformationURI string `json:"informationUri"`
				Rules          []rule `json:"rules"`
			} `json:"driver"`
		} `json:"tool"`
		Results []result `json:"results"`
	}{}
	run.Tool.Driver.Name = "context-ci"
	run.Tool.Driver.Version = toolVersion
	run.Tool.Driver.InformationURI = "https://github.com/evidary/context-ci"
	// Empty, not nil: a nil slice marshals as `null`, and the SARIF schema types both as arrays.
	// A diff with no change is the common case for a gate, and it must still be a valid report.
	run.Tool.Driver.Rules = []rule{}
	run.Results = []result{}

	seen := map[diff.Class]bool{}
	for _, entry := range r.Entries {
		if !seen[entry.Class] {
			seen[entry.Class] = true
			one := rule{ID: string(entry.Class), Name: string(entry.Class)}
			one.ShortDescription.Text = entry.Summary
			one.DefaultConfiguration.Level = sarifLevel(entry.Severity)
			run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, one)
		}
		item := result{RuleID: string(entry.Class), Level: sarifLevel(entry.Severity)}
		item.Message.Text = entry.Summary
		properties := map[string]string{}
		if entry.Before != "" {
			properties["before"] = entry.Before
		}
		if entry.After != "" {
			properties["after"] = entry.After
		}
		if len(properties) > 0 {
			item.Properties = properties
		}
		run.Results = append(run.Results, item)
	}
	// Rules are sorted so the same diff renders byte-identically, which is what lets a report be committed or diffed.
	sort.SliceStable(run.Tool.Driver.Rules, func(i, j int) bool {
		return run.Tool.Driver.Rules[i].ID < run.Tool.Driver.Rules[j].ID
	})
	document.Runs = append(document.Runs, run)
	return encodeJSON(w, document)
}

// sarifLevel maps our three severities onto SARIF's four.
func sarifLevel(s diff.Severity) string {
	switch s {
	case diff.SeverityFail:
		return "error"
	case diff.SeverityWarn:
		return "warning"
	default:
		return "note"
	}
}

// writeJUnit renders the diff as JUnit XML: one test case per changed class. Every CI can read it,
// and a reviewer who sees ten red cases for ten changed classes understands the shape at once.
//
// JUnit has no warning level, so a warn-severity change passes and carries its message in
// system-out rather than being dressed up as a failure. Calling a warning a failure is how a team
// learns to ignore red.
func writeJUnit(w io.Writer, r *diff.Result) error {
	type failure struct {
		Message string `xml:"message,attr"`
		Body    string `xml:",chardata"`
	}
	type testCase struct {
		Name      string   `xml:"name,attr"`
		ClassName string   `xml:"classname,attr"`
		Failure   *failure `xml:"failure,omitempty"`
		Skipped   *struct {
			Message string `xml:"message,attr,omitempty"`
		} `xml:"skipped,omitempty"`
		SystemOut *struct {
			Body string `xml:",chardata"`
		} `xml:"system-out,omitempty"`
	}
	type suite struct {
		XMLName  xml.Name `xml:"testsuite"`
		Name     string   `xml:"name,attr"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		// An error is a report that could not be produced; a change is never one. The attribute is
		// required by common JUnit schemas, so it is stated as zero rather than omitted.
		Errors  int        `xml:"errors,attr"`
		Skipped int        `xml:"skipped,attr"`
		Cases   []testCase `xml:"testcase"`
	}

	entries := append([]diff.Entry(nil), r.Entries...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Class < entries[j].Class })

	out := suite{Name: "context-ci", Tests: len(entries)}
	for _, entry := range entries {
		item := testCase{Name: string(entry.Class), ClassName: "context-ci"}
		message := entry.Summary
		if entry.Before != "" || entry.After != "" {
			message = fmt.Sprintf("%s (%s -> %s)", message, orDash(entry.Before), orDash(entry.After))
		}
		switch entry.Severity {
		case diff.SeverityFail:
			item.Failure = &failure{Message: message, Body: entry.Detail}
			out.Failures++
		case diff.SeverityWarn:
			item.SystemOut = &struct {
				Body string `xml:",chardata"`
			}{Body: "warning: " + message}
		default:
			item.Skipped = &struct {
				Message string `xml:"message,attr,omitempty"`
			}{Message: "severity is ignore: " + message}
			out.Skipped++
		}
		out.Cases = append(out.Cases, item)
	}
	fmt.Fprint(w, xml.Header)
	return xml.NewEncoder(w).Encode(out)
}

// writeGitHubAttribution emits workflow commands, so an attribution lands on the pull request
// beside the diff instead of only in a job log.
//
// The level states how strong the claim is, not whether the build failed: `attribute` exits 0
// whatever it finds, because gating is check's job. A class that a regressed case declares a
// dependency on is an error, a regressed case is a warning, and a changed class that nothing
// points at is a notice — printed with zero rather than omitted, because silence reads as
// innocence.
//
// No annotation carries file= or line=. The report names eval cases and context classes, and
// neither records where it lives; a location here would be invented, and an annotation pointing at
// an unrelated line is worse than one pointing at nothing.
func writeGitHubAttribution(w io.Writer, report attribute.Report) {
	summary := fmt.Sprintf("%s: %s: %d compared, %d regressed, %d improved, %d unchanged, %d incomparable",
		report.Suite, report.Mode, report.Compared, report.Regressed, report.Improved,
		report.Unchanged, report.Incomparable)
	// The confounding statement rides on the summary, which is the one annotation a reader sees
	// before the findings: an error-level class below it must not be read as a proven cause.
	if caveat := report.Caveat(); caveat != "" {
		summary += ". " + caveat
	}
	fmt.Fprintf(w, "::notice title=%s::%s\n", escapeProperty("context-ci attribute"), escapeData(summary))

	for _, implicated := range report.Implicated {
		level, message := "notice", implicated.Summary
		switch {
		case implicated.Regressed > 0 && report.Mode == attribute.ModeAttribution:
			level = "error"
			message = fmt.Sprintf("%d of %d regressed cases (%.0f%%) declare it: %s",
				implicated.Regressed, report.Regressed, implicated.Share*100, implicated.Summary)
		case implicated.Regressed > 0:
			level = "warning"
			message = fmt.Sprintf("%d of %d regressed cases (%.0f%%) are consistent with it: %s",
				implicated.Regressed, report.Regressed, implicated.Share*100, implicated.Summary)
		case report.Regressed > 0:
			message = implicated.Summary + "; no regressed case declares it"
		default:
			message = implicated.Summary + "; nothing regressed"
		}
		fmt.Fprintf(w, "::%s title=%s::%s\n", level,
			escapeProperty("context-ci attribute "+implicated.Class), escapeData(message))
	}

	for _, id := range report.Regressions {
		fmt.Fprintf(w, "::warning title=%s::regressed by more than %v\n",
			escapeProperty("context-ci attribute case "+id), attribute.Threshold)
	}
}

// encodeJSON writes a document the way the rest of the tool does: indented, so a human reading a
// failing pipeline can see what changed.
func encodeJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
