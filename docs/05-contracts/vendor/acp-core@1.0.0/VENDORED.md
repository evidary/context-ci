# Vendored contracts - acp-core@1.0.0

**Upstream repository:** `acp-core`
**Contract set version:** 1.0.0
**Vendored on:** 30 October 2026
**Vendored by:** repository bootstrap, at repository creation

## Why these files exist here

Cost events so the platform can meter and attribute this tool's own usage; the module manifest so the platform can discover it. Both optional: this product is fully functional with no platform present.

## Files and digests

| File | SHA-256 |
|---|---|
| `cost-event.v1.json` | `e22d4ce07e85410b789b47290a3b6f311ef06d3f18b5c4ccadf696e81bfe5ed4` |
| `module-manifest.v1.json` | `b6f37e854967fb3e04bb7d0ad2348585bf110729361c6af78564ded234879cbb` |

## Rules

1. **Never edit a vendored file in place.** If a contract must change, it changes in
   `acp-core` with a version bump, then this repository re-vendors at the new version.
2. `tools/check_vendor.py` recomputes these digests in CI and fails on any mismatch.
3. Moving to a new contract set version is this repository's decision, not the
   platform's. N-1 is supported; there is no forced upgrade.
4. When re-vendoring, update this file: version, date, digests, and one line saying
   what changed and why this repository moved now.

## Upgrade history

| Date | From | To | Reason |
|---|---|---|---|
| 29 Oct 2026 | - | 1.0.0 | Initial vendoring at repository creation |
| 30 Oct 2026 | 1.0.0 | 1.0.0 (amended) | Upstream amended the set before first release: the action record nests the policy binding and carries alg and key_id, the receipt no longer references a batch, and `hashed-record` and `checkpoint` were published. No behaviour change in this repository yet, because there is no code. |
