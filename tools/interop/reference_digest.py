#!/usr/bin/env python3
"""Recompute a context-ci manifest digest without using the Go code.

This exists so that the pinned digest in internal/manifest/manifest_test.go is a
claim two implementations agree on, not a number the implementation produced and
then remembered.

The digest is sha256 over the RFC 8785 canonical form of the manifest with
`created_at`, `commit`, `digest` and `content` removed.

Usage:
    python tools/interop/reference_digest.py            # check the fixture digest
    python tools/interop/reference_digest.py <file>     # digest a manifest file
"""

import hashlib
import json
import sys

# The manifest produced by sample() in internal/manifest/manifest_test.go, with the
# excluded fields already absent.
SAMPLE = {
    "manifest_version": 2,
    "capture": {"path": "asserted"},
    "model": {
        "provider": "openai",
        "id": "gpt-x",
        "alias_of": "gpt-x-latest",
        "params": {"temperature": "0", "max_tokens": 1024},
    },
    "prompt": {
        "template_hash": "a" * 64,
        "rendered_hash": "b" * 64,
        "rendered_sample_hash": None,
        "variables": {"locale": "string", "tone": "string"},
    },
    "tools": [
        {
            "name": "refund.create",
            "schema_hash": "c" * 64,
            "description_hash": "e" * 64,
            "description_tokens": 120,
        },
        {
            "name": "ticket.read",
            "schema_hash": "d" * 64,
            "description_hash": "f" * 64,
            "description_tokens": 40,
        },
    ],
    "files": [
        {"group": "agent.instructions", "path": "AGENTS.md", "hash": "1" * 64},
        {
            "group": "prompts",
            "path": "prompts/tone.md",
            "hash": "2" * 64,
            "hidden_characters": 3,
        },
    ],
    "retrieval": {
        "index_id": "kb-prod",
        "embedding_model": "embed-3",
        "embedding_model_version": "2026-05",
        "chunker": "recursive-800",
        "top_k": 5,
        "filters_hash": "f" * 64,
    },
    "policy": {"version": "2026.11", "guardrails_hash": "9" * 64},
}

EXPECTED = "41e9c2f3626047504fbbfc2c36b429203befebe4f770d1446f5c030e8caeb5b1"

EXCLUDED = ("created_at", "commit", "digest", "content")


def _utf16_key(s):
    return s.encode("utf-16-be")


def canonicalise(value):
    """RFC 8785, restricted to what a manifest may contain.

    Keys sort by UTF-16 code unit, not by byte: the two order differently above
    U+FFFF, and the Go implementation sorts the same way. Floats are refused rather
    than formatted, because ECMAScript number formatting is the part of JCS that
    implementations disagree about and a manifest has no need for it.
    """
    if value is None:
        return b"null"
    if value is True:
        return b"true"
    if value is False:
        return b"false"
    if isinstance(value, int):
        return str(value).encode("ascii")
    if isinstance(value, float):
        raise TypeError("floats are not permitted in a canonicalised manifest")
    if isinstance(value, str):
        out = ['"']
        for ch in value:
            if ch == '"':
                out.append('\\"')
            elif ch == "\\":
                out.append("\\\\")
            elif ch == "\n":
                out.append("\\n")
            elif ch == "\r":
                out.append("\\r")
            elif ch == "\t":
                out.append("\\t")
            elif ch == "\b":
                out.append("\\b")
            elif ch == "\f":
                out.append("\\f")
            elif ord(ch) < 0x20:
                out.append("\\u%04x" % ord(ch))
            else:
                out.append(ch)
        out.append('"')
        return "".join(out).encode("utf-8")
    if isinstance(value, list):
        return b"[" + b",".join(canonicalise(v) for v in value) + b"]"
    if isinstance(value, dict):
        keys = sorted(value.keys(), key=_utf16_key)
        parts = []
        for k in keys:
            parts.append(canonicalise(k) + b":" + canonicalise(value[k]))
        return b"{" + b",".join(parts) + b"}"
    raise TypeError("cannot canonicalise %r" % type(value))


def digest_of(document):
    body = {k: v for k, v in document.items() if k not in EXCLUDED}
    return hashlib.sha256(canonicalise(body)).hexdigest()


def main(argv):
    if len(argv) > 1:
        with open(argv[1], "r", encoding="utf-8") as fh:
            document = json.load(fh)
        print(digest_of(document))
        return 0

    got = digest_of(SAMPLE)
    if got != EXPECTED:
        print("MISMATCH")
        print("  computed %s" % got)
        print("  pinned   %s" % EXPECTED)
        return 1
    print("agrees with the pinned Go digest: %s" % got)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
