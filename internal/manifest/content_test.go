package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestContentBodyMustMatchItsHash closes the hole the digest deliberately leaves.
//
// The manifest digest covers everything except created_at, digest and content, on
// the reasoning that content is redundant with its hash. That reasoning only holds
// if the redundancy is checked. Nothing checked it: a manifest whose body said one
// thing and whose hash said another validated cleanly, so a reviewer reading the
// body saw text the model was never given.
func TestContentBodyMustMatchItsHash(t *testing.T) {
	m := Content{
		Kind: "prompt",
		Hash: sha256Hex("the real prompt"),
		Body: "the real prompt",
	}
	t.Run("a matching body is accepted", func(t *testing.T) {
		if err := m.Validate(); err != nil {
			t.Fatalf("a matching body was rejected: %v", err)
		}
	})

	t.Run("an edited body is rejected", func(t *testing.T) {
		tampered := m
		tampered.Body = "something else entirely"
		err := tampered.Validate()
		if err == nil {
			t.Fatal("a body that does not match its hash was accepted")
		}
	})
}

func TestContentValidation(t *testing.T) {
	good := Content{Kind: "prompt", Hash: sha256Hex("x"), Body: "x"}

	t.Run("an unknown kind is rejected", func(t *testing.T) {
		c := good
		c.Kind = "whatever"
		if err := c.Validate(); err == nil {
			t.Fatal("an unknown content kind was accepted")
		}
	})

	t.Run("a malformed hash is rejected", func(t *testing.T) {
		c := good
		c.Hash = "not-a-hash"
		if err := c.Validate(); err == nil {
			t.Fatal("a malformed content hash was accepted")
		}
	})
}

// TestManifestRejectsTamperedContent is the end-to-end version: the digest is
// computed over a document that excludes content, so the content check is the only
// thing standing between a reader and a lie.
func TestManifestRejectsTamperedContent(t *testing.T) {
	m := sample()
	m.Content = []Content{{Kind: "prompt", Hash: sha256Hex("real"), Body: "real"}}
	if _, err := m.Compute(); err != nil {
		t.Fatalf("compute: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("a valid manifest with content was rejected: %v", err)
	}

	m.Content[0].Body = "a prompt nobody sent"
	if err := m.Validate(); err == nil {
		t.Fatal("a manifest with a tampered content body validated")
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
