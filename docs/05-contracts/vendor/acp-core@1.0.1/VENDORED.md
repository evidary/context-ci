# Vendored contracts - acp-core@1.0.1

**Upstream repository:** `acp-core`
**Contract set version:** 1.0.1
**Vendored on:** 1 November 2026 (re-vendored from 1.0.0; see the history below)
**Vendored by:** the version-alignment pass across all four module repositories

## Why these files exist here

Cost events so the platform can meter and attribute this tool's own usage; the module manifest
so the platform can discover it. Both optional: this product is fully functional with no
platform present.

## Files and digests

| File | SHA-256 |
|---|---|
| `cost-event.v1.json` | `e22d4ce07e85410b789b47290a3b6f311ef06d3f18b5c4ccadf696e81bfe5ed4` |
| `module-manifest.v1.json` | `b6f37e854967fb3e04bb7d0ad2348585bf110729361c6af78564ded234879cbb` |

## Nothing about this module changed in 1.0.1

Contract set 1.0.1 added `payload_hash` and `payload_length` to `action-record.v1.json`, as
required fields. That change matters to every module that records actions. This one does not
record actions: it consumes `cost-event.v1.json` and `module-manifest.v1.json`, and both files
are byte-identical between 1.0.0 and 1.0.1 — the digests above are the digests that were
recorded against 1.0.0.

The re-vendor is therefore bookkeeping, and this paragraph exists so that the next reader does
not have to diff two contract sets to discover that.

## Rules

1. **Never edit a vendored file in place.** If a contract must change, it changes in
   `acp-core` with a version bump, then this repository re-vendors at the new version.
2. `tools/check_vendor.py` recomputes these digests in CI and fails on any mismatch. It also
   checks that the directory name and the set version above agree.
3. The digests here are recorded in this repository, so a *stale* copy passes forever. That is
   why the set version is written at the top of this file: compare it against `acp-core`'s
   `CHECKSUMS.txt` when you re-plan, because nothing will remind you.

## Upgrade history

| Date | From | To | Reason |
|---|---|---|---|
| 30 Oct 2026 | - | 1.0.0 | Initial vendoring at repository creation |
| 1 Nov 2026 | 1.0.0 | 1.0.1 | Align with the current set. The two contracts consumed here are unchanged |
