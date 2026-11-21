#!/usr/bin/env python3
"""Verify that vendored ledger contracts match the digests in VENDORED.md.

A vendored contract must never be edited in place. If a digest does not match,
either the upstream contract changed and this repo must re-vendor, or somebody
edited a vendored file. Both are failures.
"""
import hashlib
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
VENDOR = os.path.join(ROOT, "docs", "05-contracts", "vendor")


def main():
    if not os.path.isdir(VENDOR):
        print("no vendored contracts; nothing to verify")
        return 0
    errors = []
    checked = 0
    for setname in sorted(os.listdir(VENDOR)):
        d = os.path.join(VENDOR, setname)
        if not os.path.isdir(d):
            continue
        manifest = os.path.join(d, "VENDORED.md")
        if not os.path.exists(manifest):
            errors.append("%s: missing VENDORED.md" % setname)
            continue
        text = open(manifest, encoding="utf-8").read()
        declared = re.search(r"\*\*Contract set version:\*\*\s*`?([0-9][^`\s]*)", text)
        if "@" in setname:
            name_version = setname.rsplit("@", 1)[1]
            if not declared:
                errors.append("%s: VENDORED.md does not state a contract set version" % setname)
            elif declared.group(1) != name_version:
                errors.append(
                    "%s: directory says %s but VENDORED.md says %s"
                    % (setname, name_version, declared.group(1))
                )
        recorded = dict(re.findall(r"\|\s*`([^`]+)`\s*\|\s*`([a-f0-9]{64})`\s*\|", text))
        if not recorded:
            errors.append("%s: VENDORED.md records no digests" % setname)
            continue
        for name, digest in sorted(recorded.items()):
            path = os.path.join(d, name)
            if not os.path.exists(path):
                errors.append("%s: %s recorded but missing" % (setname, name))
                continue
            actual = hashlib.sha256(open(path, "rb").read()).hexdigest()
            checked += 1
            if actual != digest:
                errors.append("%s: %s digest mismatch (recorded %s, actual %s)"
                              % (setname, name, digest[:12], actual[:12]))
    print("verified %d vendored file(s)" % checked)
    for e in errors:
        print("FAIL  " + e)
    if errors:
        return 1
    print("all vendored contracts match")
    return 0


if __name__ == "__main__":
    sys.exit(main())
