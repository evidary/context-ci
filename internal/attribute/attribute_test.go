package attribute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func yes() *bool               { v := true; return &v }
func no() *bool                { v := false; return &v }
func score(v float64) *float64 { s := v; return &s }

func changes() []ClassChange {
	return []ClassChange{
		{Class: "retrieval.top_k", Summary: "the retrieval depth changed (5 -> 50)"},
		{Class: "prompt.content", Summary: "the prompt body changed"},
	}
}

// TestDeclaredDependenciesGiveAttribution is the differentiator: the cases say what they depend
// on, so a regression can be pinned to a class rather than merely noticed alongside it.
func TestDeclaredDependenciesGiveAttribution(t *testing.T) {
	base := Results{Version: 1, Suite: "support", Cases: []Case{
		{ID: "a", Passed: yes(), DependsOn: []string{"retrieval.top_k"}},
		{ID: "b", Passed: yes(), DependsOn: []string{"retrieval.top_k"}},
		{ID: "c", Passed: yes(), DependsOn: []string{"prompt.content"}},
		{ID: "d", Passed: yes(), DependsOn: []string{"retrieval.top_k"}},
	}}
	head := Results{Version: 1, Suite: "support", Cases: []Case{
		{ID: "a", Passed: no(), DependsOn: []string{"retrieval.top_k"}},
		{ID: "b", Passed: no(), DependsOn: []string{"retrieval.top_k"}},
		{ID: "c", Passed: yes(), DependsOn: []string{"prompt.content"}},
		{ID: "d", Passed: yes(), DependsOn: []string{"retrieval.top_k"}},
	}}
	report := Attribute(base, head, changes())
	if report.Mode != ModeAttribution {
		t.Fatalf("mode %q", report.Mode)
	}
	if report.Compared != 4 || report.Regressed != 2 || report.Unchanged != 2 {
		t.Fatalf("counts %+v", report)
	}
	if len(report.Implicated) != 2 {
		t.Fatalf("implicated %+v", report.Implicated)
	}
	top := report.Implicated[0]
	if top.Class != "retrieval.top_k" || top.Regressed != 2 || top.Share != 1 {
		t.Fatalf("top implication %+v", top)
	}
	if report.Implicated[1].Class != "prompt.content" || report.Implicated[1].Regressed != 0 {
		t.Fatalf("a class with no evidence should be reported with zero, not dropped: %+v", report.Implicated[1])
	}
	if !strings.Contains(report.Explain(), "implicated by evidence") {
		t.Fatalf("explain %q", report.Explain())
	}
}

// TestWithoutDeclarationsItSaysCorrelation is the honesty clause. The same numbers, no
// dependencies, and the tool must not claim what it cannot know.
func TestWithoutDeclarationsItSaysCorrelation(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0.9)}, {ID: "b", Score: score(0.8)}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0.4)}, {ID: "b", Score: score(0.8)}}}
	report := Attribute(base, head, changes())
	if report.Mode != ModeCorrelation {
		t.Fatalf("mode %q", report.Mode)
	}
	if report.Implicated[0].Regressed != 0 {
		t.Fatalf("a correlation implicated a class: %+v", report.Implicated[0])
	}
	if !strings.Contains(report.Explain(), "correlation") {
		t.Fatalf("explain %q", report.Explain())
	}
}

func TestNoMovementIsNotAnAttribution(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0.5), DependsOn: []string{"retrieval.top_k"}}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0.5000001), DependsOn: []string{"retrieval.top_k"}}}}
	report := Attribute(base, head, changes())
	if report.Mode != ModeNoChange {
		t.Fatalf("mode %q", report.Mode)
	}
	if report.Regressed != 0 || report.Unchanged != 1 {
		t.Fatalf("counts %+v", report)
	}
	if !strings.Contains(report.Explain(), "nothing to attribute") {
		t.Fatalf("explain %q", report.Explain())
	}
}

func TestImprovementsAreReportedNotImplicated(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{{ID: "a", Passed: no(), DependsOn: []string{"retrieval.top_k"}}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Passed: yes(), DependsOn: []string{"retrieval.top_k"}}}}
	report := Attribute(base, head, changes())
	if report.Improved != 1 || report.Regressed != 0 {
		t.Fatalf("counts %+v", report)
	}
	if len(report.Implicated) != 2 || report.Implicated[0].Regressed != 0 {
		t.Fatalf("an improvement implicated a class: %+v", report.Implicated)
	}
	if len(report.Improvements) != 1 || report.Improvements[0] != "a" {
		t.Fatalf("improvements %v", report.Improvements)
	}
}

func TestCasesThatVanishOrChangeMeasureAreIncomparable(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{
		{ID: "gone", Passed: yes()},
		{ID: "switched", Passed: yes()},
	}}
	head := Results{Version: 1, Cases: []Case{
		{ID: "switched", Score: score(0.1)},
		{ID: "new", Passed: yes()},
	}}
	report := Attribute(base, head, changes())
	if report.Compared != 0 {
		t.Fatalf("compared %d", report.Compared)
	}
	if report.Incomparable != 3 {
		t.Fatalf("incomparable %d, want 3 (one gone, one switched, one new)", report.Incomparable)
	}
	if report.Mode != ModeNoChange {
		t.Fatalf("mode %q", report.Mode)
	}
}

// TestAZeroScoreIsAResultAndAnAbsentOneIsNot: the difference between "scored zero" and "not
// measured" is the difference between a regression and silence.
func TestAZeroScoreIsAResultAndAnAbsentOneIsNot(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0.8)}, {ID: "b", Score: score(0.8)}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Score: score(0)}, {ID: "b"}}}
	report := Attribute(base, head, changes())
	if report.Regressed != 1 || report.Regressions[0] != "a" {
		t.Fatalf("a zero score was not counted as a regression: %+v", report)
	}
	if report.Incomparable != 1 {
		t.Fatalf("a missing score was counted as a result: %+v", report)
	}
}

func TestLoadRefusesWhatWouldSilentlyChangeTheClaim(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	good := write("good.json", `{"version":1,"suite":"s","cases":[{"id":"a","score":0.5}]}`)
	if _, err := Load(good); err != nil {
		t.Fatalf("a good file was refused: %v", err)
	}
	for name, body := range map[string]string{
		"unknown field":  `{"version":1,"cases":[{"id":"a","score":0.5,"depends-on":["x"]}]}`,
		"no cases":       `{"version":1,"cases":[]}`,
		"no id":          `{"version":1,"cases":[{"score":0.5}]}`,
		"duplicate id":   `{"version":1,"cases":[{"id":"a"},{"id":"a"}]}`,
		"score too high": `{"version":1,"cases":[{"id":"a","score":1.5}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(strings.ReplaceAll(name, " ", "-")+".json", body)); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

func TestAnEmptyDiffCannotImplicateAnything(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{{ID: "a", Passed: yes(), DependsOn: []string{"retrieval.top_k"}}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Passed: no(), DependsOn: []string{"retrieval.top_k"}}}}
	report := Attribute(base, head, nil)
	if len(report.Implicated) != 0 {
		t.Fatalf("nothing changed but something was implicated: %+v", report.Implicated)
	}
	if report.Mode != ModeAttribution {
		t.Fatalf("mode %q", report.Mode)
	}
	if report.Regressed != 1 {
		t.Fatalf("regressed %d", report.Regressed)
	}
}
