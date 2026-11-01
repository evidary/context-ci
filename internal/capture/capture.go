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
		return nil, fmt.Errorf("capture: %s does not exist: %w", rel, err)
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
	// A misspelled key is silently ignored by an unstructured decode, which hashes
	// null and leaves the real value outside the digest: a schema edit would then be
	// invisible. `schema` is deliberately `any`, so this rejects typos in the tools
	// file's own keys without constraining the JSON Schema inside.
	dec.KnownFields(true)
	if err := dec.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("capture: tools: %w", err)
	}
	// A second document in the same file used to be ignored, so the half of the file
	// that was read could disagree with the half that was written.
	var extra any
	if err := dec.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("capture: tools: %w", err)
	} else if err == nil {
		return nil, errors.New("capture: tools: the file holds more than one YAML document")
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

		schema := redactSecretValues(stripCredentials(t.Schema))
		canonical, err := json.Marshal(schema)
		if err != nil {
			return nil, fmt.Errorf("capture: tools: %q schema: %w", t.Name, err)
		}
		sum := sha256.Sum256(canonical)
		descriptionHash := sha256.Sum256([]byte(t.Description))
		out = append(out, manifest.Tool{
			Name:            t.Name,
			SchemaHash:      hex.EncodeToString(sum[:]),
			DescriptionHash: ptrTo(hex.EncodeToString(descriptionHash[:])),
			// Kept because a human reads it, and never used to detect a change: an edit
			// that preserves the word count leaves it identical.
			DescriptionTokens: EstimateTokens(t.Description),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func ptrTo(s string) *string { return &s }

// credentialKeys are field names whose values never enter a hash input.
var credentialKeys = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true,
	"api_key": true, "apikey": true, "access_key": true, "secret_key": true,
	"authorization": true, "auth": true, "credential": true, "credentials": true,
	"private_key": true, "client_secret": true, "session_token": true,
	"bearer": true, "cookie": true, "set_cookie": true,
	"passphrase": true, "connection_string": true, "subscription_key": true,
	// Spellings that camelCase or concatenation produces, and which an exact
	// match on the underscore form missed.
	"access_token": true, "refresh_token": true, "id_token": true,
	"auth_token": true, "bearer_token": true, "api_secret": true,
	"accesskey": true, "secretkey": true, "privatekey": true,
	"accesstoken": true, "refreshtoken": true, "idtoken": true,
	"authtoken": true, "bearertoken": true, "apisecret": true,
	"clientsecret": true, "subscriptionkey": true, "connectionstring": true,
}

// isCredentialKey reports whether a field name holds a secret. Names are compared
// with separators removed as well as exactly, so access_token, accessToken and
// access-token all match one entry.
//
// The match is still exact rather than a substring test on purpose: max_tokens,
// token_budget and secret_name are ordinary configuration, and removing them
// would quietly delete schema content from the hash input.
func isCredentialKey(name string) bool {
	lower := strings.ToLower(name)
	if credentialKeys[lower] {
		return true
	}
	if !strings.ContainsAny(lower, "_-.") {
		return false
	}
	return credentialKeys[strings.NewReplacer("_", "", "-", "", ".", "").Replace(lower)]
}

// stripCredentials removes credential-shaped fields from a decoded schema. It
// matches on key name rather than value shape, because a value-shape matcher
// produces false negatives that look like success.
func stripCredentials(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if isCredentialKey(k) {
				continue
			}
			out[k] = stripCredentials(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			name := fmt.Sprint(k)
			if isCredentialKey(name) {
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

// secretValuePatterns are shapes that are a credential whatever the field is called.
// Key-name matching cannot see a token stored under `example`, `default` or
// `sample`, and those are exactly where a copied-and-pasted real token ends up.
//
// This is a second line, not a replacement: a random secret with no recognisable
// prefix is still missed. The key-name pass catches the field; this catches the
// value that leaked into a field nobody thought to name.
var secretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),                     // provider API keys
	regexp.MustCompile(`\bsk_(live|test)_[A-Za-z0-9]{16,}`),           // payment processors
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),                        // AWS access key id
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}`),                // GitHub tokens
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),              // Slack tokens
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.`), // JWT
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),          // PEM private key
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),                   // Google API key
}

// redactedValue replaces a recognised secret. It is a fixed marker rather than a
// hash: a hash of a rotated token would change the digest on every rotation, which
// is the noise redaction exists to remove.
const redactedValue = "<redacted>"

// redactSecretValues walks a decoded schema and replaces any string that looks like
// a credential. The field itself stays, so the shape of the schema is still part of
// the digest and a schema change is still visible.
func redactSecretValues(v any) any {
	switch x := v.(type) {
	case string:
		for _, re := range secretValuePatterns {
			if re.MatchString(x) {
				return redactedValue
			}
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = redactSecretValues(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = redactSecretValues(item)
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
