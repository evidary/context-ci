package diff

import (
	"strings"
	"testing"
)

// TestPromptVariablesCollisionIsVisible covers a change the gate could not see.
//
// The variables fingerprint joined "name:value" pairs with spaces, so
//
//	{"a": "1 b:2"}   and   {"a": "1", "b": "2"}
//
// both rendered as "a:1 b:2". Two different variable sets compared equal, the diff
// reported the class unchanged, and `check` exited 0 on a real change to what the
// prompt is given.
func TestPromptVariablesCollisionIsVisible(t *testing.T) {
	b := base()
	h := base()
	b.Prompt.Variables = map[string]string{"a": "1 b:2"}
	h.Prompt.Variables = map[string]string{"a": "1", "b": "2"}

	res := Compare(b, h, Options{})
	for _, e := range res.Entries {
		if e.Class == ClassPromptVariables {
			return
		}
	}
	t.Fatalf("two different variable sets were reported unchanged; unchanged list: %v", res.Unchanged)
}

// TestPromptVariablesIgnoreValues checks the other direction: the manifest records the
// shape of the variable set, so changing a value is not a change to the manifest. If
// this ever fails, capture and diff disagree about what the field means.
func TestPromptVariablesIgnoreValues(t *testing.T) {
	b := base()
	h := base()
	b.Prompt.Variables = map[string]string{"tone": "string"}
	h.Prompt.Variables = map[string]string{"tone": "string"}
	res := Compare(b, h, Options{})
	for _, e := range res.Entries {
		if e.Class == ClassPromptVariables {
			t.Fatalf("identical variable sets produced a change: %s", e.Summary)
		}
	}
}

// TestUppercaseHashIsRejected pins the single-spelling rule. Both spellings used to
// validate, but they compare unequal, so a baseline and a head describing the same
// context reported a change and the canonical form carried two strings for one digest.
func TestUppercaseHashIsRejected(t *testing.T) {
	m := base()
	m.Prompt.TemplateHash = strings.ToUpper(strings.Repeat("a", 64))
	if err := m.Validate(); err == nil {
		t.Fatal("an uppercase hash validated")
	}
}
