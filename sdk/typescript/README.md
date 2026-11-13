# @evidary/context-ci

The TypeScript half of instrumented capture. It writes a capture file; `context-ci manifest
--capture` turns that into an instrumented manifest.

```ts
import { capture } from "@evidary/context-ci";

capture(prompt, { template: TEMPLATE, path: "context-capture.json" });
```

```bash
context-ci manifest --capture context-capture.json
```

No runtime dependencies, no network, no account. Node 18 or newer.

## What it records, and what it does not

| Field | Why |
|---|---|
| `rendered` | The exact text sent to the model. The one thing a config file cannot know |
| `template` | Optional; the unrendered template, so the template hash is the template's |
| `sdk`, `sdk_version` | The manifest records which SDK produced it, because an instrumented capture and an asserted one are not the same evidence |

It does **not** record variable values. The rendered text already contains them, and a manifest is
a document teams commit and share; copying customer values into it would turn a hash into a leak.
Variable *types* still come from the template in the repository, as they do for asserted capture.

The rendered prompt is measured in UTF-8 bytes against the same 4 MiB limit the CLI enforces, so a
capture the SDK writes is one the CLI will read. Anything larger is refused rather than silently
truncated.

## Messages rather than a string

Most applications hold a message list, not one string. `captureMessages` joins it the way a
provider would — `role: content`, one per line — so two callers do not each invent a different join.

```ts
import { captureMessages } from "@evidary/context-ci";

captureMessages(
  [
    { role: "system", content: system },
    { role: "user", content: question },
  ],
  { path: "context-capture.json" },
);
```

## The capture file contains the prompt

Treat it like the prompt itself: it belongs in `.gitignore`, and it should not be attached to a
ticket. Only the hash reaches the manifest.

## Tests

```bash
npm install
npm test
```

To also prove the CLI accepts what this SDK writes, point `CONTEXT_CI_BIN` at a built binary:

```bash
go build -o bin/context-ci ./cmd/context-ci
CONTEXT_CI_BIN=$PWD/bin/context-ci npm test
```
