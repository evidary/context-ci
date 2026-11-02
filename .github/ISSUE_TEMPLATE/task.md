---
name: Task
about: Work that is not a defect
title: ""
labels: ""
---

## What and why

<!-- One paragraph. Link the plan section it comes from. -->

**Plan reference:**
**Owner:**

## Definition of done

<!--
State it as something checkable. "Documentation updated" is not checkable; "check_docs.py passes
and the index links the new file" is.
-->

- [ ]
- [ ]

## Blast radius

- [ ] Contracts (a published schema changes → contract set version bump, modules re-vendor)
- [ ] Data (migration, retention, RLS)
- [ ] Deployment (Helm, air-gap)
- [ ] Documentation only

## If this is a contract change

- [ ] `docs/05-contracts/CHECKSUMS.txt` updated
- [ ] `index.json` cross-checked
- [ ] Every consuming module has an issue labelled `contract-update`
