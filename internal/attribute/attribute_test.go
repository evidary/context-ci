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
	// Specification §6 rule 3: the output is "consistent with", never "caused by".
	explain := report.Explain()
	if !strings.Contains(explain, "candidate causes consistent with") || !strings.Contains(explain, "not proven causes") {
		t.Fatalf("explain %q", explain)
	}
	if !strings.Contains(explain, "every regressed case declares its dependencies") {
		t.Fatalf("both regressed cases declare, and the sentence does not say so: %q", explain)
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

// TestADeclarationOnAnUnregressedCaseIsNotAnAttribution is the false sentence this package used
// to print. One unchanged case declared a dependency, the case that regressed declared nothing,
// and the report said "every one of them declared dependencies" and called itself an attribution.
func TestADeclarationOnAnUnregressedCaseIsNotAnAttribution(t *testing.T) {
	base := Results{Version: 1, Cases: []Case{
		{ID: "steady", Score: score(0.9), DependsOn: []string{"retrieval.top_k"}},
		{ID: "broke", Score: score(0.9)},
	}}
	head := Results{Version: 1, Cases: []Case{
		{ID: "steady", Score: score(0.9), DependsOn: []string{"retrieval.top_k"}},
		{ID: "broke", Score: score(0.2)},
	}}
	report := Attribute(base, head, changes())
	if report.Mode != ModeCorrelation {
		t.Fatalf("mode %q, want %q: no regressed case declares anything", report.Mode, ModeCorrelation)
	}
	if report.RegressedDeclared != 0 || report.RegressedPointing != 0 {
		t.Fatalf("counts %+v", report)
	}
	explain := report.Explain()
	if strings.Contains(explain, "every") || !strings.Contains(explain, "no regressed case declares") {
		t.Fatalf("explain %q", explain)
	}
	for _, implication := range report.Implicated {
		if implication.Regressed != 0 {
			t.Fatalf("a correlation implicated %+v", implication)
		}
	}
}

// TestEverySentenceIsTrueOfItsCounts walks the sentences Explain can print and checks each
// against a run in which it holds, including the ones that used to be false.
func TestEverySentenceIsTrueOfItsCounts(t *testing.T) {
	cases := []struct {
		name       string
		base, head []Case
		changes    []ClassChange
		wantMode   string
		want       []string
		wantNot    []string
	}{
		{
			name:     "only an improvement",
			base:     []Case{{ID: "a", Score: score(0.2)}, {ID: "b", Score: score(0.5)}},
			head:     []Case{{ID: "a", Score: score(0.9)}, {ID: "b", Score: score(0.5)}},
			changes:  changes(),
			wantMode: ModeNoChange,
			want:     []string{"No case regressed", "1 improved", "no regression to attribute"},
			wantNot:  []string{"No case moved"},
		},
		{
			name:     "nothing moved at all",
			base:     []Case{{ID: "a", Score: score(0.5)}},
			head:     []Case{{ID: "a", Score: score(0.5)}},
			changes:  changes(),
			wantMode: ModeNoChange,
			want:     []string{"No case moved by more than 0.001"},
		},
		{
			name: "one regressed case declares and one does not",
			base: []Case{
				{ID: "a", Score: score(0.9), DependsOn: []string{"retrieval.top_k"}},
				{ID: "b", Score: score(0.9)},
			},
			head: []Case{
				{ID: "a", Score: score(0.1), DependsOn: []string{"retrieval.top_k"}},
				{ID: "b", Score: score(0.1)},
			},
			changes:  changes(),
			wantMode: ModeAttribution,
			want: []string{"2 of 2 cases regressed", "1 of the 2 regressed cases declare dependencies",
				"1 of them declare a dependency on a class that changed", "not proven causes"},
			wantNot: []string{"every"},
		},
		{
			name:     "the declared dependency is not what changed",
			base:     []Case{{ID: "a", Score: score(0.9), DependsOn: []string{"model.id"}}},
			head:     []Case{{ID: "a", Score: score(0.1), DependsOn: []string{"model.id"}}},
			changes:  changes(),
			wantMode: ModeAttribution,
			want: []string{"every regressed case declares its dependencies",
				"none of them declares a dependency on a class that changed", "no changed class is a candidate cause"},
			wantNot: []string{"classes below"},
		},
		{
			name:     "a regression with no context change",
			base:     []Case{{ID: "a", Score: score(0.9), DependsOn: []string{"model.id"}}},
			head:     []Case{{ID: "a", Score: score(0.1), DependsOn: []string{"model.id"}}},
			changes:  nil,
			wantMode: ModeAttribution,
			want:     []string{"no context class changed", "nothing in the context is a candidate cause"},
			wantNot:  []string{"classes below"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := Attribute(Results{Version: 1, Cases: tc.base}, Results{Version: 1, Cases: tc.head}, tc.changes)
			if report.Mode != tc.wantMode {
				t.Fatalf("mode %q, want %q", report.Mode, tc.wantMode)
			}
			explain := report.Explain()
			for _, want := range tc.want {
				if !strings.Contains(explain, want) {
					t.Errorf("explain does not say %q:\n%s", want, explain)
				}
			}
			for _, wantNot := range tc.wantNot {
				if strings.Contains(explain, wantNot) {
					t.Errorf("explain says %q, which is not true of this run:\n%s", wantNot, explain)
				}
			}
		})
	}
}

// TestTwoChangesTogetherAreStatedAsConfounded is requirement C-08, with its two-entry fixture:
// when more than one class changed the report says they cannot be separated, and with one it
// says nothing of the kind.
func TestTwoChangesTogetherAreStatedAsConfounded(t *testing.T) {
	both := []string{"retrieval.top_k", "prompt.rendered_hash"}
	base := Results{Version: 1, Cases: []Case{{ID: "a", Passed: yes(), DependsOn: both}}}
	head := Results{Version: 1, Cases: []Case{{ID: "a", Passed: no(), DependsOn: both}}}

	two := Attribute(base, head, []ClassChange{
		{Class: "retrieval.top_k", Summary: "the retrieval depth changed"},
		{Class: "prompt.rendered_hash", Summary: "the rendered prompt changed"},
	})
	if !two.Confounded {
		t.Fatal("two classes changed and the report does not call them confounded")
	}
	for _, want := range []string{"2 context classes changed together", "confounded", "not causal"} {
		if !strings.Contains(two.Caveat(), want) {
			t.Fatalf("caveat does not say %q: %q", want, two.Caveat())
		}
	}
	if two.Implicated[0].Regressed != 1 || two.Implicated[1].Regressed != 1 || two.RegressedPointing != 1 {
		t.Fatalf("one case pointing at two classes was miscounted: %+v", two)
	}

	one := Attribute(base, head, []ClassChange{{Class: "retrieval.top_k", Summary: "the retrieval depth changed"}})
	if one.Confounded || one.Caveat() != "" {
		t.Fatalf("one class changed and the report calls it confounded: %q", one.Caveat())
	}
}

// TestLoadRefusesAVersionOrAClassItDoesNotKnow: version 99 was read as version 1, and a
// depends_on naming a class that does not exist counted as a declaration that could never match.
func TestLoadRefusesAVersionOrAClassItDoesNotKnow(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct{ body, want string }{
		"a future version": {`{"version":99,"suite":"s","cases":[{"id":"a","score":0.5}]}`, "version 99 is not supported"},
		"no version":       {`{"suite":"s","cases":[{"id":"a","score":0.5}]}`, "version 0 is not supported"},
		"an unknown class": {`{"version":1,"cases":[{"id":"a","score":0.5,"depends_on":["prompt.content"]}]}`, `depends_on "prompt.content"`},
		"a misspelled one": {`{"version":1,"cases":[{"id":"a","score":0.5,"depends_on":["model.id","retrieval.topk"]}]}`, "context-ci classes"},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil {
				t.Fatalf("%s was accepted", name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("the error does not say %q: %v", tc.want, err)
			}
		})
	}
	known := filepath.Join(dir, "known.json")
	body := `{"version":1,"cases":[{"id":"a","score":0.5,"depends_on":["model.id","tools"]}]}`
	if err := os.WriteFile(known, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(known); err != nil {
		t.Fatalf("real class names were refused: %v", err)
	}
}
