package capture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/evidary/context-ci/internal/manifest"
)

// ParseTools reads tool definitions in any of the shapes a repository is likely to hold them in,
// from YAML or JSON:
//
//   - this tool's own: `tools: [{name, description, schema}]`
//   - a bare list, or a `tools:` list, of provider definitions:
//     `{type: function, function: {name, description, parameters}}` (chat completions),
//     `{type: function, name, description, parameters}` (the flattened form),
//     `{name, description, input_schema}`, and `{functionDeclarations: [...]}`
//   - a saved MCP tools/list response: `{tools: [{name, description, inputSchema, ...}]}`, with or
//     without the JSON-RPC envelope around it
//
// A tool is its name, its description and everything else. The description is hashed alone,
// because it is text the model reads as instructions. Everything else — the schema under whichever
// key it arrived, and any other field such as strict, annotations or an output schema — is hashed
// together, so no field of a definition can change without the hash changing.
//
// The file is still read strictly where a mistake would hide a tool: a top-level key that is not
// part of any of these shapes is refused, and so is a tool with no name.
func ParseTools(data []byte) ([]manifest.Tool, error) {
	documents, err := decodeYAML(data)
	if err != nil {
		return nil, err
	}
	if len(documents) > 1 {
		return nil, errors.New("the file holds more than one YAML document")
	}
	var items []any
	if len(documents) == 1 {
		if items, err = toolItems(documents[0]); err != nil {
			return nil, err
		}
	}

	out := make([]manifest.Tool, 0, len(items))
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		definitions, err := toolDefinitions(item)
		if err != nil {
			return nil, fmt.Errorf("tool %d: %w", i+1, err)
		}
		for _, d := range definitions {
			if seen[d.name] {
				return nil, fmt.Errorf("%q appears twice", d.name)
			}
			seen[d.name] = true
			canonical, err := json.Marshal(redact(d.rest, false))
			if err != nil {
				return nil, fmt.Errorf("%q: %w", d.name, err)
			}
			sum := sha256.Sum256(canonical)
			descriptionHash := hashString(d.description)
			hidden, _ := HiddenCharacters(d.description)
			out = append(out, manifest.Tool{
				Name:            d.name,
				SchemaHash:      hex.EncodeToString(sum[:]),
				DescriptionHash: &descriptionHash,
				// Kept because a human reads it, and never used to detect a change: an edit
				// that preserves the word count leaves it identical.
				DescriptionTokens: EstimateTokens(d.description),
				HiddenCharacters:  hidden,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// envelopeKeys are the top-level keys a tools file may carry besides the list: the JSON-RPC
// envelope of a saved tools/list response and its paging fields.
var envelopeKeys = map[string]bool{"tools": true, "result": true, "jsonrpc": true, "id": true, "nextCursor": true, "_meta": true}

func toolItems(document any) ([]any, error) {
	switch doc := document.(type) {
	case nil:
		return nil, nil
	case []any:
		return doc, nil
	case map[string]any:
		if len(doc) == 0 {
			return nil, nil
		}
		if result, ok := doc["result"].(map[string]any); ok {
			doc = result
		}
		for key := range doc {
			if !envelopeKeys[key] {
				return nil, fmt.Errorf("top-level key %q is not part of a tools file; the tool definitions go in a list under `tools`", key)
			}
		}
		// A page of a longer list would be compared as if it were the whole list, and every tool
		// on the pages that were not saved would be invisible.
		if cursor, _ := doc["nextCursor"].(string); cursor != "" {
			return nil, errors.New("this is one page of a longer tools/list response (nextCursor is set); save every page's tools in one list")
		}
		switch tools := doc["tools"].(type) {
		case nil:
			if _, present := doc["tools"]; !present {
				return nil, errors.New("the file has no `tools` list")
			}
			return nil, nil
		case []any:
			return tools, nil
		default:
			return nil, errors.New("`tools` is not a list")
		}
	}
	return nil, errors.New("the file is neither a list of tools nor a mapping with a `tools` list")
}

type toolDefinition struct {
	name, description string
	rest              map[string]any
}

// schemaKeys are the names the parameter schema goes by. It is stored under one name whichever it
// arrived as, so converting a file from one shape to another is not reported as a schema change.
var schemaKeys = []string{"schema", "parameters", "input_schema", "inputSchema", "parametersJsonSchema"}

func toolDefinitions(item any) ([]toolDefinition, error) {
	body, ok := item.(map[string]any)
	if !ok {
		return nil, errors.New("a tool definition is a mapping")
	}
	for _, key := range []string{"functionDeclarations", "function_declarations"} {
		declarations, present := body[key]
		if !present {
			continue
		}
		list, ok := declarations.([]any)
		if !ok {
			return nil, fmt.Errorf("%s is not a list", key)
		}
		var out []toolDefinition
		for _, declaration := range list {
			inner, err := toolDefinitions(declaration)
			if err != nil {
				return nil, err
			}
			out = append(out, inner...)
		}
		return out, nil
	}

	rest := make(map[string]any, len(body))
	for k, v := range body {
		rest[k] = v
	}
	// The nested form keeps the definition under `function`; lifting it makes both forms one.
	if function, ok := body["function"].(map[string]any); ok {
		delete(rest, "function")
		for k, v := range function {
			rest[k] = v
		}
	}

	name, _ := rest["name"].(string)
	if name == "" {
		// A provider-hosted tool is named by its type and has no schema of its own.
		if kind, _ := rest["type"].(string); kind != "function" {
			name = kind
		}
	}
	if name == "" {
		return nil, errors.New("every tool needs a name")
	}
	description := ""
	switch d := rest["description"].(type) {
	case nil:
	case string:
		description = d
	default:
		return nil, fmt.Errorf("%q: description is not a string", name)
	}
	delete(rest, "name")
	delete(rest, "description")

	var schema any
	found := ""
	for _, key := range schemaKeys {
		value, present := rest[key]
		if !present {
			continue
		}
		if found != "" {
			return nil, fmt.Errorf("%q has both %s and %s; a tool has one parameter schema", name, found, key)
		}
		found, schema = key, value
		delete(rest, key)
	}
	rest["schema"] = schema
	return []toolDefinition{{name: name, description: description, rest: rest}}, nil
}

// credentialKeys are field names whose string values never enter a hash input.
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
// token_budget and secret_name are ordinary configuration.
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

// valueKeys are where a JSON Schema carries a literal value for the property it describes.
var valueKeys = map[string]bool{"default": true, "example": true, "examples": true, "const": true, "enum": true}

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

// redact replaces credential values in a decoded definition before it is hashed, so that rotating
// a token somebody left in a schema is not reported as a change to the tool.
//
// Three things are replaced: a string that looks like a credential wherever it is, a string stored
// directly under a credential-named key, and the literal values (default, example, const, enum) of
// a property with a credential name. Nothing is removed. An earlier version dropped every
// credential-named key, which also dropped the definition of any parameter called `token` or
// `auth`, so adding or changing such a parameter was invisible to the gate.
func redact(v any, credentialProperty bool) any {
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
			switch {
			case isCredentialKey(k) && isString(val):
				out[k] = redactedValue
			case credentialProperty && valueKeys[k]:
				out[k] = redactStrings(val)
			default:
				out[k] = redact(val, isCredentialKey(k))
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = redact(item, false)
		}
		return out
	default:
		return x
	}
}

func isString(v any) bool {
	_, ok := v.(string)
	return ok
}

// redactStrings replaces every string below v, keeping the shape.
func redactStrings(v any) any {
	switch x := v.(type) {
	case string:
		return redactedValue
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = redactStrings(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = redactStrings(val)
		}
		return out
	default:
		return x
	}
}
