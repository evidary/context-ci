# Security policy — Context CI

## Reporting

Report a suspected vulnerability privately to the maintainers. Do not open a public issue. If this repository has a private security advisory channel enabled, use it.

Include: what you tested, what you expected, what happened, and whether customer data or credentials could be exposed.

## What counts here

| Category | Example |
|---|---|
| Credential exposure | A secret written to a log, an artefact, a cache or a manifest |
| Outbound data | Any network call that this product's documentation says it never makes |
| Injection | A crafted input that causes code execution or path traversal in the CLI or action |
| Supply chain | An unsigned artefact, a missing SBOM, an unpinned dependency |
| Licence contamination | A dependency whose licence is incompatible with Apache-2.0 |
| Contract integrity | A vendored contract file that can be modified without CI failing |

## What does not count

- A missing security header on a documentation site.
- Prompt injection against a model this product does not host or call.
- Anything requiring write access to the maintainers' infrastructure.

## Response

| Severity | Acknowledge | Fix target |
|---|---|---|
| Credential exposure or code execution | 24 hours | 7 days |
| Outbound data or injection | 24 hours | 14 days |
| Supply chain or licence | 3 days | 30 days |
| Hardening | 5 days | Best effort |

## Handling

- No secret in git. CI fails on a detected secret.
- This product holds no model keys, no customer data and no credentials by design. If you find that it does, that is a critical bug, not a configuration issue.
- Released artefacts are signed and shipped with an SBOM.
