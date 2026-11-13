import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, describe, it } from "node:test";
import { fileURLToPath } from "node:url";

import {
  CAPTURE_VERSION,
  CaptureError,
  DEFAULT_FILENAME,
  MAX_RENDERED_BYTES,
  SDK_VERSION,
  capture,
  captureMessages,
} from "../src/index.js";

const scratch = mkdtempSync(join(tmpdir(), "context-ci-ts-"));
after(() => rmSync(scratch, { recursive: true, force: true }));

let counter = 0;
function scratchPath(name = "capture.json"): string {
  counter += 1;
  return join(scratch, `${counter}-${name}`);
}

function readCapture(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, "utf8")) as Record<string, unknown>;
}

function sha256(text: string): string {
  return createHash("sha256").update(text, "utf8").digest("hex");
}

describe("capture", () => {
  it("writes exactly the fields the CLI reads, and nothing else", () => {
    const path = scratchPath();
    assert.equal(capture("You are a careful assistant.", { path, template: "You are {{mood}}." }), path);

    const document = readCapture(path);
    assert.deepEqual(Object.keys(document).sort(), [
      "capture_version",
      "rendered",
      "sdk",
      "sdk_version",
      "template",
    ]);
    assert.equal(document.capture_version, CAPTURE_VERSION);
    assert.equal(document.sdk, "context-ci-typescript");
    assert.equal(document.sdk_version, SDK_VERSION);
    assert.equal(document.rendered, "You are a careful assistant.");
    assert.equal(document.template, "You are {{mood}}.");
  });

  it("omits the template rather than writing an empty one", () => {
    const path = scratchPath();
    capture("text", { path, template: "" });
    assert.equal("template" in readCapture(path), false);
  });

  it("defaults the file name", () => {
    const previous = process.cwd();
    process.chdir(scratch);
    try {
      assert.equal(capture("text"), DEFAULT_FILENAME);
      assert.equal(existsSync(join(scratch, DEFAULT_FILENAME)), true);
    } finally {
      process.chdir(previous);
    }
  });

  it("refuses a capture with no rendered prompt", () => {
    assert.throws(
      () => capture("", { path: scratchPath() }),
      (error: unknown) =>
        error instanceof CaptureError && /proves nothing/.test((error as Error).message),
    );
  });

  it("measures the limit in bytes, not characters", () => {
    const characters = MAX_RENDERED_BYTES / 2 + 1;
    assert.throws(
      () => capture("é".repeat(characters), { path: scratchPath() }),
      (error: unknown) =>
        error instanceof CaptureError && /over the \d+ limit/.test((error as Error).message),
    );
  });

  it("accepts a prompt exactly at the limit", () => {
    const path = scratchPath();
    capture("a".repeat(MAX_RENDERED_BYTES), { path });
    assert.equal(readCapture(path).rendered, "a".repeat(MAX_RENDERED_BYTES));
  });

  it("keeps the previous capture intact until the new one is complete", () => {
    const path = scratchPath();
    capture("first", { path });
    capture("second", { path });
    assert.equal(readCapture(path).rendered, "second");
    assert.equal(existsSync(`${path}.${process.pid}.tmp`), false);
  });

  it("creates missing directories", () => {
    const path = join(scratch, "deep", "deeper", "capture.json");
    capture("text", { path });
    assert.equal(readCapture(path).rendered, "text");
  });

  it("refuses a blank sdk name", () => {
    assert.throws(
      () => capture("text", { path: scratchPath(), sdk: "  " }),
      CaptureError,
    );
  });
});

describe("captureMessages", () => {
  it("joins a message list the way a provider would", () => {
    const path = scratchPath();
    captureMessages(
      [
        { role: "system", content: "Be brief." },
        { role: "user", content: "Why is the sky blue?" },
      ],
      { path },
    );
    assert.equal(readCapture(path).rendered, "system: Be brief.\nuser: Why is the sky blue?");
  });

  it("keeps a message with no role as its own text", () => {
    const path = scratchPath();
    captureMessages([{ content: "just text" }, "and a string"], { path });
    assert.equal(readCapture(path).rendered, "just text\nand a string");
  });

  it("refuses an empty list", () => {
    assert.throws(() => captureMessages([]), CaptureError);
  });
});

describe("version", () => {
  it("matches package.json", () => {
    // Compiled tests live in dist/test, so package.json is two levels up.
    const manifest = JSON.parse(
      readFileSync(fileURLToPath(new URL("../../package.json", import.meta.url)), "utf8"),
    ) as { version?: string };
    assert.equal(SDK_VERSION, manifest.version);
  });
});

// The point of the SDK is that its output is usable, and only the CLI can say whether it is.
// Set CONTEXT_CI_BIN to a built context-ci binary to run this.
const binary = process.env["CONTEXT_CI_BIN"];
describe("the CLI", { skip: binary === undefined ? "set CONTEXT_CI_BIN to run" : false }, () => {
  it("turns the capture into an instrumented manifest", () => {
    const root = mkdtempSync(join(tmpdir(), "context-ci-ts-repo-"));
    try {
      const template = "Answer the question using the notes.\n\nQuestion: {{question}}\n";
      writeFileSync(
        join(root, ".context-ci.yaml"),
        [
          "version: 1",
          "capture:",
          "  path: asserted",
          "  sources:",
          "    prompt: prompts/answer.md",
          "model:",
          "  provider: anthropic",
          "  id: claude-sonnet-4-5",
          "  params:",
          "    temperature: 0.0",
          "retrieval:",
          "  index_id: kb-1",
          "  embedding_model: text-embedding-3-large",
          "  top_k: 8",
          "",
        ].join("\n"),
        "utf8",
      );
      mkdirSync(join(root, "prompts"), { recursive: true });
      writeFileSync(join(root, "prompts", "answer.md"), template, "utf8");

      const captured = capture("Answer the question using the notes.\n\nQuestion: why now?\n", {
        path: join(root, "context-capture.json"),
        template,
      });

      const run = spawnSync(
        binary as string,
        ["manifest", "--root", root, "--commit", "0000000000000000000000000000000000000000", "--capture", captured],
        { encoding: "utf8" },
      );
      assert.equal(run.status, 0, run.stderr);
      const manifest = JSON.parse(run.stdout) as {
        capture?: { path?: string; sdk_version?: string };
        prompt?: { rendered_hash?: string; rendered_sample_hash?: string | null; template_hash?: string };
      };
      assert.equal(manifest.capture?.path, "instrumented");
      assert.equal(manifest.capture?.sdk_version, SDK_VERSION);
      assert.equal(manifest.prompt?.rendered_sample_hash, null);
      // The hash is of the exact text the application sent — not of the template file, and not of
      // a re-render the CLI did itself.
      const rendered = "Answer the question using the notes.\n\nQuestion: why now?\n";
      assert.equal(manifest.prompt?.rendered_hash, sha256(rendered));
      assert.equal(manifest.prompt?.template_hash, sha256(template));
      // The capture file itself must not appear in the manifest: only its hash does.
      assert.equal(JSON.stringify(manifest).includes("why now?"), false);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });
});
