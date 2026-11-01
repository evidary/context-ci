package capture

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestJunctionOutsideRootIsRefused closes a defence that had never been exercised.
//
// The containment check resolves both the root and the target with EvalSymlinks before
// comparing them. On Windows, EvalSymlinks does not resolve a junction: it returns the
// junction path unchanged, and fails outright on a path *below* one. That failure is the
// only thing standing between a junction and a read of an arbitrary file, because
// os.ReadFile follows it.
//
// Demonstrated before the check was made strict: with the old logic, which ignored the
// EvalSymlinks error and compared the unresolved strings, reading link/secret.md returned
// the file outside the root. The strict version refuses.
//
// A junction needs no administrator rights, unlike a file symbolic link, so this test can
// run anywhere. It skips when mklink is unavailable or refuses.
func TestJunctionOutsideRootIsRefused(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows construct")
	}
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("OUTSIDE-SECRET\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "p.md"), []byte("inside\n"), 0o600); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	link := filepath.Join(root, "link")
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, outside)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("could not create a junction here (%v): %s", err, strings.TrimSpace(string(out)))
	}
	if _, err := os.Stat(filepath.Join(link, "secret.md")); err != nil {
		t.Skipf("the junction is not traversable: %v", err)
	}

	// The file is genuinely reachable through the junction, so a plain read succeeds.
	if _, err := os.ReadFile(filepath.Join(link, "secret.md")); err != nil {
		t.Fatalf("the fixture is wrong: the file should be readable through the junction: %v", err)
	}

	// Going through readInsideRoot must not.
	got, err := readInsideRoot(root, "link/secret.md")
	if err == nil {
		t.Fatalf("a file outside the root was read through a junction: %q", string(got))
	}
	if !strings.Contains(err.Error(), "symlink or junction") {
		t.Fatalf("refused, but the message does not explain why: %v", err)
	}

	// An ordinary file inside the root is still read.
	if got, err := readInsideRoot(root, "p.md"); err != nil {
		t.Fatalf("a normal file was refused: %v", err)
	} else if string(got) != "inside\n" {
		t.Fatalf("read %q", string(got))
	}

	// A path that does not exist reports that, not a security refusal.
	if _, err := readInsideRoot(root, "absent.md"); err == nil {
		t.Fatal("a missing file was read")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("a missing file reported %v, which a reader would take for a containment refusal", err)
	}
}
