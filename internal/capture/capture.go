// Package capture builds a manifest from a configuration and a checkout.
//
// The one hard requirement here is that the same repository produces the same
// bytes on Linux, macOS and Windows. Three rules make that true:
//
//  1. File content is hashed with CRLF normalised to LF. A checkout with Windows
//     line endings is the same context as one with Unix line endings, and a digest
//     that disagreed would make every Windows developer's pull request noisy.
//  2. Paths are resolved relative to the repository root, rejected if they escape
//     it, and never appear in the manifest with a platform separator.
//  3. Nothing derived from iteration order, the clock or the environment reaches
//     the manifest except created_at, which the digest excludes.
package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sigilary/context-ci/internal/config"
	"github.com/sigilary/context-ci/internal/manifest"
)

// Options are the inputs that are not part of the configuration file.
type Options struct {
	// Commit is the repository revision being captured.
	Commit string
	// Now is the capture time. The digest excludes it, but it is recorded so a
	// reader can tell when the manifest was produced.
	Now time.Time
	// SDKVersion is recorded when the capture path is instrumented.
	SDKVersion string
	// Observed, when set, is the rendered prompt an SDK saw at request time. It replaces the
	// file-hashed rendered prompt and marks the capture path instrumented. It exists because the
	// rendered text is the one thing a configuration file cannot know.
	Observed *Observed
	// Lenient builds what it can instead of stopping at the first unreadable source. It is for a
	// baseline built from an older commit: a source that did not exist there, or a file that did
	// not parse there, is part of what the comparison has to report, and refusing to build would
	// fail every later pull request, including the one that fixes it.
	Lenient bool
	// Warn receives what a reader should know and the manifest cannot say: a pattern that matched
	// nothing, a source a lenient build skipped. Nil discards them.
	Warn func(string)
}

func (o Options) warn(format string, args ...any) {
	if o.Warn != nil {
		o.Warn(fmt.Sprintf(format, args...))
	}
}

// ErrOutsideRoot is returned for a source path that escapes the repository.
type ErrOutsideRoot struct{ Path string }

func (e ErrOutsideRoot) Error() string {
	return fmt.Sprintf("capture: source path %q resolves outside the repository", e.Path)
}

// Build produces a manifest for the checkout at root.
func Build(cfg *config.Config, root string, opts Options) (*manifest.Manifest, error) {
	return BuildTree(cfg, DirTree(root), opts)
}

// BuildTree produces a manifest for any tree: a directory, or the tree of a commit.
func BuildTree(cfg *config.Config, tree Tree, opts Options) (*manifest.Manifest, error) {
	if cfg == nil {
		return nil, errors.New("capture: no configuration")
	}
	if opts.Commit == "" {
		return nil, errors.New("capture: commit is required; a manifest without one cannot be reproduced")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	// The tools file is parsed, not hashed whole, so it is read where it is used.
	hashes := make(map[string]string, len(cfg.Capture.Sources))
	var template []byte
	for _, kind := range sourceKinds(cfg) {
		rel := cfg.Capture.Sources[kind]
		data, err := tree.ReadFile(rel)
		if err != nil {
			if opts.Lenient {
				opts.warn("%s has no readable %s (capture.sources.%s), so that source is absent from it", tree.Describe(), rel, kind)
				continue
			}
			return nil, sourceError(kind, rel, tree.Describe(), err)
		}
		switch kind {
		case config.SourceTools:
			continue
		case config.SourceRetrieval, config.SourceGuardrails:
			sum, err := hashStructured(rel, data)
			if err != nil {
				if !opts.Lenient {
					return nil, fmt.Errorf("capture: capture.sources.%s: %w", kind, err)
				}
				opts.warn("%s in %s does not parse, so its bytes are compared instead of its values", rel, tree.Describe())
				sum = HashNormalised(data)
			}
			hashes[kind] = sum
		default:
			if kind == config.SourcePrompt {
				template = data
			}
			hashes[kind] = HashNormalised(data)
		}
	}

	m := &manifest.Manifest{
		ManifestVersion: manifest.Version,
		CreatedAt:       opts.Now.UTC().Format(time.RFC3339),
		Commit:          opts.Commit,
		Capture: manifest.Capture{
			Path:       cfg.Capture.Path,
			SDKVersion: opts.SDKVersion,
		},
		Tools: []manifest.Tool{},
		Files: []manifest.File{},
		Retrieval: manifest.Retrieval{
			IndexID:               cfg.Retrieval.IndexID,
			EmbeddingModel:        cfg.Retrieval.EmbeddingModel,
			EmbeddingModelVersion: cfg.Retrieval.EmbeddingModelVersion,
			Chunker:               cfg.Retrieval.Chunker,
			TopK:                  cfg.Retrieval.TopK,
			FiltersHash:           optional(hashes[config.SourceRetrieval]),
		},
		Policy: manifest.Policy{
			Version:        cfg.Policy.Version,
			GuardrailsHash: optional(hashes[config.SourceGuardrails]),
		},
	}

	if cfg.HasModel() {
		m.Model = manifest.Model{
			Provider: cfg.Model.Provider,
			ID:       cfg.Model.ID,
			AliasOf:  optional(cfg.Model.AliasOf),
			Params: manifest.ModelParam{
				Temperature: optional(string(cfg.Model.Params.Temperature)),
				MaxTokens:   cfg.Model.Params.MaxTokens,
				Seed:        optional(string(cfg.Model.Params.Seed)),
				TopP:        optional(string(cfg.Model.Params.TopP)),
			},
		}
	}
	promptHash, hasPrompt := hashes[config.SourcePrompt]
	if hasPrompt {
		hidden, _ := HiddenCharacters(string(template))
		m.Prompt = manifest.Prompt{
			TemplateHash:     promptHash,
			RenderedHash:     promptHash,
			Variables:        variablesIn(template),
			HiddenCharacters: hidden,
		}
	}

	if opts.Observed != nil {
		// The rendered prompt came from the SDK, so the capture path is instrumented by
		// definition: no file in the repository holds this text.
		if m.Prompt.Variables == nil {
			m.Prompt.Variables = map[string]string{}
		}
		m.Capture.Path = "instrumented"
		m.Capture.SDKVersion = opts.Observed.SDKVersion
		m.Capture.Reason = ""
		m.Prompt.RenderedHash = hashString(opts.Observed.Rendered)
		m.Prompt.RenderedSampleHash = nil
		if opts.Observed.Template != "" {
			m.Prompt.TemplateHash = hashString(opts.Observed.Template)
		}
	} else if rendered, ok := hashes[config.SourceRendered]; ok && hasPrompt {
		m.Prompt.RenderedHash = rendered
		m.Prompt.RenderedSampleHash = &rendered
	} else if hasPrompt {
		// No rendered sample was supplied. Saying the rendered hash equals the
		// template hash is true for an unrendered template and has to be stated,
		// because a reader who assumed it was rendered would over-trust the result.
		m.Capture.Reason = "no rendered sample supplied, so rendered_hash equals template_hash"
	}

	if toolsPath, ok := cfg.Capture.Sources[config.SourceTools]; ok {
		data, err := tree.ReadFile(toolsPath)
		if err == nil {
			var tools []manifest.Tool
			if tools, err = ParseTools(data); err == nil {
				m.Tools = tools
			}
		}
		// A missing tools file was reported, or skipped, by the loop above.
		var missing ErrMissingSource
		if err != nil && !errors.As(err, &missing) {
			if !opts.Lenient {
				return nil, fmt.Errorf("capture: tools: %w", err)
			}
			opts.warn("%s in %s cannot be read as tool definitions (%v), so it lists no tools", toolsPath, tree.Describe(), err)
		}
	}

	files, err := collectFiles(cfg, tree, opts)
	if err != nil {
		return nil, err
	}
	m.Files = files

	if _, err := m.Compute(); err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	return m, nil
}

// CheckSources reports the first configured source that cannot be read, without building anything.
// It lets a caller say "this file is missing" before it asks for anything else a manifest needs,
// so a first run reports the file the user has to create rather than whatever happened to be
// checked first.
func CheckSources(cfg *config.Config, root string) error {
	if cfg == nil {
		return errors.New("capture: no configuration")
	}
	for _, kind := range sourceKinds(cfg) {
		if _, err := readInsideRoot(root, cfg.Capture.Sources[kind]); err != nil {
			return sourceError(kind, cfg.Capture.Sources[kind], root, err)
		}
	}
	return nil
}

// maxGroupFiles bounds one group. A pattern that matches tens of thousands of files is a pattern
// aimed at the wrong directory, and a manifest that size is one nobody reviews.
const maxGroupFiles = 5000

// collectFiles hashes every file the named groups and the built-in agent groups match.
func collectFiles(cfg *config.Config, tree Tree, opts Options) ([]manifest.File, error) {
	type group struct {
		name     string
		patterns []string
		builtin  bool
	}
	var groups []group
	for name, patterns := range cfg.Capture.Files {
		groups = append(groups, group{name: name, patterns: patterns})
	}
	if cfg.AgentFilesEnabled() {
		groups = append(groups,
			group{name: GroupAgentInstructions, patterns: AgentInstructionPatterns, builtin: true},
			group{name: GroupAgentConfig, patterns: AgentConfigPatterns, builtin: true})
	}
	out := []manifest.File{}
	if len(groups) == 0 {
		return out, nil
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })

	listing, err := tree.List()
	if err != nil {
		return nil, err
	}
	// The prompt source is reported as the prompt. A group whose pattern also covers it, as
	// prompts/** does, would report the same edit a second time under another class.
	promptSource := ""
	if rel, ok := cfg.Capture.Sources[config.SourcePrompt]; ok {
		promptSource = path.Clean(filepath.ToSlash(rel))
	}
	for _, g := range groups {
		matched := map[string]bool{}
		for _, pattern := range g.patterns {
			found := false
			for _, name := range listing {
				if !g.builtin && name == promptSource {
					found = found || MatchPattern(pattern, name)
					continue
				}
				if MatchPattern(pattern, name) {
					matched[name] = true
					found = true
				}
			}
			if found || g.builtin {
				continue
			}
			// The listing leaves out what git ignores, and a path written out in full is a file
			// somebody means, ignored or not.
			if literalPath(pattern) {
				if _, err := tree.ReadFile(pattern); err == nil {
					matched[path.Clean(pattern)] = true
					continue
				}
				if !opts.Lenient {
					return nil, fmt.Errorf("capture: capture.files.%s names %s, which does not exist in %s: "+
						"create it, or remove the line", g.name, pattern, tree.Describe())
				}
			}
			opts.warn("capture.files.%s: %s matches no file in %s", g.name, pattern, tree.Describe())
		}
		if len(matched) > maxGroupFiles {
			return nil, fmt.Errorf("capture: capture.files.%s matches %d files, over the limit of %d; narrow its patterns",
				g.name, len(matched), maxGroupFiles)
		}
		names := make([]string, 0, len(matched))
		for name := range matched {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			data, err := tree.ReadFile(name)
			if err != nil {
				// A built-in pattern can match a link that leaves the repository. Nobody named
				// that file, so it is skipped and said, rather than failing a build over it.
				if opts.Lenient || g.builtin {
					opts.warn("%s in %s was not read (%v)", name, tree.Describe(), err)
					continue
				}
				return nil, fmt.Errorf("capture: capture.files.%s: %w", g.name, err)
			}
			hidden, _ := HiddenCharacters(string(data))
			out = append(out, manifest.File{Group: g.name, Path: name, Hash: HashNormalised(data), HiddenCharacters: hidden})
		}
	}
	return out, nil
}

// hashStructured hashes a YAML or JSON file by what it says rather than how it is written, so a
// comment, a reordered key or a reformatted list is not a change. Any other file is hashed as
// text. A retrieval or guardrails file is read by a program, and a program sees the values.
func hashStructured(rel string, data []byte) (string, error) {
	switch strings.ToLower(path.Ext(filepath.ToSlash(rel))) {
	case ".yaml", ".yml", ".json":
	default:
		return HashNormalised(data), nil
	}
	documents, err := decodeYAML(data)
	if err != nil {
		return "", fmt.Errorf("%s is not valid YAML: %w", rel, err)
	}
	canonical, err := json.Marshal(documents)
	if err != nil {
		return "", fmt.Errorf("%s: %w", rel, err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// decodeYAML returns every document in data as plain JSON-shaped values.
func decodeYAML(data []byte) ([]any, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	documents := []any{}
	for {
		var doc any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return documents, nil
			}
			return nil, err
		}
		documents = append(documents, plain(doc))
	}
}

// plain turns what the YAML decoder produces into what the JSON encoder accepts and orders: every
// map keyed by string, and no value it would refuse.
func plain(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = plain(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[fmt.Sprint(k)] = plain(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = plain(item)
		}
		return out
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Sprint(x)
		}
		return x
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	default:
		return x
	}
}

// sourceKinds returns the configured source kinds in a fixed order, so that when two sources are
// unreadable the one reported does not depend on map iteration.
func sourceKinds(cfg *config.Config) []string {
	kinds := make([]string, 0, len(cfg.Capture.Sources))
	for kind := range cfg.Capture.Sources {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// ErrMissingSource is returned for a source path with no file behind it. It is a type of its own
// because it is the commonest first-run failure and the only one whose fix is "create the file":
// the operating system's wording for it differs by platform and names no configuration key.
type ErrMissingSource struct{ Path string }

func (e ErrMissingSource) Error() string {
	return fmt.Sprintf("capture: %s does not exist", e.Path)
}

// sourceError names the configuration key as well as the path, because the key is what the reader
// has to edit when the path is wrong.
func sourceError(kind, rel, root string, err error) error {
	var missing ErrMissingSource
	if errors.As(err, &missing) {
		return fmt.Errorf("capture: capture.sources.%s names %s, which does not exist under %s: "+
			"create that file, or point capture.sources.%s at the file that holds it", kind, rel, root, kind)
	}
	return fmt.Errorf("capture: source %s: %w", kind, err)
}

// HashNormalised is the hash every text file in a manifest goes through. CRLF and
// LF input of the same text produce the same digest, which is what makes a Windows
// checkout equivalent to a Unix one.
func HashNormalised(data []byte) string {
	normalised := strings.ReplaceAll(string(data), "\r\n", "\n")
	sum := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(sum[:])
}

// readInsideRoot reads a path that must stay inside root. A configuration naming
// ../../etc/passwd is either a mistake or an attempt to hash a file the tool should
// not touch, and both are refused.
func readInsideRoot(root, rel string) ([]byte, error) {
	if filepath.IsAbs(rel) {
		return nil, ErrOutsideRoot{Path: rel}
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, ErrOutsideRoot{Path: rel}
	}
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("capture: resolve root: %w", err)
	}
	resolved, err := filepath.Abs(filepath.Join(root, clean))
	if err != nil {
		return nil, fmt.Errorf("capture: resolve %s: %w", rel, err)
	}
	// A clean path comparison is not enough on a filesystem with symbolic links or
	// Windows junctions: a link inside the repository can point anywhere, and reading
	// through it would hash a file the operator never agreed to expose.
	//
	// Resolution is strict. It used to ignore the error and fall back to the unresolved
	// path, which is fail-open: when EvalSymlinks could not resolve a component - a
	// junction, a permission the process lacks, a path the platform cannot canonicalise
	// - the containment check compared two strings that both looked inside the root and
	// the read then followed the link. A missing target is an error here, which is the
	// same outcome as before, with a clearer message.
	realRoot, err := filepath.EvalSymlinks(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("capture: resolve root: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		// A path that exists but cannot be canonicalised is a symlink or junction the
		// platform will not resolve through - on Windows, EvalSymlinks returns a
		// junction unchanged and fails outright on a path below one. os.ReadFile would
		// follow it, so the read is refused rather than performed on a path that cannot
		// be shown to stay inside the root. Distinguishing "exists but unresolvable"
		// from "does not exist" matters: the first is a security refusal and the second
		// is a typo, and they used to produce the same message.
		if _, lerr := os.Lstat(resolved); lerr == nil {
			return nil, fmt.Errorf(
				"capture: %s exists but cannot be resolved to a real path (%v); a symlink or junction is refused because it cannot be shown to stay inside %s",
				rel, err, root)
		}
		return nil, ErrMissingSource{Path: rel}
	}
	if realTarget != realRoot && !strings.HasPrefix(realTarget, realRoot+string(filepath.Separator)) {
		return nil, ErrOutsideRoot{Path: rel}
	}
	return os.ReadFile(realTarget)
}

var variablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\}\}`)

// variablesIn lists the template placeholders, sorted, with no values. The manifest
// records the shape of the variable set and not the data, so changing a value does
// not change the digest but changing the set does.
func variablesIn(template []byte) map[string]string {
	out := map[string]string{}
	for _, match := range variablePattern.FindAllStringSubmatch(string(template), -1) {
		out[match[1]] = "string"
	}
	return out
}

// EstimateTokens approximates a token count as the number of whitespace-separated
// words. It is an estimate and is documented as one: the manifest has no tokeniser,
// and pretending to exactness would make a cost projection look authoritative when
// it is not.
func EstimateTokens(text string) int {
	return len(strings.Fields(text))
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// hashString is the digest the manifest records for a body of text.
func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
