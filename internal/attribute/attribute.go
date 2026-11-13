// Package attribute ties a movement in eval results to the context change that plausibly
// caused it.
//
// The distinction this package exists to keep is attribution versus correlation. A case that
// declares which context classes it depends on can be attributed to: if it regressed and only
// one of its dependencies changed, that dependency is implicated by evidence. A case that
// declares nothing can only be correlated: the score moved and the configuration moved, and
// nothing ties them together beyond the fact that both happened. A tool that printed the second
// as if it were the first would be inventing the answer this product exists to give.
package attribute

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

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
	// Undeclared is how many cases declare no dependencies.
	Undeclared int `json:"undeclared"`
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
			for _, dependency := range headCase.DependsOn {
				if changed[dependency] {
					implicated[dependency]++
				}
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
	sort.Strings(report.Regressions)
	sort.Strings(report.Improvements)

	switch {
	case report.Compared == 0:
		report.Mode = ModeNoChange
	case report.Regressed == 0:
		report.Mode = ModeNoChange
	case declared == 0:
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

// Explain renders the one sentence a reader needs to know what the numbers mean.
func (r Report) Explain() string {
	switch r.Mode {
	case ModeAttribution:
		return fmt.Sprintf("%d of %d cases regressed, and every one of them declared dependencies, so the classes below are implicated by evidence rather than by coincidence.", r.Regressed, r.Compared)
	case ModeCorrelation:
		return fmt.Sprintf("%d of %d cases regressed, but no case declares which context classes it depends on, so this is a correlation: the scores moved and the configuration moved, and nothing here ties them together beyond that.", r.Regressed, r.Compared)
	default:
		if r.Compared == 0 {
			return "The two runs share no case with a comparable measure, so nothing can be attributed."
		}
		return fmt.Sprintf("No case moved by more than %.3f, so there is nothing to attribute.", Threshold)
	}
}

// ClassNames returns the changed classes as a comma-separated list, for a one-line summary.
func (r Report) ClassNames() string { return strings.Join(r.Changed, ", ") }
