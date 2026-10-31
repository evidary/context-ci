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
MOJIBAKE = (
    "\u00e2\u20ac",  # a UTF-8 em dash or quote decoded as cp1252
    "\u00c3\u00a2",  # the same, double-encoded
    "\u00ef\u00bf\u00bd",  # U+FFFD, a replacement character already in the text
    "\u00c2\u00a0",  # a non-breaking space decoded as cp1252
    "\u00e2\u0080",  # the cp1252 byte pair left by a BOM or a smart quote
    "\u00c3\u00a9",  # an accented letter decoded as cp1252
)

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
    """Resolve every local link, including its fragment.

    The previous version used the text inside the parentheses as the filename, so
    `[x](<path with spaces>)` was looked up with its angle brackets and reported broken
    although the file exists, and `[q](real.md?x=1)` was reported broken for a file that
    exists. A folder full of false positives is a rule people stop reading, so both are
    handled. Fragments were not checked at all; a link to a heading that no longer exists
    is broken in the same way a missing file is, so it is now reported too.
    """
    base = os.path.dirname(path)
    for m in LINK.finditer(text):
        target = m.group(2).strip()
        if target.startswith(("http://", "https://", "mailto:")):
            continue
        # Angle brackets are markdown's way of allowing spaces in a path.
        if target.startswith("<") and target.endswith(">"):
            target = target[1:-1]
        target = target.split("?", 1)[0]
        file_part, _, fragment = target.partition("#")

        if file_part:
            resolved = os.path.normpath(os.path.join(base, file_part))
            if not os.path.exists(resolved):
                errors.append("%s: broken link -> %s" % (rel(path), m.group(2)))
                continue
        else:
            resolved = path

        if fragment and resolved.endswith(".md") and os.path.exists(resolved):
            if fragment.lower() not in anchors_in(resolved):
                errors.append("%s: link to a heading that does not exist -> %s"
                              % (rel(path), m.group(2)))


def slugify_heading(line):
    """Turn a markdown heading into the anchor form GitHub generates.

    Lowercase, drop anything that is not a letter, digit, space, hyphen or underscore,
    then join words with hyphens. Close enough for this repository's headings, which is
    where the check is applied.
    """
    text = line.strip().lstrip("#").strip().lower()
    text = re.sub(r"[^\w\s-]", "", text, flags=re.UNICODE)
    text = re.sub(r"\s+", "-", text.strip())
    return text




def anchors_in(path):
    found = set()
    for line in open(path, encoding="utf-8"):
        if line.lstrip().startswith("#"):
            found.add(slugify_heading(line))
    return found




def check_tables(path, text):
    """Every row of a table must have the same cell count as its header.

    The previous version compared the first data row against the separator line and
    skipped every row after it, so a table could lose a cell anywhere except the first
    row and the gate stayed quiet. It now tracks the header for the whole block.
    """
    lines = text.splitlines()

    def cells(line):
        return line.strip().strip("|").count("|") + 1

    i = 0
    while i < len(lines):
        if not (TABLE_ROW.match(lines[i]) and i + 1 < len(lines) and TABLE_SEP.match(lines[i + 1])):
            i += 1
            continue
        header = cells(lines[i])
        j = i + 2
        while j < len(lines) and TABLE_ROW.match(lines[j]):
            if cells(lines[j]) != header:
                errors.append("%s:%d: table has %d cells, header has %d"
                              % (rel(path), j + 1, cells(lines[j]), header))
            j += 1
        i = j


def check_index_coverage(md_files):
    """Every doc must be linked from the index by its path, not merely mentioned.

    The rule used to accept a basename appearing anywhere in the index, so a file
    called code-map.md was considered covered if the words appeared in a sentence, and
    a file sharing a name with another was covered by that other file's entry. It now
    requires the path from docs/ to appear.
    """
    index = os.path.join(ROOT, "docs", "00-index.md")
    if not os.path.exists(index):
        errors.append("docs/00-index.md is missing")
        return
    text = open(index, encoding="utf-8").read()
    for p in md_files:
        r = rel(p)
        if not r.startswith("docs/") or r == "docs/00-index.md":
            continue
        if any(r.startswith(x.rstrip("/") + "/") for x in INDEX_EXEMPT):
            continue
        from_docs = os.path.relpath(p, os.path.join(ROOT, "docs")).replace(os.sep, "/")
        if from_docs not in text:
            errors.append("%s: not linked from docs/00-index.md (expected a reference to %s)"
                          % (r, from_docs))


def check_contracts():
    cdir = os.path.join(ROOT, "docs", "05-contracts")
    sdir = os.path.join(cdir, "schemas")
    sums = os.path.join(cdir, "CHECKSUMS.txt")
    schemas = sorted(f for f in os.listdir(sdir)) if os.path.isdir(sdir) else []
    schemas = [f for f in schemas if f.endswith(".json")]

    if not os.path.exists(sums):
        # The gate used to return here, so a repository with schemas and no checksums at
        # all passed the check whose whole purpose is to notice that.
        if schemas:
            errors.append("docs/05-contracts/schemas exists but CHECKSUMS.txt does not")
        return

    listed = set()
    for line in open(sums, encoding="utf-8"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 1)
        if len(parts) != 2:
            # A malformed line used to raise ValueError and abort the whole run with a
            # traceback, which reads as a tooling bug rather than a failure.
            errors.append("CHECKSUMS.txt: cannot parse %r" % line)
            continue
        digest, name = parts[0], parts[1].strip()
        listed.add(os.path.basename(name))
        target = os.path.normpath(os.path.join(cdir, name))
        if not os.path.exists(target):
            errors.append("CHECKSUMS.txt references missing file %s" % name)
            continue
        actual = hashlib.sha256(open(target, "rb").read()).hexdigest()
        if actual != digest:
            errors.append("%s: checksum mismatch (expected %s, got %s)" % (name, digest[:12], actual[:12]))

    # A schema that is not listed was never checked, which is how a published contract
    # could change without any gate noticing.
    for f in schemas:
        if f == "index.json":
            continue
        if f not in listed:
            errors.append("schemas/%s is not listed in CHECKSUMS.txt" % f)
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
