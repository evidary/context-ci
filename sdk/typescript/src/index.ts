/**
 * Context CI capture SDK.
 *
 * The one thing a configuration file cannot know is the text actually sent to the model, after
 * retrieval and templating. This SDK records it, at the point where the request is assembled, into
 * a capture file that `context-ci manifest --capture` turns into an instrumented manifest.
 *
 *     import { capture } from "@evidary/context-ci";
 *
 *     capture(prompt, { template: TEMPLATE, path: "context-capture.json" });
 *
 * No network, no account, no daemon: the file is the whole interface.
 */

import { mkdirSync, renameSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

/** Version of this SDK, recorded in every capture. Kept in step with package.json by a test. */
export const SDK_VERSION = "0.1.0";

/** The capture format this SDK writes. The CLI refuses any other. */
export const CAPTURE_VERSION = 1;

/**
 * Matches the CLI's own bound. A rendered prompt is a page or two; a capture far larger than that
 * means somebody passed the wrong variable, and the CLI refuses it anyway.
 */
export const MAX_RENDERED_BYTES = 4 << 20;

export const DEFAULT_FILENAME = "context-capture.json";

const DEFAULT_SDK = "context-ci-typescript";

/** A capture that would not prove anything. */
export class CaptureError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "CaptureError";
  }
}

export interface CaptureOptions {
  /** Where to write the capture. Defaults to `context-capture.json` in the working directory. */
  readonly path?: string;
  /** The unrendered template, when the caller has it. */
  readonly template?: string;
  /** Recorded SDK name. Only set this when wrapping this SDK, so provenance stays true. */
  readonly sdk?: string;
}

export interface ChatMessage {
  readonly role?: string;
  readonly content?: string;
}

export type MessageLike = ChatMessage | string;

/**
 * Write a capture file for one assembled request and return its path.
 *
 * `rendered` is the exact text sent to the model, and is required: a capture with no rendered
 * prompt is refused rather than written. `template` is optional, because a rendered prompt is
 * enough to hash and a wrong template is worse than none.
 */
export function capture(rendered: string, options: CaptureOptions = {}): string {
  if (typeof rendered !== "string" || rendered === "") {
    throw new CaptureError(
      "rendered must be a non-empty string: a capture with no rendered prompt proves nothing",
    );
  }
  // Bytes, not characters: the CLI's bound and its prompt hash both count bytes, and a 4 MiB string
  // of non-ASCII text is not 4 MiB of characters.
  const size = new TextEncoder().encode(rendered).length;
  if (size > MAX_RENDERED_BYTES) {
    throw new CaptureError(
      `rendered prompt is ${size} bytes, over the ${MAX_RENDERED_BYTES} limit`,
    );
  }

  const template = options.template;
  if (template !== undefined && typeof template !== "string") {
    throw new CaptureError("template must be a string");
  }
  const sdk = options.sdk ?? DEFAULT_SDK;
  if (typeof sdk !== "string" || sdk.trim() === "") {
    throw new CaptureError("sdk must be a non-empty string");
  }

  const document: Record<string, unknown> = {
    capture_version: CAPTURE_VERSION,
    sdk,
    sdk_version: SDK_VERSION,
    rendered,
  };
  if (template !== undefined && template !== "") {
    document.template = template;
  }

  const path = options.path ?? DEFAULT_FILENAME;
  const target = resolve(path);
  mkdirSync(dirname(target), { recursive: true });
  // Written to a temporary file and moved, so a reader never sees half a capture.
  const temporary = `${target}.${process.pid}.tmp`;
  writeFileSync(temporary, `${JSON.stringify(document, null, 2)}\n`, { encoding: "utf8" });
  renameSync(temporary, target);
  return path;
}

/**
 * Capture a chat-style message list by joining it the way a provider would.
 *
 * Provided because most callers hold messages rather than a single string, and doing the join
 * themselves is where they would drift from each other.
 */
export function captureMessages(
  messages: readonly MessageLike[],
  options: CaptureOptions = {},
): string {
  if (!Array.isArray(messages) || messages.length === 0) {
    throw new CaptureError("messages is empty");
  }
  const parts = messages.map((message) => {
    if (typeof message === "string") {
      return message;
    }
    const content = message.content ?? "";
    const role = message.role ?? "";
    return role === "" ? content : `${role}: ${content}`;
  });
  return capture(parts.join("\n"), options);
}
