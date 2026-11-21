// Package modulemanifest emits acp-core `module-manifest.v1.json`: what a platform needs in order to discover this
// module, check its health and know which contracts it speaks.
//
// The contract requires `health_path`, and it requires it for a good reason — a platform probes
// the tools it discovers the same way it probes itself. That requirement is what kept this file from existing: the product
// is a command-line tool, so it had no endpoint to name, and writing `/healthz` into a manifest without
// serving one would be a claim that a probe against it succeeds. The answer was not to weaken the contract
// or to fake the field, but to serve the endpoint (`context-ci serve`), which is why `module-manifest` and
// `serve` arrived together.
package modulemanifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Contract names the schema this package emits.
const Contract = "module-manifest.v1.json"

// ModuleID is this module's identity in the platform's namespace.
const ModuleID = "context-ci"

// DisplayName is what a human sees in a module list.
const DisplayName = "Context CI"

// Licence is the module's licence, as published in its repository.
const Licence = "Apache-2.0"

// Repository is where the source lives.
const Repository = "https://github.com/sigilary/context-ci"

// HealthPath is served by `context-ci serve` and is the value this manifest declares.
const HealthPath = "/healthz"

// ReadyPath is the readiness probe, kept separate from liveness because the platform applies that split to
// itself. This module has no store to wait for, so `context-ci serve` answers it with `stateless: true`.
const ReadyPath = "/readyz"

// semver is the form the contract accepts for a version. It has no room for a pre-release or a build
// suffix, which is deliberate: a manifest declares a released identity.
var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// versioned is one contract this module speaks, at one version.
type versioned struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Manifest is the contract shape. `additionalProperties` is false there, so nothing else is emitted.
type Manifest struct {
	ModuleID           string      `json:"module_id"`
	DisplayName        string      `json:"display_name"`
	Version            string      `json:"version"`
	Licence            string      `json:"licence"`
	Repository         string      `json:"repository,omitempty"`
	ContractsConsumed  []versioned `json:"contracts_consumed"`
	ContractsPublished []versioned `json:"contracts_published"`
	Entitlements       []string    `json:"entitlements"`
	HealthPath         string      `json:"health_path"`
}

// consumed is what this module reads from the platform: every contract it vendors, at the version the
// vendor table records. The conformance test compares this list against the files actually in
// docs/05-contracts/vendor, so a contract added to the vendor directory without being declared here — or
// declared without being vendored — breaks the test rather than producing a manifest that lies about what
// the module speaks.
var consumed = []versioned{
	{"cost-event.v1.json", "1.0.0"},
	{"module-manifest.v1.json", "1.0.0"},
}

// published is empty, and that is the honest value rather than an oversight. The context manifest
// has no published JSON Schema — docs/05-contracts/README.md §3 says so — and `contracts_published` is a
// claim that a consumer can fetch a schema. An empty list states that correctly; a version with no schema
// behind it would not.
var published = []versioned{}

// entitlements are the capabilities this module provides to a platform. They are names, not enforcements:
// the platform decides who holds them, and nothing in this repository authenticates them yet.
//
// The list is what the binary does and nothing else. It used to advertise `cost.attribute` as well, and no
// verb attributes cost: `attribute` compares eval scores and pass/fail, the `cost_usd` field of an eval case
// is read and never used, and no cost-event is emitted. An entitlement is a name a platform may bill or
// route on, so one with nothing behind it is a claim, not a placeholder.
var entitlements = []string{"context.check"}

// Build renders the manifest for a released version.
//
// A version that is not `x.y.z` is refused. The contract's pattern has no room for `dev` or a `+dirty`
// suffix, and both mean "this binary is not a release" — so the honest answer is that it has no identity to
// declare, not a cleaned-up string that would claim one.
func Build(version string) (Manifest, error) {
	if !semver.MatchString(version) {
		return Manifest{}, fmt.Errorf("manifest: version %q is not x.y.z, so this build has no released "+
			"identity to declare; a manifest names a release, and a build without one cannot", version)
	}
	return Manifest{
		ModuleID:          ModuleID,
		DisplayName:       DisplayName,
		Version:           version,
		Licence:           Licence,
		Repository:        Repository,
		ContractsConsumed: append(make([]versioned, 0, len(consumed)), consumed...),
		// make, not append onto nil: an empty slice of nil marshals as `null`, and the contract types this
		// field as an array. Nothing published is `[]`, not `null`.
		ContractsPublished: append(make([]versioned, 0, len(published)), published...),
		Entitlements:       append([]string(nil), entitlements...),
		HealthPath:         HealthPath,
	}, nil
}

// VendoredNames lists the contract files actually present in the vendor directory, so the manifest's claims
// can be compared with the repository's contents rather than with its prose.
func VendoredNames(vendorDir string) ([]string, error) {
	entries, err := os.ReadDir(vendorDir)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return nil, errors.New("manifest: the vendor directory holds no contracts, so this repository " +
			"speaks nothing and a manifest would be a fiction")
	}
	return names, nil
}

// VendorDir is where this repository keeps its vendored contracts.
func VendorDir(root string) string {
	return filepath.Join(root, "docs", "05-contracts", "vendor", "acp-core@1.0.2")
}
