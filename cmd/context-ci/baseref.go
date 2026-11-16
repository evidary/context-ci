package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/evidary/context-ci/internal/capture"
	"github.com/evidary/context-ci/internal/config"
	"github.com/evidary/context-ci/internal/diff"
	"github.com/evidary/context-ci/internal/manifest"
)

// resolveBase turns --base-ref into the commit the baseline is built from: the merge base of the
// ref and HEAD.
//
// The merge base, not the tip of the ref, because the question is what this branch changed. If the
// base branch has moved on since the branch was cut, comparing against its tip would report the
// base branch's own changes backwards, as things this branch removed. On a pull-request merge
// commit the two are the same commit, so the answer is right there too.
func resolveBase(root, ref string) (commit, note string, err error) {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return "", "", errors.New("--base-ref reads the baseline from git, and git is not installed or not on PATH")
	}
	tip, detail, gitErr := git(root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if gitErr != nil || tip == "" {
		if detail == "" || strings.HasPrefix(detail, "exit status") {
			detail = "git does not know that name"
		}
		return "", "", fmt.Errorf("--base-ref %s is not a commit in the repository at %s (%s); "+
			"in CI the checkout has to contain the base branch, which actions/checkout does with fetch-depth: 0",
			ref, root, detail)
	}
	head, _, gitErr := git(root, "rev-parse", "--verify", "--quiet", "HEAD")
	if gitErr != nil || head == "" {
		return tip, "", nil
	}
	base, _, gitErr := git(root, "merge-base", tip, head)
	if gitErr != nil || base == "" {
		return tip, fmt.Sprintf("%s and HEAD share no history in this checkout (a shallow clone does that), "+
			"so the baseline is the tip of %s and not the point this branch left it", ref, ref), nil
	}
	return base, "", nil
}

// baseConfiguration reads the configuration as it was at the base commit.
//
// The declared model and the source paths are part of the context, so the baseline has to use its
// own. A base with no configuration is a repository that had not adopted the gate yet, and one
// with a configuration this version cannot read is one somebody has to fix: in both cases the
// baseline falls back to the defaults, and the comparison reports everything the head declares as
// new, instead of failing every pull request including the one that repairs it.
func baseConfiguration(tree capture.Tree, rel string) (*config.Config, string) {
	raw, err := tree.ReadFile(rel)
	if err != nil {
		var missing capture.ErrMissingSource
		if errors.As(err, &missing) {
			return config.Default(), fmt.Sprintf("%s has no %s, so the baseline is the default configuration", tree.Describe(), rel)
		}
		return config.Default(), fmt.Sprintf("%s in %s could not be read (%v), so the baseline is the default configuration", rel, tree.Describe(), err)
	}
	cfg, err := config.Parse(raw)
	if err != nil {
		return config.Default(), fmt.Sprintf("%s in %s is not a configuration this version accepts (%v), "+
			"so the baseline is the default configuration", rel, tree.Describe(), err)
	}
	return cfg, ""
}

// configRelativeToRoot returns the configuration file's path relative to the root, which is how a
// commit's tree is addressed.
func configRelativeToRoot(configPath, root string) (string, error) {
	if configPath == "" {
		return config.DefaultFile, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absConfig)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--config %s is outside --root %s, so the baseline cannot read the same file from the base commit", configPath, root)
	}
	return filepath.ToSlash(rel), nil
}

// parseAcknowledge reads --acknowledge: "all", or class names separated by commas or spaces.
func parseAcknowledge(value string) (classes map[string]bool, all bool, err error) {
	classes = map[string]bool{}
	for _, name := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if name == "all" {
			all = true
			continue
		}
		if !diff.IsKnownClass(name) {
			return nil, false, fmt.Errorf("--acknowledge %s is not a diff class; run 'context-ci classes' for the list, or write all", name)
		}
		classes[name] = true
	}
	return classes, all, nil
}

// located is where annotate looks things up.
type located struct {
	root string
	cfg  *config.Config
	// configRel is the configuration file relative to root, or "" when the defaults are in use.
	configRel string
	// baseCommit is the commit the baseline was built from, or "" when it was read from a file,
	// in which case there is no text to compare.
	baseCommit string
	noText     bool
}

// maxDiffLines bounds the text shown for one file. The report is a pointer to a change, not a
// substitute for the pull request's own diff view.
const maxDiffLines = 60

// declaredKeys maps the classes whose value is written in the configuration file to the key that
// holds it.
var declaredKeys = map[diff.Class][]string{
	diff.ClassModelID:            {"model", "id"},
	diff.ClassModelProvider:      {"model", "provider"},
	diff.ClassModelAliasOf:       {"model", "alias_of"},
	diff.ClassModelParams:        {"model", "params"},
	diff.ClassRetrievalEmbedding: {"retrieval", "embedding_model"},
	diff.ClassRetrievalTopK:      {"retrieval", "top_k"},
	diff.ClassRetrievalIndexID:   {"retrieval", "index_id"},
	diff.ClassRetrievalChunker:   {"retrieval", "chunker"},
	diff.ClassPolicyVersion:      {"policy", "version"},
}

// annotate says where each change is: the file, the line, and for a file whose text changed, the
// text. Compare works on two manifests and knows none of this; a reviewer needs all of it, because
// two truncated hashes are less than `git diff` already shows.
//
// Everything here is best effort. A location that cannot be found is left out, never guessed: an
// annotation on the wrong line is worse than one on no line.
func annotate(res *diff.Result, at located) {
	sources := at.cfg.Capture.Sources
	var configRoot *yaml.Node
	if at.configRel != "" {
		if raw, err := os.ReadFile(filepath.Join(at.root, filepath.FromSlash(at.configRel))); err == nil {
			var doc yaml.Node
			if yaml.Unmarshal(raw, &doc) == nil && len(doc.Content) == 1 {
				configRoot = doc.Content[0]
			}
		}
	}

	textual := map[string]bool{}
	for i := range res.Entries {
		e := &res.Entries[i]
		switch {
		case e.Path != "":
			textual[e.Path] = true
		case e.Source == "tools":
			e.Path = sources[config.SourceTools]
			textual[e.Path] = true
		case e.Source == "prompt", e.Class == diff.ClassPromptTemplateHash, e.Class == diff.ClassPromptVariables:
			e.Path = sources[config.SourcePrompt]
			textual[e.Path] = true
		case e.Class == diff.ClassPromptRenderedHash:
			e.Path = sources[config.SourceRendered]
			textual[e.Path] = true
		case e.Class == diff.ClassRetrievalFiltersHash:
			e.Path = sources[config.SourceRetrieval]
			textual[e.Path] = true
		case e.Class == diff.ClassPolicyGuardrailsHash:
			e.Path = sources[config.SourceGuardrails]
			textual[e.Path] = true
		default:
			if keys, ok := declaredKeys[e.Class]; ok && at.configRel != "" {
				e.Path = at.configRel
				e.Line = yamlLine(configRoot, keys)
			}
		}
	}
	delete(textual, "")

	// One diff per file, however many entries point at it.
	firstChanged := map[string]int{}
	if at.baseCommit != "" {
		for _, rel := range sortedKeys(textual) {
			hunks, line := textDiff(at.root, at.baseCommit, rel)
			if line > 0 {
				firstChanged[rel] = line
			}
			if hunks == "" || at.noText {
				continue
			}
			lines := strings.Split(hunks, "\n")
			text := diff.TextDiff{Path: rel, Diff: hunks}
			if len(lines) > maxDiffLines {
				text.Diff = strings.Join(lines[:maxDiffLines], "\n")
				text.Omitted = len(lines) - maxDiffLines
			}
			res.Texts = append(res.Texts, text)
		}
	}

	contents := map[string]string{}
	read := func(rel string) string {
		if body, ok := contents[rel]; ok {
			return body
		}
		raw, _ := capture.DirTree(at.root).ReadFile(rel)
		contents[rel] = string(raw)
		return contents[rel]
	}
	for i := range res.Entries {
		e := &res.Entries[i]
		if e.Path == "" || e.Line != 0 {
			continue
		}
		switch {
		case e.Class == diff.ClassHiddenCharacters && e.Source != "tools":
			if _, first := capture.HiddenCharacters(read(e.Path)); first != nil {
				e.Line = first.Line
				e.Detail = "the first is " + first.String() + "; " + e.Detail
			}
		case e.Source == "tools":
			e.Line = lineContaining(read(e.Path), e.Subject)
		default:
			e.Line = firstChanged[e.Path]
			// A file nobody has staged yet has no diff against the base, and all of it is new.
			if e.Line == 0 && at.baseCommit != "" && read(e.Path) != "" {
				e.Line = 1
			}
		}
	}

	// Paths were relative to --root so far, because that is how the files are read. A report is
	// read against the repository, so they leave here relative to its top.
	if prefix, _, err := git(at.root, "rev-parse", "--show-prefix"); err == nil && prefix != "" {
		for i := range res.Entries {
			if res.Entries[i].Path != "" {
				res.Entries[i].Path = prefix + res.Entries[i].Path
			}
		}
		for i := range res.Texts {
			res.Texts[i].Path = prefix + res.Texts[i].Path
		}
	}
}

// textDiff returns the hunks of a file's change between a commit and the working tree, and the
// number of the first line that is new or changed in the working tree. A file git cannot compare
// as text, or one that is untracked, has no hunks.
func textDiff(root, commit, rel string) (string, int) {
	// Run directly, not through git(): that trims its output, and a hunk's last context line can
	// be a single space.
	raw, err := exec.Command("git", "-C", root, "-c", "core.quotepath=off",
		"diff", "--no-color", "--no-ext-diff", "-U2", commit, "--", rel).Output()
	out := strings.TrimRight(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if err != nil || out == "" {
		return "", 0
	}
	start := strings.Index(out, "\n@@ ")
	if start < 0 {
		return "", 0
	}
	hunks := out[start+1:]
	first, fallback, current := 0, 0, 0
	for _, line := range strings.Split(hunks, "\n") {
		switch {
		case strings.HasPrefix(line, "@@ "):
			current = hunkStart(line)
			if fallback == 0 {
				fallback = current
			}
		case strings.HasPrefix(line, "+"):
			if first == 0 {
				first = current
			}
			current++
		case strings.HasPrefix(line, "-"), strings.HasPrefix(line, `\`):
		default:
			current++
		}
	}
	if first == 0 {
		// Only deletions: the nearest surviving line is where the hunk starts.
		first = fallback
	}
	if first < 1 {
		first = 1
	}
	return hunks, first
}

// hunkStart reads the new-file line number out of "@@ -a,b +c,d @@".
func hunkStart(header string) int {
	_, rest, ok := strings.Cut(header, " +")
	if !ok {
		return 0
	}
	digits := rest
	if i := strings.IndexAny(rest, ", "); i >= 0 {
		digits = rest[:i]
	}
	n, _ := strconv.Atoi(digits)
	return n
}

// yamlLine returns the line of the key at the end of a path, or of the deepest key on the path
// that exists, so a value that was removed still points at the block it was removed from.
func yamlLine(node *yaml.Node, keys []string) int {
	line := 0
	for _, key := range keys {
		if node == nil || node.Kind != yaml.MappingNode {
			return line
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				line = node.Content[i].Line
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			return line
		}
		node = next
	}
	return line
}

// lineContaining returns the first line that holds needle, or 0.
func lineContaining(text, needle string) int {
	if needle == "" {
		return 0
	}
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	return 0
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// hiddenIn lists where a manifest records hidden characters, for the warning `manifest` prints. A
// check reports them only for what changed; this is how a team finds the ones already there.
func hiddenIn(m *manifest.Manifest, promptPath string) []string {
	var found []string
	if m.Prompt.HiddenCharacters > 0 {
		found = append(found, fmt.Sprintf("%s (%d)", promptPath, m.Prompt.HiddenCharacters))
	}
	for _, t := range m.Tools {
		if t.HiddenCharacters > 0 {
			found = append(found, fmt.Sprintf("the description of tool %q (%d)", t.Name, t.HiddenCharacters))
		}
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		if f.HiddenCharacters > 0 && !seen[f.Path] {
			seen[f.Path] = true
			found = append(found, fmt.Sprintf("%s (%d)", f.Path, f.HiddenCharacters))
		}
	}
	return found
}

// warnTo returns a capture warning sink that writes to stderr in the tool's usual form.
func warnTo(stderr io.Writer) func(string) {
	return func(message string) { fmt.Fprintf(stderr, "context-ci: %s\n", message) }
}
