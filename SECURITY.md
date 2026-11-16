# Security policy — Context CI

## Reporting

Report a suspected vulnerability privately, through GitHub's private vulnerability reporting form: <https://github.com/evidary/context-ci/security/advisories/new>. Do not open a public issue.

Include: what you tested, what you expected, what happened, and whether customer data or credentials could be exposed.

## What counts here

| Category | Example |
|---|---|
| Credential exposure | A secret written to a log, an artefact, a cache or a manifest |
| Outbound data | Any network call that this product's documentation says it never makes |
| Injection | A crafted input that causes code execution or path traversal in the CLI or action |
| Supply chain | A release binary that does not match its `SHA256SUMS` entry, an unpinned dependency |
| Licence contamination | A dependency whose licence is incompatible with Apache-2.0 |
| Contract integrity | A vendored contract file that can be modified without CI failing |

## What does not count

- A missing security header on a documentation site.
- Prompt injection against a model this product does not host or call.
- Anything requiring write access to the maintainers' infrastructure.

## Response

These are targets for a small team, not a guarantee.

| Severity | Acknowledge | Fix target |
|---|---|---|
| Credential exposure or code execution | 24 hours | 7 days |
| Outbound data or injection | 24 hours | 14 days |
| Supply chain or licence | 3 days | 30 days |
| Hardening | 5 days | Best effort |

## Handling

- No secret in git. There is no automated secret scan in CI yet, so this rests on review.
- This product holds no model keys, no customer data and no credentials by design. If you find that it does, that is a critical bug, not a configuration issue.
- Release binaries ship with a `SHA256SUMS` file. They are not signed and there is no SBOM yet.
