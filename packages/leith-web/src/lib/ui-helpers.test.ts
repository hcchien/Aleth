import test from "node:test";
import assert from "node:assert/strict";

import { firstLine, formatAuthor, trustRequirementLabel, truncate } from "./ui-helpers.ts";

test("firstLine returns the first non-empty line", () => {
  assert.equal(firstLine("Hello world\nSecond line"), "Hello world");
});

test("firstLine falls back to Untitled when body starts empty", () => {
  assert.equal(firstLine("\n"), "Untitled");
});

test("truncate leaves short text unchanged", () => {
  assert.equal(truncate("short", 10), "short");
});

test("truncate shortens long text and adds ellipsis", () => {
  assert.equal(truncate("This is a very long sentence", 7), "This is...");
});

test("formatAuthor maps oauth identities to visitor label", () => {
  assert.equal(formatAuthor("oauth:google:token"), "訪客");
});

test("formatAuthor shortens did identities", () => {
  assert.equal(formatAuthor("did:vflow:abcdefghijklmnop"), "did:...klmnop");
});

test("trustRequirementLabel formats tier guidance", () => {
  assert.equal(trustRequirementLabel(2), "需要 L2 以上信任等級");
});
