# context-ci-capture

The Python half of instrumented capture. It writes a capture file; `context-ci manifest --capture`
turns that into an instrumented manifest.

```python
from context_ci import capture

capture(rendered=prompt, template=TEMPLATE, path="context-capture.json")
```

```bash
pip install ./sdk/python          # or: pip install context-ci-capture, once published
context-ci manifest --capture context-capture.json
```

No dependencies, no network, no account. For a chat-style request, `capture_messages(messages)` takes the
list of messages instead of one string and records the text they render to.

## What it records, and what it does not

| Field | Why |
|---|---|
| `rendered` | The exact text sent to the model. The one thing a config file cannot know |
| `template` | Optional; the unrendered template, so the template hash is the template's |
| `sdk`, `sdk_version` | The manifest records which SDK produced it, because an instrumented capture and an asserted one are not the same evidence |

It does **not** record variable values. The rendered text already contains them, and a manifest is
a document teams commit and share; copying customer values into it would turn a hash into a leak.
Variable *types* still come from the template in the repository, as they do for asserted capture.

## The capture file contains the prompt

Treat it like the prompt itself: it belongs in `.gitignore`, and it should not be attached to a
ticket. Only the hash reaches the manifest.

## Tests

```bash
python -m unittest discover -s sdk/python/tests
```