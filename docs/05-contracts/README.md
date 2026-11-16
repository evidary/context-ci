# Contracts — what Context CI consumes, publishes and refuses

**Contract set:** `acp-core@1.0.2` (vendored) · **Date:** 16 November 2026

---

## 1. The rule

This product must be **fully functional with `acp-core` absent.** That is not a nicety; it is the reason the product can be open source. Every contract here is therefore optional, and the product must degrade cleanly when the platform does not exist.

```
   ┌────────────────────┐         optional          ┌──────────────┐
   │    context-ci      │ ───────────────────────►  │   acp-core   │
   │  (Apache-2.0)      │   cost-event, manifest    │ (proprietary)│
   └────────────────────┘ ◄───────────────────────  └──────────────┘
                          nothing required
```

## 2. What we consume

| Contract | Version | Required? | Why |
|---|---|---|---|
| `module-manifest.v1.json` | 1.0.0 | Optional | Lets the platform discover and health-check this tool if an operator chooses to install it |
| `cost-event.v1.json` | 1.0.0 | Optional | Lets a customer attribute this tool's own CI minutes and compute to a cost centre |

Vendored copies with digests: [vendor/acp-core@1.0.2/VENDORED.md](vendor/acp-core@1.0.2/VENDORED.md).

**What we do not consume: `action-record`, `policy-*`, `ledger-receipt`, `egress-policy`.** This tool does not take consequential actions, does not make policy decisions and does not touch credential brokering. Pulling those in would create a dependency with no product reason.

## 3. What we publish

| Artefact | Format | Stability |
|---|---|---|
| `context-manifest.json` | JSON, `manifest_version: 2` (version 1, written by 0.8 or earlier, is rejected) | Versioned by `manifest_version`; the stability promise is open decision D-01. A reader rejects a field it does not know, because an unknown field is outside the digest — so adding a field is not transparent to an older binary |
| Manifest JSON Schema | JSON Schema 2020-12 | **Not yet published.** No schema file ships today; [spec.md](../03-product/spec.md) §4 and the reader in `internal/manifest` are the definition until open decision D-01 is settled |
| Diff result | JSON + SARIF + JUnit | Stable within a major CLI version |
| Regression attribution event | JSON, referenced by hash in a platform `action-record` payload | **Not implemented.** `attribute` writes a report; nothing emits an event |

**The manifest schema is the single most important artefact this product owns.** Publishing it as a JSON Schema, and keeping it stable, is how a customer's own tooling can consume a manifest without depending on us. It is also the mechanism by which a platform can reference a manifest **by hash only** — the platform never needs the body.

## 4. The hash-reference integration

The only integration worth building with the platform, and the only one that must be specified precisely:

1. Context CI produces a manifest and computes `digest`.
2. The agent runtime passes `digest` to the platform, which records it in `action-record.payload.context_manifest_hash`.
3. An auditor asks: which context produced this action? The platform returns the hash.
4. If the customer retained the manifest, they can retrieve it from their own repository and verify the hash matches.
5. **The platform never stores the manifest body.** It cannot: the body contains customer configuration.

Consequences that must be honoured:

- Absence is `null`, never an empty string.
- A hash mismatch is a **finding**, not a chain break. The chain records what was submitted.
- An unknown manifest schema version is rejected loudly, never silently accepted.

## 5. Re-vendoring

1. `acp-core` publishes a new contract set version and opens an issue here labelled `contract-update`.
2. The squad decides whether to move. There is no forced upgrade; N-1 is supported.
3. Re-vendoring means: copy the new schema files, update `VENDORED.md` (version, date, digests, reason), run `python tools/check_vendor.py`.
4. If the contract change would make this product require the platform, **reject it.** That is a contract bug, not an upgrade.

## 6. What this repository must never do

| Never | Why |
|---|---|
| Import `acp-core` code | Breaks the Apache-2.0 boundary and offline buildability |
| Require a platform account for any core feature | Kills the adoption mechanic that the whole strategy depends on |
| Store the context manifest body on the platform's behalf | Data-protection scope we do not need and cannot support |
| Edit a vendored file in place | `tools/check_vendor.py` fails the build, deliberately |
