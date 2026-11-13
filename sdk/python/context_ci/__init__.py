"""Context CI capture SDK.

The one thing a configuration file cannot know is the text actually sent to the model, after
retrieval and templating. This SDK records it, at the point where the request is assembled, into a
capture file that `context-ci manifest --capture` turns into an instrumented manifest.

    from context_ci import capture

    capture(rendered=prompt, template=TEMPLATE, path="context-capture.json")

No network, no account, no daemon: the file is the whole interface.
"""

from __future__ import annotations

import json
import os

__version__ = "0.1.0"

CAPTURE_VERSION = 1
# Matches the CLI's own bound. A rendered prompt is a page or two; a capture far larger than that
# means somebody passed the wrong variable, and the CLI refuses it anyway.
MAX_RENDERED_BYTES = 4 << 20

DEFAULT_FILENAME = "context-capture.json"


class CaptureError(ValueError):
    """A capture that would not prove anything."""


def capture(rendered, path=DEFAULT_FILENAME, template=None, sdk="context-ci-python"):
    """Write a capture file for one assembled request, and return its path.

    rendered  the exact text sent to the model. Required: a capture with no rendered prompt is
              refused rather than written.
    template  the unrendered template, when the caller has it. Optional, because a rendered
              prompt is enough to hash and a wrong template is worse than none.
    """
    if not isinstance(rendered, str) or rendered == "":
        raise CaptureError("rendered must be a non-empty string: a capture with no rendered prompt proves nothing")
    size = len(rendered.encode("utf-8"))
    if size > MAX_RENDERED_BYTES:
        raise CaptureError(
            "rendered prompt is %d bytes, over the %d limit" % (size, MAX_RENDERED_BYTES)
        )
    if template is not None and not isinstance(template, str):
        raise CaptureError("template must be a string or None")

    document = {
        "capture_version": CAPTURE_VERSION,
        "sdk": sdk,
        "sdk_version": __version__,
        "rendered": rendered,
    }
    if template:
        document["template"] = template

    directory = os.path.dirname(os.path.abspath(path))
    if directory:
        os.makedirs(directory, exist_ok=True)
    # Written to a temporary file and moved, so a reader never sees half a capture.
    temporary = path + ".tmp"
    with open(temporary, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(document, handle, ensure_ascii=False, indent=2)
        handle.write("\n")
    os.replace(temporary, path)
    return path


__all__ = ["capture", "capture_messages", "CaptureError", "CAPTURE_VERSION", "DEFAULT_FILENAME", "__version__"]


def capture_messages(messages, path=DEFAULT_FILENAME, **kwargs):
    """Capture a chat-style message list by joining it the way a provider would.

    Provided because most callers hold messages rather than a single string, and doing the join
    themselves is where they would drift from each other.
    """
    if not messages:
        raise CaptureError("messages is empty")
    parts = []
    for message in messages:
        if isinstance(message, dict):
            role = message.get("role", "")
            content = message.get("content", "")
            parts.append("%s: %s" % (role, content) if role else str(content))
        else:
            parts.append(str(message))
    return capture("\n".join(parts), path=path, **kwargs)