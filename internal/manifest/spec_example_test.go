package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// repoRootForSpec walks up to the module root so the test can read docs/ regardless of
// the package directory it runs in.
func repoRootForSpec(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod above the test directory")
		}
		dir = parent
	}
}

// TestSpecificationExampleIsAValidManifest keeps the published example honest.
//
// The specification used to show a skeleton full of alternatives -
// "instrumented|asserted|observed" - which is not a manifest any tool accepts, so a team
// implementing from the document produced a file the CLI rejected with exit 3. The
// example is now a real manifest, and this test fails if it drifts: it must parse, carry
// its own digest, and reproduce that digest.
func TestSpecificationExampleIsAValidManifest(t *testing.T) {
	path := filepath.Join(repoRootForSpec(t), "docs", "03-product", "spec.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the specification is missing: %v", err)
	}
	block := regexp.MustCompile("(?s)```json\n(.*?)\n```").FindSubmatch(raw)
	if block == nil {
		t.Fatal("the specification has no json example")
	}
	if !regexp.MustCompile(`"manifest_version"`).Match(block[1]) {
		t.Fatal("the first json example in the specification is not the manifest")
	}

	var m Manifest
	if err := json.Unmarshal(block[1], &m); err != nil {
		t.Fatalf("the example does not parse as a manifest: %v", err)
	}
	stored := m.Digest
	if stored == "" {
		t.Fatal("the example carries no digest, so a reader copying it gets a file the tool rejects")
	}
	recomputed, err := m.Compute()
	if err != nil {
		t.Fatalf("the example does not validate: %v", err)
	}
	if stored != recomputed {
		t.Fatalf("the example's digest is %s but its content hashes to %s; update the example", stored, recomputed)
	}
}
