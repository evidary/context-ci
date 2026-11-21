// Package attribute lists the context changes a movement in eval results is consistent with.
//
// The distinction this package exists to keep is attribution versus correlation. A case that
// declares which context classes it depends on can be attributed to: if it regressed and one of
// its declared dependencies changed, that class is a candidate cause, named by the declaration
// rather than by coincidence. A case that declares nothing can only be correlated: the score
// moved and the configuration moved, and nothing ties them together beyond the fact that both
// happened. A tool that printed the second as if it were the first would be inventing the answer
// this product exists to give.
//
// Neither is a causal claim. A declaration is the suite author's statement of what a case depends
// on, not a measurement, and when two classes changed together nothing here can separate them.
// The report says "consistent with", never "caused by", and says so in the output.
package attribute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/sigilary/context-ci/internal/diff"
)

// Version is the eval results schema version this package accepts.
const Version = 1

// Threshold is how much a score must move to count. Floats from a model do not repeat exactly,
// and a report that calls a 1e-12 wobble a regression trains its reader to ignore it.
const Threshold = 0.001

// Results is one eval run.
type Results struct {
	Version int    `json:"version"`
	Suite   string `json:"suite"`
	Cases   []Case `json:"cases"`
}

// Case is one eval case's outcome.
//
// Score and Passed are pointers so that "reported zero" and "not reported" stay different
// facts. A case that scored 0 failed; a case with no score was not measured, and counting it as
// zero would invent a regression out of silence.
type Case struct {
	ID        string   `json:"id"`
	Score     *float64 `json:"score,omitempty"`
	Passed    *bool    `json:"passed,omitempty"`
	CostUSD   string   `json:"cost_usd,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// Load reads a results file. Unknown fields are refused: a misspelled "depends_on" would
// silently turn attribution into correlation, and the reader would never know which they got.
func Load(path string) (Results, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Results{}, err
	}
	var results Results
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&results); err != nil {
		return Results{}, fmt.Errorf("attribute: %s: %w", path, err)
	}
	// A version this reader does not know may mean a different score scale or a different meaning
	// for depends_on, and reading it as version 1 would report on a file it misunderstood.
	if results.Version != Version {
		return Results{}, fmt.Errorf("attribute: %s: version %d is not supported, want %d", path, results.Version, Version)
	}
	if len(results.Cases) == 0 {
		return Results{}, fmt.Errorf("attribute: %s: no cases", path)
	}
	seen := map[string]bool{}
	for _, c := range results.Cases {
		if c.ID == "" {
			return Results{}, fmt.Errorf("attribute: %s: a case has no id", path)
		}
		if seen[c.ID] {
			return Results{}, fmt.Errorf("attribute: %s: case %q appears twice", path, c.ID)
		}
		seen[c.ID] = true
		if c.Score != nil && (*c.Score < 0 || *c.Score > 1) {
			return Results{}, fmt.Errorf("attribute: %s: case %q has score %v, want 0..1", path, c.ID, *c.Score)
		}
		// A dependency on a class that does not exist can never match a change, so a misspelled
		// name would quietly remove the case from every attribution while still counting it as
		// one that declared its dependencies.
		for _, dependency := range c.DependsOn {
			if !diff.IsKnownClass(dependency) {
				return Results{}, fmt.Errorf("attribute: %s: case %q depends_on %q, which is not a context class; "+
					"run 'context-ci classes' for the list", path, c.ID, dependency)
			}
		}
	}
	return results, nil
}

// Mode names what the report is allowed to claim.
const (
	ModeAttribution = "attribution"
	ModeCorrelation = "correlation"
	ModeNoChange    = "no-change"
)

// Implication is one changed class and the regressions that point at it.
type Implication struct {
	Class string `json:"class"`
	// Summary is the diff entry's own description of the change, not ours.
	Summary string `json:"summary"`
	// Regressed is how many regressed cases name this class among their dependencies.
	Regressed int `json:"regressed"`
	// Share is Regressed divided by the number of cases that regressed at all.
	Share float64 `json:"share"`
}

// Report is the answer.
type Report struct {
	Suite string `json:"suite"`
	// Mode is attribution, correlation or no-change. Everything below is qualified by it.
	Mode string `json:"mode"`
	// Changed lists the diff classes, in the diff's order.
	Changed []string `json:"changed"`
	// Compared is the number of cases present in both runs with a comparable measure.
	Compared int `json:"compared"`
	// Regressed, Improved and Unchanged partition Compared.
	Regressed int `json:"regressed"`
	Improved  int `json:"improved"`
	Unchanged int `json:"unchanged"`
	// Incomparable is cases missing from one run, or measured differently in each. They are
	// counted rather than dropped: a vanished case is a result too.
	Incomparable int `json:"incomparable"`
	// Undeclared is how many compared cases declare no dependencies.
	Undeclared int `json:"undeclared"`
	// RegressedDeclared is how many regressed cases declare at least one dependency, and
	// RegressedPointing how many of those name a class that changed. The sentences the report
	// prints are built from these two, because "the cases declare dependencies" is only a reason
	// to believe an attribution when the cases that regressed are the ones that declared.
	RegressedDeclared int `json:"regressed_declared"`
	RegressedPointing int `json:"regressed_pointing"`
	// Confounded is true when more than one class changed. Their effects cannot be separated by
	// two eval runs, so every class listed is a candidate and none is shown to be the cause.
	Confounded bool `json:"confounded"`
	// Implicated is one entry per changed class, ordered by share then name.
	Implicated []Implication `json:"implicated,omitempty"`
	// Regressions names the cases that got worse, so a reader can go and look.
	Regressions []string `json:"regressions,omitempty"`
	// Improvements names the cases that got better. A change that broke three cases and fixed
	// ten is not the same finding as one that broke three.
	Improvements []string `json:"improvements,omitempty"`
}

// Attribute compares two eval runs against the classes a context diff changed.
func Attribute(base, head Results, changes []ClassChange) Report {
	report := Report{Suite: head.Suite}
	if report.Suite == "" {
		report.Suite = base.Suite
	}
	for _, change := range changes {
		report.Changed = append(report.Changed, change.Class)
	}
	changed := map[string]bool{}
	for _, c := range report.Changed {
		changed[c] = true
	}

	baseCases := index(base.Cases)
	implicated := map[string]int{}
	declared := 0
	for _, headCase := range head.Cases {
		baseCase, present := baseCases[headCase.ID]
		if !present {
			report.Incomparable++
			continue
		}
		delta, comparable := movement(baseCase, headCase)
		if !comparable {
			report.Incomparable++
			continue
		}
		report.Compared++
		if len(headCase.DependsOn) > 0 {
			declared++
		}
		switch {
		case delta < -Threshold:
			report.Regressed++
			report.Regressions = append(report.Regressions, headCase.ID)
			if len(headCase.DependsOn) > 0 {
				report.RegressedDeclared++
			}
			pointing := false
			counted := map[string]bool{}
			for _, dependency := range headCase.DependsOn {
				// A dependency written twice is still one case pointing at one class.
				if changed[dependency] && !counted[dependency] {
					counted[dependency] = true
					implicated[dependency]++
					pointing = true
				}
			}
			if pointing {
				report.RegressedPointing++
			}
		case delta > Threshold:
			report.Improved++
			report.Improvements = append(report.Improvements, headCase.ID)
		default:
			report.Unchanged++
		}
	}
	headIndex := index(head.Cases)
	// Cases present in the baseline and gone from the head are incomparable, not unchanged.
	for _, baseCase := range base.Cases {
		if _, present := headIndex[baseCase.ID]; !present {
			report.Incomparable++
		}
	}
	report.Undeclared = report.Compared - declared
	report.Confounded = len(report.Changed) > 1
	sort.Strings(report.Regressions)
	sort.Strings(report.Improvements)

	// The mode is decided by the cases that regressed. It used to be decided by whether any
	// compared case declared a dependency, so one unchanged case with a declaration turned a
	// run in which no regressed case declared anything into an "attribution".
	switch {
	case report.Compared == 0:
		report.Mode = ModeNoChange
	case report.Regressed == 0:
		report.Mode = ModeNoChange
	case report.RegressedDeclared == 0:
		report.Mode = ModeCorrelation
	default:
		report.Mode = ModeAttribution
	}

	for _, change := range changes {
		entry := Implication{Class: change.Class, Summary: change.Summary, Regressed: implicated[change.Class]}
		if report.Regressed > 0 {
			entry.Share = float64(entry.Regressed) / float64(report.Regressed)
		}
		report.Implicated = append(report.Implicated, entry)
	}
	sort.SliceStable(report.Implicated, func(i, j int) bool {
		if report.Implicated[i].Share != report.Implicated[j].Share {
			return report.Implicated[i].Share > report.Implicated[j].Share
		}
		return report.Implicated[i].Class < report.Implicated[j].Class
	})
	return report
}

// ClassChange is the part of a diff entry this package needs. It is a copy of the fields rather
// than an import of the diff entry so that the attribution rules do not change when the diff
// grows a field.
type ClassChange struct {
	Class   string
	Summary string
}

// movement returns the change in a case's measure, and whether the two runs measured the same
// thing at all.
func movement(base, head Case) (float64, bool) {
	switch {
	case base.Passed != nil && head.Passed != nil:
		if *base.Passed == *head.Passed {
			return 0, true
		}
		if *base.Passed {
			return -1, true // passed, then failed
		}
		return 1, true
	case base.Score != nil && head.Score != nil:
		return sanitise(*head.Score - *base.Score), true
	default:
		// One run measured a pass and the other a score, or one measured nothing. Neither is a
		// regression; both are a change in how the suite reports itself, which the caller has to
		// fix before the comparison means anything.
		return 0, false
	}
}

func sanitise(delta float64) float64 {
	if math.IsNaN(delta) || math.IsInf(delta, 0) {
		return 0
	}
	return delta
}

func index(cases []Case) map[string]Case {
	out := make(map[string]Case, len(cases))
	for _, c := range cases {
		out[c.ID] = c
	}
	return out
}

// Explain renders the sentence a reader needs to know what the numbers mean. Every clause is
// computed from the counts it states, so it cannot say "every one of them declared dependencies"
// about a run in which none did.
func (r Report) Explain() string {
	switch r.Mode {
	case ModeAttribution:
		head := fmt.Sprintf("%d of %d cases regressed", r.Regressed, r.Compared)
		declared := fmt.Sprintf("%d of the %d regressed cases declare dependencies", r.RegressedDeclared, r.Regressed)
		if r.RegressedDeclared == r.Regressed {
			declared = "every regressed case declares its dependencies"
		}
		switch {
		case len(r.Changed) == 0:
			return fmt.Sprintf("%s and %s, but no context class changed between the two manifests, "+
				"so nothing in the context is a candidate cause.", head, declared)
		case r.RegressedPointing == 0:
			return fmt.Sprintf("%s and %s, but none of them declares a dependency on a class that changed, "+
				"so no changed class is a candidate cause on this evidence.", head, declared)
		default:
			return fmt.Sprintf("%s and %s; %d of them declare a dependency on a class that changed, so the classes "+
				"below are candidate causes consistent with those declarations, not proven causes.",
				head, declared, r.RegressedPointing)
		}
	case ModeCorrelation:
		return fmt.Sprintf("%d of %d cases regressed, but no regressed case declares which context classes it depends on, "+
			"so this is a correlation: the scores moved and the configuration moved, and nothing here ties them together beyond that.",
			r.Regressed, r.Compared)
	default:
		if r.Compared == 0 {
			return "The two runs share no case with a comparable measure, so nothing can be attributed."
		}
		if r.Improved > 0 {
			return fmt.Sprintf("No case regressed by more than %.3f (%d improved), so there is no regression to attribute.",
				Threshold, r.Improved)
		}
		return fmt.Sprintf("No case moved by more than %.3f, so there is nothing to attribute.", Threshold)
	}
}

// Caveat is the confounding statement, or "" when at most one class changed. It is separate from
// Explain because it is true in every mode: two changes made together cannot be told apart by
// two eval runs, whatever the cases declare.
func (r Report) Caveat() string {
	if !r.Confounded {
		return ""
	}
	return fmt.Sprintf("%d context classes changed together, so their effects are confounded: this report is not causal, "+
		"and it cannot say which of those changes, if any, caused a movement in the scores.", len(r.Changed))
}

// ClassNames returns the changed classes as a comma-separated list, for a one-line summary.
func (r Report) ClassNames() string { return strings.Join(r.Changed, ", ") }
