# Vendored contracts — acp-core@1.0.0

**Upstream repository:** `acp-core`
**Contract set version:** 1.0.0
**Vendored on:** 29 October 2026
**Vendored by:** repository bootstrap, at repository creation

## Why these files exist here

Cost events so the platform can meter and attribute this tool's own usage; the module manifest so the platform can discover it. Both optional: this product is fully functional with no platform present.

## Files and digests

| File | SHA-256 |
|---|---|
| `cost-event.v1.json` | `b387a92fd9f8653e7fa53a19c47d9d93d21430dfe83039492d900f97aed07856` |
| `module-manifest.v1.json` | `0c655e7ae39971228d1c22dd5cc0c19a293a077773136a45e76becc80cc0f444` |

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
| 29 Oct 2026 | — | 1.0.0 | Initial vendoring at repository creation |
