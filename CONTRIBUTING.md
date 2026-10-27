# Contributing to Context CI

**Licence:** Apache-2.0. External contributions are welcome once v0.1 is public; until then the repository is pre-release and maintainers may restructure it freely.

---

## Before you start

- Read [docs/00-index.md](docs/00-index.md). It routes every question to one document. If your change contradicts a document, change the document in the same pull request or say why in the description.
- Read [docs/01-thesis.md](docs/01-thesis.md). It states what this product is and, more usefully, what it refuses to be.

## Running the checks

```
python tools/check_docs.py     # links, tables, encoding, index coverage
python tools/check_vendor.py   # vendored contract digests
```

Both must pass. CI runs exactly these two commands.

## What needs an ADR first

| Change | Needs an ADR |
|---|---|
| A published schema or output format | Yes — version bump and a migration note |
| Adding a dependency that changes the licence position | Yes |
| Changing the paid boundary | Yes |
| Anything expensive to reverse | Yes |

Write it as `docs/07-decisions/adr-NNNN-short-title.md` and update the register in the same pull request.

## Style

- **State facts with sources.** Prices, market sizes and third-party claims carry a URL or are labelled as estimates.
- **Numbers carry units, conditions and dates.** "20 seconds on a 2-core GitHub runner, measured 29 Oct 2026".
- **No hype.** No "seamless", no "revolutionary". If a sentence would embarrass you in a security review, cut it.
- **Prefer a table for comparisons, prose for reasoning.** Do not put paragraphs in table cells.
- **File names are lowercase and hyphenated.**

## Commit messages

```
<area>: <imperative summary>

Why this changed, and what would break without it.
```

## Review

One maintainer approval. Two for a schema change, the paid boundary, or the licence.

## Support

Questions and bugs: GitHub Discussions.
