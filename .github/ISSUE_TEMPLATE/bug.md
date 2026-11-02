---
name: Defect
about: Something behaves differently from what the documents say
title: ""
labels: bug
---

## What is wrong

<!-- One sentence. If you need two, you may be describing two defects. -->

## Where

- File, package or document:
- Commit:

## Evidence

<!-- The command you ran, its output, and what you expected instead. Paste the failure. -->

```
$ 
```

## Which claim does this break

<!--
Most defects in this repository are a document claiming something the code does not do, or a
check that cannot fail. Name the document or the test, so the fix can be verified rather than
guessed at:
-->

- [ ] A document claims this works
- [ ] A test should have caught this and did not
- [ ] A gate should have caught this and did not
- [ ] No claim was made; this is new work that is broken

## Environment

- OS:
- Go / Python version:
- Anything unusual (no network, air-gapped, non-UTC timezone):
