# Pull request

## What changed

<!-- One paragraph. What is different after this merges? -->

## Why

<!-- The problem, the user, the evidence. -->

## Checklist

- [ ] `python tools/check_docs.py` passes
- [ ] `python tools/check_vendor.py` passes
- [ ] Any new document is listed in `docs/00-index.md`
- [ ] No vendored contract file was edited in place; upstream change goes through `ledger`
- [ ] Any number I added has a source, a date, or is labelled an estimate
- [ ] This product still works with `ledger` absent
- [ ] No secret, credential or customer data in the diff
