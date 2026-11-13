package main

import "testing"

// TestResolveVersion pins the rule the binary uses to name itself. The case that matters is the
// second one: `go install ...@v0.4.1` builds without the release workflow's ldflags, and the module
// version the toolchain records is the only thing that can stop the binary reporting "dev" — which
// is what it did before this existed.
func TestResolveVersion(t *testing.T) {
	cases := []struct {
		name   string
		linked string
		module string
		want   string
	}{
		{"release build with ldflags", "0.4.1", "v0.4.1", "0.4.1"},
		{"go install with a version", "", "v0.4.1", "0.4.1"},
		{"go install with a pseudo-version", "", "v0.0.0-20261113120000-abcdef123456", "0.0.0-20261113120000-abcdef123456"},
		{"pre-release keeps its suffix", "", "v0.4.1-rc.1", "0.4.1-rc.1"},
		{"build inside a checkout", "", "(devel)", "dev"},
		{"no build info at all", "", "", "dev"},
		{"linked value wins over a different module version", "0.4.1", "v9.9.9", "0.4.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveVersion(c.linked, c.module); got != c.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", c.linked, c.module, got, c.want)
			}
		})
	}
}
