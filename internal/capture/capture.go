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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/context-ci/context-ci/internal/config"
	"github.com/context-ci/context-ci/internal/manifest"
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
}

// ErrOutsideRoot is returned for a source path that escapes the repository.
type ErrOutsideRoot struct{ Path string }

func (e ErrOutsideRoot) Error() string {
	return fmt.Sprintf("capture: source path %q resolves outside the repository", e.Path)
}

// Build produces a manifest for the checkout at root.
func Build(cfg *config.Config, root string, opts Options) (*manifest.Manifest, error) {
	if cfg == nil {
		return nil, errors.New("capture: no configuration")
	}
	if opts.Commit == "" {
		return nil, errors.New("capture: commit is required; a manifest without one cannot be reproduced")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	hashes := make(map[string]string, len(cfg.Capture.Sources))
	for kind, rel := range cfg.Capture.Sources {
		sum, err := hashFile(root, rel)
		if err != nil {
			return nil, fmt.Errorf("capture: source %s: %w", kind, err)
		}
		hashes[kind] = sum
	}

	m := &manifest.Manifest{
		ManifestVersion: manifest.Version,
		CreatedAt:       opts.Now.UTC().Format(time.RFC3339),
		Commit:          opts.Commit,
		Capture: manifest.Capture{
			Path:       cfg.Capture.Path,
			SDKVersion: opts.SDKVersion,
		},
		Model: manifest.Model{
			Provider: cfg.Model.Provider,
			ID:       cfg.Model.ID,
			AliasOf:  optional(cfg.Model.AliasOf),
			Params: manifest.ModelParam{
				Temperature: optional(string(cfg.Model.Params.Temperature)),
				MaxTokens:   cfg.Model.Params.MaxTokens,
				Seed:        optional(string(cfg.Model.Params.Seed)),
				TopP:        optional(string(cfg.Model.Params.TopP)),
			},
		},
		Prompt: manifest.Prompt{
			TemplateHash: hashes[config.SourcePrompt],
			RenderedHash: hashes[config.SourcePrompt],
			Variables:    map[string]string{},
		},
		Tools: []manifest.Tool{},
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

	if rendered, ok := hashes[config.SourceRendered]; ok {
		m.Prompt.RenderedHash = rendered
		m.Prompt.RenderedSampleHash = &rendered
	} else {
		// No rendered sample was supplied. Saying the rendered hash equals the
		// template hash is true for an unrendered template and has to be stated,
		// because a reader who assumed it was rendered would over-trust the result.
		m.Capture.Reason = "no rendered sample supplied, so rendered_hash equals template_hash"
	}

	templatePath := cfg.Capture.Sources[config.SourcePrompt]
	variables, err := variablesIn(root, templatePath)
	if err != nil {
		return nil, err
	}
	m.Prompt.Variables = variables

	if toolsPath, ok := cfg.Capture.Sources[config.SourceTools]; ok {
		tools, err := readTools(root, toolsPath)
		if err != nil {
			return nil, err
		}
		m.Tools = tools
	}

	if _, err := m.Compute(); err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	return m, nil
}

// hashFile hashes a file's content with line endings normalised to LF.
func hashFile(root, rel string) (string, error) {
	data, err := readInsideRoot(root, rel)
	if err != nil {
		return "", err
	}
	return HashNormalised(data), nil
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
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return nil, ErrOutsideRoot{Path: rel}
	}
	return os.ReadFile(resolved)
}

var variablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\}\}`)

// variablesIn lists the template placeholders, sorted, with no values. The manifest
// records the shape of the variable set and not the data, so changing a value does
// not change the digest but changing the set does.
func variablesIn(root, rel string) (map[string]string, error) {
	data, err := readInsideRoot(root, rel)
	if err != nil {
		return nil, fmt.Errorf("capture: prompt: %w", err)
	}
	out := map[string]string{}
	for _, match := range variablePattern.FindAllStringSubmatch(string(data), -1) {
		out[match[1]] = "string"
	}
	return out, nil
}

// toolsFile is the shape of capture.sources.tools.
type toolsFile struct {
	Tools []toolEntry `yaml:"tools"`
}

type toolEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Schema      any    `yaml:"schema"`
}

// readTools parses a tools file into manifest entries. Each schema is hashed after
// credential-shaped fields are removed, so a schema carrying a token in a default
// value cannot put that token into a committed manifest.
func readTools(root, rel string) ([]manifest.Tool, error) {
	data, err := readInsideRoot(root, rel)
	if err != nil {
		return nil, fmt.Errorf("capture: tools: %w", err)
	}
	var parsed toolsFile
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("capture: tools: %w", err)
	}
	out := make([]manifest.Tool, 0, len(parsed.Tools))
	seen := make(map[string]bool, len(parsed.Tools))
	for _, t := range parsed.Tools {
		if t.Name == "" {
			return nil, errors.New("capture: tools: every tool needs a name")
		}
		if seen[t.Name] {
			return nil, fmt.Errorf("capture: tools: %q appears twice", t.Name)
		}
		seen[t.Name] = true

		schema := stripCredentials(t.Schema)
		canonical, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("capture: tools: %q schema: %w", t.Name, err)
		}
		sum := sha256.Sum256(canonical)
		out = append(out, manifest.Tool{
			Name:              t.Name,
			SchemaHash:        hex.EncodeToString(sum[:]),
			DescriptionTokens: EstimateTokens(t.Description),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// credentialKeys are field names whose values never enter a hash input.
var credentialKeys = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true,
	"api_key": true, "apikey": true, "access_key": true, "secret_key": true,
	"authorization": true, "auth": true, "credential": true, "credentials": true,
	"private_key": true, "client_secret": true, "session_token": true,
	"bearer": true, "cookie": true, "set_cookie": true,
}

// stripCredentials removes credential-shaped fields from a decoded schema. It
// matches on key name rather than value shape, because a value-shape matcher
// produces false negatives that look like success.
func stripCredentials(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if credentialKeys[strings.ToLower(k)] {
				continue
			}
			out[k] = stripCredentials(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			name := fmt.Sprint(k)
			if credentialKeys[strings.ToLower(name)] {
				continue
			}
			out[name] = stripCredentials(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = stripCredentials(item)
		}
		return out
	default:
		return x
	}
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
