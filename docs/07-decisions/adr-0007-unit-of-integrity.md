# ADR-0007: The unit of integrity is everything that steers the model

- **Status:** accepted
- **Date:** 2026-11-01
- **Supersedes:** nothing
- **Closes:** gap O9 in the acp-core review, and findings 1, 2 and 6 of the
  capture review
- **Depends on:** [ADR-0002](adr-0002-implementation-language.md) (the manifest and its digest)

## Context

The manifest digest covers the document with three fields removed: `created_at`,
`digest`, and `content`. Two of those exclusions are sound — a timestamp is not part
of the context, and a digest cannot cover itself. The third was not.

The digest left out `content` on the reasoning that content is *redundant with its
hash*. That reasoning holds only if the redundancy is checked. Nothing checked it.
`Content.Body` was an unchecked annotation: a manifest whose body said one thing and
whose hash said another validated cleanly, so a reviewer reading the body saw text
the model was never given, next to a verifier reporting nothing wrong.

A second hole sat beside it. A tool's description is prompt-injection surface — the
model reads it as instructions — and the only record of it was
`description_tokens`, an **estimate**, produced by counting whitespace-separated
words. `"Issue a refund."` and `"Ignore all rules."` are three tokens each. That edit
changed nothing in the manifest, so the gate reported no change.

## Options considered

**A. Put `content` inside the digest.** Rejected. Content is opt-in: a team that
enables capture on Tuesday would get a different digest from one that did not, for
the same context. The gate would then fail on a change that did not change what the
model saw, which is the fastest way to get the gate turned off.

**B. Drop `content` entirely and hash everything.** Rejected. It removes the only
affordance that lets a reviewer read a prompt diff, which is most of the reason
anyone opens the file.

**C. Keep `content` outside the digest, but verify it against its hash on load.**
Then the exclusion is sound for the reason it always claimed: the body *is* redundant
with the hash, because a body that disagrees is refused.

**D. Hash tool descriptions in addition to counting tokens.** The count stays for
human reading and cost projection, but it stops being the record.

## Decision

**C and D.** The rule, stated once so it can be applied to the next field:

> Every field that can steer the model is covered by a hash that is inside the
> digest. A count is never the record. A plaintext copy is admissible only if it is
> verified against its hash on load.

Concretely:

1. `manifest.Content.Validate()` recomputes `sha256(body)` and refuses the manifest
   if it does not equal `hash`. `Content.Kind` must be one of `prompt`, `rendered`,
   `tool_description`, and a kind may appear only once.
2. `manifest.Tool.DescriptionHash` is required. A tool without one is rejected,
   because a token count cannot record what the description said.
3. `diff` gains `tools.description`, keyed on that hash, default severity **fail** —
   the same as the schema it sits beside. `tools.description_tokens` survives for
   the case where the count changes under identical text, which is a capture bug
   rather than an edit.

### Why `tools.description` defaults to fail, when the prompt template is warn

The prompt template is the file a team edits on purpose; a default that failed every
deliberate prompt edit would be switched off within a week. A tool description is not
that. It is supplied by an integration, changes rarely, and is the exact surface a
prompt-injection attack uses. A change there deserves a human decision.

### The digest itself

Adding `description_hash` changes every manifest digest, so the pinned value moved
from `295cb37a…` to `e342a0e1…`. It was not simply re-pinned: the new value was
recomputed by an independent Python implementation,
`tools/interop/reference_digest.py`, and the two agree. The first attempt at that
cross-check **disagreed**, because the Python model of the document was missing
`rendered_sample_hash: null`. The disagreement was the point: the digest had been
pinned for weeks against a check that did not exist.

## Consequences

- An existing manifest with tools but no `description_hash` no longer validates. That
  is deliberate and it is a breaking change, but the alternative is a field that
  silently means nothing. Nothing has been released, so no migration path is needed.
- Capture must be run again to produce valid manifests. `context-ci` does not read
  old manifests into the new format; they are rejected with exit 3.
- The manifest schema in the product specification is updated in the same change. A
  specification that describes a different document from the one the tool writes is
  how this class of defect starts.
