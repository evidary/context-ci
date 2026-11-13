package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// MaxObservedRendered bounds a capture file. A rendered prompt is a page or two; a hundred
// megabytes means somebody pointed the SDK at the wrong variable, and hashing it silently would
// hide that.
const MaxObservedRendered = 4 << 20

// Observed is what an SDK saw at request-assembly time.
//
// This is the only part of an instrumented capture that a configuration file cannot know: the
// text actually sent to the model, after retrieval and templating. Tools and retrieval settings
// stay in the configuration, because they are configuration; capturing them twice would create
// two sources of truth for the same fact.
type Observed struct {
	CaptureVersion int    `json:"capture_version"`
	SDK            string `json:"sdk"`
	SDKVersion     string `json:"sdk_version"`
	Rendered       string `json:"rendered"`
	// Template is the unrendered template, when the SDK has it. Without it the template hash
	// falls back to whatever the configuration asserts.
	Template string `json:"template,omitempty"`
}

// LoadObserved reads a capture file written by an SDK.
//
// Unknown fields are refused. A capture that spelled a field differently would produce a
// manifest with an empty prompt hash, and the manifest would look valid while proving nothing.
func LoadObserved(path string) (*Observed, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var observed Observed
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observed); err != nil {
		return nil, fmt.Errorf("capture: %s: %w", path, err)
	}
	if observed.CaptureVersion != 1 {
		return nil, fmt.Errorf("capture: %s: capture_version is %d, want 1", path, observed.CaptureVersion)
	}
	if observed.Rendered == "" {
		return nil, fmt.Errorf("capture: %s: rendered is empty; a capture with no rendered prompt proves nothing", path)
	}
	if len(observed.Rendered) > MaxObservedRendered {
		return nil, fmt.Errorf("capture: %s: rendered prompt is %d bytes, over the %d limit", path, len(observed.Rendered), MaxObservedRendered)
	}
	if observed.SDKVersion == "" {
		return nil, fmt.Errorf("capture: %s: sdk_version is required: an instrumented manifest records which SDK produced it", path)
	}
	return &observed, nil
}
