package modulemanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// TestTheManifestCarriesEveryRequiredField reads the vendored contract rather than restating it.
func TestTheManifestCarriesEveryRequiredField(t *testing.T) {
	built, err := Build("0.4.0")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	encoded, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var emitted map[string]any
	if err := json.Unmarshal(encoded, &emitted); err != nil {
		t.Fatal(err)
	}

	schema := loadContract(t)
	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("the vendored contract has no required list")
	}
	for _, name := range required {
		if _, present := emitted[name.(string)]; !present {
			t.Fatalf("the manifest omits required field %q; it has %v", name, keysOf(emitted))
		}
	}
	properties := schema["properties"].(map[string]any)
	for name := range emitted {
		if _, allowed := properties[name]; !allowed {
			t.Fatalf("the manifest carries %q, which the contract does not permit", name)
		}
	}
}

// TestTheIdentityMatchesTheContractsPatterns checks the two fields a platform indexes on, since a manifest
// with a malformed id or version is rejected rather than repaired.
func TestTheIdentityMatchesTheContractsPatterns(t *testing.T) {
	schema := loadContract(t)
	properties := schema["properties"].(map[string]any)
	for field, value := range map[string]string{"module_id": ModuleID, "version": "0.4.0", "health_path": HealthPath} {
		pattern := properties[field].(map[string]any)["pattern"].(string)
		matched, err := match(pattern, value)
		if err != nil {
			t.Fatalf("%s: the contract's pattern does not compile: %v", field, err)
		}
		if !matched {
			t.Fatalf("%s = %q does not match the contract's pattern %s", field, value, pattern)
		}
	}
}

// TestItRefusesADevelopmentVersion pins the refusal that keeps the manifest from claiming an identity it does
// not have.
func TestItRefusesADevelopmentVersion(t *testing.T) {
	for _, version := range []string{"dev", "", "0.4.0+dirty", "v0.4.0", "0.4", "1.2.3-rc1"} {
		if _, err := Build(version); err == nil {
			t.Fatalf("version %q was accepted, and the contract forbids it", version)
		}
	}
}

// TestTheConsumedContractsAreTheVendoredOnes is the check that stops this file drifting from the vendor
// directory in either direction: a contract vendored but not declared would be invisible to the platform,
// and one declared but not vendored would be a claim about a file this repository does not have.
func TestTheConsumedContractsAreTheVendoredOnes(t *testing.T) {
	vendored, err := VendoredNames(VendorDir(filepath.Join("..", "..")))
	if err != nil {
		t.Fatalf("VendoredNames() error = %v", err)
	}
	declared := make([]string, 0, len(consumed))
	for _, contract := range consumed {
		declared = append(declared, contract.Name)
	}
	sort.Strings(vendored)
	sort.Strings(declared)
	if joined(vendored) != joined(declared) {
		t.Fatalf("vendored contracts %v\ndeclared consumed %v\nthe manifest and the vendor directory disagree",
			vendored, declared)
	}
	for _, contract := range consumed {
		if contract.Version == "" {
			t.Fatalf("%s is declared with no version", contract.Name)
		}
	}
}

// TestTheHealthPathIsTheOneWeServe keeps the manifest and the server in step. It is a small test for a
// small claim, and the claim is the one that made this contract look unimplementable: a manifest names an
// endpoint, and an endpoint nobody serves is worse than no manifest.
func TestTheHealthPathIsTheOneWeServe(t *testing.T) {
	built, err := Build("0.4.0")
	if err != nil {
		t.Fatal(err)
	}
	if built.HealthPath != "/healthz" {
		t.Fatalf("health_path = %q, but `review-router serve` answers /healthz", built.HealthPath)
	}
	if built.ContractsPublished == nil {
		t.Fatal("contracts_published is null; the contract types it as an array, and null is not an array")
	}
	if built.ContractsConsumed == nil || len(built.ContractsConsumed) == 0 {
		t.Fatal("contracts_consumed is empty, and the contract requires at least one")
	}
}

func loadContract(t *testing.T) map[string]any {
	t.Helper()
	path := filepath.Join(VendorDir(filepath.Join("..", "..")), Contract)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the vendored %s: %v", Contract, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatalf("the vendored %s is not JSON: %v", Contract, err)
	}
	return schema
}

func match(pattern, value string) (bool, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(value), nil
}

func keysOf(values map[string]any) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func joined(values []string) string {
	body, _ := json.Marshal(values)
	return string(body)
}
