package capture

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Tree is a set of files a manifest can be built from: a directory on disk, or the tree of a git
// commit. The second is what lets a baseline be built from the base branch instead of read from a
// file somebody had to remember to regenerate and commit.
type Tree interface {
	// ReadFile returns the content of a path relative to the root, written with either
	// separator. A path with no file behind it is ErrMissingSource.
	ReadFile(rel string) ([]byte, error)
	// List returns every file path, relative to the root, with forward slashes, sorted. It is
	// asked for only when a pattern has to be matched.
	List() ([]string, error)
	// Describe names the tree in an error message.
	Describe() string
}

// DirTree is a directory on disk. Reads are confined to it: see readInsideRoot.
//
// Inside a git work tree the listing is the files git tracks plus the untracked files it does not
// ignore, which is the set a commit could contain. Outside one it is every file under the root
// except .git and node_modules. The difference matters for patterns only: a dependency directory
// holds agent files nobody in the repository wrote, and reporting them would bury the ones
// somebody did.
func DirTree(root string) Tree { return &dirTree{root: root} }

type dirTree struct {
	root   string
	listed bool
	files  []string
	err    error
}

func (t *dirTree) Describe() string { return t.root }

func (t *dirTree) ReadFile(rel string) ([]byte, error) { return readInsideRoot(t.root, rel) }

func (t *dirTree) List() ([]string, error) {
	if !t.listed {
		t.listed = true
		t.files, t.err = t.list()
	}
	return t.files, t.err
}

func (t *dirTree) list() ([]string, error) {
	if out, err := runGit(t.root, "ls-files", "-z", "--cached", "--others", "--exclude-standard"); err == nil {
		var files []string
		seen := map[string]bool{}
		for _, name := range strings.Split(string(out), "\x00") {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			// The index still lists a file deleted from the working tree, and the working tree
			// is what is being captured.
			info, err := os.Lstat(filepath.Join(t.root, filepath.FromSlash(name)))
			if err != nil || info.IsDir() {
				continue
			}
			files = append(files, name)
		}
		sort.Strings(files)
		return files, nil
	}
	var files []string
	err := filepath.WalkDir(t.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != t.root && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(t.root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("capture: list %s: %w", t.root, err)
	}
	sort.Strings(files)
	return files, nil
}

// GitTree is the tree of one commit, read with `git show` and `git ls-tree` from the repository
// that contains root. Paths are relative to root, as they are for DirTree, so a configuration
// means the same thing against either.
//
// No file is written and no network is touched: the commit has to be in the local repository
// already, which in CI means a checkout with enough history to contain it.
func GitTree(root, commit string) Tree { return &gitTree{root: root, commit: commit} }

type gitTree struct {
	root, commit string
	listed       bool
	modes        map[string]string
	files        []string
	err          error
}

func (t *gitTree) Describe() string { return "commit " + shortCommit(t.commit) }

func (t *gitTree) load() error {
	if t.listed {
		return t.err
	}
	t.listed = true
	// Run from root, ls-tree lists the entries below it, with paths relative to it.
	out, err := runGit(t.root, "ls-tree", "-r", "-z", t.commit)
	if err != nil {
		t.err = fmt.Errorf("capture: list %s: %w", t.Describe(), err)
		return t.err
	}
	t.modes = map[string]string{}
	for _, record := range strings.Split(string(out), "\x00") {
		// <mode> SP <type> SP <object> TAB <path>
		meta, name, ok := strings.Cut(record, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 2 || fields[1] != "blob" {
			continue // a submodule has no content here
		}
		t.modes[name] = fields[0]
		t.files = append(t.files, name)
	}
	sort.Strings(t.files)
	return nil
}

func (t *gitTree) List() ([]string, error) {
	if err := t.load(); err != nil {
		return nil, err
	}
	return t.files, nil
}

func (t *gitTree) ReadFile(rel string) ([]byte, error) {
	clean := path.Clean(filepath.ToSlash(rel))
	if path.IsAbs(clean) || filepath.IsAbs(rel) || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, ErrOutsideRoot{Path: rel}
	}
	if err := t.load(); err != nil {
		return nil, err
	}
	mode, ok := t.modes[clean]
	if !ok {
		return nil, ErrMissingSource{Path: rel}
	}
	if mode == "120000" {
		// The blob of a symbolic link is the path it points at, not the file's content, and
		// following it would need the same containment proof the working tree gets.
		return nil, fmt.Errorf("capture: %s is a symbolic link in %s, which is not followed", rel, t.Describe())
	}
	out, err := runGit(t.root, "show", t.commit+":./"+clean)
	if err != nil {
		return nil, fmt.Errorf("capture: read %s from %s: %w", rel, t.Describe(), err)
	}
	return out, nil
}

// runGit runs one git command in root and returns its stdout. The error carries the first line git
// wrote to stderr, because "exit status 128" tells a reader nothing.
func runGit(root string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git is not installed or not on PATH")
	}
	full := append([]string{"-C", root, "-c", "core.quotepath=off"}, args...)
	cmd := exec.Command("git", full...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errOut.String())
		if i := strings.IndexByte(detail, '\n'); i >= 0 {
			detail = strings.TrimSpace(detail[:i])
		}
		if detail == "" {
			detail = err.Error()
		}
		return nil, errors.New(detail)
	}
	return out.Bytes(), nil
}

func shortCommit(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}
