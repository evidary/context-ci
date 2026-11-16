# Vendored contracts - acp-core@1.0.2

**Upstream repository:** `acp-core`
**Contract set version:** 1.0.2
**Vendored on:** 2026-11-14
**Vendored by:** the maintainers

## Why these files exist here

The module manifest so a platform can discover this tool: `context-ci module-manifest` emits one. The
cost-event contract is vendored for a planned use, metering the tool's own usage, and nothing emits a
cost event today. Both are optional: this product is fully functional with no platform present.

## Files and digests

| File | SHA-256 |
|---|---|
| `cost-event.v1.json` | `10306d1ef45d1c87d76b364a1f33b5be46ef46e617fb69792210af9d12369297` |
| `module-manifest.v1.json` | `b6f37e854967fb3e04bb7d0ad2348585bf110729361c6af78564ded234879cbb` |

## 1.0.2 matters to this module, and 1.0.1 did not

Contract set 1.0.1 added `payload_hash` and `payload_length` to `action-record.v1.json` as required
fields. That change matters to every module that records actions. This one does not record actions:
it vendors `cost-event.v1.json` and `module-manifest.v1.json`, and both were byte-identical between
1.0.0 and 1.0.1, so re-vendoring at 1.0.1 was bookkeeping.

**1.0.2 is not.** It adds optional `customer_id` and `workflow` to `cost-event.v1.json`, and this
module is meant to emit cost events so a platform can meter its usage (it does not yet; see below). Without those fields a customer
cannot attribute this tool's spend to a team or a workflow, which is the only reason the field exists.
The vendored copy is now the current one; before this re-vendor it was a 1.0.0-era copy recorded as
1.0.1, and `tools/check_vendor.py` passed the whole time — because that script compares the file
against the digest in this table, and both were stale together.

The upstream repository runs a check that compares this directory against its own schemas, so a copy
that drifts from the platform is reported there rather than recorded here.

Whether this module's own emitted `cost_event` objects now carry the two new fields is a separate
question, and the answer today is no: there is no cost-event emission in this release at all. The
contract is current; the code that would use it is not written and not scheduled.

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
| 14 Nov 2026 | 1.0.1 | 1.0.2 | `cost-event.v1.json` gains optional `customer_id` and `workflow`, which this module would need if it emitted cost events for its own usage (it does not yet): without them a customer could not attribute that spend to a team or a workflow. The copy recorded as 1.0.1 was in fact the 1.0.0 bytes, and `tools/check_vendor.py` passed because the file and the recorded digest were stale together. |
