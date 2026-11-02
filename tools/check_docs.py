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
MALFORMED_LINK = re.compile(r"\[[^\]\n]*\.(?:md|csv|json|ya?ml)[^\]\n]*\)")
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


def mask_code(text):
    """Blank out fenced blocks and inline code spans, keeping line structure intact.

    A `[label](path)` inside backticks is shown, not followed. Masking it keeps the line
    count the same so reported line numbers stay correct, and lets the two link checks run
    over the prose only.
    """
    out = []
    fenced = False
    for line in text.splitlines(True):
        stripped = line.lstrip()
        if stripped.startswith("```") or stripped.startswith("~~~"):
            fenced = not fenced
            out.append("\n" if line.endswith("\n") else "")
            continue
        if fenced:
            out.append("\n" if line.endswith("\n") else "")
            continue
        # Inline spans: replace each `...` run with spaces of the same length.
        masked = re.sub(r"`[^`\n]*`", lambda m: " " * len(m.group(0)), line)
        out.append(masked)
    return "".join(out)


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
    for m in LINK.finditer(mask_code(text)):
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



# The punctuation github-slugger removes before turning spaces into hyphens: the general
# punctuation block, the supplemental punctuation block, and ASCII punctuation.
GITHUB_PUNCT = re.compile(
    "[\\u2000-\\u206F\\u2E00-\\u2E7F\\\\'!\"#$%&()*+,./:;<=>?@\\[\\]^`{|}~]"
)

def slugify_heading(line):
    """Turn a markdown heading into the anchor form GitHub generates.

    GitHub lowercases, trims, deletes a fixed set of punctuation, then replaces *each*
    remaining space with a hyphen. Replacing each space matters: a heading with an em dash
    loses the dash but keeps both neighbouring spaces, so it produces two hyphens in a row.
    Collapsing runs of whitespace into one hyphen -- what this function used to do -- made
    every such anchor look missing.
    """
    text = line.strip().lstrip("#").strip().lower()
    text = GITHUB_PUNCT.sub("", text)
    return text.replace(" ", "-")




def anchors_in(path):
    found = set()
    seen = {}
    for line in open(path, encoding="utf-8"):
        if line.lstrip().startswith("#"):
            slug = slugify_heading(line)
            count = seen.get(slug, 0)
            seen[slug] = count + 1
            # GitHub disambiguates repeats by appending -1, -2, ...
            found.add(slug if count == 0 else "%s-%d" % (slug, count))
    return found




def check_link_syntax(path, text):
    """Catch a bracket that names a file but is never closed into a link.

    `[label](target.md)` is checked by check_links. `[label../target.md)` is not a link at
    all -- it renders as ordinary text with a stray parenthesis -- so check_links cannot see
    it, and a file full of them passes. That is exactly what a careless find-and-replace
    produces, which is why this rule exists.
    """
    for number, line in enumerate(mask_code(text).splitlines(), 1):
        if MALFORMED_LINK.search(line):
            errors.append(
                "%s:%d: malformed link -- '[' is never closed into '](' so it renders as "
                "text" % (rel(path), number)
            )


def check_tables(path, text):
    """Every row of a table must have the same cell count as its header.

    The previous version compared the first data row against the separator line and
    skipped every row after it, so a table could lose a cell anywhere except the first
    row and the gate stayed quiet. It now tracks the header for the whole block.
    """
    lines = text.splitlines()

    def cells(line):
        # A backslash-escaped pipe is a literal character inside a cell, not a separator.
        # Counting it as a separator reported correct rows -- shell pipelines and regular
        # expressions are common in tables -- as malformed.
        unescaped = re.sub(r"\\\|", "\x00", line)
        return unescaped.strip().strip("|").count("|") + 1

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

    # index.json repeats every digest and byte count. It is a second copy, so it goes
    # stale independently of CHECKSUMS.txt - and it did: the contract moved to 1.0.1 and
    # the index still described the previous action-record.v1.json, while this gate
    # skipped the file and reported success. A reader following the index concludes the
    # published schema is corrupt.
    check_schema_index(sdir, schemas)


def check_schema_index(sdir, schemas):
    path = os.path.join(sdir, "index.json")
    if not os.path.exists(path):
        if schemas:
            errors.append("schemas/index.json is missing")
        return
    try:
        index = json.load(open(path, encoding="utf-8"))
    except json.JSONDecodeError as e:
        errors.append("schemas/index.json: invalid JSON (%s)" % e)
        return

    entries = index.get("schemas")
    if not isinstance(entries, list) or not entries:
        errors.append("schemas/index.json lists no schemas")
        return

    listed = set()
    for entry in entries:
        name = entry.get("file")
        if not name:
            errors.append("schemas/index.json has an entry with no file name")
            continue
        listed.add(name)
        target = os.path.join(sdir, name)
        if not os.path.exists(target):
            errors.append("schemas/index.json references missing file %s" % name)
            continue
        data = open(target, "rb").read()
        actual = hashlib.sha256(data).hexdigest()
        if entry.get("sha256") != actual:
            errors.append("schemas/index.json: %s digest is %s but the file is %s"
                          % (name, str(entry.get("sha256"))[:12], actual[:12]))
        if entry.get("bytes") != len(data):
            errors.append("schemas/index.json: %s says %s bytes but the file is %d"
                          % (name, entry.get("bytes"), len(data)))

    for f in schemas:
        if f == "index.json":
            continue
        if f not in listed:
            errors.append("schemas/index.json does not list %s" % f)




def check_version_matches_changelog():
    """VERSION and the newest released CHANGELOG heading must name the same version.

    A number that disagrees with itself costs nothing to prevent and is expensive to
    discover, because every reader assumes the other one is right.
    """
    version_path = os.path.join(ROOT, "VERSION")
    changelog_path = os.path.join(ROOT, "CHANGELOG.md")
    if not os.path.exists(version_path) or not os.path.exists(changelog_path):
        return
    version = open(version_path, encoding="utf-8").read().strip()
    newest = None
    for line in open(changelog_path, encoding="utf-8"):
        m = re.match(r"^##\s*\[([^\]]+)\]", line)
        if not m:
            continue
        if m.group(1).lower().startswith("unreleased"):
            continue
        newest = m.group(1)
        break
    if newest is None:
        errors.append("CHANGELOG.md: no released version heading found")
    elif newest != version:
        errors.append(
            "CHANGELOG.md: newest released version is %s but VERSION says %s"
            % (newest, version)
        )


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
            check_link_syntax(path, text)
            check_tables(path, text)
    check_index_coverage(md_files)
    check_version_matches_changelog()
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
