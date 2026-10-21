#!/usr/bin/env python3
"""Repository documentation checks.

Fails the build on:
  1. a broken internal markdown link
  2. a malformed markdown table (row pipe count != header pipe count)
  3. a file that is not valid UTF-8, has a BOM, or contains mojibake
  4. a markdown file under docs/ that is not referenced from docs/00-index.md
  5. a contract schema whose checksum does not match CHECKSUMS.txt (when present)

No third-party dependencies. Run from the repository root:
    python tools/check_docs.py
"""
import hashlib
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SKIP_DIRS = {".git", "node_modules", "dist", "build", ".venv"}
LINK = re.compile(r"(!?)\[[^\]]*\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)")
TABLE_ROW = re.compile(r"^\s*\|.*\|\s*$")
TABLE_SEP = re.compile(r"^\s*\|[\s:\-|]+\|\s*$")
MOJIBAKE = ("\u00e2\u20ac", "\u00c3\u00a2", "\u00ef\u00bf\u00bd")

# Paths that are allowed to exist without being listed in docs/00-index.md
INDEX_EXEMPT = {"docs/05-contracts/vendor"}

errors = []


def walk():
    for base, dirs, files in os.walk(ROOT):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if f.endswith((".md", ".json", ".yml", ".yaml", ".txt", ".csv")):
                yield os.path.join(base, f)


def rel(p):
    return os.path.relpath(p, ROOT).replace("\\", "/")


def check_encoding(path):
    raw = open(path, "rb").read()
    if raw[:3] == b"\xef\xbb\xbf":
        errors.append("%s: UTF-8 BOM present" % rel(path))
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as e:
        errors.append("%s: not valid UTF-8 (%s)" % (rel(path), e))
        return None
    for m in MOJIBAKE:
        if m in text:
            errors.append("%s: mojibake sequence %r" % (rel(path), m))
    return text


def check_links(path, text):
    base = os.path.dirname(path)
    for m in LINK.finditer(text):
        target = m.group(2)
        if target.startswith(("http://", "https://", "mailto:", "#")):
            continue
        clean = target.split("#")[0]
        if not clean:
            continue
        if not os.path.exists(os.path.normpath(os.path.join(base, clean))):
            errors.append("%s: broken link -> %s" % (rel(path), target))


def check_tables(path, text):
    lines = text.splitlines()
    for i, line in enumerate(lines):
        if not TABLE_ROW.match(line):
            continue
        if i == 0 or not TABLE_SEP.match(lines[i - 1]):
            continue
        header = lines[i - 1].strip().strip("|").count("|") + 1
        row = line.strip().strip("|").count("|") + 1
        if row != header:
            errors.append("%s:%d: table has %d cells, header has %d" % (rel(path), i + 1, row, header))


def check_index_coverage(md_files):
    index = os.path.join(ROOT, "docs", "00-index.md")
    if not os.path.exists(index):
        errors.append("docs/00-index.md is missing")
        return
    text = open(index, encoding="utf-8").read()
    for p in md_files:
        r = rel(p)
        if not r.startswith("docs/") or r == "docs/00-index.md":
            continue
        if any(r.startswith(x) for x in INDEX_EXEMPT):
            continue
        if os.path.basename(p) not in text:
            errors.append("%s: not referenced from docs/00-index.md" % r)


def check_contracts():
    cdir = os.path.join(ROOT, "docs", "05-contracts")
    sums = os.path.join(cdir, "CHECKSUMS.txt")
    if not os.path.exists(sums):
        return
    for line in open(sums, encoding="utf-8"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        digest, name = line.split(None, 1)
        name = name.strip()
        target = os.path.normpath(os.path.join(cdir, name))
        if not os.path.exists(target):
            errors.append("CHECKSUMS.txt references missing file %s" % name)
            continue
        actual = hashlib.sha256(open(target, "rb").read()).hexdigest()
        if actual != digest:
            errors.append("%s: checksum mismatch (expected %s, got %s)" % (name, digest[:12], actual[:12]))
    sdir = os.path.join(cdir, "schemas")
    if os.path.isdir(sdir):
        for f in sorted(os.listdir(sdir)):
            if f.endswith(".json"):
                try:
                    json.load(open(os.path.join(sdir, f), encoding="utf-8"))
                except json.JSONDecodeError as e:
                    errors.append("schemas/%s: invalid JSON (%s)" % (f, e))


def main():
    md_files = []
    total = 0
    for path in walk():
        total += 1
        text = check_encoding(path)
        if text is None:
            continue
        if path.endswith(".md"):
            md_files.append(path)
            check_links(path, text)
            check_tables(path, text)
    check_index_coverage(md_files)
    check_contracts()

    print("checked %d files, %d markdown" % (total, len(md_files)))
    for e in errors:
        print("FAIL  " + e)
    if errors:
        print("\n%d problem(s)" % len(errors))
        return 1
    print("all documentation checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
